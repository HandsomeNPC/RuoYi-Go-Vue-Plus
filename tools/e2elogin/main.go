// Command e2elogin 按 apiEncrypt 协议加密登录报文，验证登录闭环。
//
// 用法: go run ./tools/e2elogin
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/encrypt"
)

const (
	baseURL = "http://127.0.0.1:8080" // auth 进程
	// systemURL system 进程。受保护接口要在它上面探测。
	systemURL = "http://127.0.0.1:8081"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	config.Load("configs/application.yaml", "configs/auth.yaml")
	enc := config.Get().Middleware.APIEncrypt

	// 前端加密请求用的是公钥；这里直接从 privateKey 推出公钥。
	priv, err := encrypt.ParseRSAPrivateKey(enc.PrivateKey)
	if err != nil {
		return fmt.Errorf("解析私钥失败: %w", err)
	}

	body := `{"clientId":"e5cd7e4891bf95d1d19206ce24a7b32e","grantType":"password",` +
		`"username":"admin","password":"admin123"}`

	// 协议（见 configs/application.yaml）：
	//   encrypt-key 头 = base64(RSA公钥加密( base64( AES明文密钥 ) ))
	//   请求体         = base64(AES-ECB加密( JSON 明文 ))
	aesKey := make([]byte, 16)
	if _, err := rand.Read(aesKey); err != nil {
		return err
	}
	aesKeyStr := fmt.Sprintf("%x", aesKey)[:16] // 16 字节的可打印密钥

	cipherBody, err := encrypt.EncryptByAES(body, aesKeyStr)
	if err != nil {
		return fmt.Errorf("AES 加密失败: %w", err)
	}
	encKey, err := encrypt.EncryptByRSA(encrypt.EncryptByBase64(aesKeyStr), &priv.PublicKey)
	if err != nil {
		return fmt.Errorf("RSA 加密失败: %w", err)
	}

	token, err := login(enc.HeaderFlag, encKey, cipherBody)
	if err != nil {
		return err
	}
	fmt.Printf("\n[OK] 登录成功, token 前 20 位: %s...\n\n", token[:20])

	return probeProtected(token)
}

// login 发加密登录请求并返回 access_token。
func login(headerFlag, encKey, cipherBody string) (string, error) {
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/auth/login",
		bytes.NewReader([]byte(cipherBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerFlag, encKey)
	req.Header.Set("clientid", "e5cd7e4891bf95d1d19206ce24a7b32e")

	status, raw, err := doRaw(req)
	if err != nil {
		return "", err
	}
	fmt.Printf("1. 加密登录 -> HTTP %d %s\n", status, raw)

	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
		Data struct {
			AccessToken string `json:"access_token"`
			ExpireIn    int64  `json:"expire_in"`
			ClientID    string `json:"client_id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", fmt.Errorf("解析响应失败: %w", err)
	}
	if r.Code != 200 {
		return "", fmt.Errorf("登录失败: code=%d msg=%s", r.Code, r.Msg)
	}
	return r.Data.AccessToken, nil
}

// probeProtected 用拿到的 token 走一遍鉴权中间件的各条分支。
func probeProtected(token string) error {
	const clientID = "e5cd7e4891bf95d1d19206ce24a7b32e"

	cases := []struct {
		name, url, path, token, clientID string
	}{
		{"2. auth 进程上的乱路径(未注册 -> 不鉴权, 应 404 而非 401)",
			baseURL, "/nonexistent", "", ""},
		{"3. system 进程上的乱路径(同上, 应 404)",
			systemURL, "/nonexistent", "", ""},
		{"4. 受保护接口不带 token(应 401)",
			systemURL, "/system/ping", "", ""},
		{"5. 带 token 但不带 clientid(应 401)",
			systemURL, "/system/ping", token, ""},
		{"6. 带 token 但 clientid 不匹配(应 401)",
			systemURL, "/system/ping", token, "428a8310cd442757ae699df5d894f051"},
		{"7. 带 token 且 clientid 匹配(应 200 —— 鉴权通过)",
			systemURL, "/system/ping", token, clientID},
	}

	for _, c := range cases {
		req, _ := http.NewRequest(http.MethodGet, c.url+c.path, nil)
		if c.token != "" {
			req.Header.Set("Authorization", "Bearer "+c.token)
		}
		if c.clientID != "" {
			req.Header.Set("clientid", c.clientID)
		}
		body, err := do(req)
		if err != nil {
			return err
		}
		fmt.Printf("%s\n   -> %s\n", c.name, body)
	}

	// 8. 登出后原 token 失效。
	req, _ := http.NewRequest(http.MethodPost, baseURL+"/auth/logout", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	body, err := do(req)
	if err != nil {
		return err
	}
	fmt.Printf("8. 登出 -> %s\n", body)

	req, _ = http.NewRequest(http.MethodGet, systemURL+"/system/ping", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("clientid", clientID)
	body, err = do(req)
	if err != nil {
		return err
	}
	fmt.Printf("9. 登出后再用原 token(应 401, 会话已删) -> %s\n", body)
	return nil
}

// do 发请求并返回「HTTP状态码 + 响应体」的单行摘要。
func do(req *http.Request) (string, error) {
	status, b, err := doRaw(req)
	if err != nil {
		return "", err
	}
	if len(b) == 0 {
		return fmt.Sprintf("HTTP %d (空响应体)", status), nil
	}
	return fmt.Sprintf("HTTP %d %s", status, b), nil
}

// doRaw 发请求并返回状态码与原始响应体。
func doRaw(req *http.Request) (int, []byte, error) {
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return 0, nil, err
	}
	return resp.StatusCode, b, nil
}

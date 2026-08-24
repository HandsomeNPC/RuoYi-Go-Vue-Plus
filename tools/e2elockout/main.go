// Command e2elockout 验证密码错误锁定，走真实 MySQL + Redis。
//
// 与 e2elogin 同为**临时联调工具**，不属于产品代码。
// 单独一个是因为它会把 admin 账号刷到锁定，跑完必须清 Redis 计数键，
// 混进 e2elogin 会让那边的用例互相干扰。
//
// 用法: go run ./tools/e2elockout
package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"

	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/encrypt"
)

const (
	baseURL      = "http://127.0.0.1:8080"
	seedClientID = "e5cd7e4891bf95d1d19206ce24a7b32e"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load("configs/application.yaml", "configs/auth.yaml")
	if err != nil {
		return err
	}

	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr(),
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	defer func() { _ = rdb.Close() }()

	ctx := context.Background()
	key := constant.PwdErrCntKeyPrefix + "admin"

	// 先清干净，否则上一次运行残留的计数会让次数对不上。
	if err := rdb.Del(ctx, key).Err(); err != nil {
		return fmt.Errorf("清理计数键失败: %w", err)
	}
	// 跑完也清掉，别把 admin 真的锁在那里 10 分钟。
	defer func() {
		if err := rdb.Del(ctx, key).Err(); err != nil {
			log.Printf("清理计数键失败: %v", err)
		} else {
			fmt.Printf("\n[清理] 已删除 %s\n", key)
		}
	}()

	maxRetry := cfg.User.Password.MaxRetryCount
	fmt.Printf("配置: maxRetryCount=%d lockTime=%d 分钟\n\n",
		maxRetry, cfg.User.Password.LockTime)

	enc := cfg.Middleware.APIEncrypt
	priv, err := encrypt.ParseRSAPrivateKey(enc.PrivateKey)
	if err != nil {
		return err
	}

	// 连错 maxRetry 次。
	for i := 1; i <= maxRetry; i++ {
		msg, err := attempt(enc.HeaderFlag, &priv.PublicKey, "wrong-password")
		if err != nil {
			return err
		}
		ttl, _ := rdb.TTL(ctx, key).Result()
		fmt.Printf("第 %d 次错误密码 -> %s (计数键 TTL=%v)\n", i, msg, ttl.Round(1e9))
	}

	// 锁定后即使密码正确也应被拒 —— 这才是「锁定」的意义。
	msg, err := attempt(enc.HeaderFlag, &priv.PublicKey, "admin123")
	if err != nil {
		return err
	}
	fmt.Printf("\n锁定后用**正确**密码 -> %s\n", msg)
	return nil
}

// attempt 发一次加密登录，返回响应体里的提示文案。
func attempt(headerFlag string, pub *rsa.PublicKey, password string) (string, error) {
	body := fmt.Sprintf(
		`{"clientId":%q,"grantType":"password","username":"admin","password":%q}`,
		seedClientID, password)

	// 协议见 configs/application.yaml：
	//   encrypt-key 头 = base64(RSA公钥加密( base64( AES明文密钥 ) ))
	//   请求体         = base64(AES-ECB加密( JSON 明文 ))
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	aesKey := fmt.Sprintf("%x", raw)[:16]

	cipherBody, err := encrypt.EncryptByAES(body, aesKey)
	if err != nil {
		return "", err
	}
	encKey, err := encrypt.EncryptByRSA(encrypt.EncryptByBase64(aesKey), pub)
	if err != nil {
		return "", err
	}

	req, _ := http.NewRequest(http.MethodPost, baseURL+"/auth/login",
		bytes.NewReader([]byte(cipherBody)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set(headerFlag, encKey)
	req.Header.Set("clientid", seedClientID)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	b, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", err
	}

	var r struct {
		Code int    `json:"code"`
		Msg  string `json:"msg"`
	}
	if err := json.Unmarshal(b, &r); err != nil {
		return "", fmt.Errorf("解析响应失败: %w (body=%s)", err, b)
	}
	return fmt.Sprintf("code=%d msg=%s", r.Code, r.Msg), nil
}

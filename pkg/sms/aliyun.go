package sms

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/jsonx"
)

// 阿里云 SendSms 的固定参数，取自 sms4j AlibabaConfig 的默认值。
const (
	aliyunAction           = "SendSms"
	aliyunVersion          = "2017-05-25"
	aliyunSignatureMethod  = "HMAC-SHA1"
	aliyunSignatureVersion = "1.0"
	aliyunFormat           = "JSON"
	// aliyunSuccessCode 业务成功标志，响应 Code 非 OK 即失败。
	aliyunSuccessCode = "OK"
)

// httpTimeout 单次请求超时。不设的话短信网关抽风会把 handler 协程挂死。
const httpTimeout = 10 * time.Second

// aliyunSender 阿里云短信发送器。
//
// 手写 RPC 签名而非引官方 SDK：签名算法二十来行，而
// alibabacloud-go/dysmsapi 会连带拉进近十个 alibabacloud-go/* 包。
// 签名算法须与实际在用的实现逐字一致。
type aliyunSender struct {
	cfg    config.SMSConfig
	client *http.Client
}

// newAliyunSender 构造阿里云发送器。
func newAliyunSender(cfg config.SMSConfig) *aliyunSender {
	return &aliyunSender{
		cfg:    cfg,
		client: &http.Client{Timeout: httpTimeout},
	}
}

// aliyunResponse 阿里云的响应体，只取判定成功与报错所需的字段。
type aliyunResponse struct {
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	RequestID string `json:"RequestId"`
	BizID     string `json:"BizId"`
}

// Send 发送短信。
func (s *aliyunSender) Send(ctx context.Context, phone string, params map[string]string) error {
	templateParam, err := jsonx.Marshal(params)
	if err != nil {
		return fmt.Errorf("sms: 序列化模板参数失败: %w", err)
	}

	query := map[string]string{
		// 公共参数。
		"SignatureMethod":  aliyunSignatureMethod,
		"SignatureNonce":   uuid.NewString(),
		"SignatureVersion": aliyunSignatureVersion,
		"AccessKeyId":      s.cfg.AccessKeyID,
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"Format":           aliyunFormat,
		"Action":           aliyunAction,
		"Version":          aliyunVersion,
		"RegionId":         s.cfg.RegionID,
		// 业务参数。
		"PhoneNumbers":  phone,
		"SignName":      s.cfg.SignName,
		"TemplateCode":  s.cfg.TemplateCode,
		"TemplateParam": string(templateParam),
	}
	query["Signature"] = sign(query, s.cfg.AccessKeySecret)

	endpoint := "https://" + s.cfg.Endpoint + "/"
	form := make(url.Values, len(query))
	for k, v := range query {
		form.Set(k, v)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint,
		strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("sms: 构造请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("sms: 请求短信网关失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("sms: 读取响应失败: %w", err)
	}

	var out aliyunResponse
	if err := jsonx.Unmarshal(body, &out); err != nil {
		// 网关返回非 JSON（如网关层的 HTML 错误页）时，原样带上响应体便于排查。
		return fmt.Errorf("sms: 解析响应失败: %w, 响应: %s", err, truncate(string(body), 200))
	}
	if out.Code != aliyunSuccessCode {
		// 把厂商文案透出去：调用方要把它当作给前端的提示。
		return fmt.Errorf("sms: 短信发送失败: %s", firstNonEmpty(out.Message, out.Code))
	}
	return nil
}

// sign 计算 RPC 风格签名，返回 Base64 结果。
//
// 算法取自 sms4j AliyunUtils.generateSendSmsRequestUrl，四步不可调整：
//  1. 参数按键名字典序排列（见 canonicalizedQuery）
//  2. 逐对做 specialEncode 后拼成 k=v&k=v
//  3. 待签串 = "POST&" + encode("/") + "&" + encode(上一步结果)
//  4. HMAC-SHA1，密钥是 accessKeySecret + "&"
//
// 动词是 POST 而非 GET：阿里云文档正文举的是 GET 例子，两者只差待签串首段，
// 但签出的值完全不同，混用会一律 SignatureDoesNotMatch。
func sign(params map[string]string, accessKeySecret string) string {
	stringToSign := "POST&" + specialEncode("/") + "&" + specialEncode(canonicalizedQuery(params))

	// 密钥末尾那个 & 是阿里云的规定，漏掉会一律签名不通过。
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

// canonicalizedQuery 按键名字典序拼规范化查询串。
//
// 必须排序：入参是 map，而 Go 的 map 遍历顺序随机，
// 不排序会让同样的参数每次算出不同签名。
func canonicalizedQuery(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var sb strings.Builder
	for i, k := range keys {
		if i > 0 {
			sb.WriteByte('&')
		}
		sb.WriteString(specialEncode(k))
		sb.WriteByte('=')
		sb.WriteString(specialEncode(params[k]))
	}
	return sb.String()
}

// specialEncode 阿里云要求的 URL 编码。
//
// url.QueryEscape 之后必须再做三次替换，缺一次签名就对不上：
// 它把空格编成 +（阿里云要 %20）、放过 *（要编成 %2A）、
// 把 ~ 编成 %7E（阿里云要求保留字面量）。
func specialEncode(s string) string {
	e := url.QueryEscape(s)
	e = strings.ReplaceAll(e, "+", "%20")
	e = strings.ReplaceAll(e, "*", "%2A")
	e = strings.ReplaceAll(e, "%7E", "~")
	return e
}

// firstNonEmpty 返回首个非空串。
func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// truncate 截断过长的字符串，避免把整页 HTML 塞进日志。
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

// 保证 aliyunSender 实现了 Sender。
var _ Sender = (*aliyunSender)(nil)

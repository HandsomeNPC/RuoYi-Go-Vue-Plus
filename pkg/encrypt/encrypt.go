// Package encrypt 加解密原语。
package encrypt

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"fmt"
)

// MinRSAKeySize RSA 密钥最小位数。
const MinRSAKeySize = 1024

// aesKeySizes AES 允许的密钥字节数。
var aesKeySizes = [3]int{16, 24, 32}

// EncryptByBase64 base64 编码。
func EncryptByBase64(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

// DecryptByBase64 base64 解码。
func DecryptByBase64(data string) (string, error) {
	out, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("encrypt: base64 解码失败: %w", err)
	}
	return string(out), nil
}

// EncryptByAES AES 加密，返回 base64 密文。
func EncryptByAES(data, password string) (string, error) {
	block, err := aesKey(password)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ecbEncrypt(block, pkcs7Pad([]byte(data), block.BlockSize()))), nil
}

// DecryptByAES AES 解密 base64 密文。
func DecryptByAES(data, password string) (string, error) {
	block, err := aesKey(password)
	if err != nil {
		return "", err
	}

	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("encrypt: AES 密文 base64 解码失败: %w", err)
	}
	size := block.BlockSize()
	if len(raw) == 0 || len(raw)%size != 0 {
		return "", fmt.Errorf("encrypt: AES 密文长度 %d 不是分组长度 %d 的正整数倍", len(raw), size)
	}

	plain, err := pkcs7Unpad(ecbDecrypt(block, raw), size)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// aesKey 校验密钥长度并构造分组密码。
func aesKey(password string) (cipher.Block, error) {
	if password == "" {
		return nil, fmt.Errorf("encrypt: AES 需要传入秘钥信息")
	}
	key := []byte(password)

	ok := false
	for _, n := range aesKeySizes {
		if len(key) == n {
			ok = true
			break
		}
	}
	if !ok {
		return nil, fmt.Errorf("encrypt: AES 秘钥长度要求为 16/24/32 字节，当前为 %d", len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("encrypt: 构造 AES 失败: %w", err)
	}
	return block, nil
}

// ecbEncrypt 逐块 ECB 加密。
func ecbEncrypt(block cipher.Block, src []byte) []byte {
	size := block.BlockSize()
	dst := make([]byte, len(src))
	for i := 0; i < len(src); i += size {
		block.Encrypt(dst[i:i+size], src[i:i+size])
	}
	return dst
}

// ecbDecrypt 逐块 ECB 解密。
func ecbDecrypt(block cipher.Block, src []byte) []byte {
	size := block.BlockSize()
	dst := make([]byte, len(src))
	for i := 0; i < len(src); i += size {
		block.Decrypt(dst[i:i+size], src[i:i+size])
	}
	return dst
}

// pkcs7Pad 按 PKCS#7 补齐。
func pkcs7Pad(src []byte, size int) []byte {
	n := size - len(src)%size
	out := make([]byte, len(src)+n)
	copy(out, src)
	for i := len(src); i < len(out); i++ {
		out[i] = byte(n)
	}
	return out
}

// pkcs7Unpad 校验并剥掉 PKCS#7 填充。
func pkcs7Unpad(src []byte, size int) ([]byte, error) {
	if len(src) == 0 || len(src)%size != 0 {
		return nil, fmt.Errorf("encrypt: AES 解密结果长度 %d 非法", len(src))
	}

	n := int(src[len(src)-1])
	if n == 0 || n > size {
		return nil, fmt.Errorf("encrypt: AES 填充非法（秘钥可能不匹配）")
	}
	for _, b := range src[len(src)-n:] {
		if int(b) != n {
			return nil, fmt.Errorf("encrypt: AES 填充非法（秘钥可能不匹配）")
		}
	}
	return src[:len(src)-n], nil
}

// ParseRSAPrivateKey 解析 base64 编码的 PKCS#8 私钥。
func ParseRSAPrivateKey(base64Key string) (*rsa.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("encrypt: RSA 私钥 base64 解码失败: %w", err)
	}

	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		if k, err1 := x509.ParsePKCS1PrivateKey(der); err1 == nil {
			return checkRSAKeySize(k)
		}
		return nil, fmt.Errorf("encrypt: RSA 私钥格式错误（需为 base64 编码的 PKCS#8）: %w", err)
	}

	key, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("encrypt: 私钥不是 RSA 密钥（实际为 %T）", parsed)
	}
	return checkRSAKeySize(key)
}

// ParseRSAPublicKey 解析 base64 编码的 X.509 公钥。
func ParseRSAPublicKey(base64Key string) (*rsa.PublicKey, error) {
	der, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("encrypt: RSA 公钥 base64 解码失败: %w", err)
	}

	parsed, err := x509.ParsePKIXPublicKey(der)
	if err != nil {
		return nil, fmt.Errorf("encrypt: RSA 公钥格式错误（需为 base64 编码的 X.509 SubjectPublicKeyInfo）: %w", err)
	}

	key, ok := parsed.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("encrypt: 公钥不是 RSA 密钥（实际为 %T）", parsed)
	}
	if size := key.N.BitLen(); size < MinRSAKeySize {
		return nil, fmt.Errorf("encrypt: RSA 密钥长度不能低于 %d 位，当前为 %d 位", MinRSAKeySize, size)
	}
	return key, nil
}

// checkRSAKeySize 校验私钥位数。
func checkRSAKeySize(key *rsa.PrivateKey) (*rsa.PrivateKey, error) {
	if size := key.N.BitLen(); size < MinRSAKeySize {
		return nil, fmt.Errorf("encrypt: RSA 密钥长度不能低于 %d 位，当前为 %d 位", MinRSAKeySize, size)
	}
	return key, nil
}

// DecryptByRSA 用私钥解密 base64 密文。
func DecryptByRSA(data string, key *rsa.PrivateKey) (string, error) {
	if key == nil {
		return "", fmt.Errorf("encrypt: RSA 需要传入私钥进行解密")
	}

	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("encrypt: RSA 密文 base64 解码失败: %w", err)
	}

	k := key.Size()
	if len(raw) == 0 || len(raw)%k != 0 {
		return "", fmt.Errorf("encrypt: RSA 密文长度 %d 不是密钥长度 %d 的正整数倍", len(raw), k)
	}

	var out []byte
	for i := 0; i < len(raw); i += k {
		block, err := rsa.DecryptPKCS1v15(nil, key, raw[i:i+k])
		if err != nil {
			return "", fmt.Errorf("encrypt: RSA 解密失败: %w", err)
		}
		out = append(out, block...)
	}
	return string(out), nil
}

// EncryptByRSA 用公钥加密并返回 base64 密文。
func EncryptByRSA(data string, key *rsa.PublicKey) (string, error) {
	if key == nil {
		return "", fmt.Errorf("encrypt: RSA 需要传入公钥进行加密")
	}

	src := []byte(data)
	max := key.Size() - 11
	if max <= 0 {
		return "", fmt.Errorf("encrypt: RSA 密钥长度 %d 过短", key.Size())
	}

	var out []byte
	for i := 0; i < len(src) || i == 0; i += max {
		end := min(i+max, len(src))
		block, err := rsa.EncryptPKCS1v15(rand.Reader, key, src[i:end])
		if err != nil {
			return "", fmt.Errorf("encrypt: RSA 加密失败: %w", err)
		}
		out = append(out, block...)
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

// GenerateAESPassword 生成 32 字符随机 AES 密钥。
func GenerateAESPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("encrypt: 生成 AES 秘钥失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

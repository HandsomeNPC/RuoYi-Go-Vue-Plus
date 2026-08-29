package encrypt

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
)

// 开发密钥（1024 位），用于跨实现验证。
const (
	devPrivateKey = "MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBAO8QO5Eg4zehk9aP1SShzmlCSVHg8Ufr9yWeN4WqMMsiAPJC+PGGCoBlAD4T14Pqq7oWxc+Yrx2Nwv6eHdwUfPilfjveMO87dK977zIvdVFDSfalGBDZrTUwmzL5bBNkIFhZ/RWctEi8A1ShZCDL2/P3irtVrjh2DsDX/cgJ/7EDAgMBAAECgYEAhNZAQyRDHWZq/45soS5Hw7VRiG21pIE5k22W7G7lLfp3DCaqrYoNy8pTmCruVh7PzVdaE0CEDaf38gNqFCBOT8iTFQiYV3am4W3hsEQM5wmVBeTvCM5P2jsaaBQbqmneRjiZVbs6ha205JSho1Oc85NbaZa8gFVjwZgZWJrbzgECQQD/iZWhkRPtbdeai/Xk7D/eIXKh1Gxid0rWKQq8ikxbaiergn47XzNKrpROVyka3Gn85o7jJphgxp99R3r8sH71AkEA738Dn7xs+I4Y+MLa2EcT78JG3f/VhlWS/ks3qGJ2dfqwS7ntnmf5Q+2Xw+9UcuiK/TxD8K/0inSCkIMeWBOFFwJBAIoTebq3faEJfTqQ7ekojsokIKC4+2epNdLKknaV8/RhQ9Y0yKikJD7yXkiGaDuPZeW1Xvf2XtfL+1niSd5IMBECQDCOOMbe5dzyuj9dCg+FQZZ/dey2XK0Slm22BD/ATrIWtD12IaXXAKNz/Sv9TsrJOLykxkV69wJHIt13p+RFeNsCQGn5XGRn4ZCRVCesJYXyx29MTqkl8sD/gzYcURTZYjHqX2EvtvAyC6gBm9H0EbxmHIi4Oq0tITzklCXj5SpvBEw="
	devPublicKey  = "MIGfMA0GCSqGSIb3DQEBAQUAA4GNADCBiQKBgQDm6u8q8X78onmbU4wGMq3b4ufbWE18YzuWo5jwkUlwWPTPcENbZYtveyTepp2Od1CTDcjhTUmVYvFkhaCF46UfOxZrwoSZc3jf3WXXd0hLOPBHuulynknj2KsWvKDuRig7J2o4KbDhyl0nnlUiMrIiD0tv1rBKpNCJ/T+MN0bERQIDAQAB"
)

// 已知明文，供各条用例复用。
const (
	knownAESKey       = "1234567890123456"
	knownAESPlaintext = `{"username":"admin","password":"admin123"}`
)

// AES-128 的官方已知答案测试向量，来自 FIPS-197 附录 C.1。
var (
	fipsKey        = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	fipsPlaintext  = []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	fipsCiphertext = []byte{0x69, 0xc4, 0xe0, 0xd8, 0x6a, 0x7b, 0x04, 0x30, 0xd8, 0xcd, 0xb7, 0x80, 0x70, 0xb4, 0xc5, 0x5a}
)

// TestAESMatchesFIPSVector 验证底层分组加密与 AES-128 官方向量一致。
func TestAESMatchesFIPSVector(t *testing.T) {
	cipherText, err := EncryptByAES(string(fipsPlaintext), string(fipsKey))
	if err != nil {
		t.Fatalf("EncryptByAES: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	if len(raw) != 32 {
		t.Fatalf("密文长度 = %d, want 32（一块数据 + 一块填充）", len(raw))
	}
	if !bytes.Equal(raw[:16], fipsCiphertext) {
		t.Errorf("首个分组的密文与 FIPS-197 C.1 向量不符\ngot  = %x\nwant = %x",
			raw[:16], fipsCiphertext)
	}

	// 反向也要对：能解回标准明文。
	plain, err := DecryptByAES(cipherText, string(fipsKey))
	if err != nil {
		t.Fatalf("DecryptByAES: %v", err)
	}
	if !bytes.Equal([]byte(plain), fipsPlaintext) {
		t.Errorf("解密结果 = %x, want %x", plain, fipsPlaintext)
	}
}

// TestAESRoundTrip 验证 AES 往返。
func TestAESRoundTrip(t *testing.T) {
	for _, key := range []string{
		"1234567890123456",
		"123456789012345678901234",
		"12345678901234567890123456789012",
	} {
		t.Run(key[:4]+"...", func(t *testing.T) {
			cipherText, err := EncryptByAES(knownAESPlaintext, key)
			if err != nil {
				t.Fatalf("EncryptByAES: %v", err)
			}
			if _, err := base64.StdEncoding.DecodeString(cipherText); err != nil {
				t.Errorf("密文不是合法 base64: %v", err)
			}

			plain, err := DecryptByAES(cipherText, key)
			if err != nil {
				t.Fatalf("DecryptByAES: %v", err)
			}
			if plain != knownAESPlaintext {
				t.Errorf("往返后 = %q, want %q", plain, knownAESPlaintext)
			}
		})
	}
}

// TestAESIsDeterministicECB 验证 ECB 确定性加密。
func TestAESIsDeterministicECB(t *testing.T) {
	a, err := EncryptByAES(knownAESPlaintext, knownAESKey)
	if err != nil {
		t.Fatalf("EncryptByAES: %v", err)
	}
	b, err := EncryptByAES(knownAESPlaintext, knownAESKey)
	if err != nil {
		t.Fatalf("EncryptByAES: %v", err)
	}
	if a != b {
		t.Errorf("ECB 应为确定性加密，两次结果不同:\n%s\n%s", a, b)
	}

	// 相同的 16 字节明文块应产出相同的密文块。
	block := "AAAAAAAAAAAAAAAA"
	cipherText, err := EncryptByAES(block+block, knownAESKey)
	if err != nil {
		t.Fatalf("EncryptByAES: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	if len(raw) < 32 {
		t.Fatalf("密文长度 %d 过短", len(raw))
	}
	if string(raw[0:16]) != string(raw[16:32]) {
		t.Error("ECB 下相同明文块应产出相同密文块，实际不同 —— 模式可能已被改动")
	}
}

// TestAESWrongKeyFails 验证用错的密钥解密必须报错。
func TestAESWrongKeyFails(t *testing.T) {
	cipherText, err := EncryptByAES(knownAESPlaintext, knownAESKey)
	if err != nil {
		t.Fatalf("EncryptByAES: %v", err)
	}

	// 试足够多把错密钥。
	failed := 0
	const attempts = 64
	for i := 0; i < attempts; i++ {
		wrong := make([]byte, 16)
		if _, err := rand.Read(wrong); err != nil {
			t.Fatalf("rand: %v", err)
		}
		plain, err := DecryptByAES(cipherText, string(wrong))
		if err != nil {
			failed++
			continue
		}
		// 极小概率填充恰好合法，但明文绝不该与原文相同。
		if plain == knownAESPlaintext {
			t.Fatalf("用错误密钥解出了正确明文，密钥校验完全失效")
		}
	}
	if failed < attempts/2 {
		t.Errorf("%d 次错误密钥中只有 %d 次被拒，填充校验可能失效", attempts, failed)
	}
}

func TestAESKeyLengthValidation(t *testing.T) {
	tests := map[string]string{
		"空密钥":    "",
		"15 字节":  "123456789012345",
		"17 字节":  "12345678901234567",
		"31 字节":  "1234567890123456789012345678901",
		"33 字节":  "123456789012345678901234567890123",
		"16 个汉字": strings.Repeat("中", 16), // 48 字节
	}
	for name, key := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := EncryptByAES("x", key); err == nil {
				t.Error("非法密钥长度应报错")
			}
			if _, err := DecryptByAES("eA==", key); err == nil {
				t.Error("非法密钥长度应报错")
			}
		})
	}
}

// TestAESMalformedCiphertext 验证畸形密文不 panic 且报错。
func TestAESMalformedCiphertext(t *testing.T) {
	tests := map[string]string{
		"非 base64":  "!!!not-base64!!!",
		"空串":        "",
		"不足一个分组":    base64.StdEncoding.EncodeToString([]byte("short")),
		"非分组整数倍":    base64.StdEncoding.EncodeToString(make([]byte, 17)),
		"合法长度但全零填充": base64.StdEncoding.EncodeToString(make([]byte, 16)),
	}
	for name, data := range tests {
		t.Run(name, func(t *testing.T) {
			// 不 panic 是本用例的第一要求。
			if _, err := DecryptByAES(data, knownAESKey); err == nil {
				t.Error("畸形密文应报错")
			}
		})
	}
}

// TestAESPadsFullBlockOnExactMultiple 验证明文为分组整数倍时补满一整块。
func TestAESPadsFullBlockOnExactMultiple(t *testing.T) {
	exact := "0123456789ABCDEF"
	cipherText, err := EncryptByAES(exact, knownAESKey)
	if err != nil {
		t.Fatalf("EncryptByAES: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(cipherText)
	if err != nil {
		t.Fatalf("base64: %v", err)
	}
	if len(raw) != 32 {
		t.Errorf("16 字节明文应产出 32 字节密文(补满一整块), 实际 %d", len(raw))
	}

	plain, err := DecryptByAES(cipherText, knownAESKey)
	if err != nil {
		t.Fatalf("DecryptByAES: %v", err)
	}
	if plain != exact {
		t.Errorf("往返后 = %q, want %q", plain, exact)
	}
}

// TestAESEmptyPlaintext 验证空明文也能往返。
func TestAESEmptyPlaintext(t *testing.T) {
	cipherText, err := EncryptByAES("", knownAESKey)
	if err != nil {
		t.Fatalf("EncryptByAES: %v", err)
	}
	plain, err := DecryptByAES(cipherText, knownAESKey)
	if err != nil {
		t.Fatalf("DecryptByAES: %v", err)
	}
	if plain != "" {
		t.Errorf("往返后 = %q, want 空串", plain)
	}
}

// TestParseDevKeys 验证开发密钥可被解析。
func TestParseDevKeys(t *testing.T) {
	priv, err := ParseRSAPrivateKey(devPrivateKey)
	if err != nil {
		t.Fatalf("解析原项目私钥失败: %v", err)
	}
	if got := priv.N.BitLen(); got != 1024 {
		t.Errorf("私钥位数 = %d, want 1024（原项目那把是 1024 位）", got)
	}

	pub, err := ParseRSAPublicKey(devPublicKey)
	if err != nil {
		t.Fatalf("解析原项目公钥失败: %v", err)
	}
	if got := pub.N.BitLen(); got != 1024 {
		t.Errorf("公钥位数 = %d, want 1024", got)
	}

	// 这两把不是一对：用私钥解自己公钥加的密文应当失败。
	cipherText, err := EncryptByRSA("test", pub)
	if err != nil {
		t.Fatalf("EncryptByRSA: %v", err)
	}
	if _, err := DecryptByRSA(cipherText, priv); err == nil {
		t.Error("配置里的 publicKey 与 privateKey 不是一对（各自对应前端的另一半），" +
			"用它们往返竟然成功了 —— 密钥配置可能已被改动")
	}
}

// TestRSARoundTrip 验证 RSA 往返。
func TestRSARoundTrip(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	// 覆盖协议真实载荷形态：base64 编码后的 32 字符 AES 密钥。
	aesPassword, err := GenerateAESPassword()
	if err != nil {
		t.Fatalf("GenerateAESPassword: %v", err)
	}
	payload := EncryptByBase64(aesPassword)

	cipherText, err := EncryptByRSA(payload, &key.PublicKey)
	if err != nil {
		t.Fatalf("EncryptByRSA: %v", err)
	}
	got, err := DecryptByRSA(cipherText, key)
	if err != nil {
		t.Fatalf("DecryptByRSA: %v", err)
	}
	if got != payload {
		t.Errorf("往返后 = %q, want %q", got, payload)
	}

	// 再走一遍完整的双层解码，确认能还原出原始 AES 密钥。
	decoded, err := DecryptByBase64(got)
	if err != nil {
		t.Fatalf("DecryptByBase64: %v", err)
	}
	if decoded != aesPassword {
		t.Errorf("双层解码后 = %q, want %q", decoded, aesPassword)
	}
}

// TestRSAMultiBlock 验证超过单块容量的明文能分块加解密。
func TestRSAMultiBlock(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	// 1024 位密钥单块最多 117 字节，取 300 字节强制分三块。
	long := strings.Repeat("A", 300)

	cipherText, err := EncryptByRSA(long, &key.PublicKey)
	if err != nil {
		t.Fatalf("EncryptByRSA: %v", err)
	}
	got, err := DecryptByRSA(cipherText, key)
	if err != nil {
		t.Fatalf("DecryptByRSA: %v", err)
	}
	if got != long {
		t.Errorf("分块往返失败: 长度 %d, want %d", len(got), len(long))
	}
}

func TestRSAErrors(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	t.Run("nil 私钥", func(t *testing.T) {
		if _, err := DecryptByRSA("x", nil); err == nil {
			t.Error("want error")
		}
	})
	t.Run("nil 公钥", func(t *testing.T) {
		if _, err := EncryptByRSA("x", nil); err == nil {
			t.Error("want error")
		}
	})
	t.Run("密文非 base64", func(t *testing.T) {
		if _, err := DecryptByRSA("!!!", key); err == nil {
			t.Error("want error")
		}
	})
	t.Run("密文长度非密钥长度整数倍", func(t *testing.T) {
		bad := base64.StdEncoding.EncodeToString(make([]byte, 100))
		if _, err := DecryptByRSA(bad, key); err == nil {
			t.Error("want error")
		}
	})
	t.Run("密文长度对但内容是垃圾", func(t *testing.T) {
		bad := base64.StdEncoding.EncodeToString(make([]byte, 128))
		if _, err := DecryptByRSA(bad, key); err == nil {
			t.Error("want error")
		}
	})
}

// shortPrivateKey / shortPublicKey 为 512 位密钥，用于验证位数下限。
const (
	shortPrivateKey = "MIIBUwIBADANBgkqhkiG9w0BAQEFAASCAT0wggE5AgEAAkEA6E4skRL/VkmJBO6VoaasmY+Xnxx+nkQAusBvW3aNshXt0xRZDIlz+M70Ocg0MpyKS6yisNEwgqF0jGgI9/cfUwIDAQABAkAPOIAHCV2dg7fskM1RCCCq9xOSI0XQjNgXZGBnd78U+ebc0+5YpUXegf+07V3VppBJDxE6g9hbcYyhkW6NFhDBAiEA/odrtvCqL7iaz9wajPK9OSLgz1NC/u8pZLUw8hvNQiMCIQDppd+H9jdZ9wAkbhGlo4euqbqjdRml36qdEaMcMduJEQIgLdUCv2Fcs9UhA1bV7RV0n0o5gvuyL6evI3RBCQeakVMCIBuyjyoV9P/UOQ8YgT0KgrYg5sAjzJOOTTJredOI0YaRAiBb0R/mKKmA6s8PY5zvx1wJbqv/HXroyiVvz0tqxiXLXA=="
	shortPublicKey  = "MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAOhOLJES/1ZJiQTulaGmrJmPl58cfp5EALrAb1t2jbIV7dMUWQyJc/jO9DnINDKcikusorDRMIKhdIxoCPf3H1MCAwEAAQ=="
)

// TestRSAKeySizeLimit 验证密钥位数下限。
func TestRSAKeySizeLimit(t *testing.T) {
	if _, err := ParseRSAPrivateKey(shortPrivateKey); err == nil {
		t.Errorf("512 位私钥应被拒（下限 %d 位）", MinRSAKeySize)
	} else if !strings.Contains(err.Error(), "512") {
		t.Errorf("错误文案应带上实际位数，便于定位: %v", err)
	}

	if _, err := ParseRSAPublicKey(shortPublicKey); err == nil {
		t.Errorf("512 位公钥应被拒（下限 %d 位）", MinRSAKeySize)
	} else if !strings.Contains(err.Error(), "512") {
		t.Errorf("错误文案应带上实际位数，便于定位: %v", err)
	}
}

// TestParsePKCS1PrivateKey 验证 PKCS#1 私钥也可被接受。
func TestParsePKCS1PrivateKey(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	der := x509.MarshalPKCS1PrivateKey(key)
	parsed, err := ParseRSAPrivateKey(base64.StdEncoding.EncodeToString(der))
	if err != nil {
		t.Fatalf("PKCS#1 私钥应被接受: %v", err)
	}
	if parsed.N.Cmp(key.N) != 0 {
		t.Error("解析出的私钥与原私钥不一致")
	}
}

func TestParseKeyErrors(t *testing.T) {
	tests := map[string]string{
		"空串":             "",
		"非 base64":       "!!!not-base64!!!",
		"base64 但不是 DER": base64.StdEncoding.EncodeToString([]byte("hello world")),
	}
	for name, key := range tests {
		t.Run("私钥/"+name, func(t *testing.T) {
			if _, err := ParseRSAPrivateKey(key); err == nil {
				t.Error("want error")
			}
		})
		t.Run("公钥/"+name, func(t *testing.T) {
			if _, err := ParseRSAPublicKey(key); err == nil {
				t.Error("want error")
			}
		})
	}

	// 公私钥配错位置时必须报错。
	t.Run("公私钥配错位置", func(t *testing.T) {
		if _, err := ParseRSAPrivateKey(devPublicKey); err == nil {
			t.Error("公钥不该被当作私钥解析成功")
		}
		if _, err := ParseRSAPublicKey(devPrivateKey); err == nil {
			t.Error("私钥不该被当作公钥解析成功")
		}
	})
}

// TestBase64 验证 base64 往返及非法输入报错。
func TestBase64(t *testing.T) {
	const data = "hello 世界"
	encoded := EncryptByBase64(data)
	got, err := DecryptByBase64(encoded)
	if err != nil {
		t.Fatalf("DecryptByBase64: %v", err)
	}
	if got != data {
		t.Errorf("往返后 = %q, want %q", got, data)
	}

	if _, err := DecryptByBase64("!!!not-base64!!!"); err == nil {
		t.Error("非法 base64 应报错，不该静默返回尽力而为的结果")
	}
}

// TestGenerateAESPassword 验证生成的 AES 密钥长度为 32 且每次不同。
func TestGenerateAESPassword(t *testing.T) {
	seen := make(map[string]bool, 128)
	for i := 0; i < 128; i++ {
		pw, err := GenerateAESPassword()
		if err != nil {
			t.Fatalf("GenerateAESPassword: %v", err)
		}
		// 24 随机字节 base64 后恰好 32 字符。
		if len(pw) != 32 {
			t.Fatalf("密钥长度 = %d, want 32", len(pw))
		}
		// 必须能直接用作 AES 密钥。
		if _, err := EncryptByAES("x", pw); err != nil {
			t.Fatalf("生成的密钥不能用于 AES: %v", err)
		}
		if seen[pw] {
			t.Fatal("生成的 AES 密钥出现重复，随机源可能有问题")
		}
		seen[pw] = true
	}
}

// TestUnwrapJSONString 前端 axios 会把密文 JSON.stringify 一次，带上外层引号，
// 必须剥掉才能过 Go 严格的 base64 解码。
func TestUnwrapJSONString(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"带引号(前端实际形态)", `"abc123=="`, "abc123=="},
		{"无引号原样返回", "abc123==", "abc123=="},
		{"只有前引号不剥", `"abc123==`, `"abc123==`},
		{"只有后引号不剥", `abc123=="`, `abc123=="`},
		{"空串", "", ""},
		{"单个引号不越界", `"`, `"`},
		{"两个引号剥成空", `""`, ""},
		{"内部引号保留", `"a"b"`, `a"b`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := unwrapJSONString([]byte(tc.in)); got != tc.want {
				t.Errorf("unwrapJSONString(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestDecryptByAESAcceptsQuotedCipher 端到端确认：加密结果套上 JSON 引号后，
// 经 unwrapJSONString 处理仍能解回原文。
func TestDecryptByAESAcceptsQuotedCipher(t *testing.T) {
	pw, err := GenerateAESPassword()
	if err != nil {
		t.Fatalf("GenerateAESPassword: %v", err)
	}
	const plain = `{"username":"admin","password":"admin123"}`
	cipher, err := EncryptByAES(plain, pw)
	if err != nil {
		t.Fatalf("EncryptByAES: %v", err)
	}

	// 未剥引号时必须失败(复现线上报错)。
	if _, err := DecryptByAES(`"`+cipher+`"`, pw); err == nil {
		t.Error("带引号的密文直接解密应失败，否则说明 base64 解码不够严格")
	}
	// 剥引号后必须解回原文。
	got, err := DecryptByAES(unwrapJSONString([]byte(`"`+cipher+`"`)), pw)
	if err != nil {
		t.Fatalf("剥引号后解密失败: %v", err)
	}
	if got != plain {
		t.Errorf("解密结果 = %q, want %q", got, plain)
	}
}

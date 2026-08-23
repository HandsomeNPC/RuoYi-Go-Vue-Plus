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

// devPrivateKey / devPublicKey 原项目 application.yml:156,158 里那对 1024 位开发密钥。
//
// 用它们而不是每次现生成：这两个串是从原项目手抄过来的，本文件顺带充当
// 「抄对了」的证据。ParseRSA*Key 能解析成功、且用它们能完成一轮加解密，
// 就说明格式与 Java 侧的 PKCS#8 / X.509 编码假设一致。
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
//
// 这是本文件里唯一能**跨实现**验证的一条。往返测试（自己加密自己解密）无法
// 发现「两边算法不一致」—— 模式选错（CBC 而非 ECB）、字节序搞反都能自洽地
// 往返成功，却与前端完全对不上。拿一段由标准规定的密文来比对，
// 才能证明底层确实是 AES-ECB。
//
// 理想的验证应当是拿一段 hutool 真实产出的密文来解，但那需要跑 Java；
// 退一步用 FIPS 向量至少锁住了「分组密码 + ECB 模式」这一层。
// **仍未跨语言核实的是 PKCS#7 填充与 base64 编码那两层** ——
// 它们由 EncryptUtils 的代码阅读推得（SecureUtil.aes 走 JCE 默认的
// AES/ECB/PKCS5Padding，encryptBase64 走 base64），首次前后端联调时应重点验。
var (
	fipsKey        = []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x0e, 0x0f}
	fipsPlaintext  = []byte{0x00, 0x11, 0x22, 0x33, 0x44, 0x55, 0x66, 0x77, 0x88, 0x99, 0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0xff}
	fipsCiphertext = []byte{0x69, 0xc4, 0xe0, 0xd8, 0x6a, 0x7b, 0x04, 0x30, 0xd8, 0xcd, 0xb7, 0x80, 0x70, 0xb4, 0xc5, 0x5a}
)

// 底层分组加密必须与 AES-128 的官方向量一致。
//
// 明文恰好一个分组，PKCS#7 会补满第二块，所以密文的**前 16 字节**
// 就是标准向量规定的值。
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

// AES 往返：加密再解密应还原明文。
func TestAESRoundTrip(t *testing.T) {
	// 三种合法密钥长度都要覆盖：aesKey 的校验按**字节**判，
	// 而 Java 按 length() 判字符数，含非 ASCII 时两者会分叉。
	for _, key := range []string{
		"1234567890123456",                 // 16 字节 AES-128
		"123456789012345678901234",         // 24 字节 AES-192
		"12345678901234567890123456789012", // 32 字节 AES-256
	} {
		t.Run(key[:4]+"...", func(t *testing.T) {
			cipherText, err := EncryptByAES(knownAESPlaintext, key)
			if err != nil {
				t.Fatalf("EncryptByAES: %v", err)
			}
			// 密文必须是合法 base64（协议要求），否则前端解不开。
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

// ECB 无 IV，相同明文 + 相同密钥恒产出相同密文。
//
// 这条既是对齐 Java（那边同样是确定性的），也是把 ECB 的这个性质**显式记录
// 下来**：它意味着密文会暴露明文的重复结构。将来若有人把模式换成 CBC/GCM，
// 这条用例会失败 —— 那正是提醒「这是协议变更，得连前端一起改」的地方。
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

	// 相同的 16 字节明文块应产出相同的密文块 —— ECB 的定义性特征。
	// 两个完全相同的分组，密文的前 16 字节应与次 16 字节相同。
	block := "AAAAAAAAAAAAAAAA" // 恰好一个分组
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

// 用错的密钥解密必须**报错**，而不是返回一段垃圾明文。
//
// 这条锁住 pkcs7Unpad 的逐字节校验：只读最后一个字节当长度的话，
// 用错密钥有约 1/16 的概率解出一个「看起来成功」的结果，
// 那段垃圾会被当成 JSON 交给 handler。
func TestAESWrongKeyFails(t *testing.T) {
	cipherText, err := EncryptByAES(knownAESPlaintext, knownAESKey)
	if err != nil {
		t.Fatalf("EncryptByAES: %v", err)
	}

	// 试足够多把错密钥，确保不是靠运气过的。
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
	// 允许极少数因填充碰巧合法而「解成功」（那是 PKCS#7 的固有性质，
	// 概率约 1/256），但绝大多数必须被拒。
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
		"16 个汉字": strings.Repeat("中", 16), // 48 字节：Java 按字符数会放过，这里按字节拒掉
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

// 畸形密文不能 panic，必须返回错误。
//
// crypto/cipher 的 Decrypt 对不足一个分组的输入会 panic，
// 而这些输入全都来自网络 —— 少一个长度检查就是一条远程打挂进程的路径。
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
			// 不 panic 是本用例的第一要求；返回错误是第二。
			if _, err := DecryptByAES(data, knownAESKey); err == nil {
				t.Error("畸形密文应报错")
			}
		})
	}
}

// PKCS#7 在明文恰好为分组整数倍时要补满一整块。
//
// 少了这一块，解密方无法区分「最后一块是数据」还是「是填充」，
// 会把明文尾部截掉 16 字节。
func TestAESPadsFullBlockOnExactMultiple(t *testing.T) {
	exact := "0123456789ABCDEF" // 恰好 16 字节
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

// 空明文也要能往返 —— 一个 {} 的请求体是合法的。
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

// 原项目那对开发密钥必须能被解析出来。
//
// 抄错一个字符不会有编译期症状，只会让所有加密接口在运行期失败。
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

	// 这两把**不是**一对：privateKey 对应前端的加密公钥，
	// publicKey 对应前端的解密私钥。用私钥去解自己公钥加的密文应当失败 ——
	// 把这件事记录下来，免得有人误以为可以拿它们做往返测试。
	cipherText, err := EncryptByRSA("test", pub)
	if err != nil {
		t.Fatalf("EncryptByRSA: %v", err)
	}
	if _, err := DecryptByRSA(cipherText, priv); err == nil {
		t.Error("配置里的 publicKey 与 privateKey 不是一对（各自对应前端的另一半），" +
			"用它们往返竟然成功了 —— 密钥配置可能已被改动")
	}
}

// RSA 往返：用同一对密钥加密再解密。
func TestRSARoundTrip(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}

	// 覆盖协议里的真实载荷形态：base64 编码后的 32 字符 AES 密钥（44 字节）。
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

// 超过单块容量的明文要能分块加解密。
//
// 本协议里待加密的只是一个 44 字节的密钥载荷，恒为单块；
// 这条覆盖的是响应加密里可能出现的长载荷，以及「分块规则与 hutool 一致」。
func TestRSAMultiBlock(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	// 1024 位密钥单块最多 128-11 = 117 字节，取 300 字节强制分三块。
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

// 512 位密钥，用于验证位数下限。
//
// 写成常量而非 rsa.GenerateKey(512)：Go 1.24 起标准库**拒绝生成**低于
// 1024 位的密钥（返回 "512-bit keys are insecure"），没法在测试里现生成。
// 这两串由 openssl genpkey -pkeyopt rsa_keygen_bits:512 产出。
const (
	shortPrivateKey = "MIIBUwIBADANBgkqhkiG9w0BAQEFAASCAT0wggE5AgEAAkEA6E4skRL/VkmJBO6VoaasmY+Xnxx+nkQAusBvW3aNshXt0xRZDIlz+M70Ocg0MpyKS6yisNEwgqF0jGgI9/cfUwIDAQABAkAPOIAHCV2dg7fskM1RCCCq9xOSI0XQjNgXZGBnd78U+ebc0+5YpUXegf+07V3VppBJDxE6g9hbcYyhkW6NFhDBAiEA/odrtvCqL7iaz9wajPK9OSLgz1NC/u8pZLUw8hvNQiMCIQDppd+H9jdZ9wAkbhGlo4euqbqjdRml36qdEaMcMduJEQIgLdUCv2Fcs9UhA1bV7RV0n0o5gvuyL6evI3RBCQeakVMCIBuyjyoV9P/UOQ8YgT0KgrYg5sAjzJOOTTJredOI0YaRAiBb0R/mKKmA6s8PY5zvx1wJbqv/HXroyiVvz0tqxiXLXA=="
	shortPublicKey  = "MFwwDQYJKoZIhvcNAQEBBQADSwAwSAJBAOhOLJES/1ZJiQTulaGmrJmPl58cfp5EALrAb1t2jbIV7dMUWQyJc/jO9DnINDKcikusorDRMIKhdIxoCPf3H1MCAwEAAQ=="
)

// 密钥位数下限必须拦住过短的密钥，对齐 validateRsaKeySize。
//
// 这条同时锁住一件事：拦截必须发生在**我们自己的校验**里，而不是靠标准库
// 恰好也拒绝。config 侧的启动期校验依赖它给出可读的错误文案。
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

// PKCS#1 格式的私钥也应被接受（openssl 默认输出）。
//
// 协议要求的是 PKCS#8，但手工生成密钥时踩到 PKCS#1 太常见，
// 报「格式错误」会让人反复怀疑 base64 抄错了。
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

	// 把公钥喂给私钥解析函数（一个很常见的配置错位），必须报错。
	t.Run("公私钥配错位置", func(t *testing.T) {
		if _, err := ParseRSAPrivateKey(devPublicKey); err == nil {
			t.Error("公钥不该被当作私钥解析成功")
		}
		if _, err := ParseRSAPublicKey(devPrivateKey); err == nil {
			t.Error("私钥不该被当作公钥解析成功")
		}
	})
}

// base64 编解码往返，并确认 DecryptByBase64 对非法输入报错。
//
// 后半条是相对 hutool 的有意偏差：Base64.decodeStr 对非法输入静默返回
// 尽力而为的结果，于是一个被截断的密钥会一路走到 AES 那里，
// 报出一个跟真实原因无关的「秘钥长度」错误。
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

// 生成的 AES 密钥必须是 32 字节（AES-256 的合法长度）且每次都不同。
func TestGenerateAESPassword(t *testing.T) {
	seen := make(map[string]bool, 128)
	for i := 0; i < 128; i++ {
		pw, err := GenerateAESPassword()
		if err != nil {
			t.Fatalf("GenerateAESPassword: %v", err)
		}
		// 24 随机字节 base64 后恰好 32 字符、无 padding。
		if len(pw) != 32 {
			t.Fatalf("密钥长度 = %d, want 32", len(pw))
		}
		// 必须能直接用作 AES 密钥 —— 这是它存在的全部意义。
		if _, err := EncryptByAES("x", pw); err != nil {
			t.Fatalf("生成的密钥不能用于 AES: %v", err)
		}
		if seen[pw] {
			t.Fatal("生成的 AES 密钥出现重复，随机源可能有问题")
		}
		seen[pw] = true
	}
}

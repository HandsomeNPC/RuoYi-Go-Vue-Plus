// Package encrypt 加解密原语，对应原项目 ruoyi-common-encrypt 的
// utils/EncryptUtils.java。
//
// 只放「算法」，不放「策略」：谁该被加密、密钥从哪来、失败了回什么响应，
// 都在 pkg/middleware/crypto.go 与 pkg/config。分开是因为阶段 4+ 的
// @EncryptField（字段级加密，Java 侧走 MybatisEncryptInterceptor）会复用同一套
// 原语，而那条路径与 HTTP 无关、不该 import gin。
//
// # 与 EncryptUtils 的两处结构性差异
//
// 一是**密钥只解析一次**。EncryptUtils 的每个方法都收 base64 字符串、内部
// 重新 KeyFactory.generatePublic —— 那是每请求一次的 ASN.1 解析。这里改成
// ParseRSA*Key 返回 *rsa.PrivateKey / *rsa.PublicKey，由调用方在启动期解析、
// 捕获进闭包（middleware 就是这么用的）。顺带把「密钥格式错误」从运行期
// 挪到了启动期。
//
// 二是**只认 base64 密文，不认 Hex**。hutool 的 decryptStr 走 SecureUtil.decode，
// 它用 HexUtil.isHexNumber 猜编码，猜不中才当 base64。那个启发式本身是个坑：
// 一段恰好只含 [0-9a-f] 的 base64 密文会被误判成 Hex 从而解出乱码。
// 本项目的加密侧（前端 CryptoJS、以及本包的 EncryptByAES）一律产出 base64，
// 没有 Hex 的来源，所以不猜。
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

// MinRSAKeySize RSA 密钥最小位数，对应 EncryptUtils.MIN_RSA_KEY_SIZE。
//
// 1024 位按今天的标准已经偏弱（NIST 自 2013 年起建议 2048 位以上），
// 这里保留 1024 是因为原项目 yaml 里那对开发密钥就是 1024 位，
// 调高会让照抄过来的默认配置直接启动失败。它是**下限**而非推荐值 ——
// 生产环境换密钥时应当用 2048 位以上。
const MinRSAKeySize = 1024

// aesKeySizes AES 允许的密钥字节数，对应 EncryptUtils 里那个 {16, 24, 32} 数组。
//
// 对齐 Java 的校验是有必要的：Go 的 aes.NewCipher 对其他长度会返回
// KeySizeError，报错文案里带上具体长度，而 Java 侧是「AES秘钥长度要求为
// 16位、24位、32位」。两边都拒，只是文案不同。
var aesKeySizes = [3]int{16, 24, 32}

// EncryptByBase64 base64 编码，对应 EncryptUtils.encryptByBase64。
//
// 名字里的 encrypt 是照抄原项目的叫法，**它不是加密** —— base64 是编码，
// 没有密钥、可逆、不提供任何保密性。保留这个名字是为了让对照 Java 源码时
// 能一眼对上（协议里那一层 base64 就是这么套上去的）。
func EncryptByBase64(data string) string {
	return base64.StdEncoding.EncodeToString([]byte(data))
}

// DecryptByBase64 base64 解码，对应 EncryptUtils.decryptByBase64。
//
// 同上：这是解码，不是解密。
//
// 相比 hutool 多返回一个 error：Base64.decodeStr 对非法输入静默返回
// 尽力而为的结果（甚至空串），于是一个被截断的密钥会一路走到 AES 那里，
// 报出一个跟真实原因无关的「秘钥长度」错误。
func DecryptByBase64(data string) (string, error) {
	out, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return "", fmt.Errorf("encrypt: base64 解码失败: %w", err)
	}
	return string(out), nil
}

// EncryptByAES AES 加密，返回 base64 密文，对应 EncryptUtils.encryptByAes。
//
// password 是**字符串形态的密钥**，取其 UTF-8 字节作为 AES key，
// 长度必须是 16/24/32 字节 —— 对齐 Java 侧 password.getBytes(UTF_8)。
// 注意这意味着密钥的**字符数**不等于字节数：一个 16 个汉字的密码是 48 字节，
// 在 Java 侧会被 length()==16 的校验放过、然后在 JCE 里炸掉；
// 这里按字节判，行为更一致（见 aesKey）。
func EncryptByAES(data, password string) (string, error) {
	block, err := aesKey(password)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(ecbEncrypt(block, pkcs7Pad([]byte(data), block.BlockSize()))), nil
}

// DecryptByAES AES 解密 base64 密文，对应 EncryptUtils.decryptByAes。
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
	// ECB 只能整块解。长度不对时必须在这里拒掉：交给下面的循环会 panic
	// （crypto/cipher 的 Decrypt 对不足一块的输入直接 panic）。
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

// ecbEncrypt 逐块 ECB 加密。src 必须已补齐到分组长度的整数倍。
//
// # 为什么要手写
//
// Go 的 crypto/cipher **有意不提供 ECB**（提交 f0e1b1e 的注释：ECB
// 不该被当成通用模式使用，提供它只会让人误用）。而原项目用的是
// hutool 的 SecureUtil.aes(byte[])，它走 JCE 默认变换
// AES/ECB/PKCS5Padding —— 于是这里只能手写。
//
// # ECB 的性质，以及为什么仍然照抄
//
// ECB 无 IV，相同明文块恒产出相同密文块：密文因此暴露明文的**重复结构**，
// 且可被逐块重排/替换而不被察觉（无完整性保护）。放在本项目的场景下：
// 前端每次请求随机生成一次性 AES 密钥（EncryptResponseBodyWrapper
// 的响应侧就是这么做的，请求侧同理），密钥不复用则跨请求的指纹比对不成立；
// 但**同一次请求内**的重复块仍然可辨（如一个大数组里的重复元素）。
//
// 不改成 CBC/GCM 是因为这是**通信协议**，不是内部实现：前端的加密实现
// （hardcode 在 Vue 侧）与这里必须逐字节对齐，单方面换模式等于把接口打死。
// 要收紧就得前后端一起换，那是独立的一项改造。
func ecbEncrypt(block cipher.Block, src []byte) []byte {
	size := block.BlockSize()
	dst := make([]byte, len(src))
	for i := 0; i < len(src); i += size {
		block.Encrypt(dst[i:i+size], src[i:i+size])
	}
	return dst
}

// ecbDecrypt 逐块 ECB 解密。src 长度必须是分组长度的整数倍（由调用方保证）。
func ecbDecrypt(block cipher.Block, src []byte) []byte {
	size := block.BlockSize()
	dst := make([]byte, len(src))
	for i := 0; i < len(src); i += size {
		block.Decrypt(dst[i:i+size], src[i:i+size])
	}
	return dst
}

// pkcs7Pad 按 PKCS#7 补齐。
//
// 即 JCE 所称的 PKCS5Padding —— PKCS#5 原本只定义 8 字节分组，
// JCE 沿用旧名但对 16 字节分组的 AES 实际做的是 PKCS#7，两者在这里等价。
//
// 明文已经是整数块时也要补**满一整块**，否则解不出原长度。
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
//
// 必须逐字节校验，不能只读最后一个字节然后截断：填充字节由密文决定，
// 用错的密钥解出来的尾字节可以是任意值，直接拿它当长度会截出一段
// 长度随机的垃圾（甚至越界）。这也是我们能判断出「密钥不对」的唯一依据。
//
// 注意这个 error 不能原样回给客户端 —— 它连同 RSA 的失败一起，
// 在 middleware 侧被折叠成同一句文案（见那边关于 padding oracle 的说明）。
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

// ParseRSAPrivateKey 解析 base64 编码的 PKCS#8 私钥，
// 对应 EncryptUtils.validateRsaPrivateKey + SecureUtil.rsa(privateKey, null)。
//
// 编码格式对齐 Java 的 PKCS8EncodedKeySpec：generateRsaKey 用
// getPrivate().getEncoded() 导出，那就是 base64(PKCS#8 DER)，
// 也就是 PEM 里 "BEGIN PRIVATE KEY" 去掉头尾和换行后的那一段
// （**不是** "BEGIN RSA PRIVATE KEY"，后者是 PKCS#1）。
//
// 顺带做了 Java 那边的位数校验（validateRsaKeySize），
// 好让一个过短的密钥在启动期就被拦下，而不是运行到第一个加密请求。
func ParseRSAPrivateKey(base64Key string) (*rsa.PrivateKey, error) {
	der, err := base64.StdEncoding.DecodeString(base64Key)
	if err != nil {
		return nil, fmt.Errorf("encrypt: RSA 私钥 base64 解码失败: %w", err)
	}

	parsed, err := x509.ParsePKCS8PrivateKey(der)
	if err != nil {
		// 顺手兜一把 PKCS#1：手工生成密钥时用 openssl 默认输出的就是它，
		// 报「格式错误」而不指出这一点会让人反复怀疑是 base64 抄错了。
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

// ParseRSAPublicKey 解析 base64 编码的 X.509 (SubjectPublicKeyInfo) 公钥，
// 对应 EncryptUtils.validateRsaPublicKey。
//
// 对齐 Java 的 X509EncodedKeySpec，即 base64(SPKI DER)，
// PEM 里 "BEGIN PUBLIC KEY" 的那一段。
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

// checkRSAKeySize 校验私钥位数，对应 validateRsaKeySize。
func checkRSAKeySize(key *rsa.PrivateKey) (*rsa.PrivateKey, error) {
	if size := key.N.BitLen(); size < MinRSAKeySize {
		return nil, fmt.Errorf("encrypt: RSA 密钥长度不能低于 %d 位，当前为 %d 位", MinRSAKeySize, size)
	}
	return key, nil
}

// DecryptByRSA 用私钥解密 base64 密文，对应 EncryptUtils.decryptByRsa。
//
// 填充是 PKCS#1 v1.5，对齐 hutool RSA 的默认变换 RSA/ECB/PKCS1Padding
// （hutool 的 RSA 类常量 ALGORITHM_RSA 就是这个值）。**不是 OAEP** ——
// 换成 OAEP 前端就解不开了。
//
// 分块与 hutool 一致：AsymmetricCrypto 把密文按密钥长度（k = 位数/8）切块
// 逐块解，再把结果拼起来。本协议里待解的只是一个 44 字节的 AES 密钥载荷，
// 恒为单块；支持多块是为了让「密文长度不对」表现为一句明确的错误，
// 而不是 rsa 包一句 "decryption error"。
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
		// 用 DecryptPKCS1v15 而非 DecryptPKCS1v15SessionKey：后者要求预先
		// 知道明文长度，而这里的明文是变长的 base64 串。padding oracle
		// 的防护改由调用方承担 —— middleware 把所有解密失败折叠成同一句
		// 文案、不区分是 RSA 还是 AES 阶段失败（见那边的说明）。
		block, err := rsa.DecryptPKCS1v15(nil, key, raw[i:i+k])
		if err != nil {
			return "", fmt.Errorf("encrypt: RSA 解密失败: %w", err)
		}
		out = append(out, block...)
	}
	return string(out), nil
}

// EncryptByRSA 用公钥加密并返回 base64 密文，对应 EncryptUtils.encryptByRsa。
//
// 分块规则对齐 hutool：PKCS#1 v1.5 的每块最多装 k-11 字节，超出就切块。
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
	// 空明文也要走一次加密：PKCS#1 v1.5 对空输入产出一个完整的块，
	// 直接返回空串会让对端拿到一个解不开的空密文。
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

// GenerateAESPassword 生成一个 32 字符的随机 AES 密钥（字符串形态），
// 对应 EncryptResponseBodyWrapper.generateAesPassword。
//
// 24 个随机字节 base64 编码后恰好是 32 个字符（无 padding），
// 取其 UTF-8 字节即 32 字节 = AES-256 的密钥。这个「先随机再 base64」
// 的绕法是照抄原项目的：协议里传的是**字符串**密钥，
// 而 base64 保证它落在 ASCII 内，字节数才可控。
//
// 用 crypto/rand 而非 math/rand/v2：这是真正的密钥，需要不可预测
// （与 trace.go 里链路 id 的取舍正好相反 —— 那个只需不重复）。
// 对齐 Java 侧的 SecureRandom。
func GenerateAESPassword() (string, error) {
	buf := make([]byte, 24)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("encrypt: 生成 AES 秘钥失败: %w", err)
	}
	return base64.StdEncoding.EncodeToString(buf), nil
}

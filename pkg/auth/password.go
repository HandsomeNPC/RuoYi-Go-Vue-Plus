package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// bcryptCost bcrypt 的代价因子（对数轮数）。
//
// 10 对齐原项目：hutool 的 BCrypt.hashpw(String) 单参重载走 gensalt() 默认值，
// 即 log_rounds=10、变体 $2a$。种子数据里的哈希都是 $2a$10$ 开头。
//
// **不要为了"更安全"调高**：那不会让已有哈希更强（cost 编码在哈希串里，
// 校验时用的是串里那个值），只会让新密码与原项目的产物不一致。
// 真要调整得连同原项目一起改，且要有存量密码的重哈希方案。
const bcryptCost = 10

// ErrPasswordMismatch 密码不匹配。
//
// 与「哈希串本身损坏」区分开：前者是正常的登录失败（应计入错误次数），
// 后者是数据问题（库里存了非 bcrypt 格式的值，比如迁移时漏了加密的明文），
// 那种情况计入错误次数只会掩盖真正的原因。
var ErrPasswordMismatch = errors.New("auth: 密码不匹配")

// HashPassword 生成密码哈希，对应 hutool BCrypt.hashpw(password)。
//
// 产物形如 $2a$10$ + 22 位盐 + 31 位哈希，共 60 字符
// （sys_user.password 列宽 varchar(100)，放得下）。
//
// 每次调用盐都不同，故同一个密码两次哈希结果不同 —— 这是设计而非缺陷，
// 校验走 VerifyPassword 而不是比较哈希串。
//
// 注意 Go 的 bcrypt 生成的是 **$2a$** 前缀，与 hutool 一致，两边可互相校验。
func HashPassword(password string) (string, error) {
	// bcrypt 只取密码的前 72 字节，超长部分被静默忽略。
	// 显式拦下而不是让它静默截断：截断意味着两个不同的长密码可能等价，
	// 而调用方对此毫无察觉。原项目的入参校验限制密码 5-30 字符，
	// 正常路径碰不到这个上限。
	if len(password) > 72 {
		return "", errors.New("auth: 密码长度超过 bcrypt 上限(72 字节)")
	}
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("auth: 生成密码哈希失败: %w", err)
	}
	return string(hashed), nil
}

// VerifyPassword 校验密码，对应 hutool BCrypt.checkpw(password, hashed)。
//
// 密码不符返回 ErrPasswordMismatch，哈希串格式非法返回其他错误 ——
// 调用方（登录服务）据此决定要不要计入密码错误次数。
//
// bcrypt 的比较是**恒定时间**的（库内部用 subtle.ConstantTimeCompare），
// 不必也不该自己拼比较逻辑。
//
// 空哈希串直接判为不匹配：sys_user.password 的列默认值是空串，
// 一个没设过密码的账号不该因为传空密码就登录成功。
func VerifyPassword(password, hashed string) error {
	if hashed == "" {
		return ErrPasswordMismatch
	}
	err := bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
	if err == nil {
		return nil
	}
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return ErrPasswordMismatch
	}
	// 走到这里说明哈希串不是合法的 bcrypt 格式（长度不对、前缀不认、cost 非法）。
	// 这是数据问题不是密码问题，原样上抛让调用方打日志。
	return fmt.Errorf("auth: 密码哈希格式非法: %w", err)
}

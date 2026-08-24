package config

import (
	"time"

	"github.com/spf13/viper"
)

// User 用户相关业务配置，对应原项目 application.yml 的 user 段（:38-43）。
//
// **键路径与 Java 一致**（user.password.*），没有收进 middleware 段 ——
// 这不是中间件配置：密码重试限制作用在登录 service 里，与 HTTP 无关，
// 阶段 4 的短信/邮箱登录也会复用同一份限制。
type User struct {
	Password UserPassword `mapstructure:"password"`
}

// UserPassword 密码策略，对应 Java SysLoginService 里
// @Value("${user.password.maxRetryCount}") 与 lockTime 两项。
type UserPassword struct {
	// MaxRetryCount 密码最大错误次数，达到即锁定。对应 application.yml:41。
	MaxRetryCount int `mapstructure:"maxRetryCount"`

	// LockTime 锁定时长（分钟）。对应 application.yml:43。
	//
	// 它同时是 Redis 计数键 pwd_err_cnt:<username> 的 TTL，且**每次失败都会
	// 重置**（滑动窗口，非固定窗口）—— 即每 9 分钟错一次、错满 5 次跨越
	// 40 分钟，依然会锁。这是原项目的行为，见 SysLoginService.checkLogin。
	LockTime int `mapstructure:"lockTime"`
}

// defaultUser 返回对齐原项目 yaml 的默认配置。
func defaultUser() User {
	return User{
		Password: UserPassword{
			MaxRetryCount: 5,
			LockTime:      10,
		},
	}
}

// Lock 返回锁定时长。
//
// 与 JWT.Expire()、Redis 各 Timeout() 同样的做法：把「这个 int 的单位是分钟」
// 这件事收在一处，调用方不必各自记得乘 time.Minute（少乘一次就是锁定 10 纳秒）。
func (p UserPassword) Lock() time.Duration {
	return time.Duration(p.LockTime) * time.Minute
}

// setUserDefaults 把默认值铺给 viper，必须在读配置文件之前调用。
func setUserDefaults(v *viper.Viper) {
	d := defaultUser()
	v.SetDefault("user.password.maxRetryCount", d.Password.MaxRetryCount)
	v.SetDefault("user.password.lockTime", d.Password.LockTime)
}

// validate 校验用户配置。
//
// 两项都必须为正数。零或负数在这里不是「不限制」而是**恒定锁死**：
// MaxRetryCount<=0 时「已错次数 >= 上限」在第一次尝试就成立，
// 谁都登不进去；LockTime<=0 会让计数键的 TTL 非法。
// 想关掉重试限制不该靠填 0，那需要一个显式的开关（当前没有这个需求）。
func (u User) validate() error {
	if u.Password.MaxRetryCount <= 0 {
		return errInvalid("user.password.maxRetryCount", "必须大于 0")
	}
	if u.Password.LockTime <= 0 {
		return errInvalid("user.password.lockTime", "必须大于 0")
	}
	return nil
}

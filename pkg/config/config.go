// Package config 应用配置加载。
//
// 读取 yaml 配置文件并绑定到结构体。各子配置分文件定义：
// server.go / datasource.go / redis.go / jwt.go / middleware.go，各自实现 validate()。
//
// Load 接收多个文件路径，按传入顺序依次合并，后者覆盖前者：
//
//	config.Load("configs/application.yaml", "configs/system.yaml")
//
// 公共配置放 application.yaml，进程独有配置(端口等)放 <module>.yaml。
//
// Load 加载失败直接 panic，配置本身写入包级实例，一律用 Get() 取回 ——
// 包括 main 自己。**必须先 Load 再读 Get()**，否则 Get() 会 panic。
//
// 两者都 panic 是同一个判断：配置错误是启动期编排问题，进程本就无法工作，
// 把 error 逐层往上传只是把「立刻崩」延后成「崩在别处」。
package config

import (
	"fmt"
	"sync"

	"github.com/spf13/viper"
)

// Config 应用完整配置。
type Config struct {
	Server     Server     `mapstructure:"server"`
	Datasource Datasource `mapstructure:"datasource"`
	Redis      Redis      `mapstructure:"redis"`
	JWT        JWT        `mapstructure:"jwt"`
	Middleware Middleware `mapstructure:"middleware"`
	User       User       `mapstructure:"user"`
}

// Load 按顺序读取并合并多个 yaml 配置文件，后者覆盖前者，随后写入包级实例。
//
// 至少需要一个路径。任一文件不存在、格式错误或未通过 validate 都会 **panic** ——
// 与 Get() 同源：配置错误是启动期编排问题，进程本就无法工作，不值得让每个
// main 写一遍 if err != nil { return err }。失败时不改动包级实例。
//
// 配置不由本函数返回，加载完用 Get() 取。
func Load(paths ...string) {
	if len(paths) == 0 {
		panic(fmt.Errorf("config: 至少需要一个配置文件路径"))
	}

	v := viper.New()
	v.SetConfigType("yaml")

	// 必须在读文件之前铺默认值：viper 对缺失的键给零值，
	// 而中间件多数字段的零值是「有意义但错误」的配置（详见 setMiddlewareDefaults）。
	setMiddlewareDefaults(v)
	// user 段同理：MaxRetryCount 的零值不是「不限制」而是**恒定锁死**
	// （「已错次数 >= 0」在第一次尝试就成立），yaml 里漏写这段会让谁都登不进来。
	setUserDefaults(v)

	for i, path := range paths {
		v.SetConfigFile(path)

		// 第一个文件用 ReadInConfig 建立基准，后续用 MergeInConfig 叠加覆盖。
		read := v.MergeInConfig
		if i == 0 {
			read = v.ReadInConfig
		}
		if err := read(); err != nil {
			panic(fmt.Errorf("config: 读取 %s 失败: %w", path, err))
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		panic(fmt.Errorf("config: 解析配置失败: %w", err))
	}

	if err := cfg.validate(); err != nil {
		panic(err)
	}

	mu.Lock()
	current = &cfg
	mu.Unlock()
}

// 包级默认实例。写于 Load 成功之后，读写加锁以免竞态。
var (
	mu      sync.RWMutex
	current *Config
)

// Get 返回包级默认实例。未成功 Load 过会 panic——
// 这是启动期编排错误，不该留到运行时才发现。
//
// 这是取配置的**唯一**入口（Load 不再返回配置）。因为它会 panic，
// 只应在启动期调用（main 开头、中间件在 r.Use(...) 那一刻读一次并捕获进闭包），
// 不要放进每请求的路径。
func Get() *Config {
	mu.RLock()
	cfg := current
	mu.RUnlock()
	if cfg == nil {
		panic("config: 尚未初始化，请先调用 config.Load")
	}
	return cfg
}

// validate 依次校验各子配置。
func (c *Config) validate() error {
	validators := []func() error{
		c.Server.validate,
		c.Datasource.validate,
		c.Redis.validate,
		c.JWT.validate,
		c.Middleware.validate,
		c.User.validate,
	}
	for _, fn := range validators {
		if err := fn(); err != nil {
			return err
		}
	}
	return nil
}

// errMissing 构造必填项缺失错误。
func errMissing(key string) error {
	return fmt.Errorf("config: %s 未配置", key)
}

// errInvalid 构造取值非法错误。
func errInvalid(key, reason string) error {
	return fmt.Errorf("config: %s %s", key, reason)
}

package config

// Server 当前进程的服务配置。
type Server struct {
	Name string `mapstructure:"name"` // 模块名，如 system / auth
	Addr string `mapstructure:"addr"` // 监听地址，如 :8081
}

// validate 校验服务配置。
func (s Server) validate() error {
	if s.Addr == "" {
		return errMissing("server.addr")
	}
	return nil
}

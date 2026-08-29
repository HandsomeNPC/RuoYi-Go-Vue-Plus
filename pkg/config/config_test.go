package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"ruoyi-go-vue-plus/pkg/i18n"
)

const (
	commonYAML = "../../configs/application.yaml"
	systemYAML = "../../configs/system.yaml"
	authYAML   = "../../configs/auth.yaml"
)

func TestLoadMergesInOrder(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)

	if got, want := cfg.Server.Addr, ":8081"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
	if got, want := cfg.Server.Name, "system"; got != want {
		t.Errorf("Server.Name = %q, want %q", got, want)
	}
	if got, want := cfg.Datasource.DBName, "ry-cloud"; got != want {
		t.Errorf("Datasource.DBName = %q, want %q", got, want)
	}
	if got, want := cfg.Datasource.MaxOpenConns, 100; got != want {
		t.Errorf("Datasource.MaxOpenConns = %d, want %d", got, want)
	}
	if got, want := cfg.Redis.Port, 6379; got != want {
		t.Errorf("Redis.Port = %d, want %d", got, want)
	}
	if got, want := cfg.SAToken.TokenName, "Authorization"; got != want {
		t.Errorf("SAToken.TokenName = %q, want %q", got, want)
	}
	if got, want := cfg.SAToken.IsShare, false; got != want {
		t.Errorf("SAToken.IsShare = %v, want %v", got, want)
	}
}

func TestLoadAuthDiffersOnlyByServer(t *testing.T) {
	cfg := mustLoad(t, commonYAML, authYAML)
	if got, want := cfg.Server.Addr, ":8080"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
	if got, want := cfg.Server.Name, "auth"; got != want {
		t.Errorf("Server.Name = %q, want %q", got, want)
	}
	if got, want := cfg.Datasource.DSN(),
		"root:ruoyi123@tcp(127.0.0.1:3306)/ry-cloud?charset=utf8mb4&parseTime=True&loc=Local&timeout=5s"; got != want {
		t.Errorf("DSN() = %q, want %q", got, want)
	}
}

func TestLoadSingleFile(t *testing.T) {
	path := writeYAML(t, fullYAML)
	cfg := mustLoad(t, path)
	if got, want := cfg.Server.Addr, ":9000"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
}

func TestLaterFileOverridesEarlier(t *testing.T) {
	base := writeYAML(t, fullYAML)
	override := writeYAML(t, `
server:
  addr: ":9999"
datasource:
  host: mysql-prod
`)

	cfg := mustLoad(t, base, override)
	if got, want := cfg.Server.Addr, ":9999"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
	if got, want := cfg.Datasource.Host, "mysql-prod"; got != want {
		t.Errorf("Datasource.Host = %q, want %q", got, want)
	}
	if got, want := cfg.Datasource.DBName, "ry-vue"; got != want {
		t.Errorf("Datasource.DBName = %q, want %q", got, want)
	}
	if got, want := cfg.Server.Name, "demo"; got != want {
		t.Errorf("Server.Name = %q, want %q", got, want)
	}
}

func TestThreeFilesLastWins(t *testing.T) {
	a := writeYAML(t, fullYAML)
	b := writeYAML(t, "server:\n  addr: \":8001\"\n")
	c := writeYAML(t, "server:\n  addr: \":8002\"\n")

	cfg := mustLoad(t, a, b, c)
	if got, want := cfg.Server.Addr, ":8002"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Run("未传路径", func(t *testing.T) {
		if err := loadErr(t); err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("文件不存在", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope.yaml")
		if err := loadErr(t, missing); err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("第二个文件不存在", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope.yaml")
		if err := loadErr(t, commonYAML, missing); err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("yaml 格式非法", func(t *testing.T) {
		path := writeYAML(t, "server:\n\taddr: bad-tab\n")
		if err := loadErr(t, path); err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("配置不完整未通过校验", func(t *testing.T) {
		path := writeYAML(t, "server:\n  name: x\n  addr: \":1\"\n")
		if err := loadErr(t, path); err == nil {
			t.Fatal("want error, got nil")
		}
	})
}

func TestLoadPanicsOnError(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("残缺配置应 panic")
		}
		err, ok := r.(error)
		if !ok {
			t.Fatalf("panic 值应是 error，got %T: %v", r, r)
		}
		if !strings.Contains(err.Error(), "config:") {
			t.Errorf("panic 的错误信息应带 config: 前缀: %v", err)
		}
	}()

	Load(writeYAML(t, "server:\n  name: x\n  addr: \":1\"\n"))
}

func TestLoadSucceedsWithoutPanic(t *testing.T) {
	Load(commonYAML, systemYAML)
	if got, want := Get().Server.Addr, ":8081"; got != want {
		t.Errorf("Get().Server.Addr = %q, want %q", got, want)
	}
}

func TestServerValidate(t *testing.T) {
	if err := (ServerConfig{Name: "system", Addr: ":8081"}).validate(); err != nil {
		t.Errorf("完整配置不应报错: %v", err)
	}
	if err := (ServerConfig{Name: "system"}).validate(); err == nil {
		t.Error("缺 addr 应报错")
	}
}

func TestDatasourceValidate(t *testing.T) {
	valid := DatasourceConfig{Host: "127.0.0.1", Port: 3306, DBName: "ry-vue", MaxIdleConns: 10, MaxOpenConns: 100}
	if err := valid.validate(); err != nil {
		t.Errorf("完整配置不应报错: %v", err)
	}

	tests := map[string]func(*DatasourceConfig){
		"缺 host":       func(d *DatasourceConfig) { d.Host = "" },
		"缺 dbname":     func(d *DatasourceConfig) { d.DBName = "" },
		"port 为 0":     func(d *DatasourceConfig) { d.Port = 0 },
		"idle 大于 open": func(d *DatasourceConfig) { d.MaxIdleConns = 200 },
		"不支持的 driver":  func(d *DatasourceConfig) { d.Driver = "postgres" },
		"非法 logLevel":  func(d *DatasourceConfig) { d.LogLevel = "verbose" },
	}
	for name, breakIt := range tests {
		t.Run(name, func(t *testing.T) {
			d := valid
			breakIt(&d)
			if err := d.validate(); err == nil {
				t.Error("want error, got nil")
			}
		})
	}

	// driver 与 logLevel 留空表示用默认值，应当放行。
	t.Run("driver/logLevel 留空", func(t *testing.T) {
		d := valid
		d.Driver, d.LogLevel = "", ""
		if err := d.validate(); err != nil {
			t.Errorf("留空应放行: %v", err)
		}
	})
}

func TestRedisValidate(t *testing.T) {
	valid := RedisConfig{Host: "127.0.0.1", Port: 6379, DB: 0}
	if err := valid.validate(); err != nil {
		t.Errorf("完整配置不应报错: %v", err)
	}

	tests := map[string]func(*RedisConfig){
		"缺 host":           func(r *RedisConfig) { r.Host = "" },
		"port 为 0":         func(r *RedisConfig) { r.Port = 0 },
		"db 为负":            func(r *RedisConfig) { r.DB = -1 },
		"idle 大于 poolSize": func(r *RedisConfig) { r.PoolSize, r.MinIdleConns = 8, 32 },
	}
	for name, breakIt := range tests {
		t.Run(name, func(t *testing.T) {
			r := valid
			breakIt(&r)
			if err := r.validate(); err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}

func TestSATokenValidate(t *testing.T) {
	valid := SATokenConfig{TokenName: "Authorization", IsConcurrent: true, IsShare: false, JwtSecretKey: "test-secret"}
	if err := valid.validate(); err != nil {
		t.Errorf("完整配置不应报错: %v", err)
	}

	tests := map[string]func(*SATokenConfig){
		"缺 tokenName":  func(s *SATokenConfig) { s.TokenName = "" },
		"缺 jwt secret": func(s *SATokenConfig) { s.JwtSecretKey = "" },
	}
	for name, breakIt := range tests {
		t.Run(name, func(t *testing.T) {
			s := valid
			breakIt(&s)
			if err := s.validate(); err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}

// TestCaptchaValidate 验证码配置校验。
func TestCaptchaValidate(t *testing.T) {
	valid := CaptchaConfig{Enable: true, Type: CaptchaTypeMath, NumberLength: 1, CharLength: 4}
	if err := valid.validate(); err != nil {
		t.Errorf("完整配置不应报错: %v", err)
	}

	// 关闭时其余项怎么填都放行。
	off := CaptchaConfig{Enable: false, Type: "bogus", NumberLength: 0, CharLength: 0}
	if err := off.validate(); err != nil {
		t.Errorf("关闭时应放行: %v", err)
	}

	tests := map[string]func(*CaptchaConfig){
		"非 math/char 类型":   func(c *CaptchaConfig) { c.Type = "bogus" },
		"numberLength 为 0": func(c *CaptchaConfig) { c.NumberLength = 0 },
		"charLength 为 0":   func(c *CaptchaConfig) { c.CharLength = 0 },
	}
	for name, breakIt := range tests {
		t.Run(name, func(t *testing.T) {
			c := valid
			breakIt(&c)
			if err := c.validate(); err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}

func TestHelpers(t *testing.T) {
	d := DatasourceConfig{ConnMaxLifetime: 3600}
	if got, want := d.MaxLifetime().Hours(), 1.0; got != want {
		t.Errorf("MaxLifetime() = %v hours, want %v", got, want)
	}
	r := RedisConfig{Host: "127.0.0.1", Port: 6379}
	if got, want := r.Addr(), "127.0.0.1:6379"; got != want {
		t.Errorf("Addr() = %q, want %q", got, want)
	}
}

// fullYAML 一份能通过校验的完整配置。
const fullYAML = `
server:
  name: demo
  addr: ":9000"
datasource:
  driver: mysql
  host: 127.0.0.1
  port: 3306
  username: root
  password: root
  dbname: ry-vue
  params: charset=utf8mb4&parseTime=True&loc=Local
  maxIdleConns: 10
  maxOpenConns: 100
  connMaxLifetime: 3600
redis:
  host: 127.0.0.1
  port: 6379
  password: ""
  db: 0
satoken:
  tokenName: Authorization
  isConcurrent: true
  isShare: false
  jwtSecretKey: test-secret
`

// writeYAML 把内容写入临时 yaml 文件并返回路径。
func writeYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cfg.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("写入临时配置失败: %v", err)
	}
	return path
}

// mustLoad 加载配置并返回包级实例，失败即终止用例。
func mustLoad(t *testing.T, paths ...string) *Config {
	t.Helper()
	Load(paths...)
	return Get()
}

// loadErr 调 Load 并接住 panic，返回其中的 error。
func loadErr(t *testing.T, paths ...string) (err error) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			return
		}
		e, ok := r.(error)
		if !ok {
			panic(r)
		}
		err = e
	}()
	Load(paths...)
	return nil
}

// TestMiddlewareDefaultsMatchSetDefault 缺中间件各段时必须落到默认值。
func TestMiddlewareDefaultsMatchSetDefault(t *testing.T) {
	path := writeYAML(t, fullYAML)
	cfg := mustLoad(t, path)

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"CORS", cfg.CORS, DefaultCORS()},
		{"XSS", cfg.XSS, DefaultXSS()},
		{"AccessLog", cfg.AccessLog, DefaultAccessLog()},
		{"TraceID", cfg.TraceID, DefaultTraceID()},
		{"RepeatableBody", cfg.RepeatableBody, DefaultRepeatableBody()},
		{"I18n", cfg.I18n, DefaultI18n()},
		{"APIEncrypt", cfg.APIEncrypt, DefaultAPIEncrypt()},
		{"Captcha", cfg.Captcha, DefaultCaptcha()},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("%s 缺省时与默认值不一致\ngot  = %+v\nwant = %+v", c.name, c.got, c.want)
		}
	}
}

// TestRealYAMLMatchesDefaults 仓库 application.yaml 的中间件各段(除 apiEncrypt)应等于默认值。
func TestRealYAMLMatchesDefaults(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"CORS", cfg.CORS, DefaultCORS()},
		{"XSS", cfg.XSS, DefaultXSS()},
		{"AccessLog", cfg.AccessLog, DefaultAccessLog()},
		{"TraceID", cfg.TraceID, DefaultTraceID()},
		{"RepeatableBody", cfg.RepeatableBody, DefaultRepeatableBody()},
		{"I18n", cfg.I18n, DefaultI18n()},
		{"Captcha", cfg.Captcha, DefaultCaptcha()},
	}
	for _, c := range checks {
		if !reflect.DeepEqual(c.got, c.want) {
			t.Errorf("application.yaml 的 %s 与默认值不一致\ngot  = %+v\nwant = %+v",
				c.name, c.got, c.want)
		}
	}
}

// TestRealYAMLEnablesAPIEncrypt application.yaml 的 apiEncrypt 段启用且密钥可解析。
func TestRealYAMLEnablesAPIEncrypt(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)
	a := cfg.APIEncrypt

	if !a.Enabled {
		t.Error("apiEncrypt.enabled 应为 true（对齐原项目）")
	}
	if got, want := a.HeaderFlag, DefaultAPIEncryptHeader; got != want {
		t.Errorf("apiEncrypt.headerFlag = %q, want %q", got, want)
	}
	// 密钥格式校验由 encrypt.Init 负责（config 包不再 import encrypt，避免循环依赖）。
	if a.PrivateKey == "" {
		t.Error("apiEncrypt.privateKey 应非空")
	}
	if a.PublicKey == "" {
		t.Error("apiEncrypt.publicKey 应非空")
	}
}

// TestMiddlewareYAMLOverridesDefaults yaml 显式配置必须覆盖默认值。
func TestMiddlewareYAMLOverridesDefaults(t *testing.T) {
	path := writeYAML(t, fullYAML+`
cors:
  maxAgeSeconds: 60
  allowCredentials: false
xss:
  excludeUrls:
    - /custom/path
i18n:
  default: en-us
`)
	cfg := mustLoad(t, path)

	if got, want := cfg.CORS.MaxAgeSeconds, 60; got != want {
		t.Errorf("CORS.MaxAgeSeconds = %d, want %d", got, want)
	}
	if cfg.CORS.AllowCredentials {
		t.Error("CORS.AllowCredentials 应被 yaml 覆盖为 false")
	}
	if got, want := cfg.XSS.ExcludeURLs, []string{"/custom/path"}; !reflect.DeepEqual(got, want) {
		t.Errorf("XSS.ExcludeURLs = %v, want %v", got, want)
	}
	if got, want := cfg.I18n.Default, i18n.LocaleEnUS; got != want {
		t.Errorf("I18n.Default = %q, want %q", got, want)
	}

	if got, want := cfg.CORS.AllowedOriginPatterns, []string{"*"}; !reflect.DeepEqual(got, want) {
		t.Errorf("CORS.AllowedOriginPatterns = %v, 未在 yaml 提及应保留默认值 %v", got, want)
	}
	if got, want := cfg.RepeatableBody.MaxBodySize, DefaultRepeatableBody().MaxBodySize; got != want {
		t.Errorf("RepeatableBody.MaxBodySize = %d, 未在 yaml 提及应保留默认值 %d", got, want)
	}
}

func TestMiddlewareValidate(t *testing.T) {
	t.Run("CORS", func(t *testing.T) {
		valid := DefaultCORS()
		if err := valid.validate(); err != nil {
			t.Errorf("默认配置不应报错: %v", err)
		}
		tests := map[string]func(*CORSConfig){
			"maxAgeSeconds 为负":  func(c *CORSConfig) { c.MaxAgeSeconds = -1 },
			"allowedOrigins 为空": func(c *CORSConfig) { c.AllowedOriginPatterns = nil },
		}
		for name, breakIt := range tests {
			t.Run(name, func(t *testing.T) {
				c := valid
				breakIt(&c)
				if err := c.validate(); err == nil {
					t.Error("want error, got nil")
				}
			})
		}
	})

	t.Run("RepeatableBody", func(t *testing.T) {
		c := DefaultRepeatableBody()
		c.MaxBodySize = -1
		if err := c.validate(); err == nil {
			t.Error("maxBodySize 为负应报错")
		}
	})

	t.Run("AccessLog", func(t *testing.T) {
		a := DefaultAccessLog()
		a.MaxParamLength = -1
		if err := a.validate(); err == nil {
			t.Error("maxParamLength 为负应报错")
		}
	})

	t.Run("I18n", func(t *testing.T) {
		i := DefaultI18n()
		i.Default = "不是语言标记"
		if err := i.validate(); err == nil {
			t.Error("i18n.default 非法应报错")
		}

		t.Run("default 留空", func(t *testing.T) {
			i := DefaultI18n()
			i.Default = ""
			if err := i.validate(); err != nil {
				t.Errorf("留空应放行: %v", err)
			}
		})
	})

	t.Run("APIEncrypt", func(t *testing.T) {
		// 默认关闭，不校验密钥。
		base := DefaultAPIEncrypt()
		if err := base.validate(); err != nil {
			t.Errorf("关闭时应放行: %v", err)
		}
		// 启用但缺私钥应报错（密钥格式校验在 encrypt.Init，不在 config.validate）。
		a := DefaultAPIEncrypt()
		a.Enabled = true
		if err := a.validate(); err == nil {
			t.Error("启用但缺私钥应报错")
		}
		// 启用且配齐私钥（含/不含公钥）应放行。
		a = DefaultAPIEncrypt()
		a.Enabled = true
		a.PrivateKey = testRSAPrivateKey
		if err := a.validate(); err != nil {
			t.Errorf("启用且配齐私钥应放行: %v", err)
		}
		a.PublicKey = "any-non-empty"
		if err := a.validate(); err != nil {
			t.Errorf("配了公钥应放行（格式校验在 encrypt.Init）: %v", err)
		}
	})
}

// testRSAPrivateKey 测试用 RSA 私钥。
const testRSAPrivateKey = "MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBAO8QO5Eg4zehk9aP1SShzmlCSVHg8Ufr9yWeN4WqMMsiAPJC+PGGCoBlAD4T14Pqq7oWxc+Yrx2Nwv6eHdwUfPilfjveMO87dK977zIvdVFDSfalGBDZrTUwmzL5bBNkIFhZ/RWctEi8A1ShZCDL2/P3irtVrjh2DsDX/cgJ/7EDAgMBAAECgYEAhNZAQyRDHWZq/45soS5Hw7VRiG21pIE5k22W7G7lLfp3DCaqrYoNy8pTmCruVh7PzVdaE0CEDaf38gNqFCBOT8iTFQiYV3am4W3hsEQM5wmVBeTvCM5P2jsaaBQbqmneRjiZVbs6ha205JSho1Oc85NbaZa8gFVjwZgZWJrbzgECQQD/iZWhkRPtbdeai/Xk7D/eIXKh1Gxid0rWKQq8ikxbaiergn47XzNKrpROVyka3Gn85o7jJphgxp99R3r8sH71AkEA738Dn7xs+I4Y+MLa2EcT78JG3f/VhlWS/ks3qGJ2dfqwS7ntnmf5Q+2Xw+9UcuiK/TxD8K/0inSCkIMeWBOFFwJBAIoTebq3faEJfTqQ7ekojsokIKC4+2epNdLKknaV8/RhQ9Y0yKikJD7yXkiGaDuPZeW1Xvf2XtfL+1niSd5IMBECQDCOOMbe5dzyuj9dCg+FQZZ/dey2XK0Slm22BD/ATrIWtD12IaXXAKNz/Sv9TsrJOLykxkV69wJHIt13p+RFeNsCQGn5XGRn4ZCRVCesJYXyx29MTqkl8sD/gzYcURTZYjHqX2EvtvAyC6gBm9H0EbxmHIi4Oq0tITzklCXj5SpvBEw="

// TestGetReturnsLoadedConfig Load 成功后 Get() 应返回刚加载的那份配置。
func TestGetReturnsLoadedConfig(t *testing.T) {
	mustLoad(t, commonYAML, systemYAML)
	if got, want := Get().Server.Addr, ":8081"; got != want {
		t.Errorf("Get().Server.Addr = %q, want %q", got, want)
	}

	mustLoad(t, writeYAML(t, fullYAML))
	if got, want := Get().Server.Addr, ":9000"; got != want {
		t.Errorf("重新 Load 后 Get().Server.Addr = %q, want %q", got, want)
	}
}

// TestFailedLoadDoesNotOverwriteCurrent Load 失败不应污染包级实例。
func TestFailedLoadDoesNotOverwriteCurrent(t *testing.T) {
	mustLoad(t, commonYAML, systemYAML)
	good := Get()

	bad := writeYAML(t, "server:\n  name: x\n  addr: \":1\"\n")
	if err := loadErr(t, bad); err == nil {
		t.Fatal("残缺配置应加载失败")
	}

	if got := Get(); got != good {
		t.Error("Load 失败后 Get() 应仍返回上一次成功的配置")
	}
}

// TestCORSMaxAge CORS.MaxAgeSeconds 到 time.Duration 的换算。
func TestCORSMaxAge(t *testing.T) {
	if got, want := (CORSConfig{MaxAgeSeconds: 1800}).MaxAge().Minutes(), 30.0; got != want {
		t.Errorf("MaxAge() = %v minutes, want %v", got, want)
	}
}

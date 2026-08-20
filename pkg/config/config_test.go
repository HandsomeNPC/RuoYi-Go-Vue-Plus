package config

import (
	"os"
	"path/filepath"
	"testing"
)

// 仓库内真实配置文件，相对 pkg/config 的路径。
const (
	commonYAML = "../../configs/application.yaml"
	systemYAML = "../../configs/system.yaml"
	authYAML   = "../../configs/auth.yaml"
)

func TestLoadMergesInOrder(t *testing.T) {
	cfg, err := Load(commonYAML, systemYAML)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// 来自 system.yaml
	if got, want := cfg.Server.Addr, ":8081"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
	if got, want := cfg.Server.Name, "system"; got != want {
		t.Errorf("Server.Name = %q, want %q", got, want)
	}
	// 来自 application.yaml
	if got, want := cfg.Datasource.DBName, "ry-cloud"; got != want {
		t.Errorf("Datasource.DBName = %q, want %q", got, want)
	}
	if got, want := cfg.Datasource.MaxOpenConns, 100; got != want {
		t.Errorf("Datasource.MaxOpenConns = %d, want %d", got, want)
	}
	if got, want := cfg.Redis.Port, 6379; got != want {
		t.Errorf("Redis.Port = %d, want %d", got, want)
	}
	if got, want := cfg.JWT.ExpireMinutes, 720; got != want {
		t.Errorf("JWT.ExpireMinutes = %d, want %d", got, want)
	}
}

// auth 与 system 共用公共配置，只有 server 段不同。
func TestLoadAuthDiffersOnlyByServer(t *testing.T) {
	cfg, err := Load(commonYAML, authYAML)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
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

// 单文件加载：只要内容完整就该通过。
func TestLoadSingleFile(t *testing.T) {
	path := writeYAML(t, fullYAML)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.Server.Addr, ":9000"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
}

// 后传入的文件覆盖先传入的，未提及的键保持不变。
func TestLaterFileOverridesEarlier(t *testing.T) {
	base := writeYAML(t, fullYAML)
	override := writeYAML(t, `
server:
  addr: ":9999"
datasource:
  host: mysql-prod
`)

	cfg, err := Load(base, override)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.Server.Addr, ":9999"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
	if got, want := cfg.Datasource.Host, "mysql-prod"; got != want {
		t.Errorf("Datasource.Host = %q, want %q", got, want)
	}
	// 未被覆盖的键保留 base 的值
	if got, want := cfg.Datasource.DBName, "ry-vue"; got != want {
		t.Errorf("Datasource.DBName = %q, want %q", got, want)
	}
	if got, want := cfg.Server.Name, "demo"; got != want {
		t.Errorf("Server.Name = %q, want %q", got, want)
	}
}

// 三个文件按顺序叠加，最后一个胜出。
func TestThreeFilesLastWins(t *testing.T) {
	a := writeYAML(t, fullYAML)
	b := writeYAML(t, "server:\n  addr: \":8001\"\n")
	c := writeYAML(t, "server:\n  addr: \":8002\"\n")

	cfg, err := Load(a, b, c)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := cfg.Server.Addr, ":8002"; got != want {
		t.Errorf("Server.Addr = %q, want %q", got, want)
	}
}

func TestLoadErrors(t *testing.T) {
	t.Run("未传路径", func(t *testing.T) {
		if _, err := Load(); err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("文件不存在", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope.yaml")
		if _, err := Load(missing); err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("第二个文件不存在", func(t *testing.T) {
		missing := filepath.Join(t.TempDir(), "nope.yaml")
		if _, err := Load(commonYAML, missing); err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("yaml 格式非法", func(t *testing.T) {
		path := writeYAML(t, "server:\n\taddr: bad-tab\n")
		if _, err := Load(path); err == nil {
			t.Fatal("want error, got nil")
		}
	})

	t.Run("配置不完整未通过校验", func(t *testing.T) {
		// 只有 server 段，缺 datasource/redis/jwt
		path := writeYAML(t, "server:\n  name: x\n  addr: \":1\"\n")
		if _, err := Load(path); err == nil {
			t.Fatal("want error, got nil")
		}
	})
}

func TestServerValidate(t *testing.T) {
	if err := (Server{Name: "system", Addr: ":8081"}).validate(); err != nil {
		t.Errorf("完整配置不应报错: %v", err)
	}
	if err := (Server{Name: "system"}).validate(); err == nil {
		t.Error("缺 addr 应报错")
	}
}

func TestDatasourceValidate(t *testing.T) {
	valid := Datasource{Host: "127.0.0.1", Port: 3306, DBName: "ry-vue", MaxIdleConns: 10, MaxOpenConns: 100}
	if err := valid.validate(); err != nil {
		t.Errorf("完整配置不应报错: %v", err)
	}

	tests := map[string]func(*Datasource){
		"缺 host":       func(d *Datasource) { d.Host = "" },
		"缺 dbname":     func(d *Datasource) { d.DBName = "" },
		"port 为 0":     func(d *Datasource) { d.Port = 0 },
		"idle 大于 open": func(d *Datasource) { d.MaxIdleConns = 200 },
		"不支持的 driver":  func(d *Datasource) { d.Driver = "postgres" },
		"非法 logLevel":  func(d *Datasource) { d.LogLevel = "verbose" },
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
	valid := Redis{Host: "127.0.0.1", Port: 6379, DB: 0}
	if err := valid.validate(); err != nil {
		t.Errorf("完整配置不应报错: %v", err)
	}

	tests := map[string]func(*Redis){
		"缺 host":           func(r *Redis) { r.Host = "" },
		"port 为 0":         func(r *Redis) { r.Port = 0 },
		"db 为负":            func(r *Redis) { r.DB = -1 },
		"idle 大于 poolSize": func(r *Redis) { r.PoolSize, r.MinIdleConns = 8, 32 },
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

func TestJWTValidate(t *testing.T) {
	valid := JWT{Secret: "s", ExpireMinutes: 720, Header: "Authorization"}
	if err := valid.validate(); err != nil {
		t.Errorf("完整配置不应报错: %v", err)
	}

	tests := map[string]func(*JWT){
		"缺 secret":         func(j *JWT) { j.Secret = "" },
		"expireMinutes 0":  func(j *JWT) { j.ExpireMinutes = 0 },
		"expireMinutes 为负": func(j *JWT) { j.ExpireMinutes = -1 },
	}
	for name, breakIt := range tests {
		t.Run(name, func(t *testing.T) {
			j := valid
			breakIt(&j)
			if err := j.validate(); err == nil {
				t.Error("want error, got nil")
			}
		})
	}
}

func TestHelpers(t *testing.T) {
	d := Datasource{ConnMaxLifetime: 3600}
	if got, want := d.MaxLifetime().Hours(), 1.0; got != want {
		t.Errorf("MaxLifetime() = %v hours, want %v", got, want)
	}
	j := JWT{ExpireMinutes: 720}
	if got, want := j.Expire().Hours(), 12.0; got != want {
		t.Errorf("Expire() = %v hours, want %v", got, want)
	}
	r := Redis{Host: "127.0.0.1", Port: 6379}
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
jwt:
  secret: test-secret
  expireMinutes: 720
  header: Authorization
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

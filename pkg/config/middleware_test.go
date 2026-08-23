package config

import (
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/pkg/i18n"
)

// 缺 middleware 段时必须落到默认值，而不是 viper 给的零值。
//
// 这条锁住的是 setMiddlewareDefaults 与 DefaultMiddleware 的一致性：
// 前者少铺一项，该项就会静默变成零值 —— 而零值多数是**有意义但错误**的配置，
// CORS.AllowedOriginPatterns 为空意味着「拒绝所有来源」，跨域会全挂。
func TestMiddlewareDefaultsMatchSetDefault(t *testing.T) {
	// fullYAML 只有 server/datasource/redis/jwt，没有 middleware 段。
	path := writeYAML(t, fullYAML)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.Middleware, DefaultMiddleware(); !reflect.DeepEqual(got, want) {
		t.Errorf("缺 middleware 段时的配置与 DefaultMiddleware() 不一致\ngot  = %+v\nwant = %+v",
			got, want)
	}
}

// 仓库真实 application.yaml 里的 middleware 段写的是默认值的显式重述，
// 抄错一个数就该被发现。
//
// 与上一条的区别：那条测「没写会怎样」，这条测「写了的是不是真等于默认值」。
// 两条都过才能说明 yaml 是安全的文档 —— 读者照着改能预期行为，
// 而删掉整段也不会改行为。
//
// 注意 DeepEqual 区分 nil 与空切片：yaml 里写 `skipPaths: []` 会得到非 nil 的
// 空切片，与默认值 nil 不相等（虽然行为一致）。所以那种「默认为空」的项在
// yaml 里应注释掉而非写成空列表 —— 本用例会替你发现这件事。
func TestRealYAMLMatchesDefaults(t *testing.T) {
	cfg, err := Load(commonYAML, systemYAML)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if got, want := cfg.Middleware, DefaultMiddleware(); !reflect.DeepEqual(got, want) {
		t.Errorf("application.yaml 的 middleware 段与 DefaultMiddleware() 不一致\ngot  = %+v\nwant = %+v",
			got, want)
	}
}

// yaml 里显式配的值必须真的覆盖默认值。
//
// 没有这条的话，前两条测试在「SetDefault 铺了值但 Unmarshal 根本没读文件」
// 这种坏法下也会通过 —— 那时配置文件完全是装饰品。
func TestMiddlewareYAMLOverridesDefaults(t *testing.T) {
	path := writeYAML(t, fullYAML+`
middleware:
  cors:
    maxAgeSeconds: 60
    allowCredentials: false
  xss:
    excludeUrls:
      - /custom/path
  i18n:
    default: en-us
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	m := cfg.Middleware
	if got, want := m.CORS.MaxAgeSeconds, 60; got != want {
		t.Errorf("CORS.MaxAgeSeconds = %d, want %d", got, want)
	}
	if m.CORS.AllowCredentials {
		t.Error("CORS.AllowCredentials 应被 yaml 覆盖为 false")
	}
	if got, want := m.XSS.ExcludeURLs, []string{"/custom/path"}; !reflect.DeepEqual(got, want) {
		t.Errorf("XSS.ExcludeURLs = %v, want %v", got, want)
	}
	if got, want := m.I18n.Default, i18n.LocaleEnUS; got != want {
		t.Errorf("I18n.Default = %q, want %q", got, want)
	}

	// 同一段里没提到的键仍应保留默认值，而不是被整段替换成零值。
	if got, want := m.CORS.AllowedOriginPatterns, []string{"*"}; !reflect.DeepEqual(got, want) {
		t.Errorf("CORS.AllowedOriginPatterns = %v, 未在 yaml 提及应保留默认值 %v", got, want)
	}
	if got, want := m.RepeatableBody.MaxBodySize, DefaultMiddleware().RepeatableBody.MaxBodySize; got != want {
		t.Errorf("RepeatableBody.MaxBodySize = %d, 未在 yaml 提及应保留默认值 %d", got, want)
	}
}

func TestMiddlewareValidate(t *testing.T) {
	valid := DefaultMiddleware()
	if err := valid.validate(); err != nil {
		t.Errorf("默认配置不应报错: %v", err)
	}

	tests := map[string]func(*Middleware){
		"maxAgeSeconds 为负":  func(m *Middleware) { m.CORS.MaxAgeSeconds = -1 },
		"allowedOrigins 为空": func(m *Middleware) { m.CORS.AllowedOriginPatterns = nil },
		"maxBodySize 为负":    func(m *Middleware) { m.RepeatableBody.MaxBodySize = -1 },
		"maxParamLength 为负": func(m *Middleware) { m.AccessLog.MaxParamLength = -1 },
		"i18n.default 非法":   func(m *Middleware) { m.I18n.Default = "不是语言标记" },
	}
	for name, breakIt := range tests {
		t.Run(name, func(t *testing.T) {
			m := valid
			breakIt(&m)
			if err := m.validate(); err == nil {
				t.Error("want error, got nil")
			}
		})
	}

	// i18n.default 留空表示用 i18n.DefaultLocale，应当放行。
	t.Run("i18n.default 留空", func(t *testing.T) {
		m := valid
		m.I18n.Default = ""
		if err := m.validate(); err != nil {
			t.Errorf("留空应放行: %v", err)
		}
	})
}

// Load 成功后 Get() 应返回同一份配置。
func TestGetReturnsLoadedConfig(t *testing.T) {
	cfg, err := Load(commonYAML, systemYAML)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got := Get(); got != cfg {
		t.Errorf("Get() = %p, want %p（应是 Load 返回的同一份）", got, cfg)
	}
}

// Load 失败不应污染包级实例 —— 半成品配置比没有配置更难查。
func TestFailedLoadDoesNotOverwriteCurrent(t *testing.T) {
	good, err := Load(commonYAML, systemYAML)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	// 只有 server 段，过不了 datasource/redis/jwt 的校验。
	bad := writeYAML(t, "server:\n  name: x\n  addr: \":1\"\n")
	if _, err := Load(bad); err == nil {
		t.Fatal("残缺配置应加载失败")
	}

	if got := Get(); got != good {
		t.Error("Load 失败后 Get() 应仍返回上一次成功的配置")
	}
}

// CORS.MaxAgeSeconds 到 time.Duration 的换算。
func TestCORSMaxAge(t *testing.T) {
	if got, want := (CORS{MaxAgeSeconds: 1800}).MaxAge().Minutes(), 30.0; got != want {
		t.Errorf("MaxAge() = %v minutes, want %v", got, want)
	}
}

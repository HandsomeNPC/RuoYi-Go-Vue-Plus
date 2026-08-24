package config

import (
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/pkg/encrypt"
	"ruoyi-go-vue-plus/pkg/i18n"
)

// TestMiddlewareDefaultsMatchSetDefault 缺 middleware 段时必须落到默认值。
func TestMiddlewareDefaultsMatchSetDefault(t *testing.T) {
	path := writeYAML(t, fullYAML)
	cfg := mustLoad(t, path)

	if got, want := cfg.Middleware, DefaultMiddleware(); !reflect.DeepEqual(got, want) {
		t.Errorf("缺 middleware 段时的配置与 DefaultMiddleware() 不一致\ngot  = %+v\nwant = %+v",
			got, want)
	}
}

// TestRealYAMLMatchesDefaults 仓库 application.yaml 的 middleware 段应等于默认值。
func TestRealYAMLMatchesDefaults(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)

	got, want := cfg.Middleware, DefaultMiddleware()
	got.APIEncrypt, want.APIEncrypt = APIEncrypt{}, APIEncrypt{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("application.yaml 的 middleware 段与 DefaultMiddleware() 不一致\ngot  = %+v\nwant = %+v",
			got, want)
	}
}

// TestRealYAMLEnablesAPIEncrypt application.yaml 的 apiEncrypt 段启用且密钥可解析。
func TestRealYAMLEnablesAPIEncrypt(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)
	a := cfg.Middleware.APIEncrypt

	if !a.Enabled {
		t.Error("apiEncrypt.enabled 应为 true（对齐原项目）")
	}
	if got, want := a.HeaderFlag, DefaultAPIEncryptHeader; got != want {
		t.Errorf("apiEncrypt.headerFlag = %q, want %q", got, want)
	}

	want := []string{
		"/auth/login",
		"/auth/register",
		"/system/user/resetPwd",
		"/system/user/profile/updatePwd",
	}
	if !reflect.DeepEqual(a.RequestURLs, want) {
		t.Errorf("apiEncrypt.requestUrls = %v, want %v", a.RequestURLs, want)
	}

	if len(a.ResponseURLs) != 0 {
		t.Errorf("apiEncrypt.responseUrls = %v, 原项目从未启用响应加密，应为空", a.ResponseURLs)
	}

	if _, err := encrypt.ParseRSAPrivateKey(a.PrivateKey); err != nil {
		t.Errorf("apiEncrypt.privateKey 解析失败: %v", err)
	}
	if _, err := encrypt.ParseRSAPublicKey(a.PublicKey); err != nil {
		t.Errorf("apiEncrypt.publicKey 解析失败: %v", err)
	}
}

// TestMiddlewareYAMLOverridesDefaults yaml 显式配置必须覆盖默认值。
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
	cfg := mustLoad(t, path)

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
		"apiEncrypt 启用但缺私钥": func(m *Middleware) {
			m.APIEncrypt.Enabled = true
		},
		"apiEncrypt 私钥非法": func(m *Middleware) {
			m.APIEncrypt.Enabled = true
			m.APIEncrypt.PrivateKey = "not-base64!!"
		},
		"apiEncrypt 私钥非 PKCS8": func(m *Middleware) {
			m.APIEncrypt.Enabled = true
			m.APIEncrypt.PrivateKey = "aGVsbG8gd29ybGQ="
		},
		"apiEncrypt 响应加密缺公钥": func(m *Middleware) {
			m.APIEncrypt.Enabled = true
			m.APIEncrypt.PrivateKey = testRSAPrivateKey
			m.APIEncrypt.ResponseURLs = []string{"/auth/login"}
		},
		"apiEncrypt.maxBodySize 为负": func(m *Middleware) { m.APIEncrypt.MaxBodySize = -1 },
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

	t.Run("i18n.default 留空", func(t *testing.T) {
		m := valid
		m.I18n.Default = ""
		if err := m.validate(); err != nil {
			t.Errorf("留空应放行: %v", err)
		}
	})

	t.Run("apiEncrypt 关闭时不校验密钥", func(t *testing.T) {
		m := valid
		m.APIEncrypt.Enabled = false
		m.APIEncrypt.PrivateKey = ""
		m.APIEncrypt.PublicKey = ""
		if err := m.validate(); err != nil {
			t.Errorf("关闭时应放行: %v", err)
		}
	})

	t.Run("apiEncrypt 启用但不做响应加密时不强求公钥", func(t *testing.T) {
		m := valid
		m.APIEncrypt.Enabled = true
		m.APIEncrypt.PrivateKey = testRSAPrivateKey
		m.APIEncrypt.PublicKey = ""
		if err := m.validate(); err != nil {
			t.Errorf("未配 responseUrls 时不应要求公钥: %v", err)
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
	if got, want := (CORS{MaxAgeSeconds: 1800}).MaxAge().Minutes(), 30.0; got != want {
		t.Errorf("MaxAge() = %v minutes, want %v", got, want)
	}
}

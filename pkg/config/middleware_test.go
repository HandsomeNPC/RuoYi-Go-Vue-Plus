package config

import (
	"reflect"
	"testing"

	"ruoyi-go-vue-plus/pkg/encrypt"
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
	cfg := mustLoad(t, path)

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
//
// **APIEncrypt 被排除在外**，它是唯一一处 yaml 与代码默认值有意不同的配置：
// yaml 里 enabled=true（对齐原项目）而默认值是 false（否则未配置的进程会因
// 缺私钥而启动失败）。那段的内容由 TestRealYAMLEnablesAPIEncrypt 单独锁。
func TestRealYAMLMatchesDefaults(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)

	got, want := cfg.Middleware, DefaultMiddleware()
	// 置成同一个值再比：这样将来新增的中间件配置仍会被本用例覆盖，
	// 不必逐字段罗列（漏写一个字段就等于漏掉一项保护）。
	got.APIEncrypt, want.APIEncrypt = APIEncrypt{}, APIEncrypt{}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("application.yaml 的 middleware 段与 DefaultMiddleware() 不一致\ngot  = %+v\nwant = %+v",
			got, want)
	}
}

// application.yaml 的 apiEncrypt 段：启用、且密钥能真的解析出来。
//
// 单独一条是因为它是全段唯一与代码默认值有意不同的地方（见上一条的说明），
// 而这个差异必须是「有意的」而非「抄漏的」—— 本用例就是那份意图的记录。
//
// 密钥解析也一并验：那对密钥是从原项目 yaml 手抄过来的 base64 长串，
// 中间断一个字符不会有任何编译期症状，只会让所有加密接口在运行期解密失败。
func TestRealYAMLEnablesAPIEncrypt(t *testing.T) {
	cfg := mustLoad(t, commonYAML, systemYAML)
	a := cfg.Middleware.APIEncrypt

	// 对齐原项目 application.yml:150 的 api-decrypt.enabled: true。
	if !a.Enabled {
		t.Error("apiEncrypt.enabled 应为 true（对齐原项目）")
	}
	if got, want := a.HeaderFlag, DefaultAPIEncryptHeader; got != want {
		t.Errorf("apiEncrypt.headerFlag = %q, want %q", got, want)
	}

	// 四条强制加密路径对应原项目 4 处 @ApiEncrypt，少一条就意味着
	// 那个接口能收明文密码而不被拒。
	want := []string{
		"/auth/login",
		"/auth/register",
		"/system/user/resetPwd",
		"/system/user/profile/updatePwd",
	}
	if !reflect.DeepEqual(a.RequestURLs, want) {
		t.Errorf("apiEncrypt.requestUrls = %v, want %v", a.RequestURLs, want)
	}

	// 原项目 4 处 @ApiEncrypt 全是默认的 response=false，响应加密从未启用。
	if len(a.ResponseURLs) != 0 {
		t.Errorf("apiEncrypt.responseUrls = %v, 原项目从未启用响应加密，应为空", a.ResponseURLs)
	}

	// 私钥必须真能解析 —— validate 已经查过一遍，这里是把「配置文件里那串
	// base64 是完整的」这件事本身记录成用例。
	if _, err := encrypt.ParseRSAPrivateKey(a.PrivateKey); err != nil {
		t.Errorf("apiEncrypt.privateKey 解析失败: %v", err)
	}
	// 公钥当前用不到（没配 responseUrls），但配了就得是对的，
	// 否则将来开启响应加密时才发现抄错。
	if _, err := encrypt.ParseRSAPublicKey(a.PublicKey); err != nil {
		t.Errorf("apiEncrypt.publicKey 解析失败: %v", err)
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
		// 启用加解密但没有私钥 —— 每个加密请求都会失败，必须在启动期拦住。
		"apiEncrypt 启用但缺私钥": func(m *Middleware) {
			m.APIEncrypt.Enabled = true
		},
		"apiEncrypt 私钥非法": func(m *Middleware) {
			m.APIEncrypt.Enabled = true
			m.APIEncrypt.PrivateKey = "not-base64!!"
		},
		// 私钥能 base64 解开但不是合法的 PKCS#8 DER。
		"apiEncrypt 私钥非 PKCS8": func(m *Middleware) {
			m.APIEncrypt.Enabled = true
			m.APIEncrypt.PrivateKey = "aGVsbG8gd29ybGQ="
		},
		// 配了 responseUrls 却没有公钥，响应加密无从进行。
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

	// i18n.default 留空表示用 i18n.DefaultLocale，应当放行。
	t.Run("i18n.default 留空", func(t *testing.T) {
		m := valid
		m.I18n.Default = ""
		if err := m.validate(); err != nil {
			t.Errorf("留空应放行: %v", err)
		}
	})

	// 关闭时密钥留空是正常形态，不该逼着不用这个功能的部署去填一对无用密钥。
	t.Run("apiEncrypt 关闭时不校验密钥", func(t *testing.T) {
		m := valid
		m.APIEncrypt.Enabled = false
		m.APIEncrypt.PrivateKey = ""
		m.APIEncrypt.PublicKey = ""
		if err := m.validate(); err != nil {
			t.Errorf("关闭时应放行: %v", err)
		}
	})

	// 启用 + 有效私钥 + 不做响应加密：这是本项目的常态（原项目从未启用响应
	// 加密），此时不该强求公钥。
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

// testRSAPrivateKey 原项目 application.yml 里那把 1024 位开发私钥。
//
// 直接引用仓库配置文件里的同一个值本可以避免重复，但那会让本用例依赖
// application.yaml 的内容 —— 校验逻辑的测试不该在有人改配置时跟着红。
const testRSAPrivateKey = "MIICdwIBADANBgkqhkiG9w0BAQEFAASCAmEwggJdAgEAAoGBAO8QO5Eg4zehk9aP1SShzmlCSVHg8Ufr9yWeN4WqMMsiAPJC+PGGCoBlAD4T14Pqq7oWxc+Yrx2Nwv6eHdwUfPilfjveMO87dK977zIvdVFDSfalGBDZrTUwmzL5bBNkIFhZ/RWctEi8A1ShZCDL2/P3irtVrjh2DsDX/cgJ/7EDAgMBAAECgYEAhNZAQyRDHWZq/45soS5Hw7VRiG21pIE5k22W7G7lLfp3DCaqrYoNy8pTmCruVh7PzVdaE0CEDaf38gNqFCBOT8iTFQiYV3am4W3hsEQM5wmVBeTvCM5P2jsaaBQbqmneRjiZVbs6ha205JSho1Oc85NbaZa8gFVjwZgZWJrbzgECQQD/iZWhkRPtbdeai/Xk7D/eIXKh1Gxid0rWKQq8ikxbaiergn47XzNKrpROVyka3Gn85o7jJphgxp99R3r8sH71AkEA738Dn7xs+I4Y+MLa2EcT78JG3f/VhlWS/ks3qGJ2dfqwS7ntnmf5Q+2Xw+9UcuiK/TxD8K/0inSCkIMeWBOFFwJBAIoTebq3faEJfTqQ7ekojsokIKC4+2epNdLKknaV8/RhQ9Y0yKikJD7yXkiGaDuPZeW1Xvf2XtfL+1niSd5IMBECQDCOOMbe5dzyuj9dCg+FQZZ/dey2XK0Slm22BD/ATrIWtD12IaXXAKNz/Sv9TsrJOLykxkV69wJHIt13p+RFeNsCQGn5XGRn4ZCRVCesJYXyx29MTqkl8sD/gzYcURTZYjHqX2EvtvAyC6gBm9H0EbxmHIi4Oq0tITzklCXj5SpvBEw="

// Load 成功后 Get() 应返回刚加载的那份配置。
//
// Load 不返回配置，所以这里只能比内容：换一份 addr 明显不同的配置再 Load，
// Get() 必须跟着变 —— 锁住的是「Get() 反映最后一次成功 Load」。
func TestGetReturnsLoadedConfig(t *testing.T) {
	if err := Load(commonYAML, systemYAML); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := Get().Server.Addr, ":8081"; got != want {
		t.Errorf("Get().Server.Addr = %q, want %q", got, want)
	}

	// 再 Load 一份不同的，Get() 应指向新的那份。
	if err := Load(writeYAML(t, fullYAML)); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got, want := Get().Server.Addr, ":9000"; got != want {
		t.Errorf("重新 Load 后 Get().Server.Addr = %q, want %q", got, want)
	}
}

// Load 失败不应污染包级实例 —— 半成品配置比没有配置更难查。
func TestFailedLoadDoesNotOverwriteCurrent(t *testing.T) {
	if err := Load(commonYAML, systemYAML); err != nil {
		t.Fatalf("Load: %v", err)
	}
	good := Get()

	// 只有 server 段，过不了 datasource/redis/jwt 的校验。
	bad := writeYAML(t, "server:\n  name: x\n  addr: \":1\"\n")
	if err := Load(bad); err == nil {
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

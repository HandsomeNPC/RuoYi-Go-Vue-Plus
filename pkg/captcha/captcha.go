// Package captcha 图形验证码的生成，对照 Java CaptchaController.getCodeImpl。
//
// 初始化对照 redis.Init / encrypt.Init：captcha.Init() 无参，自读 config.Get().Captcha，
// 构造驱动并设包级全局；业务侧用包级 captcha.Generate() / captcha.Enabled()。
//
// **只管出题、画图、把答案写进 Redis**。校验(取值→删除→判空→比对)不在本包，而在
// internal/auth/service 各认证策略的 validateCaptcha 里——校验失败要记登录失败日志，
// 那要调 internal/system 的 service，而 pkg 不能 import internal/。这与 Java 一致：
// Java 的校验也写在 PasswordAuthStrategy 而非验证码组件里。答案的 Redis 键由
// constant.CaptchaCodeKey 约定，两侧共用。
//
// 底层图形绘制用 base64Captcha 的 Driver（只用 DrawCaptcha 画图），
// 不用它的 Captcha/Store —— Store 接口无 TTL 也无 ctx，塞不进
// "global:captcha_codes: 前缀 + 2 分钟过期" 的约定，Redis 写入在本包自己做。
package captcha

import (
	"context"
	"fmt"
	"log"
	"math/rand/v2"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/mojocn/base64Captcha"
	goredis "github.com/redis/go-redis/v9"

	"ruoyi-go-vue-plus/pkg/config"
	"ruoyi-go-vue-plus/pkg/constant"
	"ruoyi-go-vue-plus/pkg/redis"
)

// 图片渲染参数，对照 Java `new WaveAndCircleCaptcha(160, 60)` 与 Arial BOLD 45，
// 按 config.CaptchaConfig 的设计固定写死，不做配置项。
const (
	imgWidth  = 160 // 图片宽度(像素)
	imgHeight = 60  // 图片高度(像素)

	// noiseCount 背景干扰字符数。
	noiseCount = 0
	// showLineOptions 干扰线：只开细直线。
	// 库里另两种(空心线/正弦线)会把字符压得看不清——base64Captcha 源码自己都注了
	// "波浪线 比较丑"，实测 hollow|sine 组合基本不可读，故只保留细直线。
	showLineOptions = base64Captcha.OptionShowSlimeLine
)

// charSource 字符验证码取值集合，去掉了易混淆的 0/O/1/l/I。
const charSource = "234567890abcdefghjkmnpqrstuvwxyzABCDEFGHJKMNPQRSTUVWXYZ"

// expiration 验证码有效期，对照 Java Duration.ofMinutes(Constants.CAPTCHA_EXPIRATION)。
// 常量本身是裸 int(分钟)，此处补上单位。
const expiration = time.Duration(constant.ConstantCaptchaExpiration) * time.Minute

// Vo 图形验证码响应对象，对照 Java CaptchaController.CaptchaVo。
// 字段名与前端约定一致：login.vue 读 data.captchaEnabled / data.uuid / data.img。
type Vo struct {
	CaptchaEnabled bool   `json:"captchaEnabled"`
	UUID           string `json:"uuid"`
	// Img 为**裸 base64**，不含 "data:image/png;base64," 前缀。
	// 前端自己拼前缀(login.vue: 'data:image/gif;base64,' + data.img)，
	// 这里带前缀会拼成两截导致图裂。
	Img string `json:"img"`
}

// Captcha 验证码器，持有解析后的绘图驱动。
type Captcha struct {
	enabled bool
	// typ 验证码类型：config.CaptchaTypeMath / CaptchaTypeChar。
	typ string
	// numberLength 算术验证码每个操作数的位数。
	numberLength int
	// driver 绘图驱动，!enabled 时为 nil。
	driver base64Captcha.Driver
	// rdb 为 nil 时取包级 redis.Client()；仅测试注入独立客户端时才置值。
	rdb *goredis.Client
}

// client 返回本实例使用的 Redis 客户端。
func (c *Captcha) client() *goredis.Client {
	if c.rdb != nil {
		return c.rdb
	}
	return redis.Client()
}

// New 按配置构造 Captcha。!Enable 时返回 no-op（Generate 只回开关位，
// Validate 直接放行）。对照 encrypt.New。
func New(cfg config.CaptchaConfig) (*Captcha, error) {
	if !cfg.Enable {
		return &Captcha{}, nil
	}

	c := &Captcha{
		enabled:      true,
		typ:          cfg.Type,
		numberLength: cfg.NumberLength,
	}
	switch cfg.Type {
	case config.CaptchaTypeMath:
		// DriverMath 的取值范围写死在库里(加法 0-20 等)，不认 numberLength，
		// 故只借它画图，题面与答案由 nextMath 自行生成。
		c.driver = base64Captcha.NewDriverMath(
			imgHeight, imgWidth, noiseCount, showLineOptions, nil, nil, nil)
	case config.CaptchaTypeChar:
		c.driver = base64Captcha.NewDriverString(
			imgHeight, imgWidth, noiseCount, showLineOptions,
			cfg.CharLength, charSource, nil, nil, nil)
	default:
		return nil, fmt.Errorf("captcha: 未知验证码类型 %q", cfg.Type)
	}
	return c, nil
}

// 包级默认实例（对照 redis.defaultClient / encrypt.defaultCrypto）。
var (
	mu             sync.RWMutex
	defaultCaptcha *Captcha
)

// Init 按 config.Get().Captcha 构造并设包级默认实例。对照 redis.Init。
// 必须在 config.Load 之后调用；因 Generate/Validate 要读写 Redis，
// 还须在 redis.Init 之后调用。构造失败直接 panic（启动期 fail-fast）。
func Init() {
	c := config.Get()
	cfg := c.Captcha
	instance, err := New(cfg)
	if err != nil {
		panic(fmt.Errorf("captcha: 初始化失败: %w", err))
	}
	mu.Lock()
	defaultCaptcha = instance
	mu.Unlock()
	log.Printf("[%s] captcha 已就绪: enable=%t type=%s", c.Server.Name, cfg.Enable, cfg.Type)
}

// get 返回包级默认实例，未调用 Init 会 panic。对照 encrypt.getCrypto。
func get() *Captcha {
	mu.RLock()
	c := defaultCaptcha
	mu.RUnlock()
	if c == nil {
		panic("captcha: 尚未初始化，请先调用 captcha.Init")
	}
	return c
}

// Generate 生成验证码：出题、画图、把**答案**写入 Redis，返回图与 uuid。
// 对照 Java CaptchaController.getCodeImpl。
//
// TODO: 对照 Java @RateLimiter(time=60, count=10, limitType=IP) 补 IP 限流；
// Java 刻意把 getCode/getCodeImpl 拆两层，使开关关闭时不触发限流，
// 此处 !enabled 提前返回已等效。
func Generate(ctx context.Context) (*Vo, error) { return get().Generate(ctx) }

// Generate 见包级 Generate。
func (c *Captcha) Generate(ctx context.Context) (*Vo, error) {
	// 未启用：只回开关位，不出题不画图，对照 Java new CaptchaVo(false, null, null)。
	if !c.enabled {
		return &Vo{CaptchaEnabled: false}, nil
	}

	question, answer := c.next()
	item, err := c.driver.DrawCaptcha(question)
	if err != nil {
		return nil, fmt.Errorf("captcha: 绘制失败: %w", err)
	}

	// 对照 Java IdUtil.simpleUUID()：无连字符。
	id := strings.ReplaceAll(uuid.NewString(), "-", "")
	// 存答案而非题面：算术题图上是 "3+5=?"，Redis 里是 "8"。
	if err := c.client().Set(ctx, key(id), answer, expiration).Err(); err != nil {
		return nil, fmt.Errorf("captcha: 写入 redis 失败: %w", err)
	}

	return &Vo{CaptchaEnabled: true, UUID: id, Img: stripB64Prefix(item.EncodeB64string())}, nil
}

// Enabled 返回验证码校验是否启用，对照 Java CaptchaProperties.getEnable()。
// 校验逻辑在调用方（internal/auth 各认证策略的 validateCaptcha），本包只出题画图，
// 故开关也交由调用方判断，对照 Java `if (captchaEnabled) { validateCaptcha(...) }`。
func Enabled() bool { return get().enabled }

// next 出题，返回画在图上的题面与用于比对的答案。
// 字符验证码题面即答案；算术验证码题面是表达式、答案是计算结果，
// 对照 Java 用 SpEL 求值后只缓存结果。
func (c *Captcha) next() (question, answer string) {
	if c.typ == config.CaptchaTypeMath {
		return nextMath(c.numberLength)
	}
	// 返回值依次是 (id, 题面, 答案)；id 由本包用 uuid 另行生成，此处丢弃。
	_, q, a := c.driver.GenerateIdQuestionAnswer()
	return q, a
}

// nextMath 生成算术题，操作数为 numberLength 位。
// 对照 Java MathGenerator(numberLength, false)：只用 + - ×，且保证结果非负。
func nextMath(numberLength int) (question, answer string) {
	// numberLength 位的取值上界：1 位 → 10，2 位 → 100。
	bound := 1
	for range numberLength {
		bound *= 10
	}

	a, b := rand.IntN(bound), rand.IntN(bound)
	switch rand.IntN(3) {
	case 0:
		return fmt.Sprintf("%d+%d=?", a, b), fmt.Sprintf("%d", a+b)
	case 1:
		// 减法保证 a >= b，避免出现负数答案。
		if a < b {
			a, b = b, a
		}
		return fmt.Sprintf("%d-%d=?", a, b), fmt.Sprintf("%d", a-b)
	default:
		return fmt.Sprintf("%d*%d=?", a, b), fmt.Sprintf("%d", a*b)
	}
}

// key 拼验证码的 Redis 键：global:captcha_codes:<uuid>。
func key(id string) string {
	return constant.CaptchaCodeKey + id
}

// stripB64Prefix 去掉 base64Captcha 自带的 "data:image/png;base64," 前缀，
// 只返回裸 base64——对照 Java hutool getImageBase64()，前缀由前端自行拼接。
func stripB64Prefix(s string) string {
	if _, after, ok := strings.Cut(s, ";base64,"); ok {
		return after
	}
	return s
}

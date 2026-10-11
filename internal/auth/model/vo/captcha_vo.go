package vo

// CaptchaVo /auth/code 图片验证码响应对象。
// 字段名与前端约定一致：login.vue 读 data.captchaEnabled / data.uuid / data.img。
// 对照 Java CaptchaController.CaptchaVo(Boolean captchaEnabled, String uuid, String img)。
type CaptchaVo struct {
	// CaptchaEnabled 是否启用验证码校验。关闭时 uuid/img 为空，前端据此隐藏验证码输入。
	CaptchaEnabled bool `json:"captchaEnabled"`
	// UUID 验证码标识，前端登录时回传，后端凭它取 Redis 里的答案比对。
	UUID string `json:"uuid"`
	// Img 裸 base64，不含 "data:image/png;base64," 前缀。
	// 前端自己拼前缀(login.vue: 'data:image/gif;base64,' + data.img)，
	// 这里带前缀会拼成两截导致图裂。
	Img string `json:"img"`
}

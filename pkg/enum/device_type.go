package enum

// DeviceType 登录设备类型，对应原项目 enums.DeviceType。
//
// **重要：这是一份参考取值，不是白名单，不要用它校验设备类型。**
//
// 原项目里这个枚举是「死代码」——全项目除枚举自身定义外零引用。实际流转的
// 设备类型全部是 sys_client.device_type 的**原始字符串**，从 client 配置一路
// 透传：IAuthStrategy.setDeviceType(client.getDeviceType()) →
// LoginHelper → 落到 sys_login_info / sys_oper_log 的 device_type 列。
//
// 而 DB 种子数据里 app 客户端的 device_type 是 **"android"**，并不在下面这四个
// 取值中（见 script/sql/ry_vue.sql 的 sys_client insert）。所以一旦拿
// ParseDeviceType 去校验登录请求，app 客户端会直接登录失败。
//
// 迁移时按原项目做法处理：device_type 当自由字符串透传，不做枚举校验。
// 下面的常量仅供需要按设备维度做逻辑分支时引用（如踢下线只踢 PC 端、
// 保留 App 端在线），且分支必须允许未知取值走默认路径。
type DeviceType struct {
	Code string // 设备标识
}

// 设备类型枚举实例。取值域**不完整**，见上方类型注释。
var (
	DevicePC     = DeviceType{Code: "pc"}     // pc 端
	DeviceApp    = DeviceType{Code: "app"}    // app 端
	DeviceXcx    = DeviceType{Code: "xcx"}    // 小程序端
	DeviceSocial = DeviceType{Code: "social"} // 第三方社交登录平台
)

// deviceTypes 全部设备类型，顺序与原枚举声明一致。
var deviceTypes = []DeviceType{DevicePC, DeviceApp, DeviceXcx, DeviceSocial}

// DeviceTypes 返回全部设备类型的副本。
func DeviceTypes() []DeviceType {
	return append([]DeviceType(nil), deviceTypes...)
}

// ParseDeviceType 按 Code 精确查找设备类型，未匹配时 ok 为 false。
//
// ok=false **不代表设备类型非法**（"android" 就是个合法但不在表内的取值），
// 只代表「本枚举没有它的附加信息」。不要据此拒绝请求。
func ParseDeviceType(code string) (DeviceType, bool) {
	for _, d := range deviceTypes {
		if d.Code == code {
			return d, true
		}
	}
	return DeviceType{}, false
}

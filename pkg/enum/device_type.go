package enum

// DeviceType 登录设备类型。
type DeviceType struct {
	Code string
}

// 设备类型枚举实例。
var (
	DevicePC     = DeviceType{Code: "pc"}
	DeviceApp    = DeviceType{Code: "app"}
	DeviceXcx    = DeviceType{Code: "xcx"}
	DeviceSocial = DeviceType{Code: "social"}
)

var deviceTypes = []DeviceType{DevicePC, DeviceApp, DeviceXcx, DeviceSocial}

// DeviceTypes 返回全部设备类型的副本。
func DeviceTypes() []DeviceType {
	return append([]DeviceType(nil), deviceTypes...)
}

// ParseDeviceType 按 Code 查找设备类型，未匹配时 ok 为 false。
func ParseDeviceType(code string) (DeviceType, bool) {
	for _, d := range deviceTypes {
		if d.Code == code {
			return d, true
		}
	}
	return DeviceType{}, false
}

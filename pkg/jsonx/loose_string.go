package jsonx

import (
	"bytes"
	"fmt"
	"strconv"
)

// LooseString 反序列化时接受字符串、布尔、数字三种 JSON 标量的字符串字段。
//
// 与 Jackson 行为对齐：把 true / 42 静默强转成 "true" / "42"，前端约定如此。
// jsonx 用的 ConfigCompatibleWithStandardLibrary 与 encoding/json 一样严格，
// 遇到类型不匹配直接报错。前端在共用一个后端契约时，会把开关类配置项按布尔下发
// （如 sys.oss.previewListResource），严格解码会让这类正常调用一律卡在"参数校验失败"上。
//
// 只用在值本身就以字符串落库、而前端可能发送非字符串标量的字段上；
// 语义上确实是布尔或数字的字段应当直接声明成 bool / int64。
type LooseString string

// String 返回底层字符串。
func (s LooseString) String() string { return string(s) }

// looseNull JSON 的 null 字面量。
var looseNull = []byte("null")

// UnmarshalJSON 把标量 JSON 值统一收成字符串。
//
// 数字保留原始字面量而非走 float64 往返：后者会把 1.50 变成 1.5、
// 把大整数变成科学计数法，而这些值要原样落库。
func (s *LooseString) UnmarshalJSON(data []byte) error {
	if len(data) == 0 || bytes.Equal(data, looseNull) {
		// null 收成空串而非报错，与 Jackson 一致；是否允许为空交给 binding 校验。
		*s = ""
		return nil
	}

	switch data[0] {
	case '"':
		var v string
		if err := Unmarshal(data, &v); err != nil {
			return err
		}
		*s = LooseString(v)
	case 't', 'f':
		var v bool
		if err := Unmarshal(data, &v); err != nil {
			return err
		}
		*s = LooseString(strconv.FormatBool(v))
	case '-', '0', '1', '2', '3', '4', '5', '6', '7', '8', '9':
		// 先校验是不是合法数字，避免把 1abc 这类残缺输入原样收下。
		if _, err := strconv.ParseFloat(string(data), 64); err != nil {
			return fmt.Errorf("jsonx: %s 不是合法的数字", data)
		}
		*s = LooseString(data)
	default:
		// 对象与数组不做扁平化：那只会把结构错误藏到库里，等读取时才炸。
		return fmt.Errorf("jsonx: 期望字符串、布尔或数字，得到 %s", data)
	}
	return nil
}

// MarshalJSON 始终按字符串出参，使出参形态与普通 string 字段无差别。
func (s LooseString) MarshalJSON() ([]byte, error) {
	return Marshal(string(s))
}

// Package jsonx 全局 JSON 编解码器，复刻 Java 侧 JacksonConfig 注册的 BigNumberSerializer。
//
// 存在的唯一理由是雪花主键：id 形如 1762000000000000001，超过 JS 的
// Number.MAX_SAFE_INTEGER(9007199254740991)，直接以数字下发会被前端的 IEEE754
// 抹掉末几位，拿它回传编辑/删除就命中不到原记录。
//
// 与 Java 同为**按值判断**而非按字段：同一个 JSON key 的值域可以横跨安全线两侧
// （sys_menu/sys_dept 的 parentId 根节点是 0、子节点是 19 位雪花值），前端
// dept/index.vue 的 `form.parentId !== 0` 是严格比较，恒转字符串会破坏它。
// 故不能用「给 ID 字段挂自定义类型」的做法，只能在序列化层按实际值决定形态。
package jsonx

import (
	"io"
	"strconv"
	"unsafe"

	ginjson "github.com/gin-gonic/gin/codec/json"
	jsoniter "github.com/json-iterator/go"
)

// JS 安全整数边界，取值对齐 Java BigNumberSerializer。
const (
	maxSafeInteger = 9007199254740991
	minSafeInteger = -9007199254740991
)

// api 选 ConfigCompatibleWithStandardLibrary 而非 ConfigDefault/ConfigFastest：
// 它对齐标准库的 EscapeHTML/SortMapKeys/ValidateJsonRawMessage，
// 换 codec 只应改变 int64 的形态，不该顺带改动其余序列化行为。
var api = jsoniter.ConfigCompatibleWithStandardLibrary

// init 注册 int64 编解码器。
//
// 必须放 init 而非 Init：jsoniter 按类型缓存 encoder（frozenConfig.encoderCache），
// 某类型一旦被编码过，其 encoder 就固化下来，之后再 RegisterTypeEncoder 也挤不掉。
// 放在包初始化阶段可确保任何 Marshal 之前完成注册；Init 只负责接管 gin 的 API。
func init() {
	// 注册键是类型名字符串，jsoniter 取 encoder 时先查这张包级表、再回落内置实现，
	// 故所有裸 int64 字段一次性全覆盖，无需改任何 struct。
	jsoniter.RegisterTypeEncoder("int64", bigInt64Codec{})
	jsoniter.RegisterTypeDecoder("int64", bigInt64Codec{})
}

// Init 接管 gin 的全局 JSON codec。
//
// 须在首个 c.JSON / 参数绑定之前调用（各进程 main 开头）：ginjson.API 由 gin
// 自身 codec 的 init() 赋默认值，靠包 init 顺序抢不可靠。
func Init() {
	ginjson.API = codec{}
}

// bigInt64Codec int64 的编解码器，同时实现 ValEncoder 与 ValDecoder。
type bigInt64Codec struct{}

// Encode 超出 JS 安全整数范围时写字符串，范围内仍写数字。
func (bigInt64Codec) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	v := *((*int64)(ptr))
	if v >= minSafeInteger && v <= maxSafeInteger {
		stream.WriteInt64(v)
		return
	}
	stream.WriteString(strconv.FormatInt(v, 10))
}

// IsEmpty 与内置 int64 编码器一致，保证 omitempty 语义不变。
func (bigInt64Codec) IsEmpty(ptr unsafe.Pointer) bool {
	return *((*int64)(ptr)) == 0
}

// Decode 数字与字符串都接受。
//
// 这一侧比 Java 宽松（Java 的 Long 入参收不了字符串）且**必不可少**：出参既然把
// 大 id 变成了字符串，前端把详情响应原样回传时送来的就是字符串，若只认数字会直接 400。
func (bigInt64Codec) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	if iter.WhatIsNext() == jsoniter.StringValue {
		s := iter.ReadString()
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			iter.ReportError("jsonx.bigInt64Codec", "无法解析为 int64: "+s)
			return
		}
		*((*int64)(ptr)) = v
		return
	}
	// 非字符串分支与内置 int64Codec 逐字一致：null 跳过并保持零值。
	if !iter.ReadNil() {
		*((*int64)(ptr)) = iter.ReadInt64()
	}
}

// codec 用 jsoniter 实现 gin 的 json.Core。
type codec struct{}

func (codec) Marshal(v any) ([]byte, error) { return api.Marshal(v) }

func (codec) Unmarshal(data []byte, v any) error { return api.Unmarshal(data, v) }

func (codec) MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return api.MarshalIndent(v, prefix, indent)
}

func (codec) NewEncoder(writer io.Writer) ginjson.Encoder { return api.NewEncoder(writer) }

func (codec) NewDecoder(reader io.Reader) ginjson.Decoder { return api.NewDecoder(reader) }

// Marshal 供项目内需要与出参同构的场景使用（如 pkg/tree 的自定义 MarshalJSON）。
// 直接用 encoding/json 会绕过上面注册的 int64 编码器，导致同一个 id
// 在不同接口下形态不一致。
func Marshal(v any) ([]byte, error) { return api.Marshal(v) }

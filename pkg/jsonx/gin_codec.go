package jsonx

import (
	"io"

	ginjson "github.com/gin-gonic/gin/codec/json"
	jsoniter "github.com/json-iterator/go"
)

// OverrideGinCodec 用本包 ginCodec 覆盖 gin 的全局 codec（ginjson.API），
// 使 c.JSON 与参数绑定也走 int64 编解码器。
//
// 须在首个 c.JSON/绑定前调用（各进程 main 开头）：ginjson.API 由 gin 自身
// codec 的 init() 赋默认值，靠包 init 顺序抢不可靠。
func OverrideGinCodec() {
	ginjson.API = ginCodec{}
}

// ginCodec 用 jsoniter 实现 gin 的 json.Core，替换 gin 默认 codec。
type ginCodec struct{}

func (ginCodec) Marshal(v any) ([]byte, error) {
	return jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(v)
}

func (ginCodec) Unmarshal(data []byte, v any) error {
	return jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(data, v)
}

func (ginCodec) MarshalIndent(v any, prefix, indent string) ([]byte, error) {
	return jsoniter.ConfigCompatibleWithStandardLibrary.MarshalIndent(v, prefix, indent)
}

func (ginCodec) NewEncoder(writer io.Writer) ginjson.Encoder {
	return jsoniter.ConfigCompatibleWithStandardLibrary.NewEncoder(writer)
}

func (ginCodec) NewDecoder(reader io.Reader) ginjson.Decoder {
	return jsoniter.ConfigCompatibleWithStandardLibrary.NewDecoder(reader)
}

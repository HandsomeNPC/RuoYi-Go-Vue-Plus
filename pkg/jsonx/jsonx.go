// Package jsonx 提供与项目出参同构的 JSON 编解码，并接管 gin 的全局 codec。
//
// 核心诉求是雪花主键：19 位 id 超过 JS 的 Number.MAX_SAFE_INTEGER，以数字下发
// 会被前端 IEEE754 抹掉末位、回传时命中不到原记录，故超界值转字符串。
//
// 按「值」而非「字段」判断形态：parentId 根节点是 0、子节点是雪花值，前端用
// `!== 0` 严格比较，恒转字符串会破坏它，故不能给 ID 字段挂自定义类型。
package jsonx

import jsoniter "github.com/json-iterator/go"

// Marshal 与出参同构地序列化（如 pkg/tree 的自定义 MarshalJSON）。
// 不可用 encoding/json：会绕过 init() 注册的 int64 编码器，导致同一 id 形态不一致。
func Marshal(v any) ([]byte, error) {
	return jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(v)
}

// Unmarshal 与 Marshal 对称（如从 Redis 还原缓存值），int64 字符串/数字双向可读。
func Unmarshal(data []byte, v any) error {
	return jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal(data, v)
}

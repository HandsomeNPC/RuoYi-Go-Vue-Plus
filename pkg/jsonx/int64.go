package jsonx

import (
	"strconv"
	"unsafe"

	jsoniter "github.com/json-iterator/go"
)

// JS 安全整数边界。
const (
	maxSafeInteger = 9007199254740991
	minSafeInteger = -9007199254740991
)

func init() {
	// 注册键是类型名，覆盖所有裸 int64 字段，无需改任何 struct。
	jsoniter.RegisterTypeEncoder("int64", bigInt64Codec{})
	jsoniter.RegisterTypeDecoder("int64", bigInt64Codec{})
}

// bigInt64Codec 同时实现 int64 的 ValEncoder 与 ValDecoder。
type bigInt64Codec struct{}

// Encode 超出 JS 安全整数范围写字符串，范围内写数字。
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

// Decode 同时接受字符串与数字：出参把大 id 变成字符串后前端会原样回传，只认数字会 400。
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
	// 非字符串分支与内置 int64Codec 一致：null 保持零值。
	if !iter.ReadNil() {
		*((*int64)(ptr)) = iter.ReadInt64()
	}
}

package jsonx

import "testing"

// TestLooseStringAcceptsScalars 三种标量都收成字符串。
//
// 存在意义：前端把开关类配置按布尔下发（configValue: true），
// 而该列是 varchar。严格解码会让这类调用一律"参数校验失败"。
func TestLooseStringAcceptsScalars(t *testing.T) {
	cases := map[string]string{
		`"true"`:  "true",
		`true`:    "true",
		`false`:   "false",
		`"hello"`: "hello",
		`42`:      "42",
		`-7`:      "-7",
		`0`:       "0",
		`""`:      "",
		`null`:    "",
	}
	for in, want := range cases {
		var s LooseString
		if err := Unmarshal([]byte(in), &s); err != nil {
			t.Errorf("Unmarshal(%s) 报错: %v", in, err)
			continue
		}
		if s.String() != want {
			t.Errorf("Unmarshal(%s) = %q, want %q", in, s, want)
		}
	}
}

// TestLooseStringKeepsNumericLiteral 数字保留原始字面量。
//
// 不走 float64 往返：那会把 1.50 变成 1.5、把大整数变成科学计数法，
// 而这些值要原样落进 config_value。
func TestLooseStringKeepsNumericLiteral(t *testing.T) {
	for _, in := range []string{"1.50", "100000000000000000000", "0.0001"} {
		var s LooseString
		if err := Unmarshal([]byte(in), &s); err != nil {
			t.Errorf("Unmarshal(%s) 报错: %v", in, err)
			continue
		}
		if s.String() != in {
			t.Errorf("Unmarshal(%s) = %q，数字字面量被改写了", in, s)
		}
	}
}

// TestLooseStringRejectsComposites 对象与数组必须报错。
// 扁平化它们只会把结构错误藏进库里，等读取时才炸。
func TestLooseStringRejectsComposites(t *testing.T) {
	for _, in := range []string{`{"a":1}`, `[1,2]`, `1abc`} {
		var s LooseString
		if err := Unmarshal([]byte(in), &s); err == nil {
			t.Errorf("Unmarshal(%s) 应报错，实际得到 %q", in, s)
		}
	}
}

// TestLooseStringMarshalsAsString 出参形态与普通 string 字段无差别，
// 前端读到的始终是字符串。
func TestLooseStringMarshalsAsString(t *testing.T) {
	for _, c := range []struct {
		in   LooseString
		want string
	}{
		{"true", `"true"`},
		{"42", `"42"`},
		{"", `""`},
	} {
		got, err := Marshal(c.in)
		if err != nil {
			t.Errorf("Marshal(%q) 报错: %v", c.in, err)
			continue
		}
		if string(got) != c.want {
			t.Errorf("Marshal(%q) = %s, want %s", c.in, got, c.want)
		}
	}
}

// TestLooseStringInStruct 结构体字段场景（真实用法是 gin 的 ShouldBindJSON）。
func TestLooseStringInStruct(t *testing.T) {
	type payload struct {
		ConfigKey   string      `json:"configKey"`
		ConfigValue LooseString `json:"configValue"`
	}

	// 复刻线上真实报错的请求体。
	var p payload
	raw := `{"configKey":"sys.oss.previewListResource","configValue":true}`
	if err := Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("解析布尔形态的 configValue 报错: %v", err)
	}
	if p.ConfigValue.String() != "true" {
		t.Errorf("ConfigValue = %q, want \"true\"", p.ConfigValue)
	}

	// 字符串形态同样要能解析，不能为了兼容布尔而破坏原本可用的形态。
	p = payload{}
	raw = `{"configKey":"sys.oss.previewListResource","configValue":"false"}`
	if err := Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("解析字符串形态的 configValue 报错: %v", err)
	}
	if p.ConfigValue.String() != "false" {
		t.Errorf("ConfigValue = %q, want \"false\"", p.ConfigValue)
	}
}

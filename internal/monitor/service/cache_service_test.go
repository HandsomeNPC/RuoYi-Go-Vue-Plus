package service

import "testing"

func TestParseInfo(t *testing.T) {
	in := "# Server\r\nredis_version:7.0.5\r\n\r\n# Clients\r\nconnected_clients:3\r\n"
	got := parseInfo(in)
	if got["redis_version"] != "7.0.5" {
		t.Fatalf("redis_version: want 7.0.5, got %q", got["redis_version"])
	}
	if got["connected_clients"] != "3" {
		t.Fatalf("connected_clients: want 3, got %q", got["connected_clients"])
	}
	if _, ok := got["# Server"]; ok { // 注释行不应进 map
		t.Fatal("注释行被误收")
	}
}

func TestParseCommandStats(t *testing.T) {
	in := "# Commandstats\r\ncmdstat_get:calls=37,usec=120,usec_per_call=3.24\r\n" +
		"cmdstat_set:calls=5,usec=10,usec_per_call=2.00\r\n\r\n"
	got := parseCommandStats(in)
	if len(got) != 2 {
		t.Fatalf("条数: want 2, got %d", len(got))
	}
	// 不依赖 map 顺序，按 name 查
	byName := map[string]string{}
	for _, m := range got {
		byName[m["name"]] = m["value"]
	}
	if byName["get"] != "37" {
		t.Fatalf("get calls: want 37, got %q", byName["get"])
	}
	if byName["set"] != "5" {
		t.Fatalf("set calls: want 5, got %q", byName["set"])
	}
}

func TestSubstringBetween(t *testing.T) {
	cases := []struct{ s, start, end, want string }{
		{"calls=37,usec=120", "calls=", ",usec", "37"},
		{"calls=37", "calls=", ",usec", ""}, // 无 end
		{"x=1", "calls=", ",usec", ""},      // 无 start
		{"calls=", "calls=", ",usec", ""},   // start 后即无 end
	}
	for _, c := range cases {
		if got := substringBetween(c.s, c.start, c.end); got != c.want {
			t.Errorf("substringBetween(%q,%q,%q): want %q, got %q", c.s, c.start, c.end, c.want, got)
		}
	}
}

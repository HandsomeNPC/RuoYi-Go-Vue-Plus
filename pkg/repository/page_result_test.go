package repository

import (
	"encoding/json"
	"testing"
)

func TestPageSerializesTotalAndRows(t *testing.T) {
	b, err := json.Marshal(Page([]string{"a", "b"}, 10))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(b), `{"total":10,"rows":["a","b"]}`; got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestPageNilRowsSerializesAsEmptyArray(t *testing.T) {
	b, err := json.Marshal(Page[string](nil, 0))
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(b), `{"total":0,"rows":[]}`; got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

func TestPageOfUsesLen(t *testing.T) {
	p := PageOf([]int{1, 2, 3})
	if got, want := p.Total, int64(3); got != want {
		t.Errorf("Total = %d, want %d", got, want)
	}
}

func TestEmptyPage(t *testing.T) {
	p := EmptyPage[int]()
	if p.Total != 0 || len(p.Rows) != 0 {
		t.Errorf("EmptyPage = %+v, want total=0 rows=[]", p)
	}
	if p.Rows == nil {
		t.Error("Rows 不应为 nil，否则序列化成 null")
	}
}

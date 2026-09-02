package excel

import (
	"bytes"
	"testing"
)

// TestReadRoundTrip 写出再读回同一类型，验证列定位按表头匹配、字典反向映射、雪花 ID 以字符串往返不丢精度。
func TestReadRoundTrip(t *testing.T) {
	type row struct {
		ID     int64  `excel:"序号"`
		Name   string `excel:"名称"`
		Status string `excel:"状态" excelDict:"0=正常,1=停用"`
		Unused string // 无 tag，不参与读写
	}

	want := row{ID: 1761100000000000001, Name: "张三", Status: "0"}
	buf, err := Write([]row{want}, Options{SheetName: "用户数据"})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read[row](bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("读回 %d 行，期望 1", len(got))
	}
	if got[0].ID != want.ID {
		t.Errorf("ID = %d, 期望 %d（雪花 ID 以字符串往返不丢精度）", got[0].ID, want.ID)
	}
	if got[0].Name != want.Name {
		t.Errorf("Name = %q, 期望 %q", got[0].Name, want.Name)
	}
	// 写入时 Status="0" 经字典渲染成"正常"；读回时"正常"经字典反向换回 "0"。
	if got[0].Status != want.Status {
		t.Errorf("Status = %q, 期望 %q（字典应双向映射）", got[0].Status, want.Status)
	}
}

// TestReadEmptyFile 空表（只有表头）读回空切片，不算错。
func TestReadEmptyFile(t *testing.T) {
	type row struct {
		Name string `excel:"名称"`
	}
	buf, err := Write([]row{}, Options{SheetName: "s"})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read[row](bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("空表读回 %d 行，期望 0", len(got))
	}
}

// TestReadSkipsEmptyValueCell 空单元格留零值：只填部分列时其余字段保持零。
func TestReadSkipsEmptyValueCell(t *testing.T) {
	type row struct {
		Name   string `excel:"名称"`
		Status string `excel:"状态" excelDict:"0=正常,1=停用"`
	}
	// 只给 Name，Status 留空写出→读回应仍为空串。
	buf, err := Write([]row{{Name: "x"}}, Options{SheetName: "s"})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := Read[row](bytes.NewReader(buf.Bytes()))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if len(got) != 1 || got[0].Name != "x" || got[0].Status != "" {
		t.Errorf("空单元格应留零：got=%+v", got)
	}
}

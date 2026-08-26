package response

import (
	"encoding/json"
	"testing"
)

func TestOk(t *testing.T) {
	r := Ok(42)
	if got, want := r.Code, CodeSuccess; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
	if got, want := r.Msg, MsgSuccess; got != want {
		t.Errorf("Msg = %q, want %q", got, want)
	}
	if got, want := r.Data, 42; got != want {
		t.Errorf("Data = %d, want %d", got, want)
	}
	if !r.IsSuccess() || r.IsError() {
		t.Error("IsSuccess/IsError 判定错误")
	}
}

func TestFailEmptyMsgFallsBack(t *testing.T) {
	if got, want := Fail("").Msg, MsgFail; got != want {
		t.Errorf("Fail(\"\").Msg = %q, want %q", got, want)
	}
	if got, want := Fail("用户不存在").Msg, "用户不存在"; got != want {
		t.Errorf("Msg = %q, want %q", got, want)
	}
	if got, want := Fail("x").Code, CodeFail; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
}

func TestWarn(t *testing.T) {
	r := Warn("存在下级部门,不允许删除")
	if got, want := r.Code, 601; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
	if r.IsSuccess() {
		t.Error("警告响应不应判定为成功")
	}
}

func TestFailCode(t *testing.T) {
	if got, want := FailCode(CodeUnauthorized, "认证失败").Code, 401; got != want {
		t.Errorf("Code = %d, want %d", got, want)
	}
}

func TestVoidResponseKeepsDataField(t *testing.T) {
	b, err := json.Marshal(OkVoid())
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if got, want := string(b), `{"code":200,"msg":"操作成功","data":null}`; got != want {
		t.Errorf("JSON = %s, want %s", got, want)
	}
}

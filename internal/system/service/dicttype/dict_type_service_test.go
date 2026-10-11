package dicttype

import "testing"

// TestValidateDictTypeFormat 字典类型是 sys_dict_data 的关联键，
// 格式失守会让前端按类型取字典时静默取空——必须挡在写入之前。
func TestValidateDictTypeFormat(t *testing.T) {
	valid := []string{"sys_user_gender", "a", "a1_", "sys_normal_disable"}
	for _, s := range valid {
		t.Run("valid/"+s, func(t *testing.T) {
			if err := validateDictTypeFormat(s); err != nil {
				t.Errorf("validateDictTypeFormat(%q) = %v, 期望通过", s, err)
			}
		})
	}

	bad := []string{
		"",             // 空串
		"1abc",         // 数字开头
		"Sys_user",     // 含大写
		"sys-user",     // 含连字符
		"sys user",     // 含空格
		"x sys_user y", // 前后有内容（验证正则是整串锚定的）
		"sys_user\n",   // 尾随换行
	}
	for _, s := range bad {
		t.Run("bad/"+s, func(t *testing.T) {
			if err := validateDictTypeFormat(s); err == nil {
				t.Errorf("validateDictTypeFormat(%q) = nil, 应拒绝", s)
			}
		})
	}
}

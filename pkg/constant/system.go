package constant

const (
	StatusNormal              = "0"
	StatusDisable             = "1"
	Yes                       = "Y"
	No                        = "N"
	MenuTypeDir               = "M"
	MenuTypeMenu              = "C"
	MenuTypeButton            = "F"
	ComponentLayout           = "Layout"
	ComponentParentView       = "ParentView"
	ComponentInnerLink        = "InnerLink"
	SuperAdminUserID    int64 = 1761100000000000001
	SuperAdminRoleID    int64 = 1761300000000000001
	SuperAdminRoleKey         = "superadmin"
	RootDeptAncestors         = "0"
	DefaultDeptID       int64 = 1761000000000000100
)

var ExcludeProperties = []string{"password", "oldPassword", "newPassword", "confirmPassword"}

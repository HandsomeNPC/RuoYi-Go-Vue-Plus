package enum

// BusinessType 业务操作类型，对应 Java org.dromara.common.log.enums.BusinessType。
//
// 取值即 Java 侧 enum.ordinal()——LogAspect 落库写的是 businessType().ordinal()，
// 库里存的是数字，故顺序不可调整、不可插入新值，只能在末尾追加。
type BusinessType int

// 业务操作类型枚举实例，数值与 Java 声明顺序一一对应。
const (
	BusinessTypeOther   BusinessType = iota // 其它
	BusinessTypeInsert                      // 新增
	BusinessTypeUpdate                      // 修改
	BusinessTypeDelete                      // 删除
	BusinessTypeGrant                       // 授权
	BusinessTypeExport                      // 导出
	BusinessTypeImport                      // 导入
	BusinessTypeForce                       // 强退
	BusinessTypeGenCode                     // 生成代码
	BusinessTypeClean                       // 清空数据
)

// businessTypeInfos 各业务类型的中文描述，下标即枚举值。
var businessTypeInfos = [...]string{
	"其它", "新增", "修改", "删除", "授权", "导出", "导入", "强退", "生成代码", "清空数据",
}

// Int 返回落库数值。
func (b BusinessType) Int() int { return int(b) }

// Info 返回中文描述，越界返回 "其它"（对齐 Int 越界时的兜底语义）。
func (b BusinessType) Info() string {
	if b < 0 || int(b) >= len(businessTypeInfos) {
		return businessTypeInfos[BusinessTypeOther]
	}
	return businessTypeInfos[b]
}

// OperatorType 操作人类别，对应 Java org.dromara.common.log.enums.OperatorType。
// 取值同为 Java 的 ordinal()，不可调序。
type OperatorType int

// 操作人类别枚举实例。
const (
	OperatorTypeOther  OperatorType = iota // 其它
	OperatorTypeManage                     // 后台用户
	OperatorTypeMobile                     // 手机端用户
)

// Int 返回落库数值。
func (o OperatorType) Int() int { return int(o) }

// BusinessStatus 操作状态，对应 Java org.dromara.common.log.enums.BusinessStatus。
// 落 sys_oper_log.status（0正常 1异常），取值同为 ordinal()。
type BusinessStatus int

// 操作状态枚举实例。
const (
	BusinessStatusSuccess BusinessStatus = iota // 成功
	BusinessStatusFail                          // 失败
)

// Int 返回落库数值。
func (b BusinessStatus) Int() int { return int(b) }

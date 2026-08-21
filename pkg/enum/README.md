# pkg/enum

枚举类型定义，移植原项目 `ruoyi-common-core` 的 `enums` 包。

## 放什么

这里放 **带附加字段的枚举**（Java enum 除标识外还携带 info / 提示 key 等信息）。 单纯的字符串标识（启用停用、是否、菜单类型、布局组件名等）仍留在
`pkg/constant`。

> **拆分标准**：只有一个值、没有附加信息 → `constant`；
> 一个标识 + 若干附属字段且需要按标识反查 → `enum`。

| 原 Java            | 本包         | 文件             |
|--------------------|--------------|------------------|
| `enums.UserStatus` | `UserStatus` | `user_status.go` |
| `enums.UserType`   | `UserType`   | `user_type.go`   |
| `enums.DeviceType` | `DeviceType` | `device_type.go` |
| `enums.LoginType`  | `LoginType`  | `login_type.go`  |

原项目 `enums` 包下另有 `BusinessStatusEnum`（工作流单据状态）、
`PushSourceEnum` / `PushTypeEnum`（消息推送），分属阶段 5 的 workflow / push 模块，待迁移到对应模块时再补。

## 与原枚举的有意偏差

移植时有两处没有逐字对齐，都在对应文件的类型注释里写了原因，这里汇总：

| 位置              | 偏差                        | 原因                                                                  |
|-------------------|-----------------------------|-----------------------------------------------------------------------|
| `LoginType.Code`  | Java 无此字段               | Java 靠 enum 实例本身传递，Go 需要标识按 grantType 查表               |
| `LoginTypeSocial` | Java 只有 4 个值，无 social | Code 是查表键，须覆盖全部 5 种 grantType；Java 能少是因 social 无文案 |

## 关于 var

Go 的 `const` 只支持布尔、数值、字符串和 rune， **不支持结构体**，所以枚举实例 只能用 `var` 声明。这带来一个 Java enum
没有的风险：外部包可以重新赋值，
`enum.UserStatusOK = UserStatus{}` 能编译通过。

**约定：本包所有导出的枚举实例均为逻辑常量，任何地方都不得对其赋值。**

编译器管不了这条，靠 `enum_test.go` 兜底 —— 它锚定了全部取值，被误改会在测试阶段暴露。 同理，`UserStatuses()` /
`UserTypes()` 这类列举函数一律返回 **副本**， 避免调用方改动污染包内枚举表。

## 使用方式

枚举实例本身不入库，入库/比较用它的标识字段：

```go
user.Status = enum.UserStatusDisable.Code       // 赋值
if user.Status == enum.UserStatusOK.Code { }    // 比较

switch user.Status {                            // switch 对标识做
case enum.UserStatusDisable.Code:
}
```

反向由标识拿枚举（取 Info 展示等）用各类型的 Parse 函数：

```go
st, ok := enum.ParseUserStatus(user.Status)
if ok {
    fmt.Println(st.Info)
}
```

`ParseXxx` 均为 **精确**匹配，未命中返回 `ok=false`，请务必判断返回值， 不要拿零值继续走业务流程。

### 注意：ok=false 不等于「取值非法」

`ParseDeviceType` / `ParseLoginType` 的 `ok=false` 只表示「本枚举没有这个标识 的附加信息」， **不能据此拒绝请求**
。原项目的校验依据都在别处：

- **device_type**：原项目的 `DeviceType` 枚举是死代码（全项目零引用），实际流转 的是 `sys_client.device_type` 原始字符串。种子数据里
  app 客户端该字段是
  `"android"` —— 不在枚举内但完全合法。拿 `ParseDeviceType` 做登录校验会让 app 客户端直接登录失败。
- **grantType**：合法性由 **该客户端**的 `sys_client.grant_type` 是否包含它决定 （`AuthController` 里
  `!StringUtils.contains(client.getGrantType(), grantType)`
  即报「认证类型异常」）。是按客户端的 DB 配置，不是全局枚举 —— pc 客户端只开
  `password,social`，用枚举校验会放进它未开启的 `sms`。

### 注意：ParseUserTypeFromLoginID 是子串匹配

`UserType` 额外提供了 `ParseUserTypeFromLoginID`，对齐原项目
`UserType.getUserType(String)` —— 传入的是 `"sys_user:1"` 这类拼接串， 走 **子串**匹配而非精确相等。与 `ParseUserType`
语义不同，不要混用。

它显式拦了空串：`strings.Contains(s, "")` 恒为 true， 不拦会把空 loginId 误判成 `UserTypeSys`，在鉴权中间件里等于放行畸形
token。

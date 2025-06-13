package response

type Response struct {
	Code int64       `json:"code"`
	Data interface{} `json:"data"`
	Msg  string      `json:"msg"`
}

type PageResult struct {
	List     interface{} `json:"list"`
	Total    int64       `json:"total"`
	PageNo   int         `json:"pageNo"`
	PageSize int         `json:"pageSize"`
}

type ListResult struct {
	List  interface{} `json:"list"`
	Total int64       `json:"total"`
}

type IDName struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type UserPermission struct {
	OrgPermission UserOrgPermission `json:"orgPermission"` // 用户所在组织权限
	Language      Language          `json:"language"`      // 语言
}

type UserOrgPermission struct {
	IsAdmin     bool         `json:"isAdmin"`     // 是否系统内置管理员
	IsSystem    bool         `json:"isSystem"`    // 是否系统视角（此时org.id为空，org.name为"系统"）
	Org         IDName       `json:"org"`         // 组织
	Roles       []IDName     `json:"roles"`       // 角色列表
	Permissions []Permission `json:"permissions"` // 权限列表
}

type Permission struct {
	Perm string `json:"perm"` // 权限
	Name string `json:"name"` // 权限名（对应菜单名）
}

type Select struct {
	Select []IDName `json:"select"`
}

type DocCenter struct {
	DocCenterPath string `json:"docCenterPath"`
}

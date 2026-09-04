package vo

type RoleVo struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
	Path string
}

type RoleListVo struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
}

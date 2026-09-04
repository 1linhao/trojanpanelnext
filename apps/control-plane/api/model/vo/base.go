package vo

// 分页查询的结构体
type BaseVoPage struct {
	PageNum  uint `json:"pageNum"`  // 页号
	PageSize uint `json:"pageSize"` // 页大小
	Total    uint `json:"total"`    // 总记录数
}

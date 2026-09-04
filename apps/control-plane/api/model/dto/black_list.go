package dto

type BlackListDto struct {
	Ip *string `json:"ip" form:"ip" validate:"omitempty,min=0,max=64"`
}

type BlackListPageDto struct {
	BaseDto
	BlackListDto
}

type BlackListCreateDto struct {
	Ip *string `json:"ip" form:"ip" validate:"required,ip|fqdn,min=4,max=64,ne=127.0.0.1"`
}

type BlackListDeleteDto struct {
	Ip *string `json:"ip" form:"ip" validate:"required,ip|fqdn,min=4,max=64"`
}

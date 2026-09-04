package dto

type RoleDto struct {
	Name *string `json:"name" form:"name" validate:"omitempty,min=0,max=10"`
	Desc *string `json:"desc" form:"desc" validate:"omitempty,min=0,max=10"`
}

package dto

type NodeContainerRequestDto struct {
	NodeServerId uint `json:"nodeServerId" form:"nodeServerId" validate:"required,gt=0"`
}

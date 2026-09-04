package dto

type BaseDto struct {
	PageNum   *uint `json:"pageNum" form:"pageNum" validate:"required,gt=0"`
	PageSize  *uint `json:"pageSize" form:"pageSize" validate:"required,gt=0"`
	StartTime *uint `json:"startTime" form:"startTime" validate:"-"`
	EndTime   *uint `json:"endTime" form:"endTime" validate:"-"`
}

type RequiredIdDto struct {
	Id *uint `json:"id" form:"id" validate:"required"`
}

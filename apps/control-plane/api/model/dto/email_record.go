package dto

type EmailRecordPageDto struct {
	BaseDto
	EmailRecordDto
}

type EmailRecordDto struct {
	ToEmail *string `json:"toEmail" form:"toEmail" validate:"omitempty,min=0,max=64"`
	State   *int    `json:"state" form:"state" validate:"omitempty,oneof=-1 0 1"`
}

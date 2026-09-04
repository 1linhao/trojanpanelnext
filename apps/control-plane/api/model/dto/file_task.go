package dto

type FileTaskPageDto struct {
	BaseDto
	FileTaskDto
}

type FileTaskDto struct {
	Type            *uint   `json:"type" form:"type" validate:"omitempty,oneof=0 1 2 3 4"`
	AccountUsername *string `json:"accountUsername" form:"accountUsername" validate:"omitempty,min=0,max=20,validateStr"`
}

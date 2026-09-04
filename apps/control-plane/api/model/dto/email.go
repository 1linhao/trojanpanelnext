package dto

type SendEmailDto struct {
	FromEmailName string   `json:"fromEmailName" form:"fromEmailName" validate:"required,min=2,max=32"`
	ToEmails      []string `json:"toEmails" form:"toEmails" validate:"required,validateEmail"`
	Subject       string   `json:"subject" form:"subject" validate:"required,min=2,max=64"`
	Content       string   `json:"content" form:"content" validate:"required,min=2,max=255"`
}

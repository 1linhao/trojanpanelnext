package model

import "time"

type EmailRecord struct {
	Id         *uint      `ddb:"id"`
	ToEmail    *string    `ddb:"to_email"`
	Subject    *string    `ddb:"subject"`
	Content    *string    `ddb:"content"`
	State      *int       `ddb:"state"`
	CreateTime *time.Time `ddb:"create_time"`
	UpdateTime *time.Time `ddb:"update_time"`
}

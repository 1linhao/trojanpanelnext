package model

import "time"

type FileTask struct {
	Id              *uint      `ddb:"id"`
	Name            *string    `ddb:"name"`
	Path            *string    `ddb:"path"`
	Type            *uint      `ddb:"type"`
	Status          *int       `ddb:"status"`
	ErrMsg          *string    `ddb:"err_msg"`
	AccountId       *uint      `ddb:"account_id"`
	AccountUsername *string    `ddb:"account_username"`
	CreateTime      *time.Time `ddb:"create_time"`
	UpdateTime      *time.Time `ddb:"update_time"`
}

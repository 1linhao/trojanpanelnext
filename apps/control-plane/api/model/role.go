package model

import "time"

type Role struct {
	Id         *uint      `ddb:"id"`
	Name       *string    `ddb:"name"`
	Desc       *string    `ddb:"desc"`
	ParentId   *uint      `ddb:"parent_id"`
	Path       *string    `ddb:"path"`
	Level      *uint      `ddb:"level"`
	CreateTime *time.Time `ddb:"create_time"`
	UpdateTime *time.Time `ddb:"update_time"`
}

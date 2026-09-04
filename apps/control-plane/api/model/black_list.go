package model

import "time"

type BlackList struct {
	Id         *uint      `ddb:"id"`
	Ip         *string    `ddb:"ip"`
	CreateTime *time.Time `ddb:"create_time"`
	UpdateTime *time.Time `ddb:"update_time"`
}

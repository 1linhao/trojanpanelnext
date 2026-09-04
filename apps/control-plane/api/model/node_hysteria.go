package model

import "time"

type NodeHysteria struct {
	Id         *uint      `ddb:"id"`
	Protocol   *string    `ddb:"protocol"`
	Obfs       *string    `ddb:"obfs"`
	UpMbps     *int       `ddb:"up_mbps"`
	DownMbps   *int       `ddb:"down_mbps"`
	ServerName *string    `ddb:"server_name"`
	Insecure   *uint      `ddb:"insecure"`
	FastOpen   *uint      `ddb:"fast_open"`
	CreateTime *time.Time `ddb:"create_time"`
	UpdateTime *time.Time `ddb:"update_time"`
}

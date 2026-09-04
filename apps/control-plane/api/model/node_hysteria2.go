package model

import "time"

type NodeHysteria2 struct {
	Id           *uint      `ddb:"id"`
	ObfsPassword *string    `ddb:"obfs_password"`
	UpMbps       *int       `ddb:"up_mbps"`
	DownMbps     *int       `ddb:"down_mbps"`
	ServerName   *string    `ddb:"server_name"`
	Insecure     *uint      `ddb:"insecure"`
	PortHopping  *string    `ddb:"port_hopping"`
	HopInterval  *uint      `ddb:"hop_interval"`
	CreateTime   *time.Time `ddb:"create_time"`
	UpdateTime   *time.Time `ddb:"update_time"`
}

package model

import "time"

type NodeServer struct {
	Id                   *uint      `ddb:"id" json:"id"`
	Name                 *string    `ddb:"name" json:"name"`
	Ip                   *string    `ddb:"ip" json:"ip"`
	GrpcPort             *uint      `ddb:"grpc_port" json:"grpcPort"`
	GrpcTLSMode          *string    `ddb:"grpc_tls_mode" json:"grpcTlsMode"`
	GrpcTLSServerName    *string    `ddb:"grpc_tls_server_name" json:"grpcTlsServerName"`
	TrafficPeriod        *string    `ddb:"traffic_period" json:"trafficPeriod"`
	TrafficLimitMode     *string    `ddb:"traffic_limit_mode" json:"trafficLimitMode"`
	TrafficTotalLimit    *uint64    `ddb:"traffic_total_limit" json:"trafficTotalLimit"`
	TrafficUploadLimit   *uint64    `ddb:"traffic_upload_limit" json:"trafficUploadLimit"`
	TrafficDownloadLimit *uint64    `ddb:"traffic_download_limit" json:"trafficDownloadLimit"`
	CreateTime           *time.Time `ddb:"create_time" json:"createTime"`
	UpdateTime           *time.Time `ddb:"update_time" json:"updateTime"`
}

package vo

import "time"

type NodeServerVo struct {
	Id                   uint                  `json:"id"`
	Name                 string                `json:"name"`
	Ip                   string                `json:"ip"`
	GrpcPort             uint                  `json:"grpcPort"`
	GrpcTLSMode          string                `json:"grpcTlsMode"`
	GrpcTLSServerName    string                `json:"grpcTlsServerName"`
	TrafficPeriod        string                `json:"trafficPeriod"`
	TrafficLimitMode     string                `json:"trafficLimitMode"`
	TrafficTotalLimit    uint64                `json:"trafficTotalLimit"`
	TrafficUploadLimit   uint64                `json:"trafficUploadLimit"`
	TrafficDownloadLimit uint64                `json:"trafficDownloadLimit"`
	TrafficStatus        ServerTrafficStatusVo `json:"trafficStatus"`
	CreateTime           time.Time             `json:"createTime"`

	Status                 int    `json:"status"`
	TrojanPanelCoreVersion string `json:"trojanPanelCoreVersion"`
	KernelSummary          string `json:"kernelSummary"`
}

type NodeServerPageVo struct {
	NodeServers []NodeServerVo `json:"nodeServers"`
	BaseVoPage
}

type NodeServerOneVo struct {
	Id                   uint                  `json:"id"`
	Name                 string                `json:"name"`
	Ip                   string                `json:"ip"`
	GrpcPort             uint                  `json:"grpcPort"`
	GrpcTLSMode          string                `json:"grpcTlsMode"`
	GrpcTLSServerName    string                `json:"grpcTlsServerName"`
	TrafficPeriod        string                `json:"trafficPeriod"`
	TrafficLimitMode     string                `json:"trafficLimitMode"`
	TrafficTotalLimit    uint64                `json:"trafficTotalLimit"`
	TrafficUploadLimit   uint64                `json:"trafficUploadLimit"`
	TrafficDownloadLimit uint64                `json:"trafficDownloadLimit"`
	TrafficStatus        ServerTrafficStatusVo `json:"trafficStatus"`
	CreateTime           time.Time             `json:"createTime"`
}

type NodeServerListVo struct {
	Id   uint   `json:"id"`
	Name string `json:"name"`
}

type NodeServerInfoVo struct {
	CpuUsed  float32 `json:"cpuUsed"`
	MemUsed  float32 `json:"memUsed"`
	DiskUsed float32 `json:"diskUsed"`
}

type NodeServerExportVo struct {
	Name                 string    `json:"name" ddb:"name"`
	Ip                   string    `json:"ip" ddb:"ip"`
	GrpcPort             uint      `json:"grpcPort" ddb:"grpc_port"`
	GrpcTLSMode          string    `json:"grpcTlsMode" ddb:"grpc_tls_mode"`
	GrpcTLSServerName    string    `json:"grpcTlsServerName" ddb:"grpc_tls_server_name"`
	TrafficPeriod        string    `json:"trafficPeriod" ddb:"traffic_period"`
	TrafficLimitMode     string    `json:"trafficLimitMode" ddb:"traffic_limit_mode"`
	TrafficTotalLimit    uint64    `json:"trafficTotalLimit" ddb:"traffic_total_limit"`
	TrafficUploadLimit   uint64    `json:"trafficUploadLimit" ddb:"traffic_upload_limit"`
	TrafficDownloadLimit uint64    `json:"trafficDownloadLimit" ddb:"traffic_download_limit"`
	CreateTime           time.Time `json:"createTime" ddb:"create_time"`
}

package dto

type NodeServerPageDto struct {
	NodeServerDto
	BaseDto
}

type NodeServerDto struct {
	Name *string `json:"name" form:"name" validate:"omitempty,min=0,max=20"`
	Ip   *string `json:"ip" form:"ip" validate:"omitempty,min=0,max=64"`
}

type NodeServerCreateDto struct {
	Name                 *string `json:"name" form:"name" validate:"required,min=2,max=20"`
	Ip                   *string `json:"ip" form:"ip" validate:"required,ip|fqdn,min=4,max=64"`
	GrpcPort             *uint   `json:"grpcPort" form:"grpcPort" validate:"required,validatePort"`
	GrpcTLSServerName    *string `json:"grpcTlsServerName" form:"grpcTlsServerName" validate:"required,fqdn,min=4,max=253"`
	TrafficPeriod        *string `json:"trafficPeriod" form:"trafficPeriod" validate:"required,oneof=none day month year"`
	TrafficLimitMode     *string `json:"trafficLimitMode" form:"trafficLimitMode" validate:"required,oneof=combined separate"`
	TrafficTotalLimit    *uint64 `json:"trafficTotalLimit" form:"trafficTotalLimit" validate:"required"`
	TrafficUploadLimit   *uint64 `json:"trafficUploadLimit" form:"trafficUploadLimit" validate:"required"`
	TrafficDownloadLimit *uint64 `json:"trafficDownloadLimit" form:"trafficDownloadLimit" validate:"required"`
}

type NodeServerUpdateDto struct {
	RequiredIdDto
	Name                 *string `json:"name" form:"name" validate:"required,min=2,max=20"`
	Ip                   *string `json:"ip" form:"ip" validate:"required,ip|fqdn,min=4,max=64"`
	GrpcPort             *uint   `json:"grpcPort" form:"grpcPort" validate:"required,validatePort"`
	GrpcTLSServerName    *string `json:"grpcTlsServerName" form:"grpcTlsServerName" validate:"omitempty,fqdn,min=4,max=253"`
	TrafficPeriod        *string `json:"trafficPeriod" form:"trafficPeriod" validate:"required,oneof=none day month year"`
	TrafficLimitMode     *string `json:"trafficLimitMode" form:"trafficLimitMode" validate:"required,oneof=combined separate"`
	TrafficTotalLimit    *uint64 `json:"trafficTotalLimit" form:"trafficTotalLimit" validate:"required"`
	TrafficUploadLimit   *uint64 `json:"trafficUploadLimit" form:"trafficUploadLimit" validate:"required"`
	TrafficDownloadLimit *uint64 `json:"trafficDownloadLimit" form:"trafficDownloadLimit" validate:"required"`
}

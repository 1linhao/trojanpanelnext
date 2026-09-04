package dto

type KernelReleaseQueryDto struct {
	Kernel  string `form:"kernel" validate:"required,oneof=xray hysteria2"`
	Channel string `form:"channel" validate:"required,oneof=stable prerelease"`
	Refresh bool   `form:"refresh"`
}

type KernelInventoryQueryDto struct {
	NodeServerId uint `form:"nodeServerId" validate:"required"`
}

type KernelTargetDto struct {
	Kernel  string `json:"kernel" validate:"required,oneof=xray hysteria2"`
	Version string `json:"version" validate:"required,min=1,max=64"`
	Channel string `json:"channel" validate:"required,oneof=stable prerelease legacy"`
	Action  string `json:"action" validate:"omitempty,oneof=install rollback"`
}

type KernelTaskCreateDto struct {
	NodeServerIds      []uint            `json:"nodeServerIds" validate:"required,min=1,max=100,dive,gt=0"`
	CanaryNodeServerId uint              `json:"canaryNodeServerId"`
	Targets            []KernelTargetDto `json:"targets" validate:"required,min=1,max=2,dive"`
}

type KernelTaskPageDto struct {
	BaseDto
	Status string `form:"status" validate:"omitempty,oneof=queued running succeeded partial failed"`
}

type KernelTaskIdDto struct {
	Id uint64 `json:"id" form:"id" validate:"required"`
}

type KernelTaskRetryDto struct {
	Id      uint64   `json:"id" validate:"required"`
	ItemIds []uint64 `json:"itemIds" validate:"omitempty,dive,gt=0"`
}

type KernelMTLSProbeDto struct {
	NodeServerId uint   `json:"nodeServerId" validate:"required"`
	ServerName   string `json:"serverName" validate:"required,fqdn,min=4,max=253"`
}

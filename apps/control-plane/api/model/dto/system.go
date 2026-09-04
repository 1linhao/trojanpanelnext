package dto

type SystemUpdateDto struct {
	RequiredIdDto
	RegisterEnable              *uint `json:"registerEnable" form:"registerEnable" validate:"required,oneof=0 1"`
	RegisterQuota               *int  `json:"registerQuota" form:"registerQuota" validate:"required,gte=0,lte=1024000"`
	RegisterExpireDays          *uint `json:"registerExpireDays" form:"registerExpireDays" validate:"required,gte=0,lte=365"`
	ResetDownloadAndUploadMonth *uint `json:"resetDownloadAndUploadMonth" form:"resetDownloadAndUploadMonth" validate:"required,oneof=0 1"`
	TrafficRankEnable           *uint `json:"trafficRankEnable" form:"trafficRankEnable" validate:"required,oneof=0 1"`
	CaptchaEnable               *uint `json:"captchaEnable" form:"captchaEnable" validate:"required,oneof=0 1"`

	ExpireWarnEnable *uint   `json:"expireWarnEnable" form:"expireWarnEnable" redis:"expireWarnEnable" validate:"required,oneof=0 1"`
	ExpireWarnDay    *uint   `json:"expireWarnDay" form:"expireWarnDay" redis:"expireWarnDay" validate:"required,gte=0,lte=365"`
	EmailEnable      *uint   `json:"emailEnable" form:"emailEnable" redis:"emailEnable" validate:"required,oneof=0 1"`
	EmailHost        *string `json:"emailHost" form:"emailHost" validate:"omitempty,min=0,max=64"`
	EmailPort        *uint   `json:"emailPort" form:"emailPort" validate:"required,validatePort"`
	EmailUsername    *string `json:"emailUsername" form:"emailUsername" validate:"omitempty,min=0,max=32"`
	EmailPassword    *string `json:"emailPassword" form:"emailPassword" validate:"omitempty,min=0,max=32"`

	SystemName      *string `json:"systemName" form:"systemName" validate:"omitempty,min=0,max=32"`
	ClashRule       *string `json:"clashRule" form:"clashRule" validate:"omitempty,min=0,max=102400"`
	SingBoxTun      *string `json:"singBoxTun" form:"singBoxTun" validate:"omitempty,min=0,max=102400"`
	SingBoxOutbound *string `json:"singBoxOutbound" form:"singBoxOutbound" validate:"omitempty,min=0,max=102400"`
	XrayTemplate    *string `json:"xrayTemplate" form:"xrayTemplate" validate:"omitempty,min=0,max=10240"`

	ClashTemplateName           *string `json:"clashTemplateName" form:"clashTemplateName" validate:"omitempty,min=1,max=32"`
	SingBoxTunTemplateName      *string `json:"singBoxTunTemplateName" form:"singBoxTunTemplateName" validate:"omitempty,min=1,max=32"`
	SingBoxOutboundTemplateName *string `json:"singBoxOutboundTemplateName" form:"singBoxOutboundTemplateName" validate:"omitempty,min=1,max=32"`
	XrayTemplateName            *string `json:"xrayTemplateName" form:"xrayTemplateName" validate:"omitempty,min=1,max=32"`
}

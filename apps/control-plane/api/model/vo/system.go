package vo

type SystemVo struct {
	Id                          uint `json:"id" redis:"id"`
	RegisterEnable              uint `json:"registerEnable" redis:"registerEnable"`
	RegisterQuota               int  `json:"registerQuota" redis:"registerQuota"`
	RegisterExpireDays          uint `json:"registerExpireDays" redis:"registerExpireDays"`
	ResetDownloadAndUploadMonth uint `json:"resetDownloadAndUploadMonth" redis:"resetDownloadAndUploadMonth"`
	TrafficRankEnable           uint `json:"trafficRankEnable" redis:"trafficRankEnable"`
	CaptchaEnable               uint `json:"captchaEnable" redis:"captchaEnable"`

	ExpireWarnEnable uint   `json:"expireWarnEnable" redis:"expireWarnEnable"`
	ExpireWarnDay    uint   `json:"expireWarnDay" redis:"expireWarnDay"`
	EmailEnable      uint   `json:"emailEnable"`
	EmailHost        string `json:"emailHost" redis:"emailHost"`
	EmailPort        uint   `json:"emailPort" redis:"emailPort"`
	EmailUsername    string `json:"emailUsername" redis:"emailUsername"`
	EmailPassword    string `json:"emailPassword" redis:"emailPassword"`

	SystemName      string `json:"systemName" redis:"systemName"`
	ClashRule       string `json:"clashRule" redis:"clashRule"`
	SingBoxTun      string `json:"singBoxTun" redis:"singBoxTun"`
	SingBoxOutbound string `json:"singBoxOutbound" redis:"singBoxOutbound"`
	XrayTemplate    string `json:"xrayTemplate" redis:"xrayTemplate"`

	ClashTemplateName           string `json:"clashTemplateName" redis:"clashTemplateName"`
	SingBoxTunTemplateName      string `json:"singBoxTunTemplateName" redis:"singBoxTunTemplateName"`
	SingBoxOutboundTemplateName string `json:"singBoxOutboundTemplateName" redis:"singBoxOutboundTemplateName"`
	XrayTemplateName            string `json:"xrayTemplateName" redis:"xrayTemplateName"`
}

type ClientTemplateVo struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type ClientExportOptionVo struct {
	Id        string             `json:"id"`
	Name      string             `json:"name"`
	Templates []ClientTemplateVo `json:"templates"`
	Formats   []string           `json:"formats"`
}

type SettingVo struct {
	RegisterEnable     uint   `json:"registerEnable"`
	RegisterQuota      int    `json:"registerQuota"`
	RegisterExpireDays uint   `json:"registerExpireDays"`
	TrafficRankEnable  uint   `json:"trafficRankEnable"`
	CaptchaEnable      uint   `json:"captchaEnable" redis:"captchaEnable"`
	EmailEnable        uint   `json:"emailEnable"`
	SystemName         string `json:"systemName" redis:"systemName"`
}

package bo

type SystemAccountConfigBo struct {
	RegisterEnable              uint `json:"registerEnable"`              // 是否开放注册
	RegisterQuota               int  `json:"registerQuota"`               // 注册用户默认配额 单位/MB
	RegisterExpireDays          uint `json:"registerExpireDays"`          // 注册用户过期天数 单位/天
	ResetDownloadAndUploadMonth uint `json:"resetDownloadAndUploadMonth"` // 是否每月重设下载和上传流量
	TrafficRankEnable           uint `json:"trafficRankEnable"`           // 是否开启流量排行
	CaptchaEnable               uint `json:"captchaEnable"`               // 是否开启验证码登录
}

type SystemEmailConfigBo struct {
	ExpireWarnEnable uint   `json:"expireWarnEnable"` // 是否开启到期警告 0/否 1/是
	ExpireWarnDay    uint   `json:"expireWarnDay"`    // 到期警告 单位/天
	EmailEnable      uint   `json:"emailEnable"`      // 是否开启邮箱功能 0/否 1/是
	EmailHost        string `json:"emailHost"`        // 系统邮箱设置-host
	EmailPort        uint   `json:"emailPort"`        // 系统邮箱设置-port
	EmailUsername    string `json:"emailUsername"`    // 系统邮箱设置-username
	EmailPassword    string `json:"emailPassword"`    // 系统邮箱设置-password
}

type SystemTemplateConfigBo struct {
	SystemName                  string `json:"systemName" redis:"systemName"`
	ClashTemplateName           string `json:"clashTemplateName" redis:"clashTemplateName"`
	SingBoxTunTemplateName      string `json:"singBoxTunTemplateName" redis:"singBoxTunTemplateName"`
	SingBoxOutboundTemplateName string `json:"singBoxOutboundTemplateName" redis:"singBoxOutboundTemplateName"`
	XrayTemplateName            string `json:"xrayTemplateName" redis:"xrayTemplateName"`
}

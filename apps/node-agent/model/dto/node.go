package dto

type NodeAddDto struct {
	NodeTypeId uint   `json:"nodeTypeId" form:"nodeTypeId" validate:"required"`
	Port       uint   `json:"port" form:"port" validate:"omitempty"`
	Domain     string `json:"domain" form:"domain" validate:"omitempty"`

	XrayTemplate       string `json:"xrayTemplate" form:"xrayTemplate" validate:"omitempty"`
	XrayFlow           string `json:"xrayFlow" form:"xrayFlow" validate:"omitempty"`
	XraySSMethod       string `json:"xraySSMethod" form:"xraySSMethod" validate:"omitempty"`
	XrayProtocol       string `json:"xrayProtocol" form:"xrayProtocol" validate:"omitempty"`
	XraySettings       string `json:"xraySettings" form:"xraySettings" validate:"omitempty"`
	XrayStreamSettings string `json:"xrayStreamSettings" form:"xrayStreamSettings" validate:"omitempty"`
	XrayTag            string `json:"xrayTag" form:"xrayTag" validate:"omitempty"`
	XraySniffing       string `json:"xraySniffing" form:"xraySniffing" validate:"omitempty"`
	XrayAllocate       string `json:"xrayAllocate" form:"xrayAllocate" validate:"omitempty"`

	Hysteria2ObfsPassword string `json:"hysteria2ObfsPassword" form:"hysteria2ObfsPassword" validate:"omitempty"`
	Hysteria2UpMbps       int    `json:"hysteria2UpMbps" form:"hysteria2UpMbps" validate:"omitempty"`
	Hysteria2DownMbps     int    `json:"hysteria2DownMbps" form:"hysteria2DownMbps" validate:"omitempty"`
}

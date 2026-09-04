package vo

import "time"

// NodeVo 查询分页Node对象
type NodeVo struct {
	Id              uint      `json:"id"`
	NodeServerId    uint      `json:"nodeServerId"`
	NodeSubId       uint      `json:"nodeSubId"`
	NodeTypeId      uint      `json:"nodeTypeId"`
	Name            string    `json:"name"`
	Domain          string    `json:"domain"`
	Port            uint      `json:"port"`
	Priority        int       `json:"priority"`
	Clients         []string  `json:"clients"`
	NaiveUotEnable  uint      `json:"naiveUotEnable"`
	NaiveUotVersion uint      `json:"naiveUotVersion"`
	CreateTime      time.Time `json:"createTime"`

	Status        int                   `json:"status"`
	ServerTraffic ServerTrafficStatusVo `json:"serverTraffic"`
}

type NodePageVo struct {
	Nodes []NodeVo `json:"nodes"`
	BaseVoPage
}

// NodeOneVo 查询单个Node对象
type NodeOneVo struct {
	Id              uint      `json:"id"`
	NodeServerId    uint      `json:"nodeServerId"`
	NodeSubId       uint      `json:"nodeSubId"`
	NodeTypeId      uint      `json:"nodeTypeId"`
	Name            string    `json:"name"`
	Domain          string    `json:"domain"`
	Port            uint      `json:"port"`
	Priority        int       `json:"priority"`
	Clients         []string  `json:"clients"`
	NaiveUotEnable  uint      `json:"naiveUotEnable"`
	NaiveUotVersion uint      `json:"naiveUotVersion"`
	CreateTime      time.Time `json:"createTime"`

	Password string `json:"password"`
	Uuid     string `json:"uuid"`
	AlterId  int    `json:"alterId"`

	XrayProtocol             string                   `json:"xrayProtocol"`
	XrayFlow                 string                   `json:"xrayFlow"`
	XraySSMethod             string                   `json:"xraySSMethod"`
	XrayUotEnable            uint                     `json:"xrayUotEnable"`
	XrayUotVersion           uint                     `json:"xrayUotVersion"`
	XrayXudpEnable           uint                     `json:"xrayXudpEnable"`
	XrayMuxEnable            uint                     `json:"xrayMuxEnable"`
	RealityPbk               string                   `json:"realityPbk"`
	XraySettings             string                   `json:"xraySettings"`
	XraySettingEntity        XraySettingEntity        `json:"xraySettingsEntity"`
	XrayStreamSettingsEntity XrayStreamSettingsEntity `json:"xrayStreamSettingsEntity"`
	XrayTag                  string                   `json:"xrayTag"`
	XraySniffing             string                   `json:"xraySniffing"`
	XrayAllocate             string                   `json:"xrayAllocate"`

	TrojanGoSni             string `json:"trojanGoSni"`
	TrojanGoMuxEnable       uint   `json:"trojanGoMuxEnable"`
	TrojanGoWebsocketEnable uint   `json:"trojanGoWebsocketEnable"`
	TrojanGoWebsocketPath   string `json:"trojanGoWebsocketPath"`
	TrojanGoWebsocketHost   string `json:"trojanGoWebsocketHost"`
	TrojanGoSsEnable        uint   `json:"trojanGoSsEnable"`
	TrojanGoSsMethod        string `json:"trojanGoSsMethod"`
	TrojanGoSsPassword      string `json:"trojanGoSsPassword"`

	HysteriaProtocol   string `json:"hysteriaProtocol"`
	HysteriaObfs       string `json:"hysteriaObfs"`
	HysteriaUpMbps     int    `json:"hysteriaUpMbps"`
	HysteriaDownMbps   int    `json:"hysteriaDownMbps"`
	HysteriaServerName string `json:"hysteriaServerName"`
	HysteriaInsecure   uint   `json:"hysteriaInsecure"`
	HysteriaFastOpen   uint   `json:"hysteriaFastOpen"`

	Hysteria2ObfsPassword string `json:"hysteria2ObfsPassword"`
	Hysteria2UpMbps       int    `json:"hysteria2UpMbps"`
	Hysteria2DownMbps     int    `json:"hysteria2DownMbps"`
	Hysteria2ServerName   string `json:"hysteria2ServerName"`
	Hysteria2Insecure     uint   `json:"hysteria2Insecure"`
	Hysteria2FastOpen     uint   `json:"hysteria2FastOpen"`
	Hysteria2PortHopping  string `json:"hysteria2PortHopping"`
	Hysteria2HopInterval  uint   `json:"hysteria2HopInterval"`

	NaiveProxyUsername string `json:"naiveProxyUsername"`
}

type XrayStreamSettingsEntity struct {
	Network         string                                  `json:"network"`
	Security        string                                  `json:"security"`
	TlsSettings     XrayStreamSettingsTlsSettingsEntity     `json:"tlsSettings"`
	RealitySettings XrayStreamSettingsRealitySettingsEntity `json:"realitySettings"`
	WsSettings      XrayStreamSettingsWsSettingsEntity      `json:"wsSettings"`
}

type XraySettingEntity struct {
	Fallbacks []XrayFallback     `json:"fallbacks"`
	Network   string             `json:"network"`
	Udp       bool               `json:"udp"`
	Accounts  []XraySocksAccount `json:"accounts"`
}

type XraySocksAccount struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type XrayFallback struct {
	Name *string `json:"name"`
	Alpn *string `json:"alpn"`
	Path *string `json:"path"`
	Dest any     `json:"dest"`
	Xver *uint   `json:"xver"`
}

type XrayStreamSettingsTlsSettingsEntity struct {
	ServerName    string   `json:"serverName"`
	Alpn          []string `json:"alpn"`
	AllowInsecure bool     `json:"allowInsecure"`
	Fingerprint   string   `json:"fingerprint"`
}

type XrayStreamSettingsRealitySettingsEntity struct {
	Dest        string   `json:"dest"`
	Xver        int      `json:"xver"`
	ServerNames []string `json:"serverNames"`
	Fingerprint string   `json:"fingerprint"`
	PrivateKey  string   `json:"privateKey"`
	ShortIds    []string `json:"shortIds"`
	SpiderX     string   `json:"spiderX"`
}

type XrayStreamSettingsWsSettingsEntity struct {
	Path    string            `json:"path"`
	Headers map[string]string `json:"headers"`
}

type NodeDefaultVo struct {
	PublicKey  string `json:"publicKey"`
	PrivateKey string `json:"privateKey"`
	ShortId    string `json:"shortId"`
	SpiderX    string `json:"spiderX"`
}

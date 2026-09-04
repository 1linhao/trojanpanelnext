package bo

type StreamSettings struct {
	Network         string          `json:"network"`
	Security        string          `json:"security"`
	TlsSettings     TlsSettings     `json:"tlsSettings"`
	RealitySettings RealitySettings `json:"realitySettings"`
	WsSettings      WsSettings      `json:"wsSettings"`
}

type TlsSettings struct {
	Certificates  []Certificate `json:"certificates"`
	Fingerprint   string        `json:"fingerprint"`
	ServerName    string        `json:"serverName"`
	Alpn          []string      `json:"alpn"`
	AllowInsecure bool          `json:"allowInsecure"`
}

type Certificate struct {
	CertificateFile string `json:"certificateFile"`
	KeyFile         string `json:"keyFile"`
}

type RealitySettings struct {
	Dest        string   `json:"dest"`
	Xver        int      `json:"xver"`
	ServerNames []string `json:"serverNames"`
	Fingerprint string   `json:"fingerprint"`
	PrivateKey  string   `json:"privateKey"`
	ShortIds    []string `json:"shortIds"`
	SpiderX     string   `json:"spiderX"`
}

type WsSettings struct {
	Path    string         `json:"path"`
	Headers WsSettingsHost `json:"headers"`
}

type WsSettingsHost struct {
	Host string `json:"Host"`
}

type Settings struct {
	Encryption string         `json:"encryption"`
	Accounts   []SocksAccount `json:"accounts"`
	Udp        bool           `json:"udp"`
}

type SocksAccount struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

type XrayConfigBo struct {
	Log       TypeMessage `json:"log"`
	API       TypeMessage `json:"api"`
	DNS       TypeMessage `json:"dns"`
	Routing   TypeMessage `json:"routing"`
	Policy    TypeMessage `json:"policy"`
	Inbounds  TypeMessage `json:"inbounds"`
	Outbounds TypeMessage `json:"outbounds"`
	Transport TypeMessage `json:"transport"`
	Stats     TypeMessage `json:"stats"`
	Reverse   TypeMessage `json:"reverse"`
	FakeDNS   TypeMessage `json:"fakeDns"`
}

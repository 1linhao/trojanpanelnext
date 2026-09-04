package bo

type ClashConfigInterface interface {
	Vless | Vmess | Trojan | TrojanGo | Hysteria
}

type Vless struct {
	Name              string      `yaml:"name"`
	Type              string      `yaml:"type"`
	Server            string      `yaml:"server"`
	Port              uint        `yaml:"port"`
	Uuid              string      `yaml:"uuid"`
	Network           string      `yaml:"network"`
	Tls               bool        `yaml:"tls"`
	Udp               bool        `yaml:"udp"`
	Flow              string      `yaml:"flow"`
	ClientFingerprint string      `yaml:"client-fingerprint"`
	ServerName        string      `yaml:"servername"`
	SkipCertVerify    bool        `yaml:"skip-cert-verify,omitempty"`
	RealityOpts       RealityOpts `yaml:"reality-opts,omitempty"`
	WsOpts            WsOpts      `yaml:"ws-opts,omitempty"`
	PacketEncoding    string      `yaml:"packet-encoding,omitempty"`
	Smux              *Smux       `yaml:"smux,omitempty"`
}

type RealityOpts struct {
	PublicKey string `yaml:"public-key"`
	ShortId   string `yaml:"short-id"`
}

type Vmess struct {
	Name              string `yaml:"name"`
	Type              string `yaml:"type"`
	Server            string `yaml:"server"`
	Port              uint   `yaml:"port"`
	Uuid              string `yaml:"uuid"`
	AlterId           uint   `yaml:"alterId"`
	Cipher            string `yaml:"cipher"`
	Udp               bool   `yaml:"udp,omitempty"`
	Tls               bool   `yaml:"tls,omitempty"`
	ClientFingerprint string `yaml:"client-fingerprint,omitempty"`
	SkipCertVerify    bool   `yaml:"skip-cert-verify,omitempty"`
	ServerName        string `yaml:"servername,omitempty"`
	Network           string `yaml:"network,omitempty"`
	WsOpts            WsOpts `yaml:"ws-opts,omitempty"`
	PacketEncoding    string `yaml:"packet-encoding,omitempty"`
	Smux              *Smux  `yaml:"smux,omitempty"`
}

type Trojan struct {
	Name              string   `yaml:"name"`
	Type              string   `yaml:"type"`
	Server            string   `yaml:"server"`
	Port              uint     `yaml:"port"`
	Password          string   `yaml:"password"`
	ClientFingerprint string   `yaml:"client-fingerprint,omitempty"`
	Udp               bool     `yaml:"udp,omitempty"`
	Sni               string   `yaml:"sni,omitempty"`
	SkipCertVerify    bool     `yaml:"skip-cert-verify,omitempty"`
	Alpn              []string `yaml:"alpn,omitempty"`
	WsOpts            WsOpts   `yaml:"ws-opts,omitempty"`
	Smux              *Smux    `yaml:"smux,omitempty"`
}

type Shadowsocks struct {
	Name              string `yaml:"name"`
	Type              string `yaml:"type"`
	Server            string `yaml:"server"`
	Port              uint   `yaml:"port"`
	Cipher            string `yaml:"cipher"`
	Password          string `yaml:"password"`
	Udp               bool   `yaml:"udp"`
	UdpOverTcp        bool   `yaml:"udp-over-tcp,omitempty"`
	UdpOverTcpVersion uint   `yaml:"udp-over-tcp-version,omitempty"`
}

type Smux struct {
	Enabled  bool   `yaml:"enabled"`
	Protocol string `yaml:"protocol,omitempty"`
}

type Socks struct {
	Name           string `yaml:"name"`
	Type           string `yaml:"type"`
	Server         string `yaml:"server"`
	Port           uint   `yaml:"port"`
	Username       string `yaml:"username,omitempty"`
	Password       string `yaml:"password,omitempty"`
	Tls            bool   `yaml:"tls,omitempty"`
	Fingerprint    string `yaml:"fingerprint,omitempty"`
	SkipCertVerify bool   `yaml:"skip-cert-verify,omitempty"`
	Udp            bool   `yaml:"udp,omitempty"`
	IpVersion      string `yaml:"ip-version,omitempty"`
}

type TrojanGo struct {
	Name     string `yaml:"name"`
	Type     string `yaml:"type"`
	Server   string `yaml:"server"`
	Port     uint   `yaml:"port"`
	Password string `yaml:"password"`
	SNI      string `yaml:"sni"`
	Udp      bool   `yaml:"udp"`
	Network  string `yaml:"network"`
	WsOpts   WsOpts `yaml:"ws-opts,omitempty"`
}

type Hysteria struct {
	Name           string `yaml:"name"`
	Type           string `yaml:"type"`
	Server         string `yaml:"server"`
	Port           uint   `yaml:"port"`
	AuthStr        string `yaml:"auth-str,omitempty"`
	Obfs           string `yaml:"obfs,omitempty"`
	Protocol       string `yaml:"protocol"`
	Up             int    `yaml:"up"`
	Down           int    `yaml:"down"`
	Sni            string `yaml:"sni,omitempty"`
	SkipCertVerify bool   `yaml:"skip-cert-verify,omitempty"`
	FastOpen       bool   `yaml:"fast-open,omitempty"`
}

type Hysteria2 struct {
	Name           string `yaml:"name"`
	Type           string `yaml:"type"`
	Server         string `yaml:"server"`
	Port           uint   `yaml:"port"`
	Ports          string `yaml:"ports,omitempty"`
	HopInterval    uint   `yaml:"hop-interval,omitempty"`
	Up             int    `yaml:"up,omitempty"`
	Down           int    `yaml:"down,omitempty"`
	Password       string `yaml:"password"`
	Obfs           string `yaml:"obfs,omitempty"`
	ObfsPassword   string `yaml:"obfs-password,omitempty"`
	Sni            string `yaml:"sni,omitempty"`
	SkipCertVerify bool   `yaml:"skip-cert-verify,omitempty"`
}

type WsOpts struct {
	Path    string        `yaml:"path"`
	Headers WsOptsHeaders `yaml:"headers"`
}

type WsOptsHeaders struct {
	Host string `yaml:"Host"`
}

type ProxyGroup struct {
	Name    string   `yaml:"name"`
	Type    string   `yaml:"type"`
	Proxies []string `yaml:"proxies"`
}

type ClashConfig struct {
	Proxies     []interface{} `yaml:"proxies"`
	ProxyGroups []ProxyGroup  `yaml:"proxy-groups"`
}

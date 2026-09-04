package clientcompat

import (
	"encoding/base64"
	"encoding/json"
	"net"
	"net/url"
	"strconv"
	"strings"
)

const (
	v2RayNConfigTypeHysteria2 = 7
	v2RayNConfigVersion       = 4

	// V2RaySubscriptionStandard selects URI formats understood by released
	// v2rayNG versions. Unknown clients intentionally use this safer fallback.
	V2RaySubscriptionStandard = "v2ray-standard"
	// V2RaySubscriptionV2RayN selects v2rayN's richer inner URI format.
	V2RaySubscriptionV2RayN = "v2rayn"
)

type V2RayNHysteria2Config struct {
	Remarks        string
	Address        string
	Port           uint
	Password       string
	SNI            string
	AllowInsecure  bool
	SalamanderPass string
	UpMbps         int
	DownMbps       int
	Ports          string
	HopInterval    string
}

type v2RayNProtocolExtra struct {
	SalamanderPass string `json:"SalamanderPass,omitempty"`
	UpMbps         int    `json:"UpMbps"`
	DownMbps       int    `json:"DownMbps"`
	Ports          string `json:"Ports,omitempty"`
	HopInterval    string `json:"HopInterval,omitempty"`
}

type v2RayNHysteria2Profile struct {
	ConfigType     int                 `json:"ConfigType"`
	ConfigVersion  int                 `json:"ConfigVersion"`
	Remarks        string              `json:"Remarks"`
	Address        string              `json:"Address"`
	Port           uint                `json:"Port"`
	Password       string              `json:"Password"`
	StreamSecurity string              `json:"StreamSecurity"`
	AllowInsecure  string              `json:"AllowInsecure"`
	SNI            string              `json:"Sni,omitempty"`
	ProtoExtra     v2RayNProtocolExtra `json:"ProtoExtraObj"`
}

// V2RaySubscriptionFormat selects the dialect for the shared V2Ray
// subscription URL. v2rayNG must be checked first because its name contains
// the "v2rayn" prefix.
func V2RaySubscriptionFormat(userAgent string) string {
	userAgent = strings.ToLower(strings.TrimSpace(userAgent))
	if strings.Contains(userAgent, "v2rayng") {
		return V2RaySubscriptionStandard
	}
	if strings.Contains(userAgent, "v2rayn") {
		return V2RaySubscriptionV2RayN
	}
	return V2RaySubscriptionStandard
}

// V2RayNGHysteria2URI uses the standard Hysteria2 URI understood by released
// v2rayNG versions. Port hopping stays in query parameters so java.net.URI can
// parse the authority's port as a number.
func V2RayNGHysteria2URI(config V2RayNHysteria2Config) string {
	insecure := "0"
	if config.AllowInsecure {
		insecure = "1"
	}
	query := url.Values{
		"insecure": {insecure},
		"upmbps":   {strconv.Itoa(config.UpMbps)},
		"downmbps": {strconv.Itoa(config.DownMbps)},
	}
	if config.Ports != "" {
		query.Set("mport", config.Ports)
	}
	if config.HopInterval != "" {
		query.Set("mportHopInt", config.HopInterval)
	}
	if config.SalamanderPass != "" {
		query.Set("obfs", "salamander")
		query.Set("obfs-password", config.SalamanderPass)
	}
	if config.SNI != "" {
		query.Set("sni", config.SNI)
	}
	return (&url.URL{
		Scheme:   "hysteria2",
		User:     url.User(config.Password),
		Host:     net.JoinHostPort(config.Address, strconv.FormatUint(uint64(config.Port), 10)),
		RawQuery: query.Encode(),
		Fragment: config.Remarks,
	}).String()
}

// V2RayNHysteria2URI uses v2rayN's inner URI format because its standard
// hysteria2:// parser intentionally omits per-node bandwidth and hop interval.
func V2RayNHysteria2URI(config V2RayNHysteria2Config) (string, error) {
	allowInsecure := "false"
	if config.AllowInsecure {
		allowInsecure = "true"
	}
	profile := v2RayNHysteria2Profile{
		ConfigType:     v2RayNConfigTypeHysteria2,
		ConfigVersion:  v2RayNConfigVersion,
		Remarks:        config.Remarks,
		Address:        config.Address,
		Port:           config.Port,
		Password:       config.Password,
		StreamSecurity: "tls",
		AllowInsecure:  allowInsecure,
		SNI:            config.SNI,
		ProtoExtra: v2RayNProtocolExtra{
			SalamanderPass: config.SalamanderPass,
			UpMbps:         config.UpMbps,
			DownMbps:       config.DownMbps,
			Ports:          config.Ports,
			HopInterval:    config.HopInterval,
		},
	}
	content, err := json.Marshal(profile)
	if err != nil {
		return "", err
	}
	return "v2rayn://hysteria2/" + base64.RawURLEncoding.EncodeToString(content), nil
}

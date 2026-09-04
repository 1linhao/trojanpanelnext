package bo

import (
	"encoding/base64"
	"strings"
)

type NaiveProxyConfig struct {
	Admin   TypeMessage `json:"admin"`
	Logging TypeMessage `json:"logging"`
	Apps    Apps        `json:"apps"`
}

type Apps struct {
	Http Http `json:"http"`
	Tls  Tls  `json:"tls"`
}

type Http struct {
	Servers Servers `json:"servers"`
}

type Servers struct {
	Srv0 Srv0 `json:"srv0"`
}

type Srv0 struct {
	Listen                []string    `json:"listen"`
	Routes                []Route     `json:"routes"`
	TlsConnectionPolicies TypeMessage `json:"tls_connection_policies"`
	AutomaticHttps        TypeMessage `json:"automatic_https"`
}

type Tls struct {
	TlsCertificate TlsCertificate `json:"certificates"`
}

type TlsCertificate struct {
	LoadFiles []LoadFile `json:"load_files"`
}

type LoadFile struct {
	Certificate string `json:"certificate"`
	Key         string `json:"key"`
}

type Route struct {
	Handle []Handle `json:"handle"`
}
type Handle struct {
	Handler      TypeMessage   `json:"handler"`
	HandleRoutes []TypeMessage `json:"routes"`
}

type RouteHandle struct {
	Handle []HandleAuth `json:"handle"`
}

type HandleAuth struct {
	AuthCredentials    [][]byte    `json:"auth_credentials,omitempty"`
	AuthPassDeprecated string      `json:"auth_pass_deprecated,omitempty"`
	AuthUserDeprecated string      `json:"auth_user_deprecated,omitempty"`
	Handler            TypeMessage `json:"handler"`
	HideIp             TypeMessage `json:"hide_ip"`
	HideVia            TypeMessage `json:"hide_via"`
	ProbeResistance    TypeMessage `json:"probe_resistance"`
}

func (h HandleAuth) Pass() string {
	if h.AuthPassDeprecated != "" {
		return h.AuthPassDeprecated
	}
	for _, credential := range h.AuthCredentials {
		raw := make([]byte, base64.StdEncoding.DecodedLen(len(credential)))
		n, err := base64.StdEncoding.Decode(raw, credential)
		if err != nil {
			continue
		}
		parts := strings.SplitN(string(raw[:n]), ":", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

type NaiveProxyUserTraffic struct {
	Rx uint64 `json:"rx"`
	Tx uint64 `json:"tx"`
}

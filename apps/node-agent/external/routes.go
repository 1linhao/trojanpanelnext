package external

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	"trojan-panel-core/internal/atomicfile"
	"trojan-panel-core/kernelconfig"
	"trojan-panel-core/model/constant"
)

// Manifest version of routes.json. Bump it when a field changes meaning.
const manifestVersion = 1

// apiPortOffset is the offset the node agent adds to a node port to obtain the
// Xray API port, as used by app.StartApp for every kernel type.
const apiPortOffset uint = 30000

// Route describes one kernel listener for observation, firewall validation and
// optional plain-HTTP fallback provisioning. It is not an instruction to add a
// reverse-proxy or stream forwarding rule.
type Route struct {
	Kernel                   string `json:"kernel"`
	Protocol                 string `json:"protocol"`
	APIPort                  uint   `json:"api_port"`
	Port                     uint   `json:"port"`
	Network                  string `json:"network"`
	Security                 string `json:"security"`
	SNI                      string `json:"sni,omitempty"`
	WSPath                   string `json:"ws_path,omitempty"`
	GRPCServiceName          string `json:"grpc_service_name,omitempty"`
	FallbackPort             uint   `json:"fallback_port,omitempty"`
	ExternalFallbackRequired bool   `json:"external_fallback_listener_required"`
	TLSManagedByKernel       bool   `json:"tls_terminated_by_kernel"`
}

// Certificates lists the TLS material the kernels read at startup.
type Certificates struct {
	CrtPath string `json:"crt_path"`
	KeyPath string `json:"key_path"`
}

// Manifest is the machine readable contract written to routes.json.
type Manifest struct {
	SchemaVersion   int          `json:"schema_version"`
	GeneratedAt     string       `json:"generated_at"`
	CoreVersion     string       `json:"core_version"`
	APIPortOffset   uint         `json:"api_port_offset"`
	CamouflageRoot  string       `json:"camouflage_root"`
	Certificates    Certificates `json:"certificates"`
	Routes          []Route      `json:"routes"`
	NoConfigMessage string       `json:"error,omitempty"`
}

// WriteRoutes renders the kernel-listener manifest consumed by EntryController.
// Failures are reported to the caller so the node agent can log them without
// failing a node operation.
func WriteRoutes(purposeDomain, crtPath, keyPath string) error {
	manifest, err := buildManifest(constant.CoreBasePath, purposeDomain, crtPath, keyPath)
	if err != nil {
		return err
	}
	return writeRoutesFile(RoutesPath(), manifest)
}

// RoutesPath is the manifest location. TP_EXTERNAL_DIR overrides it so the
// contract can live outside the default mount.
func RoutesPath() string {
	if dir := strings.TrimSpace(os.Getenv("TP_EXTERNAL_DIR")); dir != "" {
		return filepath.Join(dir, "routes.json")
	}
	return constant.ExternalRoutesFile
}

// writeRoutesFile renders manifest and replaces path atomically, so a reader
// never observes a half written manifest.
func writeRoutesFile(path string, manifest Manifest) error {
	content, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	content = append(content, '\n')

	return atomicfile.Write(path, content, 0644)
}

// buildManifest scans the kernel configuration directories that the node agent
// keeps on disk. The kernel configuration files are the only source of truth for
// the ports and transports a node currently serves.
func buildManifest(basePath, purposeDomain, crtPath, keyPath string) (Manifest, error) {
	manifest := Manifest{
		SchemaVersion:  manifestVersion,
		GeneratedAt:    time.Now().UTC().Format(time.RFC3339),
		CoreVersion:    constant.TrojanPanelCoreVersion,
		APIPortOffset:  apiPortOffset,
		CamouflageRoot: constant.WebFilePath,
		Certificates: Certificates{
			CrtPath: crtPath,
			KeyPath: keyPath,
		},
		Routes: make([]Route, 0),
	}

	xrayPath := filepath.Join(basePath, constant.XrayPath)
	naiveProxyPath := filepath.Join(basePath, constant.NaiveProxyPath)
	hysteria2Path := filepath.Join(basePath, constant.Hysteria2Path)

	manifest.Routes = append(manifest.Routes, xrayRoutes(xrayPath, purposeDomain)...)
	manifest.Routes = append(manifest.Routes, naiveProxyRoutes(naiveProxyPath, purposeDomain)...)
	manifest.Routes = append(manifest.Routes, hysteria2Routes(hysteria2Path, purposeDomain)...)

	sort.Slice(manifest.Routes, func(i, j int) bool {
		return manifest.Routes[i].Port < manifest.Routes[j].Port
	})
	return manifest, nil
}

// xrayRoutes inspects every generated Xray configuration. Each file holds the
// API inbound on 127.0.0.1 plus one proxy inbound on all interfaces.
func xrayRoutes(xrayPath, purposeDomain string) []Route {
	routes := make([]Route, 0)
	entries, err := os.ReadDir(xrayPath)
	if err != nil {
		return routes
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		apiPortValue, protocol, ok := kernelconfig.ParseXrayConfigName(entry.Name())
		if !ok {
			continue
		}
		apiPort := parseUint(apiPortValue)
		config, err := readXrayConfig(filepath.Join(xrayPath, entry.Name()))
		if err != nil {
			logrus.Errorf("external: read xray config %s err: %v", entry.Name(), err)
			continue
		}
		route := Route{
			Kernel:   "xray",
			Protocol: protocol,
			APIPort:  apiPort,
		}
		for _, inbound := range config.Inbounds {
			if inbound.Protocol == "dokodemo-door" || inbound.Port == apiPort {
				continue
			}
			route.Port = inbound.Port
			route.Network = inbound.StreamSettings.Network
			route.Security = inbound.StreamSettings.Security
			route.SNI = inbound.StreamSettings.TLSSettings.ServerName
			route.WSPath = inbound.StreamSettings.WSSettings.Path
			route.GRPCServiceName = grpcServiceName(inbound.StreamSettings.GRPCSettings)
			route.FallbackPort = fallbackPort(inbound.Settings)
			break
		}
		if route.SNI == "" {
			route.SNI = purposeDomain
		}
		// The kernel terminates TLS itself and answers unknown TLS traffic with a
		// fallback to the camouflage site, so an ingress provider must not terminate
		// this protocol TLS and only needs to supply the declared plain fallback.
		route.TLSManagedByKernel = route.Security == "tls" || route.Security == "reality"
		route.ExternalFallbackRequired = route.FallbackPort != 0 && route.TLSManagedByKernel && route.Network == "tcp" &&
			(route.Protocol == "vless" || route.Protocol == "trojan")
		routes = append(routes, route)
	}
	return routes
}

// naiveProxyRoutes inspects the NaiveProxy configurations. NaiveProxy always
// terminates TLS and serves the camouflage site itself for requests without a
// proxy protocol, so no separate fallback listener is needed.
func naiveProxyRoutes(naiveProxyPath, purposeDomain string) []Route {
	routes := make([]Route, 0)
	entries, err := os.ReadDir(naiveProxyPath)
	if err != nil {
		return routes
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		apiPortValue, ok := kernelconfig.ParseSingleConfigName(entry.Name())
		if !ok {
			continue
		}
		port := portFromNaiveProxy(filepath.Join(naiveProxyPath, entry.Name()))
		if port == 0 {
			continue
		}
		routes = append(routes, Route{
			Kernel:             "naiveproxy",
			Protocol:           "naiveproxy",
			APIPort:            parseUint(apiPortValue),
			Port:               port,
			Network:            "tcp",
			Security:           "tls",
			SNI:                purposeDomain,
			TLSManagedByKernel: true,
		})
	}
	return routes
}

// hysteria2Routes inspects the Hysteria2 configurations. Hysteria2 is QUIC only,
// so it remains a direct UDP listener and terminates its own TLS.
func hysteria2Routes(hysteria2Path, purposeDomain string) []Route {
	routes := make([]Route, 0)
	entries, err := os.ReadDir(hysteria2Path)
	if err != nil {
		return routes
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		apiPortValue, ok := kernelconfig.ParseSingleConfigName(entry.Name())
		if !ok {
			continue
		}
		port := portFromHysteria2(filepath.Join(hysteria2Path, entry.Name()))
		if port == 0 {
			continue
		}
		routes = append(routes, Route{
			Kernel:             "hysteria2",
			Protocol:           "hysteria2",
			APIPort:            parseUint(apiPortValue),
			Port:               port,
			Network:            "udp",
			Security:           "tls",
			SNI:                purposeDomain,
			TLSManagedByKernel: true,
		})
	}
	return routes
}

type xrayConfig struct {
	Inbounds []xrayInbound `json:"inbounds"`
}

type xrayInbound struct {
	Protocol       string             `json:"protocol"`
	Port           uint               `json:"port"`
	Settings       json.RawMessage    `json:"settings"`
	StreamSettings xrayStreamSettings `json:"streamSettings"`
}

type xrayStreamSettings struct {
	Network     string `json:"network"`
	Security    string `json:"security"`
	TLSSettings struct {
		ServerName string `json:"serverName"`
	} `json:"tlsSettings"`
	WSSettings struct {
		Path string `json:"path"`
	} `json:"wsSettings"`
	GRPCSettings json.RawMessage `json:"grpcSettings"`
}

type xraySettings struct {
	Fallbacks []struct {
		Dest uint `json:"dest"`
	} `json:"fallbacks"`
}

func readXrayConfig(path string) (xrayConfig, error) {
	var config xrayConfig
	content, err := os.ReadFile(path)
	if err != nil {
		return config, err
	}
	err = json.Unmarshal(content, &config)
	return config, err
}

// fallbackPort reports the camouflage site port the kernel falls back to when a
// TLS handshake carries no recognised proxy protocol.
func fallbackPort(raw json.RawMessage) uint {
	if len(raw) == 0 {
		return 0
	}
	var settings xraySettings
	if err := json.Unmarshal(raw, &settings); err != nil || len(settings.Fallbacks) == 0 {
		return 0
	}
	return settings.Fallbacks[0].Dest
}

// grpcServiceName accepts both the upstream object form and a bare string.
func grpcServiceName(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var settings struct {
		ServiceName string `json:"serviceName"`
	}
	if err := json.Unmarshal(raw, &settings); err == nil && settings.ServiceName != "" {
		return settings.ServiceName
	}
	var name string
	if err := json.Unmarshal(raw, &name); err == nil {
		return name
	}
	return ""
}

type naiveProxyConfig struct {
	Apps struct {
		HTTP struct {
			Servers map[string]struct {
				Listen []string `json:"listen"`
			} `json:"servers"`
		} `json:"http"`
	} `json:"apps"`
}

func portFromNaiveProxy(path string) uint {
	var config naiveProxyConfig
	content, err := os.ReadFile(path)
	if err != nil {
		logrus.Errorf("external: read naiveproxy config %s err: %v", filepath.Base(path), err)
		return 0
	}
	if err := json.Unmarshal(content, &config); err != nil {
		logrus.Errorf("external: parse naiveproxy config %s err: %v", filepath.Base(path), err)
		return 0
	}
	for _, server := range config.Apps.HTTP.Servers {
		for _, listen := range server.Listen {
			if port := portFromListen(listen); port != 0 {
				return port
			}
		}
	}
	return 0
}

// hysteria2Config mirrors the generated document: a single listen string such as
// ":10004" plus a TLS block the kernel reads itself.
type hysteria2Config struct {
	Listen string `json:"listen"`
}

func portFromHysteria2(path string) uint {
	var config hysteria2Config
	content, err := os.ReadFile(path)
	if err != nil {
		logrus.Errorf("external: read hysteria2 config %s err: %v", filepath.Base(path), err)
		return 0
	}
	if err := json.Unmarshal(content, &config); err != nil {
		logrus.Errorf("external: parse hysteria2 config %s err: %v", filepath.Base(path), err)
		return 0
	}
	return portFromListen(config.Listen)
}

// portFromListen accepts ":443", "0.0.0.0:443" and "[::]:443".
func portFromListen(listen string) uint {
	index := strings.LastIndex(listen, ":")
	if index < 0 || index == len(listen)-1 {
		return 0
	}
	return parseUint(listen[index+1:])
}

func parseUint(value string) uint {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0
	}
	return uint(parsed)
}

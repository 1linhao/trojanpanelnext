package constant

const (
	// LogPath log folder path
	LogPath string = "logs"

	// ConfigPath global config folder path
	ConfigPath string = "config"
	// ConfigFilePath global config file path
	ConfigFilePath string = "config/config.ini"

	// SqlitePath sqlite folder path
	SqlitePath string = "config/sqlite"
	// SqliteFilePath sqlite file path
	SqliteFilePath string = "config/sqlite/trojan_panel_core.db"

	XrayPath          string = "bin/xray/config"
	NaiveProxyPath    string = "bin/naiveproxy/config"
	Hysteria2Path     string = "bin/hysteria2/config"
	XrayBinPath       string = "bin/xray"
	NaiveProxyBinPath string = "bin/naiveproxy"
	Hysteria2BinPath  string = "bin/hysteria2"
	CoreBasePath      string = "/tpdata/trojan-panel-core"
	KernelRuntimePath string = "/tpdata/trojan-panel-core/runtime"

	// ExternalManagedPath holds the machine-readable routing contract. The
	// installer mounts the same host directory into the core container.
	ExternalManagedPath string = "/tpdata/trojan-panel-core/external"
	// ExternalRoutesFile lists every proxy inbound for the external entry point.
	ExternalRoutesFile string = ExternalManagedPath + "/routes.json"
	// WebFilePath is the camouflage site root shared with the external entry.
	WebFilePath string = "/tpdata/web"

	TrojanPanelCoreVersion = "v2.3.1"
)

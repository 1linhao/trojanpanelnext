package core

import (
	"errors"
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
	"os"
	"runtime"
	"strconv"
	"strings"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/util"
)

var (
	host                   string
	user                   string
	password               string
	port                   string
	database               string
	accountTable           string
	redisHost              string
	redisPort              string
	redisUsername          string
	redisPassword          string
	redisAuthUsername      string
	redisAuthPassword      string
	redisDb                string
	redisMaxIdle           string
	redisMaxActive         string
	redisWait              string
	crtPath                string
	keyPath                string
	grpcPort               string
	grpcTLSMode            string
	grpcClientCA           string
	serverPort             string
	nodeServerID           string
	nodeDomain             string
	nodeIdentityID         string
	nodeIdentityGen        string
	nodeBootstrapChallenge string
	version                bool
)

func init() {
	flag.StringVar(&host, "host", envOr("MARIADB_IP", "mariadb_ip", "localhost"), "database address")
	flag.StringVar(&user, "user", envOr("MARIADB_USER", "mariadb_user", "root"), "database username")
	flag.StringVar(&password, "password", envOr("MARIADB_PASSWORD", "mariadb_pas", ""), "deprecated: use MARIADB_PASSWORD")
	flag.StringVar(&port, "port", envOr("MARIADB_PORT", "mariadb_port", "3306"), "database port")
	flag.StringVar(&database, "database", envOr("DATABASE", "database", "trojan_panel_db"), "database name")
	flag.StringVar(&accountTable, "accountTable", envOr("ACCOUNT_TABLE", "account_table", "account"), "account table name")
	flag.StringVar(&redisHost, "redisHost", envOr("REDIS_HOST", "redis_host", "127.0.0.1"), "redis address")
	flag.StringVar(&redisPort, "redisPort", envOr("REDIS_PORT", "redis_port", "6379"), "redis port")
	flag.StringVar(&redisUsername, "redisUsername", envOr("REDIS_USERNAME", "redis_username", ""), "Redis ACL username")
	flag.StringVar(&redisPassword, "redisPassword", envOr("REDIS_PASSWORD", "redis_pass", ""), "deprecated: use REDIS_PASSWORD")
	flag.StringVar(&redisAuthUsername, "redisAuthUsername", envOr("REDIS_AUTH_USERNAME", "redis_auth_username", ""), "Redis read-only auth ACL username")
	flag.StringVar(&redisAuthPassword, "redisAuthPassword", envOr("REDIS_AUTH_PASSWORD", "redis_auth_password", ""), "Redis read-only auth ACL password")
	flag.StringVar(&redisDb, "redisDb", "0", "redis default database")
	flag.StringVar(&redisMaxIdle, "redisMaxIdle", strconv.FormatInt(int64(runtime.NumCPU()*2), 10), "redis maximum number of idle connections")
	flag.StringVar(&redisMaxActive, "redisMaxActive", strconv.FormatInt(int64(runtime.NumCPU()*2+2), 10), "redis maximum number of connections")
	flag.StringVar(&redisWait, "redisWait", "true", "does Redis wait")
	flag.StringVar(&crtPath, "crtPath", envOr("TLS_CERT_PATH", "crt_path", ""), "TLS server certificate")
	flag.StringVar(&keyPath, "keyPath", envOr("TLS_KEY_PATH", "key_path", ""), "TLS server private key")
	flag.StringVar(&grpcPort, "grpcPort", envOr("GRPC_PORT", "grpc_port", "8100"), "gRPC port")
	flag.StringVar(&grpcTLSMode, "grpcTLSMode", envOr("GRPC_TLS_MODE", "grpc_tls_mode", "legacy"), "gRPC TLS mode: legacy or mtls")
	flag.StringVar(&grpcClientCA, "grpcClientCA", envOr("GRPC_CLIENT_CA_PATH", "grpc_client_ca_path", ""), "gRPC client CA certificate")
	flag.StringVar(&serverPort, "serverPort", envOr("SERVER_PORT", "server_port", "8082"), "service port")
	flag.StringVar(&nodeServerID, "nodeServerId", envOr("NODE_SERVER_ID", "node_server_id", "0"), "panel node_server id")
	flag.StringVar(&nodeDomain, "nodeDomain", envOr("TP_NODE_DOMAIN", "node_domain", ""), "node domain used for TLS SNI")
	flag.StringVar(&nodeIdentityID, "nodeIdentityId", envOr("TP_NODE_IDENTITY_ID", "node_identity_id", ""), "stable Node identity id")
	flag.StringVar(&nodeIdentityGen, "nodeIdentityGeneration", envOr("TP_NODE_IDENTITY_GENERATION", "node_identity_generation", "0"), "Node identity credential generation")
	flag.StringVar(&nodeBootstrapChallenge, "nodeBootstrapChallenge", envOr("TP_NODE_BOOTSTRAP_CHALLENGE", "node_bootstrap_challenge", ""), "one-install Web verification challenge")
	flag.BoolVar(&version, "version", false, "print version info")
	flag.Usage = usage
	isTest := strings.HasSuffix(os.Args[0], ".test")
	if !isTest {
		flag.Parse()
	}
	if version {
		_, _ = fmt.Fprint(os.Stdout, constant.TrojanPanelCoreVersion)
		os.Exit(0)
	}
	if isTest {
		return
	}

	// initialization log
	logPath := constant.LogPath
	if !util.Exists(logPath) {
		if err := os.MkdirAll(logPath, os.ModePerm); err != nil {
			logrus.Errorf("create logs folder err: %v", err)
			panic(err)
		}
	}

	// initialize the global distribution folder
	configPath := constant.ConfigPath
	if !util.Exists(configPath) {
		if err := os.MkdirAll(configPath, os.ModePerm); err != nil {
			logrus.Errorf("create config folder err: %v", err)
			panic(err)
		}
	}

	configFilePath := constant.ConfigFilePath
	if !util.Exists(configFilePath) {
		file, err := os.OpenFile(configFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			logrus.Errorf("create config.ini err: %v", err)
			panic(err)
		}
		defer file.Close()

		_, err = file.WriteString(fmt.Sprintf(
			`[mysql]
host=%s
user=%s
password=%s
port=%s
database=%s
account_table=%s
[redis]
host=%s
port=%s
username=%s
password=%s
auth_username=%s
auth_password=%s
db=%s
max_idle=%s
max_active=%s
wait=%s
[cert]
crt_path=%s
key_path=%s
[log]
filename=logs/trojan-panel-core.log
max_size=1
max_backups=5
max_age=30
compress=true
[grpc]
port=%s
tls_mode=%s
client_ca_path=%s
[server]
port=%s
[node]
server_id=%s
domain=%s
identity_id=%s
identity_generation=%s
bootstrap_challenge=%s
`, host, user, password, port, database, accountTable, redisHost, redisPort, redisUsername, redisPassword, redisAuthUsername, redisAuthPassword, redisDb,
			redisMaxIdle, redisMaxActive, redisWait, crtPath, keyPath, grpcPort, grpcTLSMode, grpcClientCA, serverPort, nodeServerID, nodeDomain, nodeIdentityID, nodeIdentityGen, nodeBootstrapChallenge))
		if err != nil {
			logrus.Errorf("config.ini file write err: %v", err)
			panic(err)
		}
	}

	sqlitePath := constant.SqlitePath
	if !util.Exists(sqlitePath) {
		if err := os.MkdirAll(sqlitePath, os.ModePerm); err != nil {
			logrus.Errorf("create sqlite folder err: %v", err)
			panic(err)
		}
	}
	sqliteFilePath := constant.SqliteFilePath
	if !util.Exists(sqliteFilePath) {
		file, err := os.Create(sqliteFilePath)
		if err != nil {
			logrus.Errorf("create trojan_panel_core.db err: %v", err)
			panic(err)
		}
		defer file.Close()
	}
}

func usage() {
	_, _ = fmt.Fprintln(os.Stdout, `trojan panel core manage help
Usage: trojan-panel-core [-host] [-user] [-password] [-port] [-database] [-accountTable] [-redisHost] [-redisPort] [-redisUsername] [-redisPassword] [-redisDb] [-redisMaxIdle] [-redisMaxActive] [-redisWait] [-crtPath] [-keyPath] [-grpcPort] [-serverPort] [-h] [-version]`)
	flag.PrintDefaults()
}

var Config = new(AppConfig)

// InitConfig initialize the global configuration file
func InitConfig() {
	if err := ini.MapTo(Config, constant.ConfigFilePath); err != nil {
		logrus.Errorf("configuration file failed to load err: %v", err)
		panic(err)
	}
	if err := ValidateNodeIdentityConfig(Config); err != nil {
		logrus.Errorf("unsafe shared Node identity configuration: %v", err)
		panic(err)
	}
}

func ValidateNodeIdentityConfig(config *AppConfig) error {
	if strings.TrimSpace(config.MySQLConfig.User) == "" || strings.EqualFold(config.MySQLConfig.User, "root") {
		return errors.New("Node Agent requires a non-root MariaDB identity")
	}
	if strings.TrimSpace(config.MySQLConfig.Password) == "" {
		return errors.New("Node Agent requires a non-empty MariaDB password")
	}
	if strings.TrimSpace(config.RedisConfig.Username) == "" || strings.EqualFold(config.RedisConfig.Username, "default") {
		return errors.New("Node Agent requires a non-default Redis cache identity")
	}
	if strings.TrimSpace(config.RedisConfig.Password) == "" {
		return errors.New("Node Agent requires a non-empty Redis cache password")
	}
	if strings.TrimSpace(config.RedisConfig.AuthUsername) == "" || strings.EqualFold(config.RedisConfig.AuthUsername, "default") {
		return errors.New("Node Agent requires a non-default Redis auth identity")
	}
	if strings.TrimSpace(config.RedisConfig.AuthPassword) == "" {
		return errors.New("Node Agent requires a non-empty Redis auth password")
	}
	if config.RedisConfig.Username == config.RedisConfig.AuthUsername {
		return errors.New("Node Agent Redis cache and auth identities must be distinct")
	}
	if config.NodeConfig.ServerID == 0 {
		return errors.New("Node Agent requires a positive node_server_id")
	}
	if !isUUID(config.NodeConfig.IdentityID) {
		return errors.New("Node Agent requires a UUID node_identity_id")
	}
	if config.NodeConfig.IdentityGeneration == 0 {
		return errors.New("Node Agent requires a positive node_identity_generation")
	}
	if !isLowerHex(config.NodeConfig.BootstrapChallenge, 64) {
		return errors.New("Node Agent requires a 64-character bootstrap_challenge")
	}
	return nil
}

func isLowerHex(value string, length int) bool {
	if len(value) != length {
		return false
	}
	for _, character := range value {
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f')) {
			return false
		}
	}
	return true
}

func isUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for index, character := range value {
		if index == 8 || index == 13 || index == 18 || index == 23 {
			if character != '-' {
				return false
			}
			continue
		}
		if !((character >= '0' && character <= '9') || (character >= 'a' && character <= 'f') || (character >= 'A' && character <= 'F')) {
			return false
		}
	}
	return true
}

type AppConfig struct {
	MySQLConfig  `ini:"mysql"`
	RedisConfig  `ini:"redis"`
	CertConfig   `ini:"cert"`
	LogConfig    `ini:"log"`
	GrpcConfig   `ini:"grpc"`
	ServerConfig `ini:"server"`
	NodeConfig   `ini:"node"`
}

// MySQLConfig MySQL
type MySQLConfig struct {
	Host         string `ini:"host"`
	User         string `ini:"user"`
	Password     string `ini:"password"`
	Port         int    `ini:"port"`
	Database     string `ini:"database"`
	AccountTable string `ini:"account_table"`
}

type RedisConfig struct {
	Host         string `ini:"host"`
	Port         int    `ini:"port"`
	Username     string `ini:"username"`
	Password     string `ini:"password"`
	AuthUsername string `ini:"auth_username"`
	AuthPassword string `ini:"auth_password"`
	Db           int    `ini:"db"`
	MaxIdle      int    `ini:"max_idle"`
	MaxActive    int    `ini:"max_active"`
	Wait         bool   `ini:"wait"`
}

type CertConfig struct {
	CrtPath string `ini:"crt_path"`
	KeyPath string `ini:"key_path"`
}

type NodeConfig struct {
	ServerID           uint   `ini:"server_id"`
	Domain             string `ini:"domain"`
	IdentityID         string `ini:"identity_id"`
	IdentityGeneration uint64 `ini:"identity_generation"`
	BootstrapChallenge string `ini:"bootstrap_challenge"`
}

// LogConfig log
type LogConfig struct {
	FileName   string `ini:"filename"`    // log file location
	MaxSize    int    `ini:"max_size"`    // the maximum capacity of a single file, in MB
	MaxBackups int    `ini:"max_backups"` // the maximum number of expired files to keep
	MaxAge     int    `ini:"max_age"`     // the maximum time interval for keeping expired files, in days
	Compress   bool   `ini:"compress"`    // do you need to compress the rolling log, using gzip compression
}

// GrpcConfig gRPC
type GrpcConfig struct {
	Port         string `ini:"port"` // gRPC port
	TLSMode      string `ini:"tls_mode"`
	ClientCAPath string `ini:"client_ca_path"`
}

type ServerConfig struct {
	Port int `ini:"port"` // service port
}

func envOr(primary, legacy, fallback string) string {
	if value := os.Getenv(primary); value != "" {
		return value
	}
	if value := os.Getenv(legacy); value != "" {
		return value
	}
	return fallback
}

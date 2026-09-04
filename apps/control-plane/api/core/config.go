package core

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/ini.v1"
	"os"
	"runtime"
	"strconv"
	"strings"
	"trojan-panel/model/constant"
	"trojan-panel/util"
)

var (
	host           string
	user           string
	password       string
	port           string
	redisHost      string
	redisPort      string
	redisPassword  string
	redisDb        string
	redisMaxIdle   string
	redisMaxActive string
	redisWait      string
	serverPort     string
	grpcClientCert string
	grpcClientKey  string
	grpcClientCA   string
	version        bool
)

func init() {
	flag.StringVar(&host, "host", "localhost", "database address")
	flag.StringVar(&user, "user", "root", "database username")
	flag.StringVar(&password, "password", "123456", "database password")
	flag.StringVar(&port, "port", "3306", "database port")
	flag.StringVar(&redisHost, "redisHost", "127.0.0.1", "redis address")
	flag.StringVar(&redisPort, "redisPort", "6379", "redis port")
	flag.StringVar(&redisPassword, "redisPassword", "123456", "redis password")
	flag.StringVar(&redisDb, "redisDb", "0", "redis default database")
	flag.StringVar(&redisMaxIdle, "redisMaxIdle", strconv.FormatInt(int64(runtime.NumCPU()*2), 10), "redis maximum number of idle connections")
	flag.StringVar(&redisMaxActive, "redisMaxActive", strconv.FormatInt(int64(runtime.NumCPU()*2+2), 10), "redis maximum number of connections")
	flag.StringVar(&redisWait, "redisWait", "true", "does Redis wait")
	flag.StringVar(&serverPort, "serverPort", "8081", "service port")
	flag.StringVar(&grpcClientCert, "grpcClientCert", os.Getenv("GRPC_CLIENT_CERT_PATH"), "gRPC mTLS client certificate")
	flag.StringVar(&grpcClientKey, "grpcClientKey", os.Getenv("GRPC_CLIENT_KEY_PATH"), "gRPC mTLS client private key")
	flag.StringVar(&grpcClientCA, "grpcClientCA", os.Getenv("GRPC_SERVER_CA_PATH"), "gRPC server CA bundle")
	flag.BoolVar(&version, "version", false, "print version info")
	flag.Usage = usage
	isTest := strings.HasSuffix(os.Args[0], ".test")
	if !isTest {
		flag.Parse()
	}
	if version {
		_, _ = fmt.Fprint(os.Stdout, constant.TrojanPanelVersion)
		os.Exit(0)
	}
	if isTest {
		return
	}

	logPath := constant.LogPath
	if !util.Exists(logPath) {
		if err := os.Mkdir(logPath, os.ModePerm); err != nil {
			logrus.Errorf("create logs folder err: %v", err)
			panic(err)
		}
	}

	webFilePath := constant.WebFilePath
	if !util.Exists(webFilePath) {
		if err := os.Mkdir(webFilePath, os.ModePerm); err != nil {
			logrus.Errorf("create webfile folder err: %v", err)
			panic(err)
		}
	}

	configPath := constant.ConfigPath
	if !util.Exists(configPath) {
		if err := os.Mkdir(configPath, os.ModePerm); err != nil {
			logrus.Errorf("create config folder err: %v", err)
			panic(err)
		}
	}

	templatePath := constant.TemplatePath
	if !util.Exists(templatePath) {
		if err := os.Mkdir(templatePath, os.ModePerm); err != nil {
			logrus.Errorf("create config/template folder err: %v", err)
			panic(err)
		}
	}

	// 创建Logo
	logoImagePath := constant.LogoImagePath
	if !util.Exists(logoImagePath) {
		if err := util.DownloadFile(constant.LogoImageUrl, logoImagePath); err != nil {
			logrus.Errorf("create file logo.png err: %v", err)
		}
	}

	ExportPath := constant.ExportPath
	if !util.Exists(ExportPath) {
		if err := os.Mkdir(ExportPath, os.ModePerm); err != nil {
			logrus.Errorf("create config/export folder err: %v", err)
			panic(err)
		}
	}

	// 创建AccountTemplate模板
	exportAccountTemplate := constant.ExportAccountTemplate
	if !util.Exists(exportAccountTemplate) {
		var accountTemplate []map[string]any
		accountTemplate = append(accountTemplate, map[string]any{"username": "example", "pass": "example", "hash": "example", "role_id": 3, "email": "test@example.com", "expire_time": int64(4078656000000), "deleted": 0, "quota": -1, "download": 0, "upload": 0})
		if err := util.ExportJson(exportAccountTemplate, accountTemplate); err != nil {
			logrus.Errorf("create file AccountTemplate.json err: %v", err)
			panic(err)
		}
	}

	// 创建NodeServerTemplate模板
	exportNodeServerTemplate := constant.ExportNodeServerTemplate
	if !util.Exists(exportNodeServerTemplate) {
		var nodeServerTemplate []map[string]any
		nodeServerTemplate = append(nodeServerTemplate, map[string]any{"ip": "127.0.0.1", "name": "example", "grpc_port": 8100})
		if err := util.ExportJson(exportNodeServerTemplate, nodeServerTemplate); err != nil {
			logrus.Errorf("create file NodeServerTemplate.json err: %v", err)
			panic(err)
		}
	}

	configFilePath := constant.ConfigFilePath
	if !util.Exists(configFilePath) {
		file, err := os.OpenFile(configFilePath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			logrus.Errorf("create file config.ini err: %v", err)
			panic(err)
		}
		defer file.Close()

		_, err = file.WriteString(fmt.Sprintf(
			`[mysql]
host=%s
user=%s
password=%s
port=%s
[log]
filename=logs/trojan-panel.log
max_size=1
max_backups=5
max_age=30
compress=true
[redis]
host=%s
port=%s
password=%s
db=%s
max_idle=%s
max_active=%s
wait=%s
[server]
port=%s
[grpc]
client_cert_path=%s
client_key_path=%s
server_ca_path=%s
`, host, user, password, port, redisHost, redisPort, redisPassword, redisDb,
			redisMaxIdle, redisMaxActive, redisWait, serverPort, grpcClientCert, grpcClientKey, grpcClientCA))
		if err != nil {
			logrus.Errorf("config.ini file write err: %v", err)
			panic(err)
		}

	}

	rbacModelConfigPath := constant.RbacModelFilePath
	if !util.Exists(rbacModelConfigPath) {
		file, err := os.Create(rbacModelConfigPath)
		if err != nil {
			logrus.Errorf("create file rbac_model.conf err: %v", err)
			panic(err)
		}
		defer file.Close()

		_, err = file.WriteString(
			`[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
`)
		if err != nil {
			logrus.Errorf("rbac_model.conf file write err: %v", err)
			panic(err)
		}
	}

	clashTemplateFilePath := constant.ClashTemplateFilePath
	if !util.Exists(clashTemplateFilePath) {
		file, err := os.Create(clashTemplateFilePath)
		if err != nil {
			logrus.Errorf("create file clash-rule.yaml err: %v", err)
			panic(err)
		}
		defer file.Close()

		_, err = file.WriteString(constant.ClashRules)
		if err != nil {
			logrus.Errorf("clash-rule.yaml file wirte err: %v", err)
			panic(err)
		}
	}
	singBoxTunTemplateFilePath := constant.SingBoxTunTemplateFilePath
	if !util.Exists(singBoxTunTemplateFilePath) {
		file, err := os.Create(singBoxTunTemplateFilePath)
		if err != nil {
			logrus.Errorf("create file sing-box-route.json err: %v", err)
			panic(err)
		}
		defer file.Close()

		_, err = file.WriteString(constant.SingBoxRoute)
		if err != nil {
			logrus.Errorf("sing-box-route.json file write err: %v", err)
			panic(err)
		}
	}

	singBoxOutboundFilePath := constant.SingBoxOutboundTemplateFilePath
	if !util.Exists(singBoxOutboundFilePath) {
		file, err := os.Create(singBoxOutboundFilePath)
		if err != nil {
			logrus.Errorf("create file template-sing-box-outbound.json err: %v", err)
			panic(err)
		}
		defer file.Close()

		if _, err = file.WriteString(constant.SingBoxOutbound); err != nil {
			logrus.Errorf("template-sing-box-outbound.json file write err: %v", err)
			panic(err)
		}
	}

	xrayTemplateFilePath := constant.XrayTemplateFilePath
	if !util.Exists(xrayTemplateFilePath) {
		file, err := os.Create(xrayTemplateFilePath)
		if err != nil {
			logrus.Errorf("create file template-xray.json err: %v", err)
			panic(err)
		}
		defer file.Close()

		_, err = file.WriteString(`{
  "log": {
    "loglevel": "warning"
  },
  "inbounds": [],
  "outbounds": [
    {
      "protocol": "freedom"
    }
  ],
  "api": {
    "tag": "api",
    "services": [
      "HandlerService",
      "LoggerService",
      "StatsService"
    ]
  },
  "routing": {
    "rules": [
      {
        "inboundTag": [
          "api"
        ],
        "outboundTag": "api",
        "type": "field"
      }
    ]
  },
  "stats": {},
  "policy": {
    "levels": {
      "0": {
        "statsUserUplink": true,
        "statsUserDownlink": true
      }
    },
    "system": {
      "statsInboundUplink": true,
      "statsInboundDownlink": true
    }
  }
}`)
		if err != nil {
			logrus.Errorf("template-xray.json file write err: %v", err)
			panic(err)
		}
	}
}

func usage() {
	_, _ = fmt.Fprintln(os.Stdout, `trojan panel help
Usage: trojan-panel [-host] [-password] [-port] [-redisHost] [-redisPort] [-redisPassword] [-redisDb] [-redisMaxIdle] [-redisMaxActive] [-redisWait] [-h] [-version]`)
	flag.PrintDefaults()
}

var Config = new(AppConfig)

// InitConfig 初始化全局配置文件
func InitConfig() {
	if err := ini.MapTo(Config, constant.ConfigFilePath); err != nil {
		logrus.Errorf("configuration file failed to load err: %v", err)
		panic(err)
	}
}

type AppConfig struct {
	MySQLConfig  `ini:"mysql"`
	LogConfig    `ini:"log"`
	RedisConfig  `ini:"redis"`
	ServerConfig `ini:"server"`
	GrpcConfig   `ini:"grpc"`
}

type MySQLConfig struct {
	Host     string `ini:"host"`
	User     string `ini:"user"`
	Password string `ini:"password"`
	Port     int    `ini:"port"`
}

type LogConfig struct {
	FileName   string `ini:"filename"`    // 日志文件位置
	MaxSize    int    `ini:"max_size"`    // 单文件最大容量,单位是MB
	MaxBackups int    `ini:"max_backups"` // 最大保留过期文件个数
	MaxAge     int    `ini:"max_age"`     // 保留过期文件的最大时间间隔,单位是天
	Compress   bool   `ini:"compress"`    // 是否需要压缩滚动日志, 使用的 gzip 压缩
}

type RedisConfig struct {
	Host      string `ini:"host"`
	Port      int    `ini:"port"`
	Password  string `ini:"password"`
	Db        int    `ini:"db"`
	MaxIdle   int    `ini:"max_idle"`
	MaxActive int    `ini:"max_active"`
	Wait      bool   `ini:"wait"`
}

type ServerConfig struct {
	Port int `ini:"port"`
}

type GrpcConfig struct {
	ClientCertPath string `ini:"client_cert_path"`
	ClientKeyPath  string `ini:"client_key_path"`
	ServerCAPath   string `ini:"server_ca_path"`
}

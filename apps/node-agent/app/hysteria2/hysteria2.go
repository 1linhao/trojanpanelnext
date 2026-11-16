package hysteria2

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"trojan-panel-core/core"
	"trojan-panel-core/core/process"
	"trojan-panel-core/model/bo"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/model/dto"
	"trojan-panel-core/util"
)

func InitHysteria2App() error {
	apiPorts, err := util.GetConfigApiPorts(constant.Hysteria2Path)
	if err != nil {
		return err
	}
	hysteria2Process := process.NewHysteria2Instance()
	for _, apiPort := range apiPorts {
		if err = upgradeHysteria2Auth(apiPort); err != nil {
			return err
		}
		if err = hysteria2Process.StartHysteria2(apiPort); err != nil {
			return err
		}
	}
	return nil
}

func StartHysteria2(hysteria2ConfigDto dto.Hysteria2ConfigDto) error {
	var err error
	if err = initHysteria2(hysteria2ConfigDto); err != nil {
		return err
	}
	if err = process.NewHysteria2Instance().StartHysteria2(hysteria2ConfigDto.ApiPort); err != nil {
		return err
	}
	return nil
}

func StopHysteria2(apiPort uint, removeFile bool) error {
	if err := process.NewHysteria2Instance().Stop(apiPort, removeFile); err != nil {
		logrus.Errorf("hysteria2 stop err: %v", err)
		return err
	}
	return nil
}

func RestartHysteria2(apiPort uint) error {
	if err := StopHysteria2(apiPort, false); err != nil {
		return err
	}
	if err := upgradeHysteria2Auth(apiPort); err != nil {
		return err
	}
	return process.NewHysteria2Instance().StartHysteria2(apiPort)
}

func hysteria2AuthURL(port uint) string {
	return fmt.Sprintf("http://127.0.0.1:%d/api/auth/hysteria2?port=%d", core.Config.ServerConfig.Port, port)
}

func upgradeHysteria2Auth(apiPort uint) error {
	path, err := util.GetConfigFile(constant.Hysteria2, apiPort)
	if err != nil {
		return err
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if apiPort <= 30000 {
		return fmt.Errorf("invalid Hysteria2 API port: %d", apiPort)
	}
	var config map[string]json.RawMessage
	var auth map[string]json.RawMessage
	var httpAuth map[string]json.RawMessage
	if err := json.Unmarshal(content, &config); err != nil {
		return err
	}
	if err := json.Unmarshal(config["auth"], &auth); err != nil {
		return err
	}
	if err := json.Unmarshal(auth["http"], &httpAuth); err != nil || httpAuth == nil {
		return errors.New("Hysteria2 configuration is missing HTTP authentication")
	}
	expected, err := json.Marshal(hysteria2AuthURL(apiPort - 30000))
	if err != nil {
		return err
	}
	if string(httpAuth["url"]) == string(expected) {
		return nil
	}
	httpAuth["url"] = expected
	auth["http"], err = json.Marshal(httpAuth)
	if err != nil {
		return err
	}
	config["auth"], err = json.Marshal(auth)
	if err != nil {
		return err
	}
	content, err = json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".hysteria2-auth-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(content)
	if err := errors.Join(writeErr, file.Close()); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}

func initHysteria2(hysteria2ConfigDto dto.Hysteria2ConfigDto) error {
	hysteria2ConfigFilePath, err := util.GetConfigFilePath(constant.Hysteria2, hysteria2ConfigDto.ApiPort)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(hysteria2ConfigFilePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE, 0644)
	if err != nil {
		logrus.Errorf("create hysteria2 file %s err: %v", hysteria2ConfigFilePath, err)
		return err
	}
	defer func() {
		if file != nil {
			file.Close()
		}
	}()

	certConfig := core.Config.CertConfig
	configContent := `{
  "listen": ":${port}",
  "tls": {
    "cert": "${crt_path}",
    "key": "${key_path}"
  },
  "bandwidth": {
    "up": "${up_mbps} mbps",
    "down": "${down_mbps} mbps"
  },
  "auth": {
    "type": "http",
    "http": {
      "url": "${auth_url}",
      "insecure": true
    }
  },
  "trafficStats": {
    "listen": ":${api_port}"
  }
}`
	configContent = strings.ReplaceAll(configContent, "${port}", strconv.FormatInt(int64(hysteria2ConfigDto.Port), 10))
	configContent = strings.ReplaceAll(configContent, "${crt_path}", certConfig.CrtPath)
	configContent = strings.ReplaceAll(configContent, "${key_path}", certConfig.KeyPath)
	configContent = strings.ReplaceAll(configContent, "${up_mbps}", strconv.FormatInt(int64(hysteria2ConfigDto.UpMbps), 10))
	configContent = strings.ReplaceAll(configContent, "${down_mbps}", strconv.FormatInt(int64(hysteria2ConfigDto.DownMbps), 10))
	configContent = strings.ReplaceAll(configContent, "${auth_url}", hysteria2AuthURL(hysteria2ConfigDto.Port))
	configContent = strings.ReplaceAll(configContent, "${api_port}", strconv.FormatInt(int64(hysteria2ConfigDto.ApiPort), 10))
	if len(hysteria2ConfigDto.ObfsPassword) >= 4 {
		// set obfs
		var hysteria2Config bo.Hysteria2Config
		if err = json.Unmarshal([]byte(configContent), &hysteria2Config); err != nil {
			logrus.Errorf("hysteria2 json.Unmarshal err: %v", err)
			return err
		}
		var hysteria2ConfigObfs bo.Hysteria2ConfigObfs
		hysteria2ConfigObfs.Type = "salamander"
		hysteria2ConfigObfs.Salamander.Password = hysteria2ConfigDto.ObfsPassword
		hysteria2Config.Obfs = &hysteria2ConfigObfs
		hysteria2ConfigStr, err := json.MarshalIndent(hysteria2Config, "", "    ")
		if err != nil {
			logrus.Errorf("hysteria2 json.MarshalIndent err: %v", err)
			return err
		}
		configContent = string(hysteria2ConfigStr)
	}
	_, err = file.WriteString(configContent)
	if err != nil {
		logrus.Errorf("hysteria2 config.json file write err: %v", err)
		return err
	}
	return nil
}

func InitHysteria2BinFile() error {
	hysteria2Path := constant.Hysteria2Path
	if !util.Exists(hysteria2Path) {
		if err := os.MkdirAll(hysteria2Path, os.ModePerm); err != nil {
			logrus.Errorf("create hysteria2 folder err: %v", err)
			return err
		}
	}

	binaryFilePath, err := util.GetBinaryFilePath(constant.Hysteria2)
	if err != nil {
		return err
	}
	if !util.Exists(binaryFilePath) {
		logrus.Errorf("hysteria2 binary does not exist")
		return errors.New(constant.BinaryFileNotExist)
	}
	return nil
}

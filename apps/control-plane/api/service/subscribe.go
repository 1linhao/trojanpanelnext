package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
	"strings"
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/model/bo"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service/clientcompat"
	"trojan-panel/util"
)

// SubscribeClash
/**
Clash for windows 参考文档：
1. https://docs.cfw.lbyczf.com/contents/urlscheme.html
2. https://github.com/crossutility/Quantumult/blob/master/extra-subscription-feature.md
3. https://github.com/Dreamacro/clash/wiki/Configuration
*/
func SubscribeClash(pass string) (*model.Account, string, []byte, vo.SystemVo, error) {
	account, err := dao.SelectAccountClashSubscribe(pass)
	if err != nil {
		return nil, "", []byte{}, vo.SystemVo{}, err
	}
	nodes, err := dao.SelectNodes()
	if err != nil {
		return nil, "", []byte{}, vo.SystemVo{}, err
	}

	userInfo := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
		*account.Upload,
		*account.Download,
		*account.Quota,
		*account.ExpireTime/1000)

	clashConfig := bo.ClashConfig{}
	var ClashConfigInterface []interface{}
	var proxies []string
	for _, item := range nodes {
		if !clientcompat.Includes(item.ClientTypes, constant.ClientClashMeta) {
			continue
		}
		if *item.NodeTypeId == constant.Xray {
			nodeXray, err := dao.SelectNodeXrayById(item.NodeSubId)
			if err != nil {
				return nil, "", []byte{}, vo.SystemVo{}, err
			}

			streamSettings := bo.StreamSettings{}
			if nodeXray.StreamSettings != nil && *nodeXray.StreamSettings != "" {
				if err = json.Unmarshal([]byte(*nodeXray.StreamSettings), &streamSettings); err != nil {
					logrus.Errorln(fmt.Sprintf("SystemVo JSON反转失败 err: %v", err))
					return nil, "", []byte{}, vo.SystemVo{}, errors.New(constant.SysError)
				}
			}
			settings := bo.Settings{}
			if nodeXray.Settings != nil && *nodeXray.Settings != "" {
				if err = json.Unmarshal([]byte(*nodeXray.Settings), &settings); err != nil {
					logrus.Errorln(fmt.Sprintf("SystemVo JSON反转失败 err: %v", err))
					return nil, "", []byte{}, vo.SystemVo{}, errors.New(constant.SysError)
				}
			}
			switch *nodeXray.Protocol {
			case constant.ProtocolVless:
				vless := bo.Vless{
					Name:    *item.Name,
					Type:    constant.ClashVless,
					Server:  *item.Domain,
					Port:    *item.Port,
					Uuid:    util.GenerateUUID(pass),
					Network: streamSettings.Network,
					Tls:     true,
					Udp:     true,
					Flow:    *nodeXray.XrayFlow,
				}
				if streamSettings.Security == "tls" {
					vless.ClientFingerprint = streamSettings.TlsSettings.Fingerprint
					vless.SkipCertVerify = streamSettings.TlsSettings.AllowInsecure
					vless.ServerName = streamSettings.TlsSettings.ServerName
				} else if streamSettings.Security == "reality" {
					if len(streamSettings.RealitySettings.ServerNames) > 0 {
						vless.ServerName = streamSettings.RealitySettings.ServerNames[0]
					}
					if len(streamSettings.RealitySettings.ShortIds) > 0 {
						vless.RealityOpts.ShortId = streamSettings.RealitySettings.ShortIds[0]
					}
					vless.RealityOpts.PublicKey = *nodeXray.RealityPbk
					vless.ClientFingerprint = streamSettings.RealitySettings.Fingerprint
				} else if streamSettings.Security == "none" {
					vless.Tls = false
					vless.SkipCertVerify = false
					vless.ClientFingerprint = ""
				}
				if streamSettings.Network == "ws" {
					vless.WsOpts.Path = streamSettings.WsSettings.Path
					vless.WsOpts.Headers.Host = streamSettings.WsSettings.Headers.Host
				}
				if uintValue(nodeXray.XudpEnable) == 1 {
					vless.PacketEncoding = "xudp"
				}
				if uintValue(nodeXray.MuxEnable) == 1 {
					vless.Smux = &bo.Smux{Enabled: true, Protocol: "h2mux"}
				}
				ClashConfigInterface = append(ClashConfigInterface, vless)
				proxies = append(proxies, *item.Name)
			case constant.ProtocolVmess:
				vmess := bo.Vmess{
					Name:    *item.Name,
					Type:    constant.ClashVmess,
					Server:  *item.Domain,
					Port:    *item.Port,
					Uuid:    util.GenerateUUID(pass),
					AlterId: 0,
					Tls:     true,
					Udp:     true,
					Network: streamSettings.Network,
				}
				if settings.Encryption != "none" {
					vmess.Cipher = "auto"
				} else {
					vmess.Cipher = "none"
				}
				if streamSettings.Security == "tls" {
					vmess.ClientFingerprint = streamSettings.TlsSettings.Fingerprint
					vmess.SkipCertVerify = streamSettings.TlsSettings.AllowInsecure
					vmess.ServerName = streamSettings.TlsSettings.ServerName
				} else if streamSettings.Security == "none" {
					vmess.Tls = false
					vmess.SkipCertVerify = false
					vmess.ClientFingerprint = ""
				}
				if streamSettings.Network == "ws" {
					vmess.WsOpts.Path = streamSettings.WsSettings.Path
					vmess.WsOpts.Headers.Host = streamSettings.WsSettings.Headers.Host
				}
				if uintValue(nodeXray.XudpEnable) == 1 {
					vmess.PacketEncoding = "xudp"
				}
				if uintValue(nodeXray.MuxEnable) == 1 {
					vmess.Smux = &bo.Smux{Enabled: true, Protocol: "h2mux"}
				}
				ClashConfigInterface = append(ClashConfigInterface, vmess)
				proxies = append(proxies, *item.Name)
			case constant.ProtocolTrojan:
				trojan := bo.Trojan{
					Name:     *item.Name,
					Type:     constant.ClashTrojan,
					Server:   *item.Domain,
					Port:     *item.Port,
					Password: pass,
					Udp:      true,
				}
				if streamSettings.Security == "tls" {
					trojan.ClientFingerprint = streamSettings.TlsSettings.Fingerprint
					trojan.Sni = streamSettings.TlsSettings.ServerName
					trojan.Alpn = streamSettings.TlsSettings.Alpn
					trojan.SkipCertVerify = streamSettings.TlsSettings.AllowInsecure
				} else if streamSettings.Security == "none" {
					trojan.ClientFingerprint = ""
					trojan.SkipCertVerify = false
				}
				if streamSettings.Network == "ws" {
					trojan.WsOpts.Path = streamSettings.WsSettings.Path
					trojan.WsOpts.Headers.Host = streamSettings.WsSettings.Headers.Host
				}
				if uintValue(nodeXray.MuxEnable) == 1 {
					trojan.Smux = &bo.Smux{Enabled: true, Protocol: "h2mux"}
				}
				ClashConfigInterface = append(ClashConfigInterface, trojan)
				proxies = append(proxies, *item.Name)
			case constant.ProtocolShadowsocks:
				shadowsocks := bo.Shadowsocks{
					Name:     *item.Name,
					Type:     constant.ClashShadowsocks,
					Server:   *item.Domain,
					Port:     *item.Port,
					Cipher:   *nodeXray.XraySSMethod,
					Password: pass,
					Udp:      true,
				}
				if uintValue(nodeXray.UotEnable) == 1 {
					shadowsocks.UdpOverTcp = true
					shadowsocks.UdpOverTcpVersion = xrayUotVersion(nodeXray.UotVersion)
				}
				ClashConfigInterface = append(ClashConfigInterface, shadowsocks)
				proxies = append(proxies, *item.Name)
			case constant.ProtocolSocks:
				socks := bo.Socks{
					Name:     *item.Name,
					Type:     constant.ClashSocks5,
					Server:   *item.Domain,
					Port:     *item.Port,
					Username: settings.Accounts[0].User,
					Password: settings.Accounts[0].Pass,
					Udp:      settings.Udp,
				}
				if streamSettings.Security == "tls" {
					socks.Tls = true
					socks.Fingerprint = streamSettings.TlsSettings.Fingerprint
					socks.SkipCertVerify = streamSettings.TlsSettings.AllowInsecure
				} else if streamSettings.Security == "none" {
					socks.SkipCertVerify = false
				}
				ClashConfigInterface = append(ClashConfigInterface, socks)
				proxies = append(proxies, *item.Name)
			}
		} else if *item.NodeTypeId == constant.TrojanGo {
			nodeTrojanGo, err := dao.SelectNodeTrojanGoById(item.NodeSubId)
			if err != nil {
				return nil, "", []byte{}, vo.SystemVo{}, err
			}
			trojanGo := bo.TrojanGo{
				Name:     *item.Name,
				Type:     constant.ClashTrojan,
				Server:   *item.Domain,
				Port:     *item.Port,
				Password: pass,
				Udp:      true,
				SNI:      *nodeTrojanGo.Sni,
			}
			if *nodeTrojanGo.WebsocketEnable == 1 {
				trojanGo.Network = "ws"
				trojanGo.WsOpts.Path = *nodeTrojanGo.WebsocketPath
				trojanGo.WsOpts.Headers.Host = *nodeTrojanGo.WebsocketHost
			}
			ClashConfigInterface = append(ClashConfigInterface, trojanGo)
			proxies = append(proxies, *item.Name)
		} else if *item.NodeTypeId == constant.Hysteria {
			nodeHysteria, err := dao.SelectNodeHysteriaById(item.NodeSubId)
			if err != nil {
				return nil, "", []byte{}, vo.SystemVo{}, err
			}
			hysteria := bo.Hysteria{
				Name:           *item.Name,
				Type:           constant.ClashSHysteria,
				Server:         *item.Domain,
				Port:           *item.Port,
				AuthStr:        pass,
				Obfs:           *nodeHysteria.Obfs,
				Protocol:       *nodeHysteria.Protocol,
				Up:             *nodeHysteria.UpMbps,
				Down:           *nodeHysteria.DownMbps,
				Sni:            *nodeHysteria.ServerName,
				SkipCertVerify: *nodeHysteria.Insecure == 1,
				FastOpen:       *nodeHysteria.FastOpen == 1,
			}
			ClashConfigInterface = append(ClashConfigInterface, hysteria)
			proxies = append(proxies, *item.Name)
		} else if *item.NodeTypeId == constant.Hysteria2 {
			nodeHysteria2, err := dao.SelectNodeHysteria2ById(item.NodeSubId)
			if err != nil {
				return nil, "", []byte{}, vo.SystemVo{}, err
			}
			hysteria2 := bo.Hysteria2{
				Name:           *item.Name,
				Type:           constant.ClashSHysteria2,
				Server:         *item.Domain,
				Port:           *item.Port,
				Password:       pass,
				Up:             *nodeHysteria2.UpMbps,
				Down:           *nodeHysteria2.DownMbps,
				SkipCertVerify: *nodeHysteria2.Insecure == 1,
			}
			if nodeHysteria2.ObfsPassword != nil && *nodeHysteria2.ObfsPassword != "" {
				hysteria2.Obfs = "salamander"
				hysteria2.ObfsPassword = *nodeHysteria2.ObfsPassword
			}
			if nodeHysteria2.ServerName != nil && *nodeHysteria2.ServerName != "" {
				hysteria2.Sni = *nodeHysteria2.ServerName
			}
			if portHopping := normalizedHysteria2PortHopping(nodeHysteria2.PortHopping); portHopping != "" {
				hysteria2.Ports = portHopping
				hysteria2.HopInterval = uintValue(nodeHysteria2.HopInterval)
			}
			ClashConfigInterface = append(ClashConfigInterface, hysteria2)
			proxies = append(proxies, *item.Name)
		}
	}
	if len(proxies) == 0 {
		proxies = append(proxies, "DIRECT")
	}
	proxyGroups := make([]bo.ProxyGroup, 0)
	proxyGroup := bo.ProxyGroup{
		Name:    "PROXY",
		Type:    "select",
		Proxies: proxies,
	}
	proxyGroups = append(proxyGroups, proxyGroup)
	clashConfig.ProxyGroups = proxyGroups
	clashConfig.Proxies = ClashConfigInterface

	clashConfigYaml, err := yaml.Marshal(&clashConfig)
	if err != nil {
		return nil, "", []byte{}, vo.SystemVo{}, errors.New(constant.SysError)
	}

	systemName := constant.SystemName
	systemConfig, err := SelectSystemByName(&systemName)
	if err != nil {
		return nil, "", []byte{}, vo.SystemVo{}, errors.New(constant.SysError)
	}
	return account, userInfo, clashConfigYaml, systemConfig, nil
}

func SubscribeSingBox(pass string, templateId string) (*model.Account, string, []byte, error) {
	account, err := dao.SelectAccountClashSubscribe(pass)
	if err != nil {
		return nil, "", []byte{}, err
	}
	nodes, err := dao.SelectNodes()
	if err != nil {
		return nil, "", []byte{}, err
	}

	userInfo := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
		*account.Upload,
		*account.Download,
		*account.Quota,
		*account.ExpireTime/1000)

	outbounds := make([]map[string]interface{}, 0)
	proxyTags := make([]string, 0)
	for _, item := range nodes {
		if !clientcompat.Includes(item.ClientTypes, constant.ClientSingBox) {
			continue
		}
		outbound, err := buildSingBoxOutbound(item, pass, *account.Username)
		if err != nil {
			return nil, "", []byte{}, err
		}
		if outbound == nil {
			continue
		}
		outbounds = append(outbounds, outbound)
		proxyTags = append(proxyTags, *item.Name)
	}
	systemName := constant.SystemName
	systemConfig, err := SelectSystemByName(&systemName)
	if err != nil {
		return nil, "", []byte{}, errors.New(constant.SysError)
	}
	var singBoxConfig map[string]interface{}
	if templateId == "outbound" {
		singBoxConfig, err = buildSingBoxOutboundConfig(systemConfig.SingBoxOutbound, outbounds)
	} else {
		selectorOutbounds := append([]string{}, proxyTags...)
		selectorDefault := "DIRECT"
		if len(selectorOutbounds) > 0 {
			selectorDefault = selectorOutbounds[0]
			selectorOutbounds = append(selectorOutbounds, "DIRECT")
		} else {
			selectorOutbounds = append(selectorOutbounds, "DIRECT")
		}
		outbounds = append(outbounds, map[string]interface{}{
			"type":      "selector",
			"tag":       "PROXY",
			"outbounds": selectorOutbounds,
			"default":   selectorDefault,
		})
		outbounds = append(outbounds, map[string]interface{}{"type": "direct", "tag": "DIRECT"})
		singBoxConfig, err = buildSingBoxConfig(systemConfig.SingBoxTun, outbounds)
	}
	if err != nil {
		logrus.Errorf("sing-box template config deserialization err: %v", err)
		return nil, "", []byte{}, errors.New(constant.SysError)
	}
	singBoxConfigJson, err := json.MarshalIndent(singBoxConfig, "", "  ")
	if err != nil {
		return nil, "", []byte{}, errors.New(constant.SysError)
	}
	return account, userInfo, singBoxConfigJson, nil
}

func SubscribeV2Ray(pass string, userAgent string) (*model.Account, string, []byte, error) {
	return SubscribeURI(pass, userAgent, constant.ClientV2Ray)
}

// SubscribeURI returns the standard base64 URI subscription used by V2Ray
// clients and Shadowrocket while preserving each client's node visibility.
func SubscribeURI(pass string, userAgent string, client string) (*model.Account, string, []byte, error) {
	account, err := dao.SelectAccountClashSubscribe(pass)
	if err != nil {
		return nil, "", nil, err
	}
	nodes, err := dao.SelectNodes()
	if err != nil {
		return nil, "", nil, err
	}

	urls := make([]string, 0, len(nodes))
	subscriptionFormat := clientcompat.V2RaySubscriptionFormat(userAgent)
	for _, node := range nodes {
		if !clientcompat.Includes(node.ClientTypes, client) {
			continue
		}
		nodeUrl, _, err := nodeURLForClient(account.Id, account.Username, node.Id, subscriptionFormat)
		if err != nil {
			return nil, "", nil, err
		}
		if nodeUrl != "" {
			urls = append(urls, nodeUrl)
		}
	}
	userInfo := fmt.Sprintf("upload=%d; download=%d; total=%d; expire=%d",
		*account.Upload,
		*account.Download,
		*account.Quota,
		*account.ExpireTime/1000)
	content := base64.StdEncoding.EncodeToString([]byte(strings.Join(urls, "\n")))
	return account, userInfo, []byte(content), nil
}

func ExportOptions() ([]vo.ClientExportOptionVo, error) {
	systemName := constant.SystemName
	systemConfig, err := SelectSystemByName(&systemName)
	if err != nil {
		return nil, err
	}
	defaultName := func(name string, fallback string) string {
		if name == "" {
			return fallback
		}
		return name
	}
	return []vo.ClientExportOptionVo{
		{
			Id:   "sing-box",
			Name: "sing-box",
			Templates: []vo.ClientTemplateVo{
				{Id: "tun", Name: defaultName(systemConfig.SingBoxTunTemplateName, "TUN")},
				{Id: "outbound", Name: defaultName(systemConfig.SingBoxOutboundTemplateName, "Outbound only")},
			},
			Formats: []string{"link", "file"},
		},
		{
			Id:        "clash-meta",
			Name:      "Clash.Meta",
			Templates: []vo.ClientTemplateVo{{Id: "default", Name: defaultName(systemConfig.ClashTemplateName, "Default")}},
			Formats:   []string{"link", "file"},
		},
		{
			Id:        "v2ray",
			Name:      "V2Ray",
			Templates: []vo.ClientTemplateVo{{Id: "default", Name: defaultName(systemConfig.XrayTemplateName, "Default")}},
			Formats:   []string{"link", "file", "qrcode"},
		},
		{
			Id:        "shadowrocket",
			Name:      "Shadowrocket",
			Templates: []vo.ClientTemplateVo{{Id: "default", Name: "Default"}},
			Formats:   []string{"link", "file", "qrcode"},
		},
	}, nil
}

func buildSingBoxConfig(template string, outbounds []map[string]interface{}) (map[string]interface{}, error) {
	singBoxConfig := map[string]interface{}{}
	if template != "" {
		if err := json.Unmarshal([]byte(template), &singBoxConfig); err != nil {
			return nil, err
		}
	}
	if len(singBoxConfig) == 0 {
		if err := json.Unmarshal([]byte(constant.SingBoxRoute), &singBoxConfig); err != nil {
			return nil, err
		}
	}
	delete(singBoxConfig, "outbounds")
	if _, ok := singBoxConfig["log"]; !ok {
		singBoxConfig["log"] = map[string]interface{}{"level": "info"}
	}
	if _, ok := singBoxConfig["http_clients"]; !ok {
		singBoxConfig["http_clients"] = defaultSingBoxHTTPClients()
	}
	normalizeSingBoxHTTPClients(singBoxConfig)
	dnsConfig, ok := singBoxConfig["dns"].(map[string]interface{})
	if !ok {
		dnsConfig = defaultSingBoxDNS()
		singBoxConfig["dns"] = dnsConfig
	}
	normalizeSingBoxDNS(dnsConfig)
	if _, ok := singBoxConfig["inbounds"]; !ok {
		singBoxConfig["inbounds"] = defaultSingBoxTunInbounds()
	}
	normalizeSingBoxExperimental(singBoxConfig)

	routeConfig, ok := singBoxConfig["route"].(map[string]interface{})
	if !ok {
		routeConfig = map[string]interface{}{}
		singBoxConfig["route"] = routeConfig
	}
	normalizeSingBoxRoute(routeConfig)
	normalizeSingBoxRuleSet(routeConfig)
	singBoxConfig["outbounds"] = outbounds
	return singBoxConfig, nil
}

func buildSingBoxOutboundConfig(template string, outbounds []map[string]interface{}) (map[string]interface{}, error) {
	singBoxConfig := map[string]interface{}{}
	if template != "" {
		if err := json.Unmarshal([]byte(template), &singBoxConfig); err != nil {
			return nil, err
		}
	}
	if len(singBoxConfig) == 0 {
		if err := json.Unmarshal([]byte(constant.SingBoxOutbound), &singBoxConfig); err != nil {
			return nil, err
		}
	}
	delete(singBoxConfig, "outbounds")

	baseInbound := map[string]interface{}{
		"type":        "socks",
		"tag":         "socks-in",
		"listen":      "127.0.0.1",
		"listen_port": float64(10808),
	}
	if inbounds, ok := singBoxConfig["inbounds"].([]interface{}); ok {
		for _, item := range inbounds {
			inbound, ok := item.(map[string]interface{})
			if ok && inbound["type"] == "socks" {
				baseInbound = inbound
				break
			}
		}
	}
	startPort := 10808
	switch value := baseInbound["listen_port"].(type) {
	case float64:
		startPort = int(value)
	case int:
		startPort = value
	}
	if startPort < 1 || startPort+len(outbounds)-1 > 65535 {
		return nil, errors.New("sing-box SOCKS listen port range is invalid")
	}

	inbounds := make([]map[string]interface{}, 0, len(outbounds))
	rules := make([]map[string]interface{}, 0, len(outbounds))
	for index, outbound := range outbounds {
		tag, _ := outbound["tag"].(string)
		if tag == "" {
			continue
		}
		inbound := make(map[string]interface{}, len(baseInbound))
		for key, value := range baseInbound {
			inbound[key] = value
		}
		inboundTag := fmt.Sprintf("socks-in-%d", index+1)
		inbound["type"] = "socks"
		inbound["tag"] = inboundTag
		inbound["listen_port"] = startPort + index
		inbounds = append(inbounds, inbound)
		rules = append(rules, map[string]interface{}{
			"inbound":  inboundTag,
			"action":   "route",
			"outbound": tag,
		})
	}
	singBoxConfig["inbounds"] = inbounds
	singBoxConfig["outbounds"] = outbounds
	route, ok := singBoxConfig["route"].(map[string]interface{})
	if !ok {
		route = map[string]interface{}{}
		singBoxConfig["route"] = route
	}
	route["rules"] = rules
	return singBoxConfig, nil
}

func defaultSingBoxHTTPClients() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"tag": "rule-set-downloader",
		},
	}
}

func normalizeSingBoxHTTPClients(singBoxConfig map[string]interface{}) {
	httpClientValue, ok := singBoxConfig["http_clients"]
	if !ok {
		singBoxConfig["http_clients"] = defaultSingBoxHTTPClients()
		return
	}

	switch httpClients := httpClientValue.(type) {
	case []interface{}:
		hasRuleSetDownloader := false
		for _, item := range httpClients {
			if httpClient, ok := item.(map[string]interface{}); ok {
				if httpClient["tag"] == "rule-set-downloader" {
					hasRuleSetDownloader = true
					delete(httpClient, "detour")
				}
			}
		}
		if !hasRuleSetDownloader {
			singBoxConfig["http_clients"] = append(httpClients, map[string]interface{}{"tag": "rule-set-downloader"})
		}
	case []map[string]interface{}:
		hasRuleSetDownloader := false
		for _, httpClient := range httpClients {
			if httpClient["tag"] == "rule-set-downloader" {
				hasRuleSetDownloader = true
				delete(httpClient, "detour")
			}
		}
		if !hasRuleSetDownloader {
			singBoxConfig["http_clients"] = append(httpClients, map[string]interface{}{"tag": "rule-set-downloader"})
		}
	default:
		singBoxConfig["http_clients"] = defaultSingBoxHTTPClients()
	}
}

func defaultSingBoxDNS() map[string]interface{} {
	return map[string]interface{}{
		"strategy": "ipv4_only",
		"servers": []map[string]interface{}{
			{
				"type": "local",
				"tag":  "local",
			},
			{
				"type":   "tls",
				"tag":    "remote",
				"server": "1.1.1.1",
				"detour": "PROXY",
			},
		},
		"rules": []map[string]interface{}{
			{
				"clash_mode": "direct",
				"action":     "route",
				"server":     "local",
			},
			{
				"clash_mode": "global",
				"action":     "route",
				"server":     "remote",
			},
		},
		"final": "remote",
	}
}

func normalizeSingBoxDNS(dnsConfig map[string]interface{}) {
	if _, ok := dnsConfig["strategy"]; !ok {
		dnsConfig["strategy"] = "ipv4_only"
	}
	if _, ok := dnsConfig["rules"]; !ok {
		dnsConfig["rules"] = defaultSingBoxDNSRules()
		return
	}
	rules, ok := singBoxRulesToInterfaces(dnsConfig["rules"])
	if !ok {
		return
	}
	missingRules := missingClashModeRules(rules, defaultSingBoxDNSRules())
	if len(missingRules) > 0 {
		dnsConfig["rules"] = appendSingBoxRules(missingRules, rules...)
	}
}

func defaultSingBoxDNSRules() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"clash_mode": "direct",
			"action":     "route",
			"server":     "local",
		},
		{
			"clash_mode": "global",
			"action":     "route",
			"server":     "remote",
		},
	}
}

func defaultSingBoxTunInbounds() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type": "tun",
			"tag":  "tun-in",
			"address": []string{
				"172.19.0.1/30",
				"fdfe:dcba:9876::1/126",
			},
			"auto_route":   true,
			"strict_route": true,
			"stack":        "mixed",
		},
	}
}

func normalizeSingBoxRoute(routeConfig map[string]interface{}) {
	if _, ok := routeConfig["auto_detect_interface"]; !ok {
		routeConfig["auto_detect_interface"] = true
	}
	if _, ok := routeConfig["default_http_client"]; !ok {
		routeConfig["default_http_client"] = "rule-set-downloader"
	}
	if _, ok := routeConfig["default_domain_resolver"]; !ok {
		routeConfig["default_domain_resolver"] = "local"
	}
	if _, ok := routeConfig["final"]; !ok {
		routeConfig["final"] = "PROXY"
	}
	if _, ok := routeConfig["rules"]; !ok {
		routeConfig["rules"] = defaultSingBoxRouteRules()
		return
	}
	rules, ok := singBoxRulesToInterfaces(routeConfig["rules"])
	if !ok {
		return
	}
	missingRules := missingClashModeRules(rules, []map[string]interface{}{
		{
			"clash_mode": "direct",
			"action":     "route",
			"outbound":   "DIRECT",
		},
		{
			"clash_mode": "global",
			"action":     "route",
			"outbound":   "PROXY",
		},
	})
	if len(missingRules) > 0 {
		routeConfig["rules"] = insertSingBoxRouteModeRules(rules, missingRules)
	}
}

func defaultSingBoxRouteRules() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"action": "sniff",
		},
		{
			"protocol": "dns",
			"action":   "hijack-dns",
		},
		{
			"clash_mode": "direct",
			"action":     "route",
			"outbound":   "DIRECT",
		},
		{
			"clash_mode": "global",
			"action":     "route",
			"outbound":   "PROXY",
		},
		{
			"ip_is_private": true,
			"action":        "route",
			"outbound":      "DIRECT",
		},
		{
			"rule_set": []string{
				"geoip-cn",
				"geosite-cn",
			},
			"action":   "route",
			"outbound": "DIRECT",
		},
	}
}

func singBoxRulesToInterfaces(value interface{}) ([]interface{}, bool) {
	switch rules := value.(type) {
	case []interface{}:
		return rules, true
	case []map[string]interface{}:
		result := make([]interface{}, 0, len(rules))
		for _, rule := range rules {
			result = append(result, rule)
		}
		return result, true
	default:
		return nil, false
	}
}

func missingClashModeRules(rules []interface{}, candidates []map[string]interface{}) []map[string]interface{} {
	missingRules := make([]map[string]interface{}, 0, len(candidates))
	for _, candidate := range candidates {
		if !hasClashModeRule(rules, candidate["clash_mode"]) {
			missingRules = append(missingRules, candidate)
		}
	}
	return missingRules
}

func hasClashModeRule(rules []interface{}, clashMode interface{}) bool {
	for _, item := range rules {
		rule, ok := item.(map[string]interface{})
		if ok && rule["clash_mode"] == clashMode {
			return true
		}
	}
	return false
}

func appendSingBoxRules(newRules []map[string]interface{}, existingRules ...interface{}) []interface{} {
	result := make([]interface{}, 0, len(newRules)+len(existingRules))
	for _, rule := range newRules {
		result = append(result, rule)
	}
	result = append(result, existingRules...)
	return result
}

func insertSingBoxRouteModeRules(rules []interface{}, modeRules []map[string]interface{}) []interface{} {
	insertAt := 0
	for index, item := range rules {
		rule, ok := item.(map[string]interface{})
		if !ok {
			break
		}
		action, _ := rule["action"].(string)
		protocol, _ := rule["protocol"].(string)
		if action == "sniff" || (action == "hijack-dns" && protocol == "dns") || rule["clash_mode"] == "direct" {
			insertAt = index + 1
			continue
		}
		break
	}

	result := make([]interface{}, 0, len(rules)+len(modeRules))
	result = append(result, rules[:insertAt]...)
	for _, rule := range modeRules {
		result = append(result, rule)
	}
	result = append(result, rules[insertAt:]...)
	return result
}

func normalizeSingBoxExperimental(singBoxConfig map[string]interface{}) {
	experimental, ok := singBoxConfig["experimental"].(map[string]interface{})
	if !ok {
		singBoxConfig["experimental"] = defaultSingBoxExperimental()
		return
	}
	clashAPI, ok := experimental["clash_api"].(map[string]interface{})
	if !ok {
		experimental["clash_api"] = map[string]interface{}{
			"default_mode": "rule",
		}
		return
	}
	if _, ok := clashAPI["default_mode"]; !ok {
		clashAPI["default_mode"] = "rule"
	}
}

func defaultSingBoxExperimental() map[string]interface{} {
	return map[string]interface{}{
		"clash_api": map[string]interface{}{
			"default_mode": "rule",
		},
	}
}

func normalizeSingBoxRuleSet(routeConfig map[string]interface{}) {
	ruleSetValue, ok := routeConfig["rule_set"]
	if !ok {
		routeConfig["rule_set"] = defaultSingBoxRuleSet()
		return
	}

	switch ruleSetItems := ruleSetValue.(type) {
	case []interface{}:
		for _, item := range ruleSetItems {
			if ruleSet, ok := item.(map[string]interface{}); ok {
				normalizeSingBoxRuleSetItem(ruleSet)
			}
		}
	case []map[string]interface{}:
		for _, ruleSet := range ruleSetItems {
			normalizeSingBoxRuleSetItem(ruleSet)
		}
	}
}

func normalizeSingBoxRuleSetItem(ruleSet map[string]interface{}) {
	if _, ok := ruleSet["http_client"]; ok {
		delete(ruleSet, "download_detour")
		return
	}
	if _, ok := ruleSet["download_detour"]; ok {
		ruleSet["http_client"] = "rule-set-downloader"
		delete(ruleSet, "download_detour")
	}
}

func defaultSingBoxRuleSet() []map[string]interface{} {
	return []map[string]interface{}{
		{
			"type":        "remote",
			"tag":         "geoip-cn",
			"format":      "binary",
			"url":         "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs",
			"http_client": "rule-set-downloader",
		},
		{
			"type":        "remote",
			"tag":         "geosite-cn",
			"format":      "binary",
			"url":         "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs",
			"http_client": "rule-set-downloader",
		},
	}
}

func buildSingBoxOutbound(item model.Node, pass string, username string) (map[string]interface{}, error) {
	switch *item.NodeTypeId {
	case constant.Xray:
		return buildSingBoxXrayOutbound(item, pass)
	case constant.TrojanGo:
		nodeTrojanGo, err := dao.SelectNodeTrojanGoById(item.NodeSubId)
		if err != nil {
			return nil, err
		}
		outbound := map[string]interface{}{
			"type":        "trojan",
			"tag":         *item.Name,
			"server":      *item.Domain,
			"server_port": *item.Port,
			"password":    pass,
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": *nodeTrojanGo.Sni,
			},
		}
		if *nodeTrojanGo.WebsocketEnable == 1 {
			outbound["transport"] = map[string]interface{}{
				"type":    "ws",
				"path":    *nodeTrojanGo.WebsocketPath,
				"headers": map[string]interface{}{"Host": *nodeTrojanGo.WebsocketHost},
			}
		}
		return outbound, nil
	case constant.Hysteria:
		nodeHysteria, err := dao.SelectNodeHysteriaById(item.NodeSubId)
		if err != nil {
			return nil, err
		}
		outbound := map[string]interface{}{
			"type":        "hysteria",
			"tag":         *item.Name,
			"server":      *item.Domain,
			"server_port": *item.Port,
			"auth_str":    pass,
			"up_mbps":     *nodeHysteria.UpMbps,
			"down_mbps":   *nodeHysteria.DownMbps,
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": *nodeHysteria.ServerName,
				"insecure":    *nodeHysteria.Insecure == 1,
			},
		}
		if nodeHysteria.Obfs != nil && *nodeHysteria.Obfs != "" {
			outbound["obfs"] = *nodeHysteria.Obfs
		}
		return outbound, nil
	case constant.NaiveProxy:
		outbound := map[string]interface{}{
			"type":        "naive",
			"tag":         *item.Name,
			"server":      *item.Domain,
			"server_port": *item.Port,
			"username":    username,
			"password":    pass,
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": *item.Domain,
			},
		}
		if item.NaiveUotEnable != nil && *item.NaiveUotEnable == 1 {
			version := uint(2)
			if item.NaiveUotVersion != nil && (*item.NaiveUotVersion == 1 || *item.NaiveUotVersion == 2) {
				version = *item.NaiveUotVersion
			}
			outbound["udp_over_tcp"] = map[string]interface{}{
				"enabled": true,
				"version": version,
			}
		}
		return outbound, nil
	case constant.Hysteria2:
		nodeHysteria2, err := dao.SelectNodeHysteria2ById(item.NodeSubId)
		if err != nil {
			return nil, err
		}
		outbound := map[string]interface{}{
			"type":        "hysteria2",
			"tag":         *item.Name,
			"server":      *item.Domain,
			"server_port": *item.Port,
			"password":    pass,
			"up_mbps":     *nodeHysteria2.UpMbps,
			"down_mbps":   *nodeHysteria2.DownMbps,
			"tls": map[string]interface{}{
				"enabled":     true,
				"server_name": *nodeHysteria2.ServerName,
				"insecure":    *nodeHysteria2.Insecure == 1,
			},
		}
		if nodeHysteria2.ObfsPassword != nil && *nodeHysteria2.ObfsPassword != "" {
			outbound["obfs"] = map[string]interface{}{
				"type":     "salamander",
				"password": *nodeHysteria2.ObfsPassword,
			}
		}
		if serverPorts := singBoxHysteria2ServerPorts(nodeHysteria2.PortHopping); len(serverPorts) > 0 {
			delete(outbound, "server_port")
			outbound["server_ports"] = serverPorts
			if hopInterval := uintValue(nodeHysteria2.HopInterval); hopInterval > 0 {
				outbound["hop_interval"] = fmt.Sprintf("%ds", hopInterval)
			}
		}
		return outbound, nil
	}
	return nil, nil
}

func buildSingBoxXrayOutbound(item model.Node, pass string) (map[string]interface{}, error) {
	nodeXray, err := dao.SelectNodeXrayById(item.NodeSubId)
	if err != nil {
		return nil, err
	}
	streamSettings := bo.StreamSettings{}
	if nodeXray.StreamSettings != nil && *nodeXray.StreamSettings != "" {
		if err = json.Unmarshal([]byte(*nodeXray.StreamSettings), &streamSettings); err != nil {
			logrus.Errorln(fmt.Sprintf("StreamSettings JSON deserialization err: %v", err))
			return nil, errors.New(constant.SysError)
		}
	}
	settings := bo.Settings{}
	if nodeXray.Settings != nil && *nodeXray.Settings != "" {
		if err = json.Unmarshal([]byte(*nodeXray.Settings), &settings); err != nil {
			logrus.Errorln(fmt.Sprintf("Settings JSON deserialization err: %v", err))
			return nil, errors.New(constant.SysError)
		}
	}

	outbound := map[string]interface{}{
		"type":        *nodeXray.Protocol,
		"tag":         *item.Name,
		"server":      *item.Domain,
		"server_port": *item.Port,
	}
	switch *nodeXray.Protocol {
	case constant.ProtocolVless:
		outbound["uuid"] = util.GenerateUUID(pass)
		if nodeXray.XrayFlow != nil && *nodeXray.XrayFlow != "" {
			outbound["flow"] = *nodeXray.XrayFlow
		}
	case constant.ProtocolVmess:
		outbound["uuid"] = util.GenerateUUID(pass)
		outbound["security"] = "auto"
	case constant.ProtocolTrojan:
		outbound["password"] = pass
	case constant.ProtocolShadowsocks:
		outbound["type"] = "shadowsocks"
		outbound["method"] = *nodeXray.XraySSMethod
		outbound["password"] = pass
	case constant.ProtocolSocks:
		outbound["type"] = "socks"
		if len(settings.Accounts) > 0 {
			outbound["username"] = settings.Accounts[0].User
			outbound["password"] = settings.Accounts[0].Pass
		}
	default:
		return nil, nil
	}
	applySingBoxXrayClientOptions(outbound, nodeXray)

	if streamSettings.Security == "tls" {
		tlsConfig := map[string]interface{}{
			"enabled": true,
		}
		if streamSettings.TlsSettings.ServerName != "" {
			tlsConfig["server_name"] = streamSettings.TlsSettings.ServerName
		}
		if streamSettings.TlsSettings.AllowInsecure {
			tlsConfig["insecure"] = true
		}
		if streamSettings.TlsSettings.Fingerprint != "" {
			tlsConfig["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": streamSettings.TlsSettings.Fingerprint,
			}
		}
		outbound["tls"] = tlsConfig
	} else if streamSettings.Security == "reality" {
		tlsConfig := map[string]interface{}{
			"enabled": true,
			"reality": map[string]interface{}{
				"enabled":    true,
				"public_key": *nodeXray.RealityPbk,
			},
		}
		if len(streamSettings.RealitySettings.ServerNames) > 0 {
			tlsConfig["server_name"] = streamSettings.RealitySettings.ServerNames[0]
		}
		if len(streamSettings.RealitySettings.ShortIds) > 0 {
			tlsConfig["reality"].(map[string]interface{})["short_id"] = streamSettings.RealitySettings.ShortIds[0]
		}
		if streamSettings.RealitySettings.Fingerprint != "" {
			tlsConfig["utls"] = map[string]interface{}{
				"enabled":     true,
				"fingerprint": streamSettings.RealitySettings.Fingerprint,
			}
		}
		outbound["tls"] = tlsConfig
	}
	if streamSettings.Network == "ws" {
		outbound["transport"] = map[string]interface{}{
			"type":    "ws",
			"path":    streamSettings.WsSettings.Path,
			"headers": map[string]interface{}{"Host": streamSettings.WsSettings.Headers.Host},
		}
	}
	return outbound, nil
}

func applySingBoxXrayClientOptions(outbound map[string]interface{}, nodeXray *model.NodeXray) {
	switch stringValue(nodeXray.Protocol) {
	case constant.ProtocolShadowsocks:
		if uintValue(nodeXray.UotEnable) == 1 {
			outbound["udp_over_tcp"] = map[string]interface{}{
				"enabled": true,
				"version": xrayUotVersion(nodeXray.UotVersion),
			}
		}
	case constant.ProtocolVless, constant.ProtocolVmess:
		if uintValue(nodeXray.XudpEnable) == 1 {
			outbound["packet_encoding"] = "xudp"
		}
		if uintValue(nodeXray.MuxEnable) == 1 {
			outbound["multiplex"] = map[string]interface{}{"enabled": true, "protocol": "h2mux"}
		}
	case constant.ProtocolTrojan:
		if uintValue(nodeXray.MuxEnable) == 1 {
			outbound["multiplex"] = map[string]interface{}{"enabled": true, "protocol": "h2mux"}
		}
	}
}

func xrayUotVersion(version *uint) uint {
	if version != nil && (*version == 1 || *version == 2) {
		return *version
	}
	return 2
}

package service

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/skip2/go-qrcode"
	"net/url"
	"strings"
	"sync"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/dao/redis"
	"trojan-panel/model"
	"trojan-panel/model/bo"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
	"trojan-panel/service/clientcompat"
	"trojan-panel/util"
)

func SelectNodeById(id *uint) (*vo.NodeOneVo, error) {
	node, err := dao.SelectNodeById(id)
	if err != nil {
		return nil, err
	}
	if isRetiredNodeType(*node.NodeTypeId) {
		return nil, errors.New("retired nodes are not available")
	}
	if node != nil {
		nodeOneVo := vo.NodeOneVo{
			Id:              *node.Id,
			NodeServerId:    *node.NodeServerId,
			NodeSubId:       *node.NodeSubId,
			NodeTypeId:      *node.NodeTypeId,
			Name:            *node.Name,
			Domain:          *node.Domain,
			Port:            *node.Port,
			Priority:        *node.Priority,
			Clients:         clientcompat.Decode(node.ClientTypes),
			NaiveUotEnable:  nodeUintValue(node.NaiveUotEnable, 0),
			NaiveUotVersion: nodeUintValue(node.NaiveUotVersion, 2),
			CreateTime:      *node.CreateTime,
		}
		nodeTypeId := node.NodeTypeId
		switch *nodeTypeId {
		case constant.Xray:
			nodeXray, err := dao.SelectNodeXrayById(node.NodeSubId)
			if err != nil {
				return nil, err
			}
			nodeOneVo.XrayProtocol = *nodeXray.Protocol
			nodeOneVo.XrayFlow = *nodeXray.XrayFlow
			nodeOneVo.XraySSMethod = *nodeXray.XraySSMethod
			nodeOneVo.XrayUotEnable = nodeUintValue(nodeXray.UotEnable, 0)
			nodeOneVo.XrayUotVersion = nodeUintValue(nodeXray.UotVersion, 2)
			nodeOneVo.XrayXudpEnable = nodeUintValue(nodeXray.XudpEnable, 0)
			nodeOneVo.XrayMuxEnable = nodeUintValue(nodeXray.MuxEnable, 0)
			nodeOneVo.RealityPbk = *nodeXray.RealityPbk
			xraySettingEntity := vo.XraySettingEntity{}
			if nodeXray.Settings != nil && *nodeXray.Settings != "" {
				if err = json.Unmarshal([]byte(*nodeXray.Settings), &xraySettingEntity); err != nil {
					logrus.Errorln(fmt.Sprintf("Settings deserialization err: %v", err))
					return nil, errors.New(constant.SysError)
				}
			}
			nodeOneVo.XraySettingEntity = xraySettingEntity
			xrayStreamSettingsEntity := vo.XrayStreamSettingsEntity{}
			if nodeXray.StreamSettings != nil && *nodeXray.StreamSettings != "" {
				if err = json.Unmarshal([]byte(*nodeXray.StreamSettings), &xrayStreamSettingsEntity); err != nil {
					logrus.Errorln(fmt.Sprintf("StreamSettings deserialization err: %v", err))
					return nil, errors.New(constant.SysError)
				}
			}
			nodeOneVo.XrayStreamSettingsEntity = xrayStreamSettingsEntity
			nodeOneVo.XrayTag = *nodeXray.Tag
		case constant.TrojanGo:
			nodeTrojanGo, err := dao.SelectNodeTrojanGoById(node.NodeSubId)
			if err != nil {
				return nil, err
			}
			nodeOneVo.TrojanGoSni = *nodeTrojanGo.Sni
			nodeOneVo.TrojanGoMuxEnable = *nodeTrojanGo.MuxEnable
			nodeOneVo.TrojanGoWebsocketEnable = *nodeTrojanGo.WebsocketEnable
			nodeOneVo.TrojanGoWebsocketPath = *nodeTrojanGo.WebsocketPath
			nodeOneVo.TrojanGoWebsocketHost = *nodeTrojanGo.WebsocketHost
			nodeOneVo.TrojanGoSsEnable = *nodeTrojanGo.SsEnable
			nodeOneVo.TrojanGoSsMethod = *nodeTrojanGo.SsMethod
			nodeOneVo.TrojanGoSsPassword = *nodeTrojanGo.SsPassword
		case constant.Hysteria:
			nodeHysteria, err := dao.SelectNodeHysteriaById(node.NodeSubId)
			if err != nil {
				return nil, err
			}
			nodeOneVo.HysteriaProtocol = *nodeHysteria.Protocol
			nodeOneVo.HysteriaObfs = *nodeHysteria.Obfs
			nodeOneVo.HysteriaUpMbps = *nodeHysteria.UpMbps
			nodeOneVo.HysteriaDownMbps = *nodeHysteria.DownMbps
			nodeOneVo.HysteriaServerName = *nodeHysteria.ServerName
			nodeOneVo.HysteriaInsecure = *nodeHysteria.Insecure
			nodeOneVo.HysteriaFastOpen = *nodeHysteria.FastOpen
		case constant.Hysteria2:
			nodeHysteria2, err := dao.SelectNodeHysteria2ById(node.NodeSubId)
			if err != nil {
				return nil, err
			}
			nodeOneVo.Hysteria2ObfsPassword = *nodeHysteria2.ObfsPassword
			nodeOneVo.Hysteria2UpMbps = *nodeHysteria2.UpMbps
			nodeOneVo.Hysteria2DownMbps = *nodeHysteria2.DownMbps
			nodeOneVo.Hysteria2ServerName = *nodeHysteria2.ServerName
			nodeOneVo.Hysteria2Insecure = *nodeHysteria2.Insecure
			nodeOneVo.Hysteria2PortHopping = stringValue(nodeHysteria2.PortHopping)
			nodeOneVo.Hysteria2HopInterval = uintValue(nodeHysteria2.HopInterval)
		}
		return &nodeOneVo, nil
	}
	return nil, errors.New(constant.NodeNotExist)
}
func SelectNodeInfo(id *uint, c *gin.Context) (*vo.NodeOneVo, error) {

	nodeOneVo, err := SelectNodeById(id)
	if err != nil {
		return nil, err
	}
	accountInfo := GetCurrentAccount(c)
	account, err := dao.SelectAccountByUsername(&accountInfo.Username)
	if err != nil {
		return nil, err
	}
	nodeOneVo.Password = *account.Pass
	if nodeOneVo.NodeTypeId == constant.Xray && (nodeOneVo.XrayProtocol == "vless" || nodeOneVo.XrayProtocol == "vmess") {
		nodeOneVo.Uuid = util.GenerateUUID(*account.Pass)
		nodeOneVo.AlterId = 0
	}
	if nodeOneVo.NodeTypeId == constant.NaiveProxy {
		nodeOneVo.NaiveProxyUsername = accountInfo.Username
	}
	return nodeOneVo, nil
}

func CreateNode(token string, nodeCreateDto dto.NodeCreateDto) error {
	if nodeCreateDto.NodeTypeId != nil && isRetiredNodeType(*nodeCreateDto.NodeTypeId) {
		return errors.New("Trojan-Go and Hysteria v1 are retired")
	}
	if err := clientcompat.ValidateNode(nodeCreateDto.NodeTypeId, nodeCreateDto.Clients); err != nil {
		return err
	}
	clientTypes, err := clientcompat.Encode(nodeCreateDto.Clients)
	if err != nil {
		return err
	}

	// 校验端口
	if nodeCreateDto.Port != nil && (*nodeCreateDto.Port <= 100 || *nodeCreateDto.Port >= 30000) {
		return errors.New(constant.PortRangeError)
	}

	// 校验名称
	countName, err := dao.CountNodeByNameAndNodeServerId(nil, nodeCreateDto.Name, nil)
	if err != nil {
		return err
	}
	if countName > 0 {
		return errors.New(constant.NodeNameExist)
	}

	nodeServer, err := dao.SelectNodeServer(map[string]interface{}{"id": *nodeCreateDto.NodeServerId})
	if err != nil {
		return err
	}

	systemName := constant.SystemName
	systemConfig, err := SelectSystemByName(&systemName)
	if err != nil {
		return err
	}

	var nodeId uint
	mutex, err := redis.RsLock(constant.CreateNodeLock)
	if err != nil {
		return err
	}
	defer redis.RsUnLock(mutex)
	// Grpc添加节点
	GrpcAddNode(token, *nodeServer.Ip, *nodeServer.GrpcPort, &core.NodeAddDto{
		NodeTypeId: uint64(*nodeCreateDto.NodeTypeId),
		Port:       uint64(*nodeCreateDto.Port),
		Domain:     *nodeCreateDto.Domain,

		//  Xray
		XrayTemplate:       systemConfig.XrayTemplate,
		XrayFlow:           stringValue(nodeCreateDto.XrayFlow),
		XraySSMethod:       stringValue(nodeCreateDto.XraySSMethod),
		XrayProtocol:       stringValue(nodeCreateDto.XrayProtocol),
		XraySettings:       stringValue(nodeCreateDto.XraySettings),
		XrayStreamSettings: stringValue(nodeCreateDto.XrayStreamSettings),
		XrayTag:            stringValue(nodeCreateDto.XrayTag),
		XraySniffing:       stringValue(nodeCreateDto.XraySniffing),
		XrayAllocate:       stringValue(nodeCreateDto.XrayAllocate),
		// Hysteria2
		Hysteria2ObfsPassword: stringValue(nodeCreateDto.Hysteria2ObfsPassword),
		Hysteria2UpMbps:       int64(*nodeCreateDto.Hysteria2UpMbps),
		Hysteria2DownMbps:     int64(*nodeCreateDto.Hysteria2DownMbps),
	}, nodeTransport(nodeServer))
	// 数据插入到数据库中
	if *nodeCreateDto.NodeTypeId == constant.Xray {
		nodeXray := model.NodeXray{
			Protocol:       nodeCreateDto.XrayProtocol,
			XrayFlow:       nodeCreateDto.XrayFlow,
			XraySSMethod:   nodeCreateDto.XraySSMethod,
			UotEnable:      nodeCreateDto.XrayUotEnable,
			UotVersion:     nodeCreateDto.XrayUotVersion,
			XudpEnable:     nodeCreateDto.XrayXudpEnable,
			MuxEnable:      nodeCreateDto.XrayMuxEnable,
			RealityPbk:     nodeCreateDto.RealityPbk,
			Settings:       nodeCreateDto.XraySettings,
			StreamSettings: nodeCreateDto.XrayStreamSettings,
			Tag:            nodeCreateDto.XrayTag,
			Sniffing:       nodeCreateDto.XraySniffing,
			Allocate:       nodeCreateDto.XrayAllocate,
		}
		nodeId, err = dao.CreateNodeXray(&nodeXray)
		if err != nil {
			return err
		}
	} else if *nodeCreateDto.NodeTypeId == constant.TrojanGo {
		trojanGo := model.NodeTrojanGo{
			Sni:             nodeCreateDto.TrojanGoSni,
			MuxEnable:       nodeCreateDto.TrojanGoMuxEnable,
			WebsocketEnable: nodeCreateDto.TrojanGoWebsocketEnable,
			WebsocketPath:   nodeCreateDto.TrojanGoWebsocketPath,
			WebsocketHost:   nodeCreateDto.TrojanGoWebsocketHost,
			SsEnable:        nodeCreateDto.TrojanGoSsEnable,
			SsMethod:        nodeCreateDto.TrojanGoSsMethod,
			SsPassword:      nodeCreateDto.TrojanGoSsPassword,
		}
		nodeId, err = dao.CreateNodeTrojanGo(&trojanGo)
		if err != nil {
			return err
		}
	} else if *nodeCreateDto.NodeTypeId == constant.Hysteria {
		hysteria := model.NodeHysteria{
			Protocol:   nodeCreateDto.HysteriaProtocol,
			Obfs:       nodeCreateDto.HysteriaObfs,
			UpMbps:     nodeCreateDto.HysteriaUpMbps,
			DownMbps:   nodeCreateDto.HysteriaDownMbps,
			ServerName: nodeCreateDto.HysteriaServerName,
			Insecure:   nodeCreateDto.HysteriaInsecure,
			FastOpen:   nodeCreateDto.HysteriaFastOpen,
		}
		nodeId, err = dao.CreateNodeHysteria(&hysteria)
		if err != nil {
			return err
		}
	} else if *nodeCreateDto.NodeTypeId == constant.Hysteria2 {
		hysteria2 := model.NodeHysteria2{
			ObfsPassword: nodeCreateDto.Hysteria2ObfsPassword,
			UpMbps:       nodeCreateDto.Hysteria2UpMbps,
			DownMbps:     nodeCreateDto.Hysteria2DownMbps,
			ServerName:   nodeCreateDto.Hysteria2ServerName,
			Insecure:     nodeCreateDto.Hysteria2Insecure,
			PortHopping:  nodeCreateDto.Hysteria2PortHopping,
			HopInterval:  nodeCreateDto.Hysteria2HopInterval,
		}
		nodeId, err = dao.CreateNodeHysteria2(&hysteria2)
		if err != nil {
			return err
		}
	}

	// 在主表中插入数据
	node := model.Node{
		NodeServerId:       nodeCreateDto.NodeServerId,
		NodeSubId:          &nodeId,
		NodeTypeId:         nodeCreateDto.NodeTypeId,
		Name:               nodeCreateDto.Name,
		NodeServerIp:       nodeServer.Ip,
		NodeServerGrpcPort: nodeServer.GrpcPort,
		Domain:             nodeCreateDto.Domain,
		Port:               nodeCreateDto.Port,
		Priority:           nodeCreateDto.Priority,
		ClientTypes:        &clientTypes,
		NaiveUotEnable:     nodeCreateDto.NaiveUotEnable,
		NaiveUotVersion:    nodeCreateDto.NaiveUotVersion,
	}
	if err = dao.CreateNode(&node); err != nil {
		return err
	}
	return nil
}

func SelectNodePage(queryName *string, nodeServerId *uint, pageNum *uint, pageSize *uint, c *gin.Context) (*vo.NodePageVo, error) {
	nodePage, total, err := dao.SelectNodePage(queryName, nodeServerId, pageNum, pageSize)
	if err != nil {
		return nil, err
	}
	nodeBos := make([]bo.NodeBo, 0)
	nodeServerTransports := make(map[uint]core.NodeTransport)
	for _, item := range *nodePage {
		transport, ok := nodeServerTransports[*item.NodeServerId]
		if !ok {
			nodeServer, selectErr := dao.SelectNodeServer(map[string]interface{}{"id": *item.NodeServerId})
			if selectErr != nil {
				return nil, selectErr
			}
			transport = nodeTransport(nodeServer)
			nodeServerTransports[*item.NodeServerId] = transport
		}
		nodeBo := bo.NodeBo{
			Id:                 *item.Id,
			NodeServerId:       *item.NodeServerId,
			NodeSubId:          *item.NodeSubId,
			NodeTypeId:         *item.NodeTypeId,
			Name:               *item.Name,
			NodeServerIp:       *item.NodeServerIp,
			NodeServerGrpcPort: *item.NodeServerGrpcPort,
			GrpcTLSMode:        transport.Mode,
			GrpcTLSServerName:  transport.ServerName,
			Domain:             *item.Domain,
			Port:               *item.Port,
			Priority:           *item.Priority,
			Clients:            clientcompat.Decode(item.ClientTypes),
			NaiveUotEnable:     nodeUintValue(item.NaiveUotEnable, 0),
			NaiveUotVersion:    nodeUintValue(item.NaiveUotVersion, 2),
			CreateTime:         *item.CreateTime,
		}
		nodeBos = append(nodeBos, nodeBo)
	}

	account := GetCurrentAccount(c)
	if util.IsAdmin(account.Roles) {
		token := util.GetToken(c)
		var nodeMap sync.Map
		var wg sync.WaitGroup
		wg.Add(len(nodeBos))
		for i := range nodeBos {
			indexI := i
			go func() {
				var ip = nodeBos[indexI].NodeServerIp
				var grpcPort = nodeBos[indexI].NodeServerGrpcPort
				var nodeTypeId = nodeBos[indexI].NodeTypeId
				var port = nodeBos[indexI].Port
				status, ok := nodeMap.Load(fmt.Sprintf("%s:%d:%d", ip, nodeTypeId, port))
				if ok {
					nodeBos[indexI].Status = status.(int)
				} else {
					var nodeState int
					nodeStateVo, err := core.GetNodeState(token, ip, grpcPort, nodeTypeId, port, core.NodeTransport{
						Mode:       nodeBos[indexI].GrpcTLSMode,
						ServerName: nodeBos[indexI].GrpcTLSServerName,
					})
					if err != nil || nodeStateVo.GetStatus() == 0 {
						nodeState = 0
					} else {
						nodeState = 1
					}
					nodeBos[indexI].Status = nodeState
					nodeMap.Store(fmt.Sprintf("%s:%d:%d", ip, nodeTypeId, port), nodeState)
				}
				wg.Done()
			}()
		}
		wg.Wait()
	}

	nodeVos := make([]vo.NodeVo, 0)
	serverIDs := make([]uint, 0, len(nodeServerTransports))
	for id := range nodeServerTransports {
		serverIDs = append(serverIDs, id)
	}
	trafficStatuses, err := ServerTrafficStatuses(serverIDs)
	if err != nil {
		return nil, err
	}
	for _, item := range nodeBos {
		nodeVo := vo.NodeVo{
			Id:              item.Id,
			NodeServerId:    item.NodeServerId,
			NodeSubId:       item.NodeSubId,
			NodeTypeId:      item.NodeTypeId,
			Name:            item.Name,
			Domain:          item.Domain,
			Port:            item.Port,
			Priority:        item.Priority,
			Clients:         item.Clients,
			NaiveUotEnable:  item.NaiveUotEnable,
			NaiveUotVersion: item.NaiveUotVersion,
			CreateTime:      item.CreateTime,
			Status:          item.Status,
			ServerTraffic:   trafficStatuses[item.NodeServerId],
		}
		nodeVos = append(nodeVos, nodeVo)
	}

	nodePageVo := vo.NodePageVo{
		BaseVoPage: vo.BaseVoPage{
			PageNum:  *pageNum,
			PageSize: *pageSize,
			Total:    total,
		},
		Nodes: nodeVos,
	}
	return &nodePageVo, nil
}

// DeleteNodeById 删除远程节点 删除分表 删除主表
func DeleteNodeById(token string, id *uint) error {
	mutex, err := redis.RsLock(constant.DeleteNodeByIdLock)
	if err != nil {
		return err
	}
	defer redis.RsUnLock(mutex)
	node, err := dao.SelectNodeById(id)
	if err != nil {
		return err
	}
	if isRetiredNodeType(*node.NodeTypeId) {
		return errors.New("retired nodes cannot be deleted through the panel")
	}
	nodeServer, err := dao.SelectNodeServer(map[string]interface{}{"id": *node.NodeServerId})
	if err != nil {
		return err
	}
	GrpcRemoveNode(token, *node.NodeServerIp, *node.NodeServerGrpcPort, *node.Port, *node.NodeTypeId, nodeTransport(nodeServer))
	if *node.NodeTypeId == constant.Xray {
		if err := dao.DeleteNodeXrayById(node.NodeSubId); err != nil {
			return err
		}
	} else if *node.NodeTypeId == constant.TrojanGo {
		if err := dao.DeleteNodeTrojanGoById(node.NodeSubId); err != nil {
			return err
		}
	} else if *node.NodeTypeId == constant.Hysteria {
		if err := dao.DeleteNodeHysteriaById(node.NodeSubId); err != nil {
			return err
		}
	} else if *node.NodeTypeId == constant.Hysteria2 {
		if err := dao.DeleteNodeHysteria2ById(node.NodeSubId); err != nil {
			return err
		}
	}
	if err = dao.DeleteNodeById(id); err != nil {
		return err
	}
	return nil
}

func UpdateNodeById(token string, nodeUpdateDto *dto.NodeUpdateDto) error {
	if nodeUpdateDto.NodeTypeId != nil && isRetiredNodeType(*nodeUpdateDto.NodeTypeId) {
		return errors.New("Trojan-Go and Hysteria v1 are retired")
	}
	if err := clientcompat.ValidateNode(nodeUpdateDto.NodeTypeId, nodeUpdateDto.Clients); err != nil {
		return err
	}
	var clientTypes *string
	if nodeUpdateDto.Clients != nil {
		value, err := clientcompat.Encode(nodeUpdateDto.Clients)
		if err != nil {
			return err
		}
		clientTypes = &value
	}

	// 校验端口
	if nodeUpdateDto.Port != nil && (*nodeUpdateDto.Port <= 100 || *nodeUpdateDto.Port >= 30000) {
		return errors.New(constant.PortRangeError)
	}

	// 校验名称
	count, err := dao.CountNodeByNameAndNodeServerId(nodeUpdateDto.Id, nodeUpdateDto.Name, nil)
	if err != nil {
		return err
	}
	if count > 0 {
		return errors.New(constant.NodeNameExist)
	}

	nodeServer, err := dao.SelectNodeServer(map[string]interface{}{"id": *nodeUpdateDto.NodeServerId})
	if err != nil {
		return err
	}

	systemName := constant.SystemName
	systemConfig, err := SelectSystemByName(&systemName)
	if err != nil {
		return err
	}

	mutex, err := redis.RsLock(constant.UpdateNodeByIdLock)
	if err != nil {
		return err
	}
	defer redis.RsUnLock(mutex)

	nodeEntity, err := dao.SelectNodeById(nodeUpdateDto.Id)
	if err != nil {
		return err
	}
	if isRetiredNodeType(*nodeEntity.NodeTypeId) {
		return errors.New("retired nodes cannot be edited")
	}
	oldNodeServer, err := dao.SelectNodeServer(map[string]interface{}{"id": *nodeEntity.NodeServerId})
	if err != nil {
		return err
	}
	// Grpc的操作
	GrpcRemoveNode(token, *nodeEntity.NodeServerIp, *nodeEntity.NodeServerGrpcPort, *nodeEntity.Port, *nodeEntity.NodeTypeId, nodeTransport(oldNodeServer))
	GrpcAddNode(token, *nodeServer.Ip, *nodeServer.GrpcPort, &core.NodeAddDto{
		NodeTypeId: uint64(*nodeUpdateDto.NodeTypeId),
		Port:       uint64(*nodeUpdateDto.Port),
		Domain:     *nodeUpdateDto.Domain,

		//  Xray
		XrayTemplate:       systemConfig.XrayTemplate,
		XrayProtocol:       *nodeUpdateDto.XrayProtocol,
		XrayFlow:           *nodeUpdateDto.XrayFlow,
		XraySSMethod:       *nodeUpdateDto.XraySSMethod,
		XraySettings:       *nodeUpdateDto.XraySettings,
		XrayStreamSettings: *nodeUpdateDto.XrayStreamSettings,
		XrayTag:            *nodeUpdateDto.XrayTag,
		XraySniffing:       *nodeUpdateDto.XraySniffing,
		XrayAllocate:       *nodeUpdateDto.XrayAllocate,
		// Hysteria2
		Hysteria2ObfsPassword: *nodeUpdateDto.Hysteria2ObfsPassword,
		Hysteria2UpMbps:       int64(*nodeUpdateDto.Hysteria2UpMbps),
		Hysteria2DownMbps:     int64(*nodeUpdateDto.Hysteria2DownMbps),
	}, nodeTransport(nodeServer))

	if *nodeUpdateDto.NodeTypeId == *nodeEntity.NodeTypeId {
		// 没有修改节点类型的情况
		if *nodeEntity.NodeTypeId == constant.Xray {
			nodeXray := model.NodeXray{
				Id:             nodeEntity.NodeSubId,
				Protocol:       nodeUpdateDto.XrayProtocol,
				XrayFlow:       nodeUpdateDto.XrayFlow,
				XraySSMethod:   nodeUpdateDto.XraySSMethod,
				UotEnable:      nodeUpdateDto.XrayUotEnable,
				UotVersion:     nodeUpdateDto.XrayUotVersion,
				XudpEnable:     nodeUpdateDto.XrayXudpEnable,
				MuxEnable:      nodeUpdateDto.XrayMuxEnable,
				Settings:       nodeUpdateDto.XraySettings,
				StreamSettings: nodeUpdateDto.XrayStreamSettings,
				Tag:            nodeUpdateDto.XrayTag,
				Sniffing:       nodeUpdateDto.XraySniffing,
				Allocate:       nodeUpdateDto.XrayAllocate,
			}
			if err = dao.UpdateNodeXrayById(&nodeXray); err != nil {
				return err
			}
		} else if *nodeEntity.NodeTypeId == constant.TrojanGo {
			nodeTrojanGo := model.NodeTrojanGo{
				Id:              nodeEntity.NodeSubId,
				Sni:             nodeUpdateDto.TrojanGoSni,
				MuxEnable:       nodeUpdateDto.TrojanGoMuxEnable,
				WebsocketEnable: nodeUpdateDto.TrojanGoWebsocketEnable,
				WebsocketPath:   nodeUpdateDto.TrojanGoWebsocketPath,
				SsEnable:        nodeUpdateDto.TrojanGoSsEnable,
				SsMethod:        nodeUpdateDto.TrojanGoSsMethod,
				SsPassword:      nodeUpdateDto.TrojanGoSsPassword,
			}
			if err = dao.UpdateNodeTrojanGoById(&nodeTrojanGo); err != nil {
				return err
			}
		} else if *nodeEntity.NodeTypeId == constant.Hysteria {
			nodeHysteria := model.NodeHysteria{
				Id:         nodeEntity.NodeSubId,
				Protocol:   nodeUpdateDto.HysteriaProtocol,
				Obfs:       nodeUpdateDto.HysteriaObfs,
				UpMbps:     nodeUpdateDto.HysteriaUpMbps,
				DownMbps:   nodeUpdateDto.HysteriaDownMbps,
				ServerName: nodeUpdateDto.HysteriaServerName,
				Insecure:   nodeUpdateDto.HysteriaInsecure,
				FastOpen:   nodeUpdateDto.HysteriaFastOpen,
			}
			if err = dao.UpdateNodeHysteriaById(&nodeHysteria); err != nil {
				return err
			}
		} else if *nodeEntity.NodeTypeId == constant.Hysteria2 {
			nodeHysteria2 := model.NodeHysteria2{
				Id:           nodeEntity.NodeSubId,
				ObfsPassword: nodeUpdateDto.Hysteria2ObfsPassword,
				UpMbps:       nodeUpdateDto.Hysteria2UpMbps,
				DownMbps:     nodeUpdateDto.Hysteria2DownMbps,
				ServerName:   nodeUpdateDto.Hysteria2ServerName,
				Insecure:     nodeUpdateDto.Hysteria2Insecure,
				PortHopping:  nodeUpdateDto.Hysteria2PortHopping,
				HopInterval:  nodeUpdateDto.Hysteria2HopInterval,
			}
			if err = dao.UpdateNodeHysteria2ById(&nodeHysteria2); err != nil {
				return err
			}
		}
		if *nodeEntity.NodeServerId != *nodeUpdateDto.NodeServerId ||
			*nodeEntity.Name != *nodeUpdateDto.Name ||
			*nodeEntity.NodeServerIp != *nodeServer.Ip ||
			*nodeEntity.Domain != *nodeUpdateDto.Domain ||
			*nodeEntity.Port != *nodeUpdateDto.Port ||
			*nodeEntity.Priority != *nodeUpdateDto.Priority ||
			(clientTypes != nil && stringValue(nodeEntity.ClientTypes) != *clientTypes) ||
			nodeUintValue(nodeEntity.NaiveUotEnable, 0) != nodeUintValue(nodeUpdateDto.NaiveUotEnable, 0) ||
			nodeUintValue(nodeEntity.NaiveUotVersion, 2) != nodeUintValue(nodeUpdateDto.NaiveUotVersion, 2) {
			node := model.Node{
				Id:              nodeUpdateDto.Id,
				NodeServerId:    nodeUpdateDto.NodeServerId,
				Name:            nodeUpdateDto.Name,
				NodeServerIp:    nodeServer.Ip,
				Domain:          nodeUpdateDto.Domain,
				Port:            nodeUpdateDto.Port,
				Priority:        nodeUpdateDto.Priority,
				ClientTypes:     clientTypes,
				NaiveUotEnable:  nodeUpdateDto.NaiveUotEnable,
				NaiveUotVersion: nodeUpdateDto.NaiveUotVersion,
			}
			if err = dao.UpdateNodeById(&node); err != nil {
				return err
			}
		}
	} else {
		// 修改了节点类型的情况 需要删除分库的数据，然后重新再插入
		if *nodeEntity.NodeTypeId == constant.Xray {
			if err = dao.DeleteNodeXrayById(nodeEntity.NodeSubId); err != nil {
				return err
			}
		} else if *nodeEntity.NodeTypeId == constant.TrojanGo {
			if err = dao.DeleteNodeTrojanGoById(nodeEntity.NodeSubId); err != nil {
				return err
			}
		} else if *nodeEntity.NodeTypeId == constant.Hysteria {
			if err = dao.DeleteNodeHysteriaById(nodeEntity.NodeSubId); err != nil {
				return err
			}
		} else if *nodeEntity.NodeTypeId == constant.Hysteria2 {
			if err = dao.DeleteNodeHysteria2ById(nodeEntity.NodeSubId); err != nil {
				return err
			}
		}

		// 修改了节点类型
		var nodeId uint
		if *nodeUpdateDto.NodeTypeId == constant.Xray {
			nodeXray := model.NodeXray{
				Protocol:       nodeUpdateDto.XrayProtocol,
				XrayFlow:       nodeUpdateDto.XrayFlow,
				XraySSMethod:   nodeUpdateDto.XraySSMethod,
				UotEnable:      nodeUpdateDto.XrayUotEnable,
				UotVersion:     nodeUpdateDto.XrayUotVersion,
				XudpEnable:     nodeUpdateDto.XrayXudpEnable,
				MuxEnable:      nodeUpdateDto.XrayMuxEnable,
				Settings:       nodeUpdateDto.XraySettings,
				StreamSettings: nodeUpdateDto.XrayStreamSettings,
				Tag:            nodeUpdateDto.XrayTag,
				Sniffing:       nodeUpdateDto.XraySniffing,
				Allocate:       nodeUpdateDto.XrayAllocate,
			}
			nodeId, err = dao.CreateNodeXray(&nodeXray)
			if err != nil {
				return nil
			}
		} else if *nodeUpdateDto.NodeTypeId == constant.TrojanGo {
			trojanGo := model.NodeTrojanGo{
				Sni:             nodeUpdateDto.TrojanGoSni,
				MuxEnable:       nodeUpdateDto.TrojanGoMuxEnable,
				WebsocketEnable: nodeUpdateDto.TrojanGoWebsocketEnable,
				WebsocketPath:   nodeUpdateDto.TrojanGoWebsocketPath,
				WebsocketHost:   nodeUpdateDto.TrojanGoWebsocketHost,
				SsEnable:        nodeUpdateDto.TrojanGoSsEnable,
				SsMethod:        nodeUpdateDto.TrojanGoSsMethod,
				SsPassword:      nodeUpdateDto.TrojanGoSsPassword,
			}
			nodeId, err = dao.CreateNodeTrojanGo(&trojanGo)
			if err != nil {
				return nil
			}
		} else if *nodeUpdateDto.NodeTypeId == constant.Hysteria {
			hysteria := model.NodeHysteria{
				Protocol:   nodeUpdateDto.HysteriaProtocol,
				Obfs:       nodeUpdateDto.HysteriaObfs,
				UpMbps:     nodeUpdateDto.HysteriaUpMbps,
				DownMbps:   nodeUpdateDto.HysteriaDownMbps,
				ServerName: nodeUpdateDto.HysteriaServerName,
				Insecure:   nodeUpdateDto.HysteriaInsecure,
				FastOpen:   nodeUpdateDto.HysteriaFastOpen,
			}
			nodeId, err = dao.CreateNodeHysteria(&hysteria)
			if err != nil {
				return nil
			}
		} else if *nodeUpdateDto.NodeTypeId == constant.Hysteria2 {
			hysteria2 := model.NodeHysteria2{
				ObfsPassword: nodeUpdateDto.Hysteria2ObfsPassword,
				UpMbps:       nodeUpdateDto.Hysteria2UpMbps,
				DownMbps:     nodeUpdateDto.Hysteria2DownMbps,
				ServerName:   nodeUpdateDto.Hysteria2ServerName,
				Insecure:     nodeUpdateDto.Hysteria2Insecure,
				PortHopping:  nodeUpdateDto.Hysteria2PortHopping,
				HopInterval:  nodeUpdateDto.Hysteria2HopInterval,
			}
			nodeId, err = dao.CreateNodeHysteria2(&hysteria2)
			if err != nil {
				return nil
			}
		}

		node := model.Node{
			Id:                 nodeUpdateDto.Id,
			NodeServerId:       nodeUpdateDto.NodeServerId,
			NodeSubId:          &nodeId,
			NodeTypeId:         nodeUpdateDto.NodeTypeId,
			Name:               nodeUpdateDto.Name,
			NodeServerIp:       nodeServer.Ip,
			NodeServerGrpcPort: nodeServer.GrpcPort,
			Domain:             nodeUpdateDto.Domain,
			Port:               nodeUpdateDto.Port,
			Priority:           nodeUpdateDto.Priority,
			ClientTypes:        clientTypes,
			NaiveUotEnable:     nodeUpdateDto.NaiveUotEnable,
			NaiveUotVersion:    nodeUpdateDto.NaiveUotVersion,
		}
		if err = dao.UpdateNodeById(&node); err != nil {
			return err
		}
	}
	return nil
}

func NodeQRCode(accountId *uint, username *string, id *uint) ([]byte, error) {
	nodeUrl, nodeTypeId, err := NodeURL(accountId, username, id)
	if err != nil {
		return nil, err
	}
	if nodeTypeId == constant.NaiveProxy {
		nodeUrl = strings.TrimPrefix(nodeUrl, "naive+https://")
		nodeUrl = fmt.Sprintf("https://%s", base64.StdEncoding.EncodeToString([]byte(nodeUrl)))
	}
	// 生成二维码
	qrCode, err := qrcode.Encode(nodeUrl, qrcode.Medium, 256)
	if err != nil {
		return nil, err
	}
	return qrCode, nil
}

// NodeURL
// xray: https://github.com/XTLS/Xray-core/issues/91
func NodeURL(accountId *uint, username *string, id *uint) (string, uint, error) {
	return nodeURLForClient(accountId, username, id, "")
}

func nodeURLForClient(accountId *uint, username *string, id *uint, client string) (string, uint, error) {
	node, err := dao.SelectNodeById(id)
	if err != nil {
		return "", 0, errors.New(constant.NodeURLError)
	}
	if isRetiredNodeType(*node.NodeTypeId) {
		return "", 0, errors.New(constant.NodeURLError)
	}

	nodeType, err := dao.SelectNodeTypeById(node.NodeTypeId)
	if err != nil {
		return "", 0, errors.New(constant.NodeURLError)
	}

	password, err := dao.SelectConnectPassword(accountId, nil)
	if err != nil {
		return "", 0, errors.New(constant.NodeURLError)
	}

	// 构建URL
	var headBuilder strings.Builder

	if *nodeType.Id == constant.Xray {
		nodeXray, err := dao.SelectNodeXrayById(node.NodeSubId)
		if err != nil {
			return "", 0, errors.New(constant.NodeURLError)
		}
		streamSettings := bo.StreamSettings{}
		if nodeXray.StreamSettings != nil && *nodeXray.StreamSettings != "" {
			if err := json.Unmarshal([]byte(*nodeXray.StreamSettings), &streamSettings); err != nil {
				return "", 0, errors.New(constant.NodeURLError)
			}
		}
		settings := bo.Settings{}
		if nodeXray.Settings != nil && *nodeXray.Settings != "" {
			if err := json.Unmarshal([]byte(*nodeXray.Settings), &settings); err != nil {
				return "", 0, errors.New(constant.NodeURLError)
			}
		}

		connectPass := password

		if *nodeXray.Protocol == "vless" || *nodeXray.Protocol == "vmess" || *nodeXray.Protocol == "trojan" {
			if *nodeXray.Protocol == "vless" || *nodeXray.Protocol == "vmess" {
				connectPass = util.GenerateUUID(password)
			}
			headBuilder.WriteString(fmt.Sprintf("%s://%s@%s:%d?type=%s&security=%s", *nodeXray.Protocol,
				url.PathEscape(connectPass), *node.Domain, *node.Port,
				streamSettings.Network, streamSettings.Security))
			if *nodeXray.Protocol == "vmess" {
				headBuilder.WriteString("&alterId=0")
				if settings.Encryption == "none" {
					headBuilder.WriteString("&encryption=none")
				}
			}

			if *nodeXray.Protocol == "vless" {
				headBuilder.WriteString(fmt.Sprintf("&flow=%s", *nodeXray.XrayFlow))
			}
			if (*nodeXray.Protocol == "vless" || *nodeXray.Protocol == "vmess") && uintValue(nodeXray.XudpEnable) == 1 {
				headBuilder.WriteString("&packetEncoding=xudp")
			}

			if streamSettings.Security == "tls" {
				headBuilder.WriteString(fmt.Sprintf("&sni=%s", streamSettings.TlsSettings.ServerName))
				headBuilder.WriteString(fmt.Sprintf("&fp=%s", streamSettings.TlsSettings.Fingerprint))
				if len(streamSettings.TlsSettings.Alpn) > 0 {
					alpns := strings.Replace(strings.Trim(fmt.Sprint(streamSettings.TlsSettings.Alpn), "[]"), " ", ",", -1)
					headBuilder.WriteString(fmt.Sprintf("&alpn=%s", url.PathEscape(alpns)))
				}
			} else if streamSettings.Security == "reality" {
				headBuilder.WriteString(fmt.Sprintf("&pbk=%s", *nodeXray.RealityPbk))
				headBuilder.WriteString(fmt.Sprintf("&fp=%s", streamSettings.RealitySettings.Fingerprint))
				if streamSettings.RealitySettings.SpiderX != "" {
					headBuilder.WriteString(fmt.Sprintf("&spx=%s", url.PathEscape(streamSettings.RealitySettings.SpiderX)))
				}
				shortIds := streamSettings.RealitySettings.ShortIds
				if len(shortIds) != 0 {
					headBuilder.WriteString(fmt.Sprintf("&sid=%s", shortIds[0]))
				}
				serverNames := streamSettings.RealitySettings.ServerNames
				if len(serverNames) != 0 {
					headBuilder.WriteString(fmt.Sprintf("&sni=%s", serverNames[0]))
				}
			}

			if streamSettings.Network == "ws" {
				if streamSettings.WsSettings.Path != "" {
					headBuilder.WriteString(fmt.Sprintf("&path=%s", streamSettings.WsSettings.Path))
				}
				if streamSettings.WsSettings.Headers.Host != "" {
					headBuilder.WriteString(fmt.Sprintf("&host=%s", streamSettings.WsSettings.Headers.Host))
				}
			}
		} else if *nodeXray.Protocol == "shadowsocks" {
			headBuilder.WriteString(fmt.Sprintf("ss://%s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s@%s:%d", *nodeXray.XraySSMethod,
				connectPass, *node.Domain, *node.Port)))))
		} else if *nodeXray.Protocol == "socks" {
			settings := bo.Settings{}
			if nodeXray.Settings != nil && *nodeXray.Settings != "" {
				if err := json.Unmarshal([]byte(*nodeXray.Settings), &settings); err != nil {
					return "", 0, errors.New(constant.NodeURLError)
				}
			}
			headBuilder.WriteString(fmt.Sprintf("socks://%s", base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s:%s@%s:%d", settings.Accounts[0].User,
				settings.Accounts[0].Pass, *node.Domain, *node.Port)))))
		}
	} else if *nodeType.Id == constant.TrojanGo {
		nodeTrojanGo, err := dao.SelectNodeTrojanGoById(node.NodeSubId)
		if err != nil {
			return "", 0, errors.New(constant.NodeURLError)
		}
		headBuilder.WriteString(fmt.Sprintf("trojan-go://%s@%s:%d?", url.PathEscape(password),
			*node.Domain, *node.Port))
		var sni string
		if nodeTrojanGo.Sni != nil && *nodeTrojanGo.Sni != "" {
			sni = *nodeTrojanGo.Sni
		} else {
			sni = *node.Domain
		}
		headBuilder.WriteString(fmt.Sprintf("sni=%s", url.PathEscape(sni)))
		if nodeTrojanGo.WebsocketEnable != nil && *nodeTrojanGo.WebsocketEnable != 0 &&
			nodeTrojanGo.WebsocketPath != nil && *nodeTrojanGo.WebsocketPath != "" {
			headBuilder.WriteString(fmt.Sprintf("&type=%s", url.PathEscape("ws")))
			headBuilder.WriteString(fmt.Sprintf("&path=%s",
				url.PathEscape(fmt.Sprintf("%s", *nodeTrojanGo.WebsocketPath))))
			if nodeTrojanGo.WebsocketHost != nil && *nodeTrojanGo.WebsocketHost != "" {
				headBuilder.WriteString(fmt.Sprintf("&host=%s",
					url.PathEscape(fmt.Sprintf("%s", *nodeTrojanGo.WebsocketHost))))
			}
			if nodeTrojanGo.SsEnable != nil && *nodeTrojanGo.SsEnable != 0 {
				headBuilder.WriteString(fmt.Sprintf("&encryption=%s", url.PathEscape(
					fmt.Sprintf("ss;%s:%s", *nodeTrojanGo.SsMethod, *nodeTrojanGo.SsPassword))))
			}
		}
	} else if *nodeType.Id == constant.Hysteria {
		nodeHysteria, err := dao.SelectNodeHysteriaById(node.NodeSubId)
		if err != nil {
			return "", 0, errors.New(constant.NodeURLError)
		}
		headBuilder.WriteString(fmt.Sprintf("hysteria://%s:%d?protocol=%s&auth=%s&upmbps=%d&downmbps=%d",
			*node.Domain,
			*node.Port,
			*nodeHysteria.Protocol,
			password,
			*nodeHysteria.UpMbps,
			*nodeHysteria.DownMbps))
		if nodeHysteria.Obfs != nil && *nodeHysteria.Obfs != "" {
			headBuilder.WriteString(fmt.Sprintf("&obfs=xplus&obfsParam=%s", *nodeHysteria.Obfs))
		}
		if nodeHysteria.ServerName != nil && *nodeHysteria.ServerName != "" {
			headBuilder.WriteString(fmt.Sprintf("&peer=%s", *nodeHysteria.ServerName))
		}
		if nodeHysteria.Insecure != nil {
			headBuilder.WriteString(fmt.Sprintf("&insecure=%d", *nodeHysteria.Insecure))
		}
		if nodeHysteria.FastOpen != nil {
			headBuilder.WriteString(fmt.Sprintf("&fastopen=%d", *nodeHysteria.FastOpen))
		}
	} else if *nodeType.Id == constant.Hysteria2 {
		nodeHysteria2, err := dao.SelectNodeHysteria2ById(node.NodeSubId)
		if err != nil {
			return "", 0, errors.New(constant.NodeURLError)
		}
		if client == clientcompat.V2RaySubscriptionV2RayN {
			hopInterval := ""
			if nodeHysteria2.HopInterval != nil && *nodeHysteria2.HopInterval > 0 {
				hopInterval = fmt.Sprintf("%d", *nodeHysteria2.HopInterval)
			}
			nodeURL, err := clientcompat.V2RayNHysteria2URI(clientcompat.V2RayNHysteria2Config{
				Remarks:        *node.Name,
				Address:        *node.Domain,
				Port:           *node.Port,
				Password:       password,
				SNI:            stringValue(nodeHysteria2.ServerName),
				AllowInsecure:  uintValue(nodeHysteria2.Insecure) == 1,
				SalamanderPass: stringValue(nodeHysteria2.ObfsPassword),
				UpMbps:         *nodeHysteria2.UpMbps,
				DownMbps:       *nodeHysteria2.DownMbps,
				Ports:          normalizedHysteria2PortHopping(nodeHysteria2.PortHopping),
				HopInterval:    hopInterval,
			})
			if err != nil {
				return "", 0, errors.New(constant.NodeURLError)
			}
			return nodeURL, *nodeType.Id, nil
		}
		server := fmt.Sprintf("%s:%d", *node.Domain, *node.Port)
		portHopping := normalizedHysteria2PortHopping(nodeHysteria2.PortHopping)
		if client == clientcompat.V2RaySubscriptionStandard {
			hopInterval := ""
			if nodeHysteria2.HopInterval != nil && *nodeHysteria2.HopInterval > 0 {
				hopInterval = fmt.Sprintf("%d", *nodeHysteria2.HopInterval)
			}
			return clientcompat.V2RayNGHysteria2URI(clientcompat.V2RayNHysteria2Config{
				Remarks:        *node.Name,
				Address:        *node.Domain,
				Port:           *node.Port,
				Password:       password,
				SNI:            stringValue(nodeHysteria2.ServerName),
				AllowInsecure:  uintValue(nodeHysteria2.Insecure) == 1,
				SalamanderPass: stringValue(nodeHysteria2.ObfsPassword),
				UpMbps:         *nodeHysteria2.UpMbps,
				DownMbps:       *nodeHysteria2.DownMbps,
				Ports:          portHopping,
				HopInterval:    hopInterval,
			}), *nodeType.Id, nil
		}
		if portHopping != "" {
			server = fmt.Sprintf("%s:%s", *node.Domain, portHopping)
		}
		headBuilder.WriteString(fmt.Sprintf("hysteria2://%s@%s?insecure=%d&upmbps=%d&downmbps=%d",
			password,
			server,
			*nodeHysteria2.Insecure,
			*nodeHysteria2.UpMbps,
			*nodeHysteria2.DownMbps))
		if nodeHysteria2.ObfsPassword != nil && *nodeHysteria2.ObfsPassword != "" {
			headBuilder.WriteString(fmt.Sprintf("&obfs=salamander&obfs-password=%s", *nodeHysteria2.ObfsPassword))
		}
		if nodeHysteria2.ServerName != nil && *nodeHysteria2.ServerName != "" {
			headBuilder.WriteString(fmt.Sprintf("&sni=%s", *nodeHysteria2.ServerName))
		}
	} else if *nodeType.Id == constant.NaiveProxy {
		headBuilder.WriteString(fmt.Sprintf("naive+https://%s:%s@%s:%d", *username, password, *node.Domain, *node.Port))
	}

	if node.Name != nil && *node.Name != "" {
		headBuilder.WriteString(fmt.Sprintf("#%s", url.PathEscape(*node.Name)))
	}
	return headBuilder.String(), *nodeType.Id, nil
}

func CountNode() (int, error) {
	return dao.CountNode()
}

func nodeUintValue(value *uint, fallback uint) uint {
	if value == nil {
		return fallback
	}
	return *value
}

func isRetiredNodeType(nodeType uint) bool {
	return nodeType == constant.TrojanGo || nodeType == constant.Hysteria
}

func GrpcAddNode(token string, ip string, grpcPort uint, nodeAddDto *core.NodeAddDto, transport ...core.NodeTransport) {
	_ = core.AddNode(token, ip, grpcPort, nodeAddDto, transport...)
}

func GrpcRemoveNode(token string, ip string, grpcPort uint, port uint, nodeTypeId uint, transport ...core.NodeTransport) {
	_ = core.RemoveNode(token, ip, grpcPort, &core.NodeRemoveDto{
		NodeTypeId: uint64(nodeTypeId),
		Port:       uint64(port),
	}, transport...)
}

func NodeDefault() (vo.NodeDefaultVo, error) {
	var nodeDefaultVo vo.NodeDefaultVo
	publicKey, privateKey, err := util.ExecuteX25519()
	if err != nil {
		return nodeDefaultVo, errors.New(constant.SysError)
	}
	nodeDefaultVo.PublicKey = publicKey
	nodeDefaultVo.PrivateKey = privateKey
	nodeDefaultVo.ShortId = util.GenerateShortId()
	nodeDefaultVo.SpiderX = fmt.Sprintf("/%s", util.RandString(8))
	return nodeDefaultVo, nil
}

package app

import (
	"errors"
	"github.com/sirupsen/logrus"
	"regexp"
	"sync"
	"trojan-panel-core/app/hysteria2"
	"trojan-panel-core/app/naiveproxy"
	"trojan-panel-core/app/xray"
	"trojan-panel-core/core/process"
	"trojan-panel-core/dao"
	"trojan-panel-core/dao/redis"
	"trojan-panel-core/model"
	"trojan-panel-core/model/bo"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/model/dto"
	"trojan-panel-core/model/vo"
	"trojan-panel-core/service"
	"trojan-panel-core/util"
)

var userLinkRegex = regexp.MustCompile("user>>>([^>]+)>>>traffic>>>(downlink|uplink)")

func InitApp() {
	InitBinFile()
	if initializeTrafficQuota() {
		return
	}
	if err := xray.InitXrayApp(); err != nil {
		logrus.Errorf("xray app init err: %s", err.Error())
	}
	if err := naiveproxy.InitNaiveProxyApp(); err != nil {
		logrus.Errorf("naiverpoxy app init err: %s", err.Error())
	}
	if err := hysteria2.InitHysteria2App(); err != nil {
		logrus.Errorf("hysteria2 app init err: %s", err.Error())
	}
}

func InitBinFile() {
	if err := xray.InitXrayBinFile(); err != nil {
		logrus.Errorf("download xray file err: %v", err)
		panic(err)
	}
	if err := naiveproxy.InitNaiveProxyBinFile(); err != nil {
		logrus.Errorf("download naivepxoy file err: %v", err)
		panic(err)
	}
	if err := hysteria2.InitHysteria2BinFile(); err != nil {
		logrus.Errorf("download hysteria2 file err: %v", err)
		panic(err)
	}
}

func StartApp(nodeAddDto dto.NodeAddDto) error {
	if err := CanStartProxy(); err != nil {
		return err
	}
	var mutex sync.Mutex
	defer mutex.Unlock()
	if mutex.TryLock() {
		var protocol string
		var xrayFlow string
		var xraySSMethod string
		switch nodeAddDto.NodeTypeId {
		case constant.Xray:
			if err := xray.StartXray(dto.XrayConfigDto{
				ApiPort:        nodeAddDto.Port + 30000,
				Port:           nodeAddDto.Port,
				Protocol:       nodeAddDto.XrayProtocol,
				Settings:       nodeAddDto.XraySettings,
				StreamSettings: nodeAddDto.XrayStreamSettings,
				Tag:            nodeAddDto.XrayTag,
				Sniffing:       nodeAddDto.XraySniffing,
				Allocate:       nodeAddDto.XrayAllocate,
				Template:       nodeAddDto.XrayTemplate,
			}); err != nil {
				return err
			}
			protocol = nodeAddDto.XrayProtocol
			xrayFlow = nodeAddDto.XrayFlow
			xraySSMethod = nodeAddDto.XraySSMethod
		case constant.NaiveProxy:
			if err := naiveproxy.StartNaiveProxy(dto.NaiveProxyConfigDto{
				ApiPort: nodeAddDto.Port + 30000,
				Port:    nodeAddDto.Port,
				Domain:  nodeAddDto.Domain,
			}); err != nil {
				return err
			}
		case constant.Hysteria2:
			if err := hysteria2.StartHysteria2(dto.Hysteria2ConfigDto{
				ApiPort:      nodeAddDto.Port + 30000,
				Port:         nodeAddDto.Port,
				Domain:       nodeAddDto.Domain,
				ObfsPassword: nodeAddDto.Hysteria2ObfsPassword,
				UpMbps:       nodeAddDto.Hysteria2UpMbps,
				DownMbps:     nodeAddDto.Hysteria2DownMbps,
			}); err != nil {
				return err
			}
		default:
			return errors.New(constant.NodeTypeNotExist)
		}

		nodeConfig := model.NodeConfig{
			ApiPort:      nodeAddDto.Port + 30000,
			NodeTypeId:   nodeAddDto.NodeTypeId,
			Protocol:     protocol,
			XrayFlow:     xrayFlow,
			XraySSMethod: xraySSMethod,
		}
		if err := service.InsertNodeConfig(nodeConfig); err != nil {
			return err
		}
	}
	return nil
}

func StopApp(apiPort uint, nodeTypeId uint) error {
	var mutex sync.Mutex
	defer mutex.Unlock()
	if mutex.TryLock() {
		switch nodeTypeId {
		case constant.Xray:
			if err := xray.StopXray(apiPort, true); err != nil {
				return err
			}
		case constant.NaiveProxy:
			if err := naiveproxy.StopNaiveProxy(apiPort, true); err != nil {
				return err
			}
		case constant.Hysteria2:
			if err := hysteria2.StopHysteria2(apiPort, true); err != nil {
				return err
			}
		default:
			return errors.New(constant.NodeTypeNotExist)
		}

		if err := service.DeleteNodeConfigByNodeTypeIdAndApiPort(apiPort, nodeTypeId); err != nil {
			return err
		}
	}
	return nil
}

func RestartApp(apiPort uint, nodeTypeId uint) error {
	switch nodeTypeId {
	case constant.Xray:
		if err := xray.RestartXray(apiPort); err != nil {
			return err
		}
	case constant.NaiveProxy:
		if err := naiveproxy.RestartNaiveProxy(apiPort); err != nil {
			return err
		}
	case constant.Hysteria2:
		if err := hysteria2.RestartHysteria2(apiPort); err != nil {
			return err
		}
	default:
		return errors.New(constant.NodeTypeNotExist)
	}
	return nil
}

// CronHandlerUser timing task processing user
func CronHandlerUser() {
	// valid account in the database
	accountBos, err := dao.SelectAccounts()
	if err != nil {
		logrus.Errorf("query all valid accounts error err: %v", err)
		return
	}

	accountBoLists := util.SplitArr(accountBos, 50)

	for _, accountBoList := range accountBoLists {
		// xray
		go func(accountBos []bo.AccountBo) {
			xrayInstance := process.NewXrayProcess()
			xrayCmdMap := xrayInstance.GetCmdMap()
			xrayCmdMap.Range(func(apiPort, cmd any) bool {
				go func() {
					xrayApi := xray.NewXrayApi(apiPort.(uint))
					// account in memory
					stats, err := xrayApi.QueryStats("", false)
					if err != nil {
						return
					}

					// deleted account
					var banAccountBos []bo.AccountBo
					for _, stat := range stats {
						submatch := userLinkRegex.FindStringSubmatch(stat.Name)
						if len(submatch) == 3 {
							pass := submatch[1]
							var banFlag = true
							for _, account := range accountBos {
								if account.Pass == pass {
									banFlag = false
									break
								}
							}
							if banFlag {
								banAccountBos = append(banAccountBos, bo.AccountBo{
									Pass: pass,
								})
							}
						}
					}
					for _, item := range banAccountBos {
						if err = xrayApi.DeleteUser(item.Pass); err != nil {
							logrus.Errorf("xray DeleteUser err: %v", err)
							continue
						}
					}

					// account added
					var addAccountBos []bo.AccountBo
					for _, account := range accountBos {
						var addFlag = true
						for _, stat := range stats {
							submatch := userLinkRegex.FindStringSubmatch(stat.Name)
							if len(submatch) == 3 {
								pass := submatch[1]
								if account.Pass == pass {
									addFlag = false
									break
								}
							}
						}
						if addFlag {
							addAccountBos = append(addAccountBos, bo.AccountBo{
								Pass: account.Pass,
							})
						}
					}
					protocol, err := util.GetXrayProtocolByApiPort(apiPort.(uint))
					if err != nil {
						logrus.Errorf("xray get protocol err apiPort: %s err: %v", apiPort, err)
					} else {
						for _, item := range addAccountBos {
							if err = xrayApi.AddUser(dto.XrayAddUserDto{
								Protocol: protocol,
								Password: item.Pass,
							}); err != nil {
								logrus.Errorf("xray AddUser err: %v", err)
								continue
							}
						}
					}
				}()
				return true
			})
		}(accountBoList)

		// naiveproxy
		go func(accountBos []bo.AccountBo) {
			naiveProxyInstance := process.NewNaiveProxyInstance()
			naiveProxyCmdMap := naiveProxyInstance.GetCmdMap()
			naiveProxyCmdMap.Range(func(apiPort, cmd any) bool {
				go func() {
					naiveProxyApi := naiveproxy.NewNaiveProxyApi(apiPort.(uint))
					users, err := naiveProxyApi.ListUsers()
					if err != nil {
						return
					}

					// deleted account
					var banAccountBos []bo.AccountBo
					for _, user := range *users {
						pass := user.Pass()
						var banFlag = true
						for _, account := range accountBos {
							if account.Pass == pass {
								banFlag = false
								break
							}
						}
						if banFlag {
							banAccountBos = append(banAccountBos, bo.AccountBo{
								Pass: pass,
							})
						}
					}
					for _, item := range banAccountBos {
						// call api to delete user
						if err = naiveProxyApi.DeleteUser(item.Pass); err != nil {
							logrus.Errorf("naiveproxy DeleteUser err: %v", err)
							continue
						}
					}

					// account added
					var addAccountBos []bo.AccountBo
					for _, account := range accountBos {
						var addFlag = true
						for _, user := range *users {
							pass := user.Pass()
							if account.Pass == pass {
								addFlag = false
								break
							}
						}
						if addFlag {
							addAccountBos = append(addAccountBos, bo.AccountBo{
								Username: account.Username,
								Pass:     account.Pass,
							})
						}
					}
					for _, item := range addAccountBos {
						// call api to add user
						if err = naiveProxyApi.AddUser(dto.NaiveProxyAddUserDto{
							Username: item.Username,
							Pass:     item.Pass,
						}); err != nil {
							logrus.Errorf("naiveproxy AddUser err: %v", err)
							continue
						}
					}
				}()
				return true
			})
		}(accountBoList)
	}
}

// CronHandlerDownloadAndUpload scheduled tasks: update the account's download and upload traffic in the database. Hysteria2 does not currently support traffic statistics
func CronHandlerDownloadAndUpload() {
	// xray
	go func() {
		xrayInstance := process.NewXrayProcess()
		xrayCmdMap := xrayInstance.GetCmdMap()
		xrayCmdMap.Range(func(apiPort, cmd any) bool {
			xrayApi := xray.NewXrayApi(apiPort.(uint))
			stats, err := xrayApi.QueryStats("", true)
			if err == nil {
				statLists := util.SplitArr(stats, 50)
				for _, statList := range statLists {
					go func(stats []vo.XrayStatsVo) {
						accountUpdateBos := make([]bo.AccountUpdateBo, 0)
						for _, stat := range stats {
							submatch := userLinkRegex.FindStringSubmatch(stat.Name)
							if len(submatch) == 3 {
								isDown := submatch[2] == "downlink"
								var setFlag = false
								if isDown {
									for index := range accountUpdateBos {
										if accountUpdateBos[index].Pass == submatch[1] {
											accountUpdateBos[index].Download = stat.Value
											setFlag = true
											break
										}
									}
									if !setFlag {
										accountUpdateBo := bo.AccountUpdateBo{}
										accountUpdateBo.Pass = submatch[1]
										accountUpdateBo.Download = stat.Value
										accountUpdateBos = append(accountUpdateBos, accountUpdateBo)
									}
								} else {
									for index := range accountUpdateBos {
										if accountUpdateBos[index].Pass == submatch[1] {
											accountUpdateBos[index].Upload = stat.Value
											setFlag = true
											break
										}
									}
									if !setFlag {
										accountUpdateBo := bo.AccountUpdateBo{}
										accountUpdateBo.Pass = submatch[1]
										accountUpdateBo.Upload = stat.Value
										accountUpdateBos = append(accountUpdateBos, accountUpdateBo)
									}
								}
							}
						}
						mutex, err := redis.RsLock(constant.LockXrayUpdate)
						if err != nil {
							return
						}
						for _, account := range accountUpdateBos {
							if err = dao.UpdateAccountFlowByPassOrHash(&account.Pass, nil, account.Download, account.Upload); err != nil {
								logrus.Errorf("xray UpdateAccountFlow err apiPort: %d err: %v", apiPort, err)
								continue
							}
						}
						redis.RsUnLock(mutex)
					}(statList)
				}
			}
			return true
		})
	}()

	// naiveproxy
	go func() {
		naiveProxyInstance := process.NewNaiveProxyInstance()
		naiveProxyCmdMap := naiveProxyInstance.GetCmdMap()
		naiveProxyCmdMap.Range(func(apiPort, cmd any) bool {
			naiveProxyApi := naiveproxy.NewNaiveProxyApi(apiPort.(uint))
			users, err := naiveProxyApi.ListUserTraffic(true)
			if err == nil {
				userLists := util.SplitMap(users, 50)
				for _, userList := range userLists {
					go func(users map[string]bo.NaiveProxyUserTraffic) {
						accountUpdateBos := make([]bo.AccountUpdateBo, 0)
						for pass, traffic := range users {
							accountUpdateBo := bo.AccountUpdateBo{
								Pass:     pass,
								Download: uint64ToInt(traffic.Rx),
								Upload:   uint64ToInt(traffic.Tx),
							}
							accountUpdateBos = append(accountUpdateBos, accountUpdateBo)
						}
						mutex, err := redis.RsLock(constant.LockNaiveProxyUpdate)
						if err != nil {
							return
						}
						for _, account := range accountUpdateBos {
							if err = dao.UpdateAccountFlowByPassOrHash(&account.Pass, nil, account.Download,
								account.Upload); err != nil {
								logrus.Errorf("naiveproxy UpdateAccountFlow err apiPort: %d err: %v", apiPort, err)
								continue
							}
						}
						redis.RsUnLock(mutex)
					}(userList)
				}
			}
			return true
		})
	}()

	// hysteria2
	go func() {
		hysteria2Instance := process.NewHysteria2Instance()
		hysteria2CmdMap := hysteria2Instance.GetCmdMap()
		hysteria2CmdMap.Range(func(apiPort, cmd any) bool {
			hysteria2Api := hysteria2.NewHysteria2Api(apiPort.(uint))
			users, err := hysteria2Api.ListUsers(true)
			if err == nil {
				userLists := util.SplitMap(users, 50)
				for _, userList := range userLists {
					go func(users map[string]bo.Hysteria2UserTraffic) {
						accountUpdateBos := make([]bo.AccountUpdateBo, 0)
						for pass, traffic := range users {
							accountUpdateBo := bo.AccountUpdateBo{
								Pass:     pass,
								Upload:   int(traffic.Tx),
								Download: int(traffic.Rx),
							}
							accountUpdateBos = append(accountUpdateBos, accountUpdateBo)
						}
						mutex, err := redis.RsLock(constant.LockHysteria2Update)
						if err != nil {
							return
						}
						for _, account := range accountUpdateBos {
							if err = dao.UpdateAccountFlowByPassOrHash(&account.Pass, nil, account.Download,
								account.Upload); err != nil {
								logrus.Errorf("hysteria2 UpdateAccountFlow err apiPort: %d err: %v", apiPort, err)
								continue
							}
						}
						redis.RsUnLock(mutex)
					}(userList)
				}
			}
			return true
		})
	}()
}

func uint64ToInt(value uint64) int {
	maxInt := int(^uint(0) >> 1)
	if value > uint64(maxInt) {
		return maxInt
	}
	return int(value)
}

func RemoveAccount(password string) error {
	// xray
	go func() {
		xrayInstance := process.NewXrayProcess()
		xrayCmdMap := xrayInstance.GetCmdMap()
		xrayCmdMap.Range(func(apiPort, cmd any) bool {
			go func(password string) {
				xrayApi := xray.NewXrayApi(apiPort.(uint))
				if err := xrayApi.DeleteUser(password); err != nil {
					logrus.Errorf("xray DeleteUser err: %v", err)
				}
			}(password)
			return true
		})
	}()

	// naiveproxy
	go func() {
		naiveProxyInstance := process.NewNaiveProxyInstance()
		naiveProxyCmdMap := naiveProxyInstance.GetCmdMap()
		naiveProxyCmdMap.Range(func(apiPort, cmd any) bool {
			go func(password string) {
				naiveProxyApi := naiveproxy.NewNaiveProxyApi(apiPort.(uint))
				// call api to delete user
				if err := naiveProxyApi.DeleteUser(password); err != nil {
					logrus.Errorf("naiveproxy DeleteUser err: %v", err)
				}
			}(password)
			return true
		})
	}()
	return nil
}

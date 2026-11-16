package app

import (
	"errors"
	"sync"
	"trojan-panel-core/app/naiveproxy"
	"trojan-panel-core/app/xray"
	"trojan-panel-core/core/process"
	"trojan-panel-core/dao"
	"trojan-panel-core/model"
	"trojan-panel-core/model/bo"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/model/dto"
	"trojan-panel-core/service"

	"github.com/sirupsen/logrus"
)

var accountSync sync.Mutex

type xrayListenerAccounts struct {
	process any
	users   map[string]bool
}

var xrayAccounts = make(map[uint]xrayListenerAccounts)

func CronHandlerUser() {
	if !accountSync.TryLock() {
		return
	}
	defer accountSync.Unlock()

	synced := true
	for _, listener := range []struct {
		kind      uint
		processes *sync.Map
		sync      func(uint, any, map[string]bo.AccountBo) error
	}{
		{constant.Xray, process.NewXrayProcess().GetCmdMap(), reconcileXrayAccounts},
		{constant.NaiveProxy, process.NewNaiveProxyInstance().GetCmdMap(), reconcileNaiveAccounts},
	} {
		listener.processes.Range(func(key, command any) bool {
			port := key.(uint)
			accounts, err := dao.SelectListenerAccounts(listener.kind, port-30000)
			if err == nil {
				allowed := make(map[string]bo.AccountBo, len(accounts))
				for _, account := range accounts {
					allowed[account.Pass] = account
				}
				err = listener.sync(port, command, allowed)
			}
			if err != nil {
				synced = false
				logrus.WithError(err).Errorf("sync account access on port %d", port)
			}
			return true
		})
	}
	for port, state := range xrayAccounts {
		command, _ := process.NewXrayProcess().GetCmdMap().Load(port)
		if command != state.process {
			delete(xrayAccounts, port)
		}
	}
	if synced {
		if err := dao.MarkNodeAccessSynced(); err != nil {
			logrus.WithError(err).Error("record account access sync")
		}
	}
}

func reconcileXrayAccounts(port uint, command any, allowed map[string]bo.AccountBo) error {
	state := xrayAccounts[port]
	if state.process != command {
		state = xrayListenerAccounts{process: command, users: make(map[string]bool)}
		xrayAccounts[port] = state
	}
	api := xray.NewXrayApi(port)
	var nodeConfig *model.NodeConfig
	add := func(account bo.AccountBo) error {
		if nodeConfig == nil {
			config, err := service.SelectNodeConfigByNodeTypeIdAndApiPort(port, constant.Xray)
			if err != nil {
				return err
			}
			nodeConfig = config
		}
		return api.AddUser(dto.XrayAddUserDto{
			Protocol: nodeConfig.Protocol,
			Password: account.Pass,
			Flow:     nodeConfig.XrayFlow,
			SSMethod: nodeConfig.XraySSMethod,
		})
	}
	return reconcileListenerAccounts(state.users, allowed, add, api.DeleteUser)
}

func reconcileNaiveAccounts(port uint, _ any, allowed map[string]bo.AccountBo) error {
	api := naiveproxy.NewNaiveProxyApi(port)
	loaded, err := api.ListUsers()
	if err != nil {
		return err
	}
	users := make(map[string]bool, len(*loaded))
	for _, user := range *loaded {
		if pass := user.Pass(); pass != "" {
			users[pass] = true
		}
	}
	add := func(account bo.AccountBo) error {
		return api.AddUser(dto.NaiveProxyAddUserDto{Username: account.Username, Pass: account.Pass})
	}
	return reconcileListenerAccounts(users, allowed, add, api.DeleteUser)
}

func reconcileListenerAccounts(users map[string]bool, allowed map[string]bo.AccountBo, add func(bo.AccountBo) error, remove func(string) error) error {
	var failures []error
	for pass := range users {
		if _, ok := allowed[pass]; ok {
			continue
		}
		if err := remove(pass); err != nil {
			failures = append(failures, err)
		} else {
			delete(users, pass)
		}
	}
	for pass, account := range allowed {
		if users[pass] {
			continue
		}
		err := add(account)
		// A timed-out add may have reached the kernel. Keep it eligible for removal.
		users[pass] = err == nil
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

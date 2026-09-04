package app

import (
	"errors"
	"sync"

	"github.com/sirupsen/logrus"
	"trojan-panel-core/app/hysteria2"
	"trojan-panel-core/app/naiveproxy"
	"trojan-panel-core/app/xray"
	"trojan-panel-core/core/process"
	"trojan-panel-core/dao"
)

var quotaState struct {
	sync.RWMutex
	blocked bool
}
var quotaReconcileMu sync.Mutex

func initializeTrafficQuota() bool {
	status, err := dao.CurrentServerQuota()
	if err != nil {
		logrus.Errorf("traffic quota initial check failed: %v", err)
		return false
	}
	quotaState.Lock()
	quotaState.blocked = status.Reached
	quotaState.Unlock()
	if status.Reached {
		logrus.Warnf("node server %d traffic quota reached; proxy startup suppressed", status.ServerID)
	}
	return status.Reached
}

func CanStartProxy() error {
	status, err := dao.CurrentServerQuota()
	if err != nil {
		return err
	}
	if status.Reached {
		return errors.New("node server traffic quota reached")
	}
	return nil
}

func ReconcileTrafficQuota() {
	if !quotaReconcileMu.TryLock() {
		return
	}
	defer quotaReconcileMu.Unlock()
	status, err := dao.CurrentServerQuota()
	if err != nil {
		logrus.Errorf("traffic quota reconcile failed: %v", err)
		return
	}
	quotaState.RLock()
	wasBlocked := quotaState.blocked
	quotaState.RUnlock()
	if status.Reached {
		stopAllProxyProcesses()
		quotaState.Lock()
		quotaState.blocked = true
		quotaState.Unlock()
		if !wasBlocked {
			logrus.Warnf("node server %d traffic quota reached; all proxy processes stopped", status.ServerID)
		}
	} else if !status.Reached && wasBlocked {
		if err := startConfiguredProxyProcesses(); err != nil {
			logrus.Errorf("restore proxy processes after quota reset failed: %v", err)
			return
		}
		quotaState.Lock()
		quotaState.blocked = false
		quotaState.Unlock()
		logrus.Infof("node server %d traffic quota reset; proxy processes restored", status.ServerID)
	}
}

func stopAllProxyProcesses() {
	stopMap := func(cmdMap *sync.Map, stop func(uint) error) {
		ports := make([]uint, 0)
		cmdMap.Range(func(key, _ any) bool { ports = append(ports, key.(uint)); return true })
		for _, port := range ports {
			if err := stop(port); err != nil {
				logrus.Errorf("stop quota-blocked proxy %d failed: %v", port, err)
			}
		}
	}
	stopMap(process.NewXrayProcess().GetCmdMap(), func(port uint) error { return xray.StopXray(port, false) })
	stopMap(process.NewNaiveProxyInstance().GetCmdMap(), func(port uint) error { return naiveproxy.StopNaiveProxy(port, false) })
	stopMap(process.NewHysteria2Instance().GetCmdMap(), func(port uint) error { return hysteria2.StopHysteria2(port, false) })
}

func startConfiguredProxyProcesses() error {
	if err := xray.InitXrayApp(); err != nil {
		return err
	}
	if err := naiveproxy.InitNaiveProxyApp(); err != nil {
		return err
	}
	return hysteria2.InitHysteria2App()
}

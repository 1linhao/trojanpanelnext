package process

import (
	"errors"
	"github.com/sirupsen/logrus"
	"os/exec"
	"sync"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/util"
)

var mutexHysteria2 sync.Mutex
var cmdMapHysteria2 sync.Map

type Hysteria2Process struct {
	process
}

func NewHysteria2Instance() *Hysteria2Process {
	return &Hysteria2Process{process{mutex: &mutexHysteria2, binaryType: constant.Hysteria2, cmdMap: &cmdMapHysteria2}}
}

func (h *Hysteria2Process) StopHysteria2Instance() error {
	apiPorts, err := util.GetConfigApiPorts(constant.Hysteria2Path)
	if err != nil {
		return err
	}
	for _, apiPort := range apiPorts {
		if err = h.Stop(apiPort, true); err != nil {
			return err
		}
	}
	return nil
}

func (h *Hysteria2Process) StartHysteria2(apiPort uint) error {
	defer h.mutex.Unlock()
	if h.mutex.TryLock() {
		if h.IsRunning(apiPort) {
			return nil
		}
		binaryFilePath, err := util.GetBinaryFile(constant.Hysteria2)
		if err != nil {
			return err
		}
		configFilePath, err := util.GetConfigFile(constant.Hysteria2, apiPort)
		if err != nil {
			return err
		}
		cmd := exec.Command(binaryFilePath, "-c", configFilePath, "server")
		if cmd.Err != nil {
			logrus.Errorf("hysteria2 command error err: %v", err)
			return errors.New(constant.Hysteria2StartError)
		}
		if err := cmd.Start(); err != nil {
			logrus.Errorf("start hysteria2 error err: %v", err)
			return errors.New(constant.Hysteria2StartError)
		}
		h.cmdMap.Store(apiPort, cmd)

		go func() {
			if waitErr := cmd.Wait(); waitErr != nil {
				logrus.Errorf("hysteria2 process wait error err: %v", waitErr)
				h.releaseProcess(apiPort, cmd)
			}
		}()
		return nil
	}
	logrus.Errorf("start hysteria2 error err: lock not acquired")
	return errors.New(constant.Hysteria2StartError)
}

func (h *Hysteria2Process) releaseProcess(apiPort uint, expected *exec.Cmd) {
	load, ok := h.GetCmdMap().Load(apiPort)
	if !ok || load != expected {
		return
	}
	h.cmdMap.CompareAndDelete(apiPort, expected)
}

func GetHysteria2State(apiPort uint) bool {
	_, ok := NewHysteria2Instance().GetCmdMap().Load(apiPort)
	return ok
}

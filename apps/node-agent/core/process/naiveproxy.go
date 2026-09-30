package process

import (
	"errors"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"sync"
	"time"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/util"
)

var mutexNaiveProxy sync.Mutex
var cmdMapNaiveProxy sync.Map
var doneNaiveProxy sync.Map

type NaiveProxyProcess struct {
	process
}

func NewNaiveProxyInstance() *NaiveProxyProcess {
	return &NaiveProxyProcess{process{mutex: &mutexNaiveProxy, binaryType: constant.NaiveProxy, cmdMap: &cmdMapNaiveProxy}}
}

func (n *NaiveProxyProcess) StopNaiveProxyInstance() error {
	apiPorts, err := util.GetConfigApiPorts(constant.NaiveProxyPath)
	if err != nil {
		return err
	}
	for _, apiPort := range apiPorts {
		if err = n.Stop(apiPort, true); err != nil {
			return err
		}
	}
	return nil
}

func (n *NaiveProxyProcess) StartNaiveProxy(apiPort uint) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if n.isRunningLocked(apiPort) {
		return nil
	}
	binaryFilePath, err := util.GetBinaryFile(constant.NaiveProxy)
	if err != nil {
		return err
	}
	configFilePath, err := util.GetConfigFile(constant.NaiveProxy, apiPort)
	if err != nil {
		return err
	}
	cmd := exec.Command(binaryFilePath, "run", "--config", configFilePath)
	if cmd.Err != nil {
		logrus.Errorf("naiveproxy command error err: %v", err)
		return errors.New(constant.NaiveProxyStartError)
	}
	if err := cmd.Start(); err != nil {
		logrus.Errorf("start naiveproxy error err: %v", err)
		return errors.New(constant.NaiveProxyStartError)
	}
	n.cmdMap.Store(apiPort, cmd)
	done := make(chan struct{})
	doneNaiveProxy.Store(apiPort, done)
	go func() {
		err := cmd.Wait()
		close(done)
		n.mutex.Lock()
		defer n.mutex.Unlock()
		if current, ok := n.cmdMap.Load(apiPort); ok && current == cmd {
			n.cmdMap.Delete(apiPort)
			doneNaiveProxy.Delete(apiPort)
			if err != nil {
				logrus.Errorf("naiveproxy exited on API port %d: %v", apiPort, err)
			}
		}
	}()
	return nil
}

// Stop waits for the old listener to close before the replacement is started.
// Keep saved configuration on failures so certificate maintenance can retry.
func (n *NaiveProxyProcess) Stop(apiPort uint, removeFile bool) error {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	if value, ok := n.cmdMap.Load(apiPort); ok {
		cmd := value.(*exec.Cmd)
		if err := cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
			return err
		}
		if value, ok := doneNaiveProxy.Load(apiPort); ok {
			select {
			case <-value.(chan struct{}):
			case <-time.After(5 * time.Second):
				return errors.New("naiveproxy stop timed out")
			}
		}
		n.cmdMap.Delete(apiPort)
		doneNaiveProxy.Delete(apiPort)
	}
	if removeFile {
		path, err := util.GetConfigFile(constant.NaiveProxy, apiPort)
		if err != nil {
			return err
		}
		return util.RemoveFile(path)
	}
	return nil
}

func GetNaiveProxyState(apiPort uint) bool {
	return NewNaiveProxyInstance().IsRunning(apiPort)
}

func (n *NaiveProxyProcess) IsRunning(port uint) bool {
	n.mutex.Lock()
	defer n.mutex.Unlock()
	return n.isRunningLocked(port)
}

func (n *NaiveProxyProcess) isRunningLocked(port uint) bool {
	if _, ok := n.cmdMap.Load(port); !ok {
		return false
	}
	if done, ok := doneNaiveProxy.Load(port); ok {
		select {
		case <-done.(chan struct{}):
			return false
		default:
			return true
		}
	}
	return false
}

package naiveproxy

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"
	"trojan-panel-core/core"
	"trojan-panel-core/core/process"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/util"
)

var certificateRestart sync.RWMutex

func currentCertificateFingerprint() ([32]byte, error) {
	config := core.Config.CertConfig
	pair, err := tls.LoadX509KeyPair(config.CrtPath, config.KeyPath)
	if err != nil {
		return [32]byte{}, err
	}
	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return [32]byte{}, err
	}
	now := time.Now()
	if now.Before(leaf.NotBefore) || !now.Before(leaf.NotAfter) {
		return [32]byte{}, errors.New("replacement certificate is not currently valid")
	}
	return sha256.Sum256(leaf.Raw), nil
}

// MaintainCertificates restarts only changed instances. Failed starts retain
// their snapshot, so the next pass retries automatically. Compare the served
// certificate instead of file timestamps, including changes during startup.
func MaintainCertificates() {
	certificateRestart.Lock()
	defer certificateRestart.Unlock()
	ports, err := util.GetConfigApiPorts(constant.NaiveProxyPath)
	if err != nil {
		logrus.Errorf("naiveproxy certificate inventory: %v", err)
		return
	}
	if len(ports) == 0 {
		return
	}
	fingerprint, err := currentCertificateFingerprint()
	if err != nil {
		logrus.Errorf("naiveproxy replacement certificate rejected: %v", err)
		return
	}
	for _, port := range ports {
		instance := process.NewNaiveProxyInstance()
		if instance.IsRunning(port) {
			served, probeErr := servedCertificateFingerprint(port)
			if probeErr != nil {
				logrus.Errorf("naiveproxy certificate probe on %d: %v", port, probeErr)
				continue
			}
			if served == fingerprint {
				continue
			}
		}
		if err = restartSavedInstance(port); err != nil {
			logrus.Errorf("naiveproxy certificate restart on %d failed: %v", port, err)
			continue
		}
		logrus.Infof("naiveproxy certificate loaded on API port %d", port)
	}
}

func servedCertificateFingerprint(port uint) ([32]byte, error) {
	path, err := util.GetConfigFile(constant.NaiveProxy, port)
	if err != nil {
		return [32]byte{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return [32]byte{}, err
	}
	var config struct {
		Apps struct {
			HTTP struct {
				Servers map[string]struct {
					Listen   []string `json:"listen"`
					Policies []struct {
						Match struct {
							SNI []string `json:"sni"`
						} `json:"match"`
					} `json:"tls_connection_policies"`
				} `json:"servers"`
			} `json:"http"`
		} `json:"apps"`
	}
	if err = json.Unmarshal(data, &config); err != nil {
		return [32]byte{}, err
	}
	server, ok := config.Apps.HTTP.Servers["srv0"]
	if !ok || len(server.Listen) == 0 || len(server.Policies) == 0 || len(server.Policies[0].Match.SNI) == 0 {
		return [32]byte{}, errors.New("naiveproxy TLS listener missing")
	}
	_, publicPort, err := net.SplitHostPort(server.Listen[0])
	if err != nil {
		return [32]byte{}, err
	}
	// The expected fingerprint is validated against the configured local pair;
	// this loopback probe supports private certificates without external roots.
	connection, err := tls.DialWithDialer(&net.Dialer{Timeout: 2 * time.Second}, "tcp", net.JoinHostPort("127.0.0.1", publicPort), &tls.Config{MinVersion: tls.VersionTLS12, ServerName: server.Policies[0].Match.SNI[0], InsecureSkipVerify: true})
	if err != nil {
		return [32]byte{}, err
	}
	defer connection.Close()
	return sha256.Sum256(connection.ConnectionState().PeerCertificates[0].Raw), nil
}

func restartSavedInstance(port uint) error {
	instance := process.NewNaiveProxyInstance()
	if instance.IsRunning(port) {
		if err := snapshotLiveConfig(port); err != nil {
			return err
		}
	}
	path, err := util.GetConfigFile(constant.NaiveProxy, port)
	if err != nil {
		return err
	}
	binary, err := util.GetBinaryFile(constant.NaiveProxy)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Validate the persisted live configuration before interrupting the service.
	if err = exec.CommandContext(ctx, binary, "validate", "--config", path).Run(); err != nil {
		return fmt.Errorf("naiveproxy restart preflight: %w", err)
	}
	if err := instance.Stop(port, false); err != nil {
		return err
	}
	if err := instance.StartNaiveProxy(port); err != nil {
		return err
	}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if !instance.IsRunning(port) {
			return errors.New("replacement naiveproxy exited")
		}
		connection, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))), 200*time.Millisecond)
		if err == nil {
			connection.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("replacement naiveproxy did not start listening")
}

func snapshotLiveConfig(port uint) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("http://127.0.0.1:%d/config/", port), nil)
	if err != nil {
		return err
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("live naiveproxy config HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 8*1024*1024+1))
	if err != nil {
		return err
	}
	if len(data) > 8*1024*1024 || !json.Valid(data) {
		return errors.New("invalid live naiveproxy configuration")
	}
	path, err := util.GetConfigFilePath(constant.NaiveProxy, port)
	if err != nil {
		return err
	}
	return saveConfig(path, data)
}

func saveConfig(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".naiveproxy-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0600); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(file.Name(), path)
}

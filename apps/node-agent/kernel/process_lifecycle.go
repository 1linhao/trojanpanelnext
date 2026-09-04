package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"
	"trojan-panel-core/core/process"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/util"
)

// ProcessLifecycle adapts the existing per-proxy process supervisor to the
// managed kernel interface. Only the selected kernel's children are stopped.
type ProcessLifecycle struct{}

func (ProcessLifecycle) InUse(_ context.Context, name Kernel) bool {
	var path string
	switch name {
	case KernelXray:
		path = constant.XrayPath
	case KernelHysteria2:
		path = constant.Hysteria2Path
	default:
		return false
	}
	ports, err := util.GetConfigApiPorts(path)
	return err == nil && len(ports) > 0
}

func (ProcessLifecycle) Preflight(ctx context.Context, name Kernel, candidate string) error {
	switch name {
	case KernelXray:
		configs, err := filepath.Glob(filepath.Join(constant.XrayPath, "*.json"))
		if err != nil {
			return err
		}
		for _, config := range configs {
			command := exec.CommandContext(ctx, candidate, "run", "-test", "-config", config)
			if output, commandErr := command.CombinedOutput(); commandErr != nil {
				return fmt.Errorf("xray preflight failed for %s: %w: %s", filepath.Base(config), commandErr, string(output))
			}
		}
		return nil
	case KernelHysteria2:
		configs, err := filepath.Glob(filepath.Join(constant.Hysteria2Path, "*.json"))
		if err != nil {
			return err
		}
		for _, config := range configs {
			if err = preflightHysteria2(ctx, candidate, config); err != nil {
				return err
			}
		}
		return nil
	default:
		return ErrInvalidKernel
	}
}

func (ProcessLifecycle) Restart(ctx context.Context, name Kernel) error {
	var (
		ports []uint
		stop  func(uint, bool) error
		start func(uint) error
		path  string
	)
	switch name {
	case KernelXray:
		path = constant.XrayPath
		instance := process.NewXrayProcess()
		stop = instance.Stop
		start = instance.StartXray
	case KernelHysteria2:
		path = constant.Hysteria2Path
		instance := process.NewHysteria2Instance()
		stop = instance.Stop
		start = instance.StartHysteria2
	default:
		return ErrInvalidKernel
	}
	var err error
	ports, err = util.GetConfigApiPorts(path)
	if err != nil {
		return err
	}
	for _, port := range ports {
		if err = stop(port, false); err != nil {
			return err
		}
	}
	for _, port := range ports {
		if err = start(port); err != nil {
			return err
		}
	}
	return nil
}

func (ProcessLifecycle) Observe(ctx context.Context, name Kernel, observe time.Duration) error {
	var (
		ports []uint
		state func(uint) bool
		path  string
	)
	switch name {
	case KernelXray:
		path = constant.XrayPath
		state = process.GetXrayState
	case KernelHysteria2:
		path = constant.Hysteria2Path
		state = process.GetHysteria2State
	default:
		return ErrInvalidKernel
	}
	var err error
	ports, err = util.GetConfigApiPorts(path)
	if err != nil {
		return err
	}
	if observe <= 0 {
		observe = 30 * time.Second
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	timer := time.NewTimer(observe)
	defer timer.Stop()
	listening := make(map[uint]bool, len(ports))
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for _, port := range ports {
				if !state(port) {
					return fmt.Errorf("%s instance on API port %d exited during observation", name, port)
				}
				connection, dialErr := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(int(port))), 200*time.Millisecond)
				if dialErr != nil {
					if listening[port] {
						return fmt.Errorf("%s instance on API port %d stopped listening: %w", name, port, dialErr)
					}
					continue
				}
				_ = connection.Close()
				listening[port] = true
			}
		case <-timer.C:
			for _, port := range ports {
				if !listening[port] {
					return fmt.Errorf("%s instance on API port %d did not begin listening", name, port)
				}
			}
			return nil
		}
	}
}

func preflightHysteria2(ctx context.Context, candidate, configPath string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}
	var config map[string]any
	if err = json.Unmarshal(data, &config); err != nil {
		return fmt.Errorf("hysteria2 config %s is invalid JSON: %w", filepath.Base(configPath), err)
	}
	servicePort, closeService, err := temporaryPort()
	if err != nil {
		return err
	}
	closeService()
	statsPort, closeStats, err := temporaryPort()
	if err != nil {
		return err
	}
	closeStats()
	config["listen"] = ":" + strconv.Itoa(servicePort)
	stats, ok := config["trafficStats"].(map[string]any)
	if !ok {
		stats = make(map[string]any)
		config["trafficStats"] = stats
	}
	stats["listen"] = ":" + strconv.Itoa(statsPort)
	tempDir, err := os.MkdirTemp("", "trojan-panel-hysteria2-preflight-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tempDir)
	tempConfig := filepath.Join(tempDir, "config.json")
	data, err = json.Marshal(config)
	if err != nil {
		return err
	}
	if err = os.WriteFile(tempConfig, data, 0600); err != nil {
		return err
	}

	runCtx, cancel := context.WithCancel(ctx)
	command := exec.CommandContext(runCtx, candidate, "-c", tempConfig, "server")
	if err = command.Start(); err != nil {
		cancel()
		return fmt.Errorf("hysteria2 preflight could not start: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	select {
	case err = <-done:
		cancel()
		if err == nil {
			err = errors.New("candidate exited before preflight observation completed")
		}
		return fmt.Errorf("hysteria2 preflight failed for %s: %w", filepath.Base(configPath), err)
	case <-timer.C:
		cancel()
		<-done
		return nil
	case <-ctx.Done():
		cancel()
		<-done
		return ctx.Err()
	}
}

func temporaryPort() (int, func(), error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, func() {}, err
	}
	return listener.Addr().(*net.TCPAddr).Port, func() { _ = listener.Close() }, nil
}

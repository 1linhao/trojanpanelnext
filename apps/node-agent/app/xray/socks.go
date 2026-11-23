package xray

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"time"

	"github.com/xtls/xray-core/app/proxyman/command"
	"github.com/xtls/xray-core/common"
	"google.golang.org/grpc/status"
	"trojan-panel-core/model/bo"
	"trojan-panel-core/model/constant"
	"trojan-panel-core/util"
)

// Keep the managed listener outside Xray's startup inbounds. The agent installs it
// only after loading current grants. Other template inbounds can start normally.
const socksInboundKey = "tpnextManagedSocksInbound"

func prepareSocksConfig(data []byte, port uint) ([]byte, bool, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, false, err
	}
	var inbounds []json.RawMessage
	if err := json.Unmarshal(config["inbounds"], &inbounds); err != nil {
		return nil, false, err
	}
	if _, exists := config[socksInboundKey]; exists {
		// A migrated file has no live inbound on the managed port.
		for _, raw := range inbounds {
			var header struct{ Port uint }
			if json.Unmarshal(raw, &header) == nil && header.Port == port {
				return nil, false, errors.New("conflicting managed SOCKS inbound")
			}
		}
		return data, false, nil
	}
	if len(inbounds) == 0 {
		return nil, false, errors.New("missing managed SOCKS inbound")
	}
	last := len(inbounds) - 1
	var inbound map[string]json.RawMessage
	if err := json.Unmarshal(inbounds[last], &inbound); err != nil {
		return nil, false, err
	}
	var header struct {
		Port          uint
		Protocol, Tag string
	}
	if err := json.Unmarshal(inbounds[last], &header); err != nil {
		return nil, false, err
	}
	if header.Port != port || header.Protocol != "socks" {
		return nil, false, errors.New("missing managed SOCKS inbound")
	}
	if header.Tag == "" {
		header.Tag = fmt.Sprintf("tpnext-socks-%d", port)
		inbound["tag"], _ = json.Marshal(header.Tag)
	}
	for _, raw := range inbounds[:last] {
		var other struct{ Tag string }
		if err := json.Unmarshal(raw, &other); err != nil {
			return nil, false, err
		}
		if other.Tag == header.Tag {
			return nil, false, errors.New("duplicate managed SOCKS inbound tag")
		}
	}
	var settings map[string]json.RawMessage
	if raw := inbound["settings"]; len(raw) > 0 {
		if err := json.Unmarshal(raw, &settings); err != nil {
			return nil, false, err
		}
	}
	if settings == nil {
		settings = make(map[string]json.RawMessage)
	}
	settings["auth"] = json.RawMessage(`"password"`)
	delete(settings, "accounts")
	inbound["settings"], _ = json.Marshal(settings)
	config[socksInboundKey], _ = json.Marshal(inbound)
	config["inbounds"], _ = json.Marshal(inbounds[:last])
	result, err := json.MarshalIndent(config, "", "    ")
	return result, true, err
}

type SocksSync struct {
	applied   [32]byte
	installed bool
}

// Update only this listener via the running kernel's HandlerService. Using the
// installed binary to parse inbound JSON preserves version-specific transport options.
func (s *SocksSync) Apply(port uint, allowed map[string]bo.AccountBo) error {
	path, err := util.GetConfigFile(constant.Xray, port)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var config struct {
		Inbound map[string]json.RawMessage `json:"tpnextManagedSocksInbound"`
	}
	if err = json.Unmarshal(data, &config); err != nil {
		return err
	}
	var tag string
	if err = json.Unmarshal(config.Inbound["tag"], &tag); err != nil || tag == "" {
		return errors.New("missing managed SOCKS tag")
	}
	var settings map[string]json.RawMessage
	if err = json.Unmarshal(config.Inbound["settings"], &settings); err != nil {
		return err
	}
	type credential struct {
		User string `json:"user"`
		Pass string `json:"pass"`
	}
	accounts := make([]credential, 0, len(allowed))
	for _, a := range allowed {
		if a.ID == 0 || a.Pass == "" {
			return errors.New("missing account identity")
		}
		user, pass := util.SocksCredentials(a.ID, a.Pass)
		accounts = append(accounts, credential{user, pass})
	}
	sort.Slice(accounts, func(i, j int) bool { return accounts[i].User < accounts[j].User })
	settings["accounts"], _ = json.Marshal(accounts)
	config.Inbound["settings"], _ = json.Marshal(settings)
	body, err := json.Marshal(map[string]any{"inbounds": []any{config.Inbound}})
	if err != nil {
		return err
	}
	digest := sha256.Sum256(body)
	if s.applied == digest {
		return nil
	}
	if s.installed {
		conn, ctx, closeConn, err := apiClient(port)
		if err != nil {
			return err
		}
		_, err = command.NewHandlerServiceClient(conn).RemoveInbound(ctx, &command.RemoveInboundRequest{Tag: tag})
		closeConn()
		if err != nil && status.Convert(err).Message() != common.ErrNoClue.Error() {
			return fmt.Errorf("remove managed SOCKS inbound: %w", err)
		}
		s.installed = false
	}
	s.applied = [32]byte{}
	if len(accounts) > 0 {
		binary, err := util.GetBinaryFile(constant.Xray)
		if err != nil {
			return err
		}
		ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, binary, "api", "adi", fmt.Sprintf("--server=127.0.0.1:%d", port), "stdin:")
		cmd.Stdin = bytes.NewReader(body)
		s.installed = true // A timed-out request may have reached the kernel.
		if err = cmd.Run(); err != nil {
			return fmt.Errorf("install managed SOCKS inbound: %w", err)
		}
	}
	s.applied = digest
	return nil
}

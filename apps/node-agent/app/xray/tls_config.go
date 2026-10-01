package xray

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// Keep unrecognized Xray options and their numeric precision while removing
// only the obsolete client verification option from server inbounds.
func withoutServerAllowInsecure(data []byte) ([]byte, bool, error) {
	var config map[string]json.RawMessage
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, false, err
	}
	var inbounds []json.RawMessage
	if raw, exists := config["inbounds"]; exists {
		if err := json.Unmarshal(raw, &inbounds); err != nil {
			return nil, false, err
		}
	}
	changed := false
	for i, raw := range inbounds {
		var inbound map[string]json.RawMessage
		if err := json.Unmarshal(raw, &inbound); err != nil {
			return nil, false, err
		}
		streamRaw, exists := inbound["streamSettings"]
		if !exists {
			continue
		}
		var stream map[string]json.RawMessage
		if err := json.Unmarshal(streamRaw, &stream); err != nil {
			return nil, false, err
		}
		tlsRaw, exists := stream["tlsSettings"]
		if !exists {
			continue
		}
		var tls map[string]json.RawMessage
		if err := json.Unmarshal(tlsRaw, &tls); err != nil {
			return nil, false, err
		}
		if _, exists := tls["allowInsecure"]; !exists {
			continue
		}
		delete(tls, "allowInsecure")
		stream["tlsSettings"], _ = json.Marshal(tls)
		inbound["streamSettings"], _ = json.Marshal(stream)
		inbounds[i], _ = json.Marshal(inbound)
		changed = true
	}
	if !changed {
		return data, false, nil
	}
	config["inbounds"], _ = json.Marshal(inbounds)
	result, err := json.MarshalIndent(config, "", "    ")
	return result, true, err
}

func migrateServerTLSConfig(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("Xray config %s must be a regular file", path)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	migrated, changed, err := withoutServerAllowInsecure(original)
	if err != nil {
		return fmt.Errorf("migrate Xray server TLS config %s: %w", path, err)
	}
	if !changed {
		return nil
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".xray-tls-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	if stat, ok := info.Sys().(*syscall.Stat_t); ok {
		if err = file.Chown(int(stat.Uid), int(stat.Gid)); err != nil {
			return err
		}
	}
	if _, err = file.Write(migrated); err != nil {
		return err
	}
	if err = file.Chmod(info.Mode()); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	currentInfo, err := os.Lstat(path)
	if err != nil {
		return err
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if !os.SameFile(info, currentInfo) || info.Mode() != currentInfo.Mode() || !bytes.Equal(current, original) {
		return fmt.Errorf("Xray config %s changed during TLS migration", path)
	}
	return os.Rename(file.Name(), path)
}

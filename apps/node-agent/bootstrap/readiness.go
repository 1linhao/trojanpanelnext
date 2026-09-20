// Package bootstrap records proof that the Web control plane reached this
// Node Agent through the existing client-certificate-authenticated gRPC path.
package bootstrap

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"trojan-panel-core/core"
	"trojan-panel-core/internal/atomicfile"
)

const markerSchemaVersion = 1

type marker struct {
	SchemaVersion      int    `json:"schema_version"`
	NodeIdentityID     string `json:"node_identity_id"`
	IdentityGeneration uint64 `json:"identity_generation"`
	NodeServerID       uint   `json:"node_server_id"`
}

func markerPath() string {
	if path := os.Getenv("TP_NODE_BOOTSTRAP_MARKER"); path != "" {
		return path
	}
	runtimeDir := os.Getenv("TP_KERNEL_RUNTIME")
	if runtimeDir == "" {
		runtimeDir = "/tpdata/trojan-panel-core/runtime"
	}
	return filepath.Join(runtimeDir, "bootstrap-verified.json")
}

// MarkVerified records the current identity generation only after the caller
// has authenticated at the gRPC layer.
func MarkVerified() error {
	current, err := currentMarker()
	if err != nil {
		return err
	}
	contents, err := json.Marshal(current)
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return atomicfile.Write(markerPath(), contents, 0600)
}

// Ready reports whether an authenticated Web-to-Node gRPC call has verified
// the exact identity generation currently loaded by the Node Agent.
func Ready() bool {
	info, err := os.Lstat(markerPath())
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return false
	}
	contents, err := os.ReadFile(markerPath())
	if err != nil || len(contents) > 4096 {
		return false
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var stored marker
	if err = decoder.Decode(&stored); err != nil {
		return false
	}
	var trailing interface{}
	if err = decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return false
	}
	current, err := currentMarker()
	return err == nil && stored == current
}

func currentMarker() (marker, error) {
	config := core.Config.NodeConfig
	if config.ServerID == 0 || config.IdentityID == "" || config.IdentityGeneration == 0 {
		return marker{}, errors.New("Node identity configuration is incomplete")
	}
	return marker{
		SchemaVersion: markerSchemaVersion, NodeIdentityID: config.IdentityID,
		IdentityGeneration: config.IdentityGeneration, NodeServerID: config.ServerID,
	}, nil
}

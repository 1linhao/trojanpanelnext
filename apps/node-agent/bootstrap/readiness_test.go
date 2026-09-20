package bootstrap

import (
	"os"
	"path/filepath"
	"testing"

	"trojan-panel-core/core"
)

func TestReadinessIsBoundToCurrentIdentityGeneration(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "bootstrap-verified.json")
	t.Setenv("TP_NODE_BOOTSTRAP_MARKER", marker)
	oldNode := core.Config.NodeConfig
	t.Cleanup(func() { core.Config.NodeConfig = oldNode })
	core.Config.NodeConfig = core.NodeConfig{
		ServerID: 42, IdentityID: "11111111-2222-4333-8444-555555555555", IdentityGeneration: 7,
	}
	if Ready() {
		t.Fatal("fresh Node Agent unexpectedly ready before Web mTLS verification")
	}
	if err := MarkVerified(); err != nil {
		t.Fatalf("MarkVerified: %v", err)
	}
	if !Ready() {
		t.Fatal("current identity generation did not become ready")
	}
	info, err := os.Stat(marker)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("marker mode = %o, want 0600", info.Mode().Perm())
	}
	core.Config.NodeConfig.IdentityGeneration++
	if Ready() {
		t.Fatal("old generation marker made rotated identity ready")
	}
}

func TestTamperedReadinessMarkerFailsClosed(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "bootstrap-verified.json")
	t.Setenv("TP_NODE_BOOTSTRAP_MARKER", marker)
	oldNode := core.Config.NodeConfig
	t.Cleanup(func() { core.Config.NodeConfig = oldNode })
	core.Config.NodeConfig = core.NodeConfig{
		ServerID: 42, IdentityID: "11111111-2222-4333-8444-555555555555", IdentityGeneration: 7,
	}
	if err := os.WriteFile(marker, []byte(`{"identity_id":"other","generation":7}`), 0600); err != nil {
		t.Fatal(err)
	}
	if Ready() {
		t.Fatal("tampered readiness marker was accepted")
	}
	if err := MarkVerified(); err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(marker, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = file.WriteString("{}\n"); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	if Ready() {
		t.Fatal("readiness marker with trailing JSON was accepted")
	}
}

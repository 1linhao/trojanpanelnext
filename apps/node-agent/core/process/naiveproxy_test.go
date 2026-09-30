package process

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNaiveRestartWaitsForExitAndPreservesConfiguration(t *testing.T) {
	old, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(old) })
	if err = os.MkdirAll("bin/naiveproxy/config", 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile("bin/naiveproxy/naiveproxy", []byte("#!/bin/sh\nexec sleep 3600\n"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("bin/naiveproxy/config", "config-65433.json")
	const content = `{"users":["preserve-runtime-users"]}`
	if err = os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	n := NewNaiveProxyInstance()
	t.Cleanup(func() { n.Stop(65433, false) })
	for i := 0; i < 3; i++ {
		if err = n.StartNaiveProxy(65433); err != nil {
			t.Fatal(err)
		}
		if !n.IsRunning(65433) {
			t.Fatal("process not running")
		}
		if err = n.Stop(65433, false); err != nil {
			t.Fatal(err)
		}
		if n.IsRunning(65433) {
			t.Fatal("old process still running")
		}
		got, err := os.ReadFile(path)
		if err != nil || string(got) != content {
			t.Fatal("restart damaged saved config")
		}
	}
}

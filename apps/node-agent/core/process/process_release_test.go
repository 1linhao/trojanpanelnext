package process

import (
	"os/exec"
	"testing"
)

func TestHysteria2ReleaseDoesNotDeleteReplacement(t *testing.T) {
	instance := NewHysteria2Instance()
	const port = uint(65431)
	instance.cmdMap.Delete(port)
	t.Cleanup(func() { instance.cmdMap.Delete(port) })

	finished := exec.Command("true")
	replacement := exec.Command("true")
	instance.cmdMap.Store(port, replacement)

	instance.releaseProcess(port, finished)

	got, ok := instance.cmdMap.Load(port)
	if !ok || got != replacement {
		t.Fatal("finished Hysteria2 process deleted its replacement")
	}
}

func TestXrayReleaseDoesNotDeleteReplacement(t *testing.T) {
	instance := NewXrayProcess()
	const port = uint(65432)
	instance.cmdMap.Delete(port)
	t.Cleanup(func() { instance.cmdMap.Delete(port) })

	finished := exec.Command("true")
	replacement := exec.Command("true")
	instance.cmdMap.Store(port, replacement)

	instance.releaseProcess(port, finished)

	got, ok := instance.cmdMap.Load(port)
	if !ok || got != replacement {
		t.Fatal("finished Xray process deleted its replacement")
	}
}

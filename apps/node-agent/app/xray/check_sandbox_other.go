//go:build !linux || (!amd64 && !arm64)

package xray

import (
	"errors"
	"os/exec"
)

func startRestrictedCheck(cmd *exec.Cmd) error {
	return errors.New("read-only Xray validation requires a supported Linux Node")
}

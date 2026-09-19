//go:build !linux

package dao

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func openInitialSysadminPasswordFile(path string) (*os.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve initial sysadmin password path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, fmt.Errorf("resolve initial sysadmin password symlinks: %w", err)
	}
	if resolved != filepath.Clean(absolute) {
		return nil, errors.New("initial sysadmin password path must not contain symlinks")
	}
	file, err := os.Open(absolute)
	if err != nil {
		return nil, fmt.Errorf("open initial sysadmin password: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("read initial sysadmin password metadata: %w", err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, errors.New("initial sysadmin password must be a regular non-symlink file")
	}
	if info.Mode().Perm() != 0600 {
		_ = file.Close()
		return nil, errors.New("initial sysadmin password file permissions must be exactly 0600")
	}
	return file, nil
}

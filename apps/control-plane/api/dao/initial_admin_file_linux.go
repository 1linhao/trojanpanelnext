//go:build linux

package dao

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

func openInitialSysadminPasswordFile(path string) (*os.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve initial sysadmin password path: %w", err)
	}
	parts := strings.Split(strings.TrimPrefix(filepath.Clean(absolute), string(filepath.Separator)), string(filepath.Separator))
	if len(parts) == 0 || parts[0] == "" {
		return nil, errors.New("initial sysadmin password path is invalid")
	}

	directoryFD, err := syscall.Open(string(filepath.Separator), syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open initial sysadmin password root: %w", err)
	}
	defer func() { _ = syscall.Close(directoryFD) }()

	for _, component := range parts[:len(parts)-1] {
		nextFD, openErr := syscall.Openat(directoryFD, component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if openErr != nil {
			return nil, fmt.Errorf("open initial sysadmin password parent without symlinks: %w", openErr)
		}
		_ = syscall.Close(directoryFD)
		directoryFD = nextFD
	}

	fileFD, err := syscall.Openat(directoryFD, parts[len(parts)-1], syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open initial sysadmin password without symlinks: %w", err)
	}
	file := os.NewFile(uintptr(fileFD), absolute)
	if file == nil {
		_ = syscall.Close(fileFD)
		return nil, errors.New("open initial sysadmin password: invalid file descriptor")
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

//go:build linux

package nodeidentity

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/sys/unix"
)

func openCredentialParent(path string) (int, string, string, error) {
	for _, component := range strings.Split(path, string(filepath.Separator)) {
		if component == ".." {
			return -1, "", "", errors.New("credential file path must not contain .. components")
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return -1, "", "", fmt.Errorf("resolve credential file path: %w", err)
	}
	parts := strings.Split(strings.TrimPrefix(filepath.Clean(absolute), string(filepath.Separator)), string(filepath.Separator))
	if len(parts) < 2 || parts[0] == "" {
		return -1, "", "", errors.New("credential file path is invalid")
	}
	directoryFD, err := syscall.Open(string(filepath.Separator), syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return -1, "", "", fmt.Errorf("open credential file root: %w", err)
	}
	for _, component := range parts[:len(parts)-1] {
		nextFD, openErr := syscall.Openat(directoryFD, component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if openErr != nil {
			_ = syscall.Close(directoryFD)
			return -1, "", "", fmt.Errorf("open credential file parent without symlinks: %w", openErr)
		}
		_ = syscall.Close(directoryFD)
		directoryFD = nextFD
	}
	return directoryFD, parts[len(parts)-1], absolute, nil
}

func validateCredentialPath(path string) error {
	directoryFD, _, _, err := openCredentialParent(path)
	if err != nil {
		return err
	}
	_ = syscall.Close(directoryFD)
	_, err = readCredentialFile(path)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func readCredentialFile(path string) ([]byte, error) {
	directoryFD, name, absolute, err := openCredentialParent(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = syscall.Close(directoryFD) }()
	fileFD, err := syscall.Openat(directoryFD, name, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fileFD), absolute)
	if file == nil {
		_ = syscall.Close(fileFD)
		return nil, errors.New("open credential file: invalid file descriptor")
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("read credential file metadata: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errors.New("credential file must be a regular non-symlink file with mode 0600")
	}
	return io.ReadAll(file)
}

func createCredentialFile(path string, contents []byte) error {
	directoryFD, name, absolute, err := openCredentialParent(path)
	if err != nil {
		return err
	}
	defer func() { _ = syscall.Close(directoryFD) }()
	randomSuffix := make([]byte, 12)
	if _, err = io.ReadFull(rand.Reader, randomSuffix); err != nil {
		return fmt.Errorf("create credential staging name: %w", err)
	}
	temporaryName := "." + name + "." + hex.EncodeToString(randomSuffix) + ".tmp"
	fileFD, err := syscall.Openat(directoryFD, temporaryName,
		syscall.O_WRONLY|syscall.O_CREAT|syscall.O_EXCL|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0600)
	if err != nil {
		return fmt.Errorf("create credential file without replacement: %w", err)
	}
	file := os.NewFile(uintptr(fileFD), absolute)
	if file == nil {
		_ = syscall.Close(fileFD)
		_ = syscall.Unlinkat(directoryFD, temporaryName)
		return errors.New("create credential file: invalid file descriptor")
	}
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = syscall.Unlinkat(directoryFD, temporaryName)
		}
	}()
	if _, err = io.Copy(file, bytes.NewReader(contents)); err != nil {
		return fmt.Errorf("write credential file: %w", err)
	}
	if err = syscall.Fchmod(fileFD, 0600); err != nil {
		return fmt.Errorf("protect credential file: %w", err)
	}
	if err = file.Sync(); err != nil {
		return fmt.Errorf("sync credential file: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("close credential file: %w", err)
	}
	if err = unix.Renameat2(directoryFD, temporaryName, directoryFD, name, unix.RENAME_NOREPLACE); err != nil {
		return fmt.Errorf("publish credential file without replacement: %w", err)
	}
	if err = syscall.Fsync(directoryFD); err != nil {
		return fmt.Errorf("sync credential file parent: %w", err)
	}
	complete = true
	return nil
}

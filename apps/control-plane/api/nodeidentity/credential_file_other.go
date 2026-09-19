//go:build !linux

package nodeidentity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func validateCredentialPath(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve credential file path: %w", err)
	}
	parent := filepath.Dir(absolute)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("resolve credential file parent: %w", err)
	}
	if resolved != parent {
		return errors.New("credential file parent path must not contain symlinks")
	}
	_, err = readCredentialFile(absolute)
	if err == nil || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func readCredentialFile(path string) ([]byte, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve credential file path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return nil, err
	}
	if resolved != filepath.Clean(absolute) {
		return nil, errors.New("credential file path must not contain symlinks")
	}
	info, err := os.Stat(absolute)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0600 {
		return nil, errors.New("credential file must be a regular non-symlink file with mode 0600")
	}
	return os.ReadFile(absolute)
}

func createCredentialFile(path string, contents []byte) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("resolve credential file path: %w", err)
	}
	parent := filepath.Dir(absolute)
	resolved, err := filepath.EvalSymlinks(parent)
	if err != nil {
		return fmt.Errorf("resolve credential file parent: %w", err)
	}
	if resolved != parent {
		return errors.New("credential file parent path must not contain symlinks")
	}
	file, err := os.OpenFile(absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create credential file without replacement: %w", err)
	}
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = os.Remove(absolute)
		}
	}()
	if _, err = file.Write(contents); err != nil {
		return fmt.Errorf("write credential file: %w", err)
	}
	if err = file.Sync(); err != nil {
		return fmt.Errorf("sync credential file: %w", err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("close credential file: %w", err)
	}
	complete = true
	return nil
}

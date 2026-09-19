//go:build linux && amd64 && securefiletest

package main

import (
	"errors"
	"fmt"
	"os"
	"time"
)

const (
	testReadyEnv    = "TP_SECURE_FILE_TEST_READY"
	testContinueEnv = "TP_SECURE_FILE_TEST_CONTINUE"
)

func waitAtPreCommitTestHook() error {
	ready := os.Getenv(testReadyEnv)
	continuePath := os.Getenv(testContinueEnv)
	if ready == "" && continuePath == "" {
		return nil
	}
	if ready == "" || continuePath == "" {
		return errors.New("secure-file test hook requires both ready and continue paths")
	}
	readyFile, err := os.OpenFile(ready, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return fmt.Errorf("create secure-file test ready marker: %w", err)
	}
	if err := readyFile.Close(); err != nil {
		return fmt.Errorf("close secure-file test ready marker: %w", err)
	}
	deadline := time.Now().Add(10 * time.Second)
	for {
		if _, err := os.Lstat(continuePath); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("inspect secure-file test continue marker: %w", err)
		}
		if time.Now().After(deadline) {
			return errors.New("timed out waiting for secure-file test continue marker")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

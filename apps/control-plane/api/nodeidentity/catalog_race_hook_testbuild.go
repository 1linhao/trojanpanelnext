//go:build nodeidentitycatalogracetest

package nodeidentity

import (
	"errors"
	"os"
	"path/filepath"
	"time"
)

func catalogRaceBeforeLock(nodeKey string) error {
	return catalogRaceMark(nodeKey, "entered")
}

func catalogRaceAfterPrecheck(nodeKey string) error {
	if err := catalogRaceMark(nodeKey, "ready"); err != nil {
		return err
	}
	if nodeKey != os.Getenv("TP_NODE_CATALOG_TEST_PAUSE_KEY") {
		return nil
	}
	directory := os.Getenv("TP_NODE_CATALOG_TEST_BARRIER_DIR")
	deadline := time.Now().Add(15 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(filepath.Join(directory, nodeKey+".release")); err == nil {
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		time.Sleep(20 * time.Millisecond)
	}
	return errors.New("catalog race test barrier timed out")
}

func catalogRaceMark(nodeKey, stage string) error {
	directory := os.Getenv("TP_NODE_CATALOG_TEST_BARRIER_DIR")
	if directory == "" {
		return nil
	}
	return os.WriteFile(filepath.Join(directory, nodeKey+"."+stage), nil, 0600)
}

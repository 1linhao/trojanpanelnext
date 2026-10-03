package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

// The host updater and Agent share this bind-mounted admission lock. Keeping
// it until the queued operation is persisted prevents a container switch from
// racing a direct kernel gRPC request.
func (m *Manager) lockMaintenance(ctx context.Context) (func(), error) {
	lock, err := os.OpenFile(filepath.Join(m.runtimeDir, "maintenance.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	unlock := func() { _ = syscall.Flock(int(lock.Fd()), syscall.LOCK_UN); _ = lock.Close() }
	for {
		err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			break
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) && !errors.Is(err, syscall.EINTR) {
			_ = lock.Close()
			return nil, err
		}
		select {
		case <-ctx.Done():
			_ = lock.Close()
			return nil, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
	file, err := os.Open(filepath.Join(m.runtimeDir, "container-update.json"))
	if errors.Is(err, os.ErrNotExist) {
		return unlock, nil
	}
	if err != nil {
		unlock()
		return nil, err
	}
	defer file.Close()
	var marker struct {
		Status string `json:"status"`
	}
	decoder := json.NewDecoder(io.LimitReader(file, 16384))
	if err = decoder.Decode(&marker); err == nil {
		if decoder.Decode(&struct{}{}) != io.EOF {
			err = errors.New("invalid container update state")
		}
	}
	if err != nil {
		unlock()
		return nil, fmt.Errorf("could not check container update state: %w", err)
	}
	switch marker.Status {
	case "succeeded", "failed":
		return unlock, nil
	case "queued", "running":
		unlock()
		return nil, errors.New("Node container update is active; wait before managing kernels")
	default:
		unlock()
		return nil, errors.New("invalid container update state")
	}
}

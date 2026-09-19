//go:build linux && amd64 && !securefiletest

package main

func waitAtPreCommitTestHook() error {
	return nil
}

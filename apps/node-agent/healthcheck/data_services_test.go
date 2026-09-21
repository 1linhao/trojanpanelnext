package healthcheck

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestVerifyRejectsUnknownModeWithoutNetworkAccess(t *testing.T) {
	if err := Verify("unknown"); err == nil {
		t.Fatal("unknown health mode was accepted")
	}
}

func TestWatchStopsAfterFreshCredentialAuthenticationFails(t *testing.T) {
	attempts := 0
	err := watch(context.Background(), time.Millisecond, func() error {
		attempts++
		if attempts == 2 {
			return errors.New("revoked")
		}
		return nil
	})
	if err == nil || attempts != 2 {
		t.Fatalf("watch result = %v after %d attempts, want failure after 2", err, attempts)
	}
}

func TestProductionCredentialInvalidationDeadlineIsFailClosed(t *testing.T) {
	if CredentialInvalidationDeadline > 10*time.Second {
		t.Fatalf("credential invalidation deadline = %s, want at most 10s", CredentialInvalidationDeadline)
	}
}

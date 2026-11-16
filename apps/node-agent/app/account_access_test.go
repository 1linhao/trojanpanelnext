package app

import (
	"errors"
	"fmt"
	"testing"
	"trojan-panel-core/model/bo"
)

func TestReconcileListenerAccountsKeepsAllAllowedUsers(t *testing.T) {
	allowed := make(map[string]bo.AccountBo)
	for i := 0; i < 105; i++ {
		pass := fmt.Sprintf("account-%d", i)
		allowed[pass] = bo.AccountBo{Pass: pass}
	}
	users := map[string]bool{"account-0": true, "revoked": true}
	add := func(account bo.AccountBo) error {
		if account.Pass == "account-0" {
			t.Fatal("re-added an installed account")
		}
		return nil
	}
	if err := reconcileListenerAccounts(users, allowed, add, func(pass string) error {
		if pass != "revoked" {
			t.Fatalf("removed an allowed account: %s", pass)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if len(users) != len(allowed) || users["revoked"] {
		t.Fatalf("unexpected synchronized user set: %d", len(users))
	}
}

func TestReconcileListenerAccountsRemovesAnUncertainAdd(t *testing.T) {
	users := make(map[string]bool)
	failure := errors.New("timed out")
	if err := reconcileListenerAccounts(users, map[string]bo.AccountBo{"new": {Pass: "new"}},
		func(bo.AccountBo) error { return failure }, func(string) error { return nil }); !errors.Is(err, failure) {
		t.Fatalf("expected the failed add, got %v", err)
	}
	revoked := false
	if err := reconcileListenerAccounts(users, nil, nil, func(pass string) error {
		revoked = pass == "new"
		return nil
	}); err != nil || !revoked || len(users) != 0 {
		t.Fatalf("uncertain user was not revoked, revoked=%v err=%v", revoked, err)
	}
}

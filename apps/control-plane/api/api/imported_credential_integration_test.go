package api

import (
	"encoding/json"
	"os"
	"sync"
	"testing"
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/util"
)

func TestImportedCredential(t *testing.T) {
	dsn := os.Getenv("TP_CREDENTIAL_TEST_DSN")
	if dsn == "" {
		t.Skip("requires disposable MariaDB")
	}
	db := disposableAPIStore(t, dsn, "127.0.0.1:1")
	account := func(name string) *model.Account {
		a, err := dao.SelectAccountByUsername(&name)
		if err != nil {
			t.Fatal(err)
		}
		return a
	}
	const original = "fixture-original-password"
	create := func(name string) {
		p := original
		if err := dao.CreateAccount(&model.Account{Username: &name, Pass: &p}); err != nil {
			t.Fatal(err)
		}
	}
	create("owner")
	create("unrelated")
	owner, b := account("owner"), account("unrelated")
	clone := "cloned-backup"
	if err := dao.CreateOrUpdateAccount(model.Account{Username: &clone, Pass: owner.Pass}, 0); err == nil {
		t.Fatal("backup inserted another owner of an existing connection and subscription token")
	}

	// Recreate pre-upgrade data, including a row copied under a new username.
	if _, err := db.Exec("ALTER TABLE account DROP INDEX account_connection_unique"); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO account(username,pass,hash) SELECT ?,pass,hash FROM account WHERE id=?", clone, *owner.Id); err != nil {
		t.Fatal(err)
	}
	dao.CloseDb()
	dao.InitMySQL()
	migrated, c := account("owner"), account(clone)
	if *migrated.Pass == *owner.Pass || *c.Pass == *owner.Pass || *migrated.Pass == *c.Pass || *account("unrelated").Pass != *b.Pass {
		t.Fatal("upgrade did not retire all copies independently")
	}
	if _, err := dao.SelectAccountClashSubscribe(*owner.Pass); err == nil {
		t.Fatal("retired subscription still accepted")
	}
	if !util.Sha1Match(migrated.LoginVerifier(), "owner"+original) {
		t.Fatal("original owner's panel password no longer matches")
	}
	if err := dao.CreateOrUpdateAccount(model.Account{Username: &clone, Pass: owner.Pass}, 1); err == nil {
		t.Fatal("restore reactivated retired credential")
	}
	dao.CloseDb()
	dao.InitMySQL()
	if *account("owner").Pass != *migrated.Pass {
		t.Fatal("second startup rotated credentials again")
	}
	rows, err := dao.SelectAccountUnused()
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.Username != "owner" {
			continue
		}
		raw, err := json.Marshal(r)
		if err != nil {
			t.Fatal(err)
		}
		var restored model.Account
		if err = json.Unmarshal(raw, &restored); err != nil {
			t.Fatal(err)
		}
		if err = dao.CreateOrUpdateAccount(restored, 1); err != nil {
			t.Fatal(err)
		}
		if !util.Sha1Match(account("owner").LoginVerifier(), "owner"+original) {
			t.Fatal("export lost the legacy login verifier")
		}
	}
	credential := util.Sha1String("new-import")
	var wg sync.WaitGroup
	outcomes := make(chan error, 2)
	for _, name := range []string{"concurrent-1", "concurrent-2"} {
		wg.Add(1)
		go func(n string) {
			defer wg.Done()
			outcomes <- dao.CreateOrUpdateAccount(model.Account{Username: &n, Pass: &credential}, 0)
		}(name)
	}
	wg.Wait()
	close(outcomes)
	success := 0
	for err := range outcomes {
		if err == nil {
			success++
		}
	}
	if success != 1 {
		t.Fatalf("simultaneous imports accepted %d owners", success)
	}
}

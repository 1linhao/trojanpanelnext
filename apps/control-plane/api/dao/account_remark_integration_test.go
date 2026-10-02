package dao

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"trojan-panel/model"
)

func TestAccountRemarkMigrationAndDAO(t *testing.T) {
	dsn := os.Getenv("TP_REMOVAL_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_REMOVAL_TEST_DSN is not set")
	}
	testDB := newNodeRemovalTestDB(t, dsn)
	start := strings.Index(sqlInitStr, "CREATE TABLE `account` (")
	if start < 0 {
		t.Fatal("bootstrap account table is missing")
	}
	end := strings.Index(sqlInitStr[start:], ";")
	if end < 0 {
		t.Fatal("bootstrap account table is incomplete")
	}
	// The original account schema and positional bootstrap INSERT remain
	// compatible; startup adds notes after the original import has completed.
	removalExec(t, testDB, strings.Replace(sqlInitStr[start:start+end], "CHARSET=utf8mb4", "CHARSET=utf8", 1))
	removalExec(t, testDB, `INSERT INTO account (id,username,pass,hash,role_id,email,quota) VALUES (2,'fixtureuser','fixture-pass','fixture-hash',3,'',-1)`)
	for i := 0; i < 2; i++ {
		if err := migrateAccountRemarkColumn(); err != nil {
			t.Fatal(err)
		}
	}
	assertRemovalRows(t, testDB, `SELECT id,username,remark FROM account`, [][]string{{"2", "fixtureuser", ""}})
	id := uint(2)
	note := strings.Repeat("🙂", 500)
	if err := UpdateAccountById(&model.Account{Id: &id, Remark: &note}); err != nil {
		t.Fatal(err)
	}
	if err := migrateAccountRemarkColumn(); err != nil {
		t.Fatal(err)
	}
	account, err := SelectAccountById(&id, true)
	if err != nil || account.Remark == nil || *account.Remark != note {
		t.Fatalf("Unicode note lost: %+v %v", account, err)
	}
	privateJSON, err := json.Marshal(account)
	if err != nil || strings.Contains(string(privateJSON), "remark") || strings.Contains(string(privateJSON), "🙂") {
		t.Fatalf("persistence model exposes note: %s %v", privateJSON, err)
	}
	ordinary, err := SelectAccountById(&id)
	if err != nil || ordinary.Remark != nil {
		t.Fatalf("ordinary DAO detail reads note: %+v %v", ordinary, err)
	}
	pageNumber, pageSize := uint(1), uint(20)
	for _, include := range []bool{false, true} {
		page, err := SelectAccountPage(nil, nil, nil, nil, nil, &pageNumber, &pageSize, include)
		if err != nil || len(page.AccountVos) != 1 {
			t.Fatalf("page=%+v err=%v", page, err)
		}
		if include {
			if page.AccountVos[0].Remark == nil || *page.AccountVos[0].Remark != note {
				t.Fatal("explicit management projection omitted note")
			}
		} else if page.AccountVos[0].Remark != nil {
			t.Fatal("ordinary page includes private note")
		}
	}
	email := "fixture@qq.com"
	if err := UpdateAccountById(&model.Account{Id: &id, Email: &email}); err != nil {
		t.Fatal(err)
	}
	account, err = SelectAccountById(&id, true)
	if err != nil || account.Remark == nil || *account.Remark != note {
		t.Fatal("omitted remark update changed existing note")
	}
	backups, err := SelectAccountAll()
	if err != nil {
		t.Fatal(err)
	}
	backupJSON, err := json.Marshal(backups)
	if err != nil || strings.Contains(string(backupJSON), "remark") || strings.Contains(string(backupJSON), "🙂") {
		t.Fatalf("backup exposed private note: %s %v", backupJSON, err)
	}
	unused, err := SelectAccountUnused()
	if err != nil {
		t.Fatal(err)
	}
	unusedJSON, err := json.Marshal(unused)
	if err != nil || strings.Contains(string(unusedJSON), "remark") || strings.Contains(string(unusedJSON), "🙂") {
		t.Fatal("unused account export exposed note")
	}
	var imported model.Account
	if err := json.Unmarshal([]byte(`{"username":"fixtureuser","email":"new@qq.com","remark":"injected"}`), &imported); err != nil {
		t.Fatal(err)
	}
	if imported.Remark != nil {
		t.Fatal("generic account import accepts private note")
	}
	if err := CreateOrUpdateAccount(imported, 1); err != nil {
		t.Fatal(err)
	}
	account, err = SelectAccountById(&id, true)
	if err != nil || account.Remark == nil || *account.Remark != note {
		t.Fatal("backup overwrite changed existing note")
	}
	empty := ""
	if err := UpdateAccountById(&model.Account{Id: &id, Remark: &empty}); err != nil {
		t.Fatal(err)
	}
	account, err = SelectAccountById(&id, true)
	if err != nil || account.Remark == nil || *account.Remark != "" {
		t.Fatal("explicit empty remark did not clear note")
	}
}

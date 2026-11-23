package api

import (
	"encoding/base64"
	"os"
	"strings"
	"testing"

	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/service"
)

// Run against an exclusive disposable MariaDB. The URI subscription and
// single-node share paths used here do not need a running Redis server.
func TestSameNodeAccountCredentials(t *testing.T) {
	dsn := os.Getenv("TP_CREDENTIAL_TEST_DSN")
	if dsn == "" {
		t.Skip("requires a disposable MariaDB instance")
	}
	db := disposableAPIStore(t, dsn, "127.0.0.1:1")
	first, second := "fixture-first", "fixture-second"
	oldPassword, newPassword := "test-initial-password", "test-updated-password"
	for _, username := range []string{first, second} {
		if err := dao.CreateAccount(&model.Account{Username: &username, Pass: &oldPassword}); err != nil {
			t.Fatal(err)
		}
	}
	for _, query := range []string{
		"INSERT INTO node_server (id,name) VALUES (7,'fixture')",
		"INSERT INTO node_xray (id,protocol,settings) VALUES (21,'socks','{\"accounts\":[{\"user\":\"fixture\",\"pass\":\"fixture-shared\"}]}'),(22,'vless','{}'),(23,'trojan','{}')",
		"INSERT INTO node (id,node_server_id,node_type_id,node_sub_id,name,domain,port) VALUES (21,7,1,21,'socks','localhost',2021),(22,7,1,22,'vless','localhost',2022),(23,7,1,23,'trojan','localhost',2023)",
	} {
		if _, err := db.Exec(query); err != nil {
			t.Fatal(err)
		}
	}
	account := func(name string) *model.Account {
		row, err := dao.SelectAccountByUsername(&name)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	link := func(a *model.Account, node uint) string {
		value, _, err := service.NodeURL(a.Id, a.Username, &node)
		if err != nil || value == "" {
			t.Fatal("share link unavailable", err)
		}
		return value
	}
	a, b := account(first), account(second)
	nodes := []uint{21, 22, 23}
	before := make(map[uint]string)
	for _, node := range nodes {
		before[node] = link(b, node)
		if link(a, node) == before[node] {
			t.Errorf("distinct accounts received identical connection credentials for fixture node %d", node)
		}
	}
	subscription := func(a *model.Account) string {
		t.Helper()
		row, _, encoded, err := service.SubscribeV2Ray(*a.Pass, "")
		if err != nil || *row.Id != *a.Id {
			t.Fatal("subscription token no longer identifies its owner", err)
		}
		content, err := base64.StdEncoding.DecodeString(string(encoded))
		if err != nil || len(strings.Split(string(content), "\n")) != len(nodes) {
			t.Fatal("subscription omitted fixture nodes", err)
		}
		return string(content)
	}
	originalB := subscription(b)
	if subscription(a) == originalB {
		t.Error("entire subscription was identical for both accounts")
	}
	checkSecond := func() {
		t.Helper()
		if subscription(b) != originalB {
			t.Error("old subscription for second account changed")
		}
		for _, node := range nodes {
			if link(b, node) != before[node] {
				t.Errorf("second account's connection changed for fixture node %d", node)
			}
		}
	}
	if err := dao.UpdateAccountPass(a, newPassword); err != nil {
		t.Fatal(err)
	}
	checkSecond()
	renamed, email := "fixture-renamed", "renamed@example.invalid"
	if err := dao.UpdateAccountProperty(account(first), &newPassword, &renamed, &email); err != nil {
		t.Fatal(err)
	}
	checkSecond()
}

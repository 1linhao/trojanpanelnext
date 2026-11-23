package api

import (
	"os"
	"reflect"
	"testing"
	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
	"trojan-panel/service"
)

func TestResetAllAccountTraffic(t *testing.T) {
	dsn, redisAddress := os.Getenv("TP_ACCOUNT_REMARK_TEST_DSN"), os.Getenv("TP_ACCOUNT_REMARK_TEST_REDIS")
	if dsn == "" || redisAddress == "" {
		t.Skip("requires a disposable database and Redis for account API tests")
	}
	db := disposableAPIStore(t, dsn, redisAddress)
	exec := func(statement string) {
		t.Helper()
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	exec(`UPDATE account SET download=10,upload=20 WHERE id=1`)
	exec(`INSERT INTO account (id,username,role_id,quota,download,upload,deleted) VALUES
		(2,'fixtureadmin',2,-1,30,40,0),
		(3,'fixtureuser',3,1000,50,60,1),
		(4,'fixtureunused',3,0,70,80,0)`)
	exec(`INSERT INTO account_traffic_total (account_id,upload,download) VALUES(3,160,150)`)
	exec(`INSERT INTO account_traffic_daily (traffic_date,account_id,upload,download) VALUES(CURRENT_DATE(),3,60,50)`)
	exec(`INSERT INTO account_server_traffic_daily (traffic_date,account_id,node_server_id,upload,download) VALUES(CURRENT_DATE(),3,9,60,50)`)

	token := func(id uint, roles ...string) string {
		t.Helper()
		value, err := service.GenToken(vo.AccountVo{Id: id, RoleId: id, Roles: roles})
		if err != nil {
			t.Fatal(err)
		}
		return value
	}
	adminToken := token(1, "sysadmin", "admin", "user")
	callReset := func(token, body string) map[string]interface{} {
		t.Helper()
		return remarkResponse(t, remarkRequest(t, ResetAllAccountDownloadAndUpload, "POST",
			"/api/account/resetAllAccountDownloadAndUpload", body, token))
	}
	assertAccounts := func(want [][5]uint) {
		t.Helper()
		rows, err := db.Query(`SELECT id,role_id,deleted,upload,download FROM account ORDER BY id`)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var got [][5]uint
		for rows.Next() {
			var row [5]uint
			if err = rows.Scan(&row[0], &row[1], &row[2], &row[3], &row[4]); err != nil {
				t.Fatal(err)
			}
			got = append(got, row)
		}
		if err = rows.Err(); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("accounts = %v, want %v", got, want)
		}
	}
	const confirmed = `{"confirm":true}`
	for _, body := range []string{"", "{}", `{"confirm":false}`, `{"confirm":true,"id":3}`, confirmed + "{}"} {
		if response := callReset(adminToken, body); response["message"] != constant.ValidateFailed {
			t.Fatalf("accepted invalid confirmation %q: %v", body, response)
		}
	}
	for _, unauthorized := range []string{token(2, "admin", "user"), token(3, "user")} {
		if response := callReset(unauthorized, confirmed); response["message"] != constant.ForbiddenError {
			t.Fatalf("accepted non-sysadmin: %v", response)
		}
	}
	exec(`UPDATE account SET role_id=2 WHERE id=1`)
	if response := callReset(adminToken, confirmed); response["message"] != constant.ForbiddenError {
		t.Fatalf("accepted an old sysadmin token after demotion: %v", response)
	}
	exec(`UPDATE account SET role_id=1 WHERE id=1`)
	assertAccounts([][5]uint{{1, 1, 0, 20, 10}, {2, 2, 0, 40, 30}, {3, 3, 1, 60, 50}, {4, 3, 0, 80, 70}})

	for attempt := 0; attempt < 2; attempt++ {
		if response := callReset(adminToken, confirmed); response["type"] != "success" {
			t.Fatalf("reset %d failed: %v", attempt, response)
		}
		assertAccounts([][5]uint{{1, 1, 0, 0, 0}, {2, 2, 0, 0, 0}, {3, 3, 1, 0, 0}, {4, 3, 0, 80, 70}})
	}
	for _, ledger := range []struct {
		table string
		usage [2]uint
	}{
		{"account_traffic_total", [2]uint{160, 150}},
		{"account_traffic_daily", [2]uint{60, 50}},
		{"account_server_traffic_daily", [2]uint{60, 50}},
	} {
		var got [2]uint
		if err := db.QueryRow("SELECT upload,download FROM "+ledger.table+" WHERE account_id=3").Scan(&got[0], &got[1]); err != nil || got != ledger.usage {
			t.Fatalf("%s changed: usage=%v err=%v", ledger.table, got, err)
		}
	}
}

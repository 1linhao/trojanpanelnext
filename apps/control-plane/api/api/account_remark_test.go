package api

import (
	"encoding/json"
	"strings"
	"testing"
	"trojan-panel/model/dto"
)

const accountRemarkUpdateJSON = `{"id":2,"username":"fixtureuser","quota":-1,"roleId":3,"deleted":0,"expireTime":4078656000000,"remark":`

func TestAccountRemarkUnicodeLimit(t *testing.T) {
	InitValidator()
	for _, tc := range []struct {
		name, value string
		valid       bool
	}{
		{"empty", "", true},
		{"500_unicode", strings.Repeat("备", 500), true},
		{"501_unicode", strings.Repeat("备", 501), false},
		{"500_emoji", strings.Repeat("🙂", 500), true},
		{"501_emoji", strings.Repeat("🙂", 501), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, _ := json.Marshal(tc.value)
			var update dto.AccountUpdateDto
			if err := json.Unmarshal([]byte(accountRemarkUpdateJSON+string(encoded)+"}"), &update); err != nil {
				t.Fatal(err)
			}
			err := validate.Struct(update)
			if (err == nil) != tc.valid {
				t.Fatalf("remark valid=%v, want %v: %v", err == nil, tc.valid, err)
			}
		})
	}
}

package core

import "testing"

func TestValidateNodeIdentityConfigRejectsSharedIdentities(t *testing.T) {
	valid := &AppConfig{
		MySQLConfig: MySQLConfig{User: "tpn_node"},
		RedisConfig: RedisConfig{Username: "tpn-cache", AuthUsername: "tpn-auth"},
		NodeConfig:  NodeConfig{ServerID: 1},
	}
	if err := ValidateNodeIdentityConfig(valid); err != nil {
		t.Fatalf("valid independent identity rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*AppConfig)
	}{
		{"root MariaDB", func(config *AppConfig) { config.MySQLConfig.User = "root" }},
		{"default Redis cache", func(config *AppConfig) { config.RedisConfig.Username = "default" }},
		{"default Redis auth", func(config *AppConfig) { config.RedisConfig.AuthUsername = "default" }},
		{"shared Redis users", func(config *AppConfig) { config.RedisConfig.AuthUsername = config.RedisConfig.Username }},
		{"zero server id", func(config *AppConfig) { config.NodeConfig.ServerID = 0 }},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			copy := *valid
			test.mutate(&copy)
			if err := ValidateNodeIdentityConfig(&copy); err == nil {
				t.Fatal("unsafe shared identity was accepted")
			}
		})
	}
}

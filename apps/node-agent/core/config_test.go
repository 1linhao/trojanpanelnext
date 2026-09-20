package core

import "testing"

func TestValidateNodeIdentityConfigRejectsSharedIdentities(t *testing.T) {
	valid := &AppConfig{
		MySQLConfig: MySQLConfig{User: "tpn_node", Password: "mariadb-secret"},
		RedisConfig: RedisConfig{
			Username: "tpn-cache", Password: "cache-secret",
			AuthUsername: "tpn-auth", AuthPassword: "auth-secret",
		},
		NodeConfig: NodeConfig{
			ServerID: 1, IdentityID: "11111111-2222-4333-8444-555555555555", IdentityGeneration: 7,
		},
	}
	if err := ValidateNodeIdentityConfig(valid); err != nil {
		t.Fatalf("valid independent identity rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*AppConfig)
	}{
		{"root MariaDB", func(config *AppConfig) { config.MySQLConfig.User = "root" }},
		{"empty MariaDB password", func(config *AppConfig) { config.MySQLConfig.Password = "" }},
		{"default Redis cache", func(config *AppConfig) { config.RedisConfig.Username = "default" }},
		{"empty Redis cache password", func(config *AppConfig) { config.RedisConfig.Password = "" }},
		{"default Redis auth", func(config *AppConfig) { config.RedisConfig.AuthUsername = "default" }},
		{"empty Redis auth password", func(config *AppConfig) { config.RedisConfig.AuthPassword = "" }},
		{"shared Redis users", func(config *AppConfig) { config.RedisConfig.AuthUsername = config.RedisConfig.Username }},
		{"zero server id", func(config *AppConfig) { config.NodeConfig.ServerID = 0 }},
		{"missing identity id", func(config *AppConfig) { config.NodeConfig.IdentityID = "" }},
		{"malformed identity id", func(config *AppConfig) { config.NodeConfig.IdentityID = "not-a-uuid" }},
		{"zero identity generation", func(config *AppConfig) { config.NodeConfig.IdentityGeneration = 0 }},
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

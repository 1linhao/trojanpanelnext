package redis_test

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"trojan-panel-core/core"
	noderedis "trojan-panel-core/dao/redis"
)

func TestNodeAgentAuthenticatesWithRedisACLIdentity(t *testing.T) {
	address := os.Getenv("TP_REDIS_ACL_INTEGRATION_ADDRESS")
	username := os.Getenv("TP_REDIS_ACL_INTEGRATION_USERNAME")
	password := os.Getenv("TP_REDIS_ACL_INTEGRATION_PASSWORD")
	authUsername := os.Getenv("TP_REDIS_AUTH_INTEGRATION_USERNAME")
	authPassword := os.Getenv("TP_REDIS_AUTH_INTEGRATION_PASSWORD")
	if address == "" || username == "" || password == "" || authUsername == "" || authPassword == "" {
		t.Skip("Redis ACL integration environment is not configured")
	}
	host, portText, found := strings.Cut(address, ":")
	if !found {
		t.Fatalf("invalid Redis address %q", address)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	core.Config.RedisConfig.Host = host
	core.Config.RedisConfig.Port = port
	core.Config.RedisConfig.Username = username
	core.Config.RedisConfig.Password = password
	core.Config.RedisConfig.AuthUsername = authUsername
	core.Config.RedisConfig.AuthPassword = authPassword
	core.Config.RedisConfig.Db = 0
	core.Config.RedisConfig.MaxIdle = 1
	core.Config.RedisConfig.MaxActive = 1
	core.Config.RedisConfig.Wait = true

	noderedis.InitRedis()
	t.Cleanup(noderedis.CloseRedis)
	value, err := noderedis.AuthClient.String.Get("trojan-panel:jwt-key").String()
	if err != nil {
		t.Fatalf("Node Agent Redis ACL read failed: %v", err)
	}
	if value != "integration-jwt-key" {
		t.Fatalf("JWT key = %q, want integration-jwt-key", value)
	}
}

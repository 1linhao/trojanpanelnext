// Package healthcheck verifies the external data-service boundaries with the
// same dedicated identities consumed by the Node Agent runtime.
package healthcheck

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/go-sql-driver/mysql"
	redigo "github.com/gomodule/redigo/redis"
	"trojan-panel-core/core"
)

const (
	credentialVerificationTimeout    = 8 * time.Second
	DefaultCredentialRecheckInterval = 2 * time.Second
	CredentialInvalidationDeadline   = DefaultCredentialRecheckInterval + credentialVerificationTimeout
)

func Verify(mode string) error {
	ctx, cancel := context.WithTimeout(context.Background(), credentialVerificationTimeout)
	defer cancel()
	switch mode {
	case "mariadb":
		return verifyMariaDB(ctx)
	case "redis":
		return verifyRedis(ctx)
	case "all", "":
		if err := verifyMariaDB(ctx); err != nil {
			return fmt.Errorf("MariaDB Node identity check failed: %w", err)
		}
		if err := verifyRedis(ctx); err != nil {
			return fmt.Errorf("Redis Node identity check failed: %w", err)
		}
		return nil
	default:
		return errors.New("unsupported Node data-service health mode")
	}
}

// Watch blocks until the process context ends or a fresh authentication with
// the dedicated data-service identities fails. It deliberately opens new
// connections so credential rotation/revocation cannot be hidden by an
// already-authenticated pool connection.
func Watch(ctx context.Context) error {
	return watch(ctx, DefaultCredentialRecheckInterval, func() error { return Verify("all") })
}

func watch(ctx context.Context, interval time.Duration, verify func() error) error {
	if interval <= 0 {
		return errors.New("Node credential recheck interval must be positive")
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if err := verify(); err != nil {
				return errors.New("Node data-service identity is no longer valid")
			}
		}
	}
}

func verifyMariaDB(ctx context.Context) error {
	config := core.Config.MySQLConfig
	databaseConfig := mysql.Config{
		User: config.User, Passwd: config.Password, Net: "tcp",
		Addr: fmt.Sprintf("%s:%d", config.Host, config.Port), DBName: config.Database,
		AllowNativePasswords: true, ParseTime: true, Timeout: 3 * time.Second,
	}
	dsn := databaseConfig.FormatDSN()
	database, err := sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	defer database.Close()
	if err = database.PingContext(ctx); err != nil {
		return err
	}
	var count int
	return database.QueryRowContext(ctx, "SELECT COUNT(1) FROM node_server WHERE id=?", core.Config.NodeConfig.ServerID).Scan(&count)
}

func verifyRedis(ctx context.Context) error {
	config := core.Config.RedisConfig
	address := fmt.Sprintf("%s:%d", config.Host, config.Port)
	cache, err := dialRedis(address, config.Username, config.Password)
	if err != nil {
		return err
	}
	defer cache.Close()
	if pong, pingErr := redigo.String(cache.Do("PING")); pingErr != nil || pong != "PONG" {
		return errors.New("cache ACL identity did not answer PING")
	}
	probeKey := "trojan-panel-core:bootstrap-probe:" + core.Config.NodeConfig.IdentityID
	if _, err = cache.Do("SET", probeKey, "ok", "PX", 10000); err != nil {
		return err
	}
	defer cache.Do("DEL", probeKey)
	if value, getErr := redigo.String(cache.Do("GET", probeKey)); getErr != nil || value != "ok" {
		return errors.New("cache ACL identity could not round-trip its key namespace")
	}
	auth, err := dialRedis(address, config.AuthUsername, config.AuthPassword)
	if err != nil {
		return err
	}
	defer auth.Close()
	if pong, pingErr := redigo.String(auth.Do("PING")); pingErr != nil || pong != "PONG" {
		return errors.New("auth ACL identity did not answer PING")
	}
	_, err = auth.Do("GET", "trojan-panel:jwt-key")
	return err
}

func dialRedis(address, username, password string) (redigo.Conn, error) {
	return redigo.Dial("tcp", address,
		redigo.DialUsername(username), redigo.DialPassword(password),
		redigo.DialDatabase(core.Config.RedisConfig.Db),
		redigo.DialConnectTimeout(3*time.Second), redigo.DialReadTimeout(3*time.Second),
		redigo.DialWriteTimeout(3*time.Second))
}

package nodeidentity

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	redigo "github.com/gomodule/redigo/redis"
	"trojan-panel/core"
	"trojan-panel/dao"
)

const databaseName = "trojan_panel_db"

var (
	namePattern      = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._ -]{0,63}$`)
	domainPattern    = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)$`)
	redisKeyPatterns = []string{"trojan-panel-core:*", "trojan-panel:jwt-key", "trojan-panel:token:*"}
)

type credentialFile struct {
	SchemaVersion  int                `json:"schema_version"`
	NodeIdentityID string             `json:"node_identity_id"`
	NodeServerID   uint64             `json:"node_server_id"`
	NodeName       string             `json:"node_name"`
	NodeDomain     string             `json:"node_domain"`
	PublicIP       string             `json:"public_ip"`
	Generation     uint64             `json:"generation"`
	MariaDB        databaseCredential `json:"mariadb"`
	Redis          redisCredential    `json:"redis"`
}

type databaseCredential struct {
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type redisCredential struct {
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	KeyPatterns []string `json:"key_patterns"`
}

type identity struct {
	ID              string
	NodeServerID    uint64
	Name            string
	Domain          string
	PublicIP        string
	Generation      uint64
	MariaDBUsername string
	RedisUsername   string
	CredentialPath  string
	Status          string
}

type lifecycle struct {
	db          *sql.DB
	redis       redigo.Conn
	mysqlConfig mysql.Config
}

func Run(args []string, stdout io.Writer, stderr io.Writer) int {
	commandArgs := commandArguments(args)
	if len(commandArgs) == 0 || commandArgs[0] == "--help" || commandArgs[0] == "-h" {
		writeUsage(stdout)
		return 0
	}

	switch commandArgs[0] {
	case "register":
		return runRegister(commandArgs[1:], stdout, stderr)
	case "rotate":
		return runRotate(commandArgs[1:], stdout, stderr)
	case "revoke":
		return runDeactivate(commandArgs[1:], false, stdout, stderr)
	case "force-evict":
		return runDeactivate(commandArgs[1:], true, stdout, stderr)
	case "status":
		return runStatus(commandArgs[1:], stdout, stderr)
	default:
		fmt.Fprintln(stderr, "node identity: unsupported command")
		return 2
	}
}

func parseIdentityID(args []string, command string, stderr io.Writer) (string, bool) {
	set := flag.NewFlagSet("node-identity "+command, flag.ContinueOnError)
	set.SetOutput(stderr)
	var id string
	set.StringVar(&id, "id", "", "stable Node identity id")
	if err := set.Parse(args); err != nil {
		return "", false
	}
	if !isUUID(id) || len(set.Args()) != 0 {
		fmt.Fprintf(stderr, "node identity: %s requires a UUID --id\n", command)
		return "", false
	}
	return id, true
}

func runStatus(args []string, stdout io.Writer, stderr io.Writer) int {
	id, valid := parseIdentityID(args, "status", stderr)
	if !valid {
		return 2
	}
	manager, err := openLifecycle()
	if err != nil {
		fmt.Fprintln(stderr, "node identity: control-plane data services are unavailable")
		return 1
	}
	defer manager.close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err = manager.ensureSchema(ctx); err != nil {
		fmt.Fprintln(stderr, "node identity: status is unavailable")
		return 1
	}
	registered, err := manager.identityByID(ctx, id)
	if err != nil {
		fmt.Fprintln(stderr, "node identity: identity not found")
		return 1
	}
	result := struct {
		NodeIdentityID string `json:"node_identity_id"`
		NodeServerID   uint64 `json:"node_server_id"`
		Name           string `json:"name"`
		Domain         string `json:"domain"`
		PublicIP       string `json:"public_ip"`
		Generation     uint64 `json:"generation"`
		Status         string `json:"status"`
	}{registered.ID, registered.NodeServerID, registered.Name, registered.Domain, registered.PublicIP, registered.Generation, registered.Status}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(result); err != nil {
		fmt.Fprintln(stderr, "node identity: could not write status")
		return 1
	}
	return 0
}

func runDeactivate(args []string, evict bool, stdout io.Writer, stderr io.Writer) int {
	command := "revoke"
	if evict {
		command = "force-evict"
	}
	id, valid := parseIdentityID(args, command, stderr)
	if !valid {
		return 2
	}
	manager, err := openLifecycle()
	if err != nil {
		fmt.Fprintln(stderr, "node identity: control-plane data services are unavailable")
		return 1
	}
	defer manager.close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	deactivated, err := manager.deactivate(ctx, id, evict)
	if err != nil {
		fmt.Fprintf(stderr, "node identity: %s failed; retry the same command\n", command)
		return 1
	}
	fmt.Fprintf(stdout, "Node identity %s: %s\n", deactivated.Status, deactivated.ID)
	return 0
}

func runRotate(args []string, stdout io.Writer, stderr io.Writer) int {
	set := flag.NewFlagSet("node-identity rotate", flag.ContinueOnError)
	set.SetOutput(stderr)
	var id, credentialPath string
	set.StringVar(&id, "id", "", "stable Node identity id")
	set.StringVar(&credentialPath, "credential-file", "", "restricted credential output file")
	if err := set.Parse(args); err != nil {
		return 2
	}
	if !isUUID(id) || credentialPath == "" || !filepath.IsAbs(credentialPath) || len(set.Args()) != 0 {
		fmt.Fprintln(stderr, "node identity: rotate requires a UUID --id and absolute --credential-file")
		return 2
	}
	manager, err := openLifecycle()
	if err != nil {
		fmt.Fprintln(stderr, "node identity: control-plane data services are unavailable")
		return 1
	}
	defer manager.close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	rotated, err := manager.rotate(ctx, id, credentialPath)
	if err != nil {
		fmt.Fprintln(stderr, "node identity: rotation failed; retry with the same credential file")
		return 1
	}
	fmt.Fprintf(stdout, "Node identity rotated: %s (generation %d)\nCredentials written to: %s\n", rotated.ID, rotated.Generation, credentialPath)
	return 0
}

func runRegister(args []string, stdout io.Writer, stderr io.Writer) int {
	set := flag.NewFlagSet("node-identity register", flag.ContinueOnError)
	set.SetOutput(stderr)
	var name, domain, publicIP, credentialPath string
	set.StringVar(&name, "name", "", "stable Node name")
	set.StringVar(&domain, "domain", "", "Node domain")
	set.StringVar(&publicIP, "public-ip", "", "Node public IP")
	set.StringVar(&credentialPath, "credential-file", "", "restricted credential output file")
	if err := set.Parse(args); err != nil {
		return 2
	}
	if err := validateRegisterInput(name, domain, publicIP, credentialPath, set.Args()); err != nil {
		fmt.Fprintf(stderr, "node identity: %v\n", err)
		return 2
	}

	manager, err := openLifecycle()
	if err != nil {
		fmt.Fprintln(stderr, "node identity: control-plane data services are unavailable")
		return 1
	}
	defer manager.close()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	created, err := manager.register(ctx, name, strings.ToLower(domain), publicIP, credentialPath)
	if err != nil {
		fmt.Fprintln(stderr, "node identity: registration failed; no credential was written to the terminal")
		return 1
	}
	fmt.Fprintf(stdout, "Node identity registered: %s\nCredentials written to: %s\n", created.ID, credentialPath)
	return 0
}

func validateRegisterInput(name, domain, publicIP, credentialPath string, positional []string) error {
	if !namePattern.MatchString(name) {
		return errors.New("--name must be 1-64 ASCII letters, digits, spaces, dots, underscores, or hyphens")
	}
	if len(domain) > 253 || !domainPattern.MatchString(domain) {
		return errors.New("--domain must be a valid DNS hostname")
	}
	parsedIP := net.ParseIP(publicIP)
	if parsedIP == nil || !parsedIP.IsGlobalUnicast() || parsedIP.IsPrivate() {
		return errors.New("--public-ip must be a routable IP literal")
	}
	if credentialPath == "" || !filepath.IsAbs(credentialPath) {
		return errors.New("--credential-file must be an absolute path")
	}
	if strings.IndexFunc(credentialPath, func(character rune) bool { return character < 32 || character == 127 }) >= 0 {
		return errors.New("--credential-file must not contain control characters")
	}
	if len(positional) != 0 {
		return errors.New("unexpected positional arguments")
	}
	return nil
}

func openLifecycle() (*lifecycle, error) {
	core.InitConfig()
	mysqlConfig := mysql.Config{
		User:                 core.Config.MySQLConfig.User,
		Passwd:               core.Config.MySQLConfig.Password,
		Net:                  "tcp",
		Addr:                 fmt.Sprintf("%s:%d", core.Config.MySQLConfig.Host, core.Config.MySQLConfig.Port),
		DBName:               databaseName,
		AllowNativePasswords: true,
		InterpolateParams:    true,
		ParseTime:            true,
		Timeout:              3 * time.Second,
	}
	db, err := sql.Open("mysql", mysqlConfig.FormatDSN())
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	redisAddress := fmt.Sprintf("%s:%d", core.Config.RedisConfig.Host, core.Config.RedisConfig.Port)
	redisConn, err := redigo.Dial("tcp", redisAddress,
		redigo.DialPassword(core.Config.RedisConfig.Password),
		redigo.DialDatabase(core.Config.RedisConfig.Db),
		redigo.DialConnectTimeout(3*time.Second),
		redigo.DialReadTimeout(3*time.Second),
		redigo.DialWriteTimeout(3*time.Second),
	)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	return &lifecycle{db: db, redis: redisConn, mysqlConfig: mysqlConfig}, nil
}

func (manager *lifecycle) close() {
	_ = manager.redis.Close()
	_ = manager.db.Close()
}

func (manager *lifecycle) register(ctx context.Context, name, domain, publicIP, path string) (identity, error) {
	if err := validateCredentialPath(path); err != nil {
		return identity{}, err
	}
	if err := manager.ensureSchema(ctx); err != nil {
		return identity{}, err
	}
	registered, err := manager.reserveIdentity(ctx, name, domain, publicIP, filepath.Clean(path))
	if err != nil {
		return identity{}, err
	}
	credentials, err := credentialsForIdentity(path, registered, registered.Status == "provisioning")
	if err != nil {
		return identity{}, err
	}
	if registered.Status == "active" {
		if err = manager.verifyCredentials(ctx, credentials); err != nil {
			manager.recordEvent(ctx, registered.ID, registered.Generation, "register", "failed", "credential_replay_failed")
			return identity{}, err
		}
		manager.recordEvent(ctx, registered.ID, registered.Generation, "register", "succeeded", "")
		return registered, nil
	}
	if err = manager.provisionMariaDB(ctx, registered.MariaDBUsername, credentials.MariaDB.Password); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, "register", "failed", "mariadb_provision_failed")
		return identity{}, err
	}
	if err = manager.provisionRedis(registered.RedisUsername, credentials.Redis.Password); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, "register", "failed", "redis_provision_failed")
		return identity{}, err
	}
	if err = manager.verifyCredentials(ctx, credentials); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, "register", "failed", "credential_verification_failed")
		return identity{}, err
	}
	if _, err = manager.db.ExecContext(ctx, `UPDATE node_identity SET status='active', update_time=CURRENT_TIMESTAMP WHERE identity_id=? AND generation=?`, registered.ID, registered.Generation); err != nil {
		return identity{}, err
	}
	manager.recordEvent(ctx, registered.ID, registered.Generation, "register", "succeeded", "")
	registered.Status = "active"
	return registered, nil
}

func (manager *lifecycle) rotate(ctx context.Context, id, path string) (identity, error) {
	if err := manager.ensureSchema(ctx); err != nil {
		return identity{}, err
	}
	registered, err := manager.identityByID(ctx, id)
	if err != nil {
		return identity{}, err
	}
	switch registered.Status {
	case "active":
		registered.Generation++
		if _, err = credentialsForIdentity(path, registered, true); err != nil {
			return identity{}, err
		}
		result, updateErr := manager.db.ExecContext(ctx, `UPDATE node_identity
			SET generation=?,credential_path=?,status='rotating',update_time=CURRENT_TIMESTAMP
			WHERE identity_id=? AND generation=? AND status='active'`, registered.Generation, filepath.Clean(path), registered.ID, registered.Generation-1)
		if updateErr != nil {
			return identity{}, updateErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return identity{}, errors.New("Node identity changed during rotation")
		}
		registered.Status = "rotating"
		registered.CredentialPath = filepath.Clean(path)
	case "rotating":
		if filepath.Clean(path) != registered.CredentialPath {
			return identity{}, errors.New("an interrupted rotation must reuse its credential file")
		}
		if _, statErr := os.Stat(path); statErr != nil {
			return identity{}, errors.New("an interrupted rotation must reuse its credential file")
		}
	default:
		return identity{}, errors.New("only an active Node identity can be rotated")
	}
	credentials, err := credentialsForIdentity(path, registered, registered.Status == "rotating")
	if err != nil {
		return identity{}, err
	}
	if err = manager.provisionMariaDB(ctx, registered.MariaDBUsername, credentials.MariaDB.Password); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, "rotate", "failed", "mariadb_rotation_failed")
		return identity{}, err
	}
	if err = manager.provisionRedis(registered.RedisUsername, credentials.Redis.Password); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, "rotate", "failed", "redis_rotation_failed")
		return identity{}, err
	}
	if err = manager.verifyCredentials(ctx, credentials); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, "rotate", "failed", "credential_verification_failed")
		return identity{}, err
	}
	result, err := manager.db.ExecContext(ctx, `UPDATE node_identity SET status='active',update_time=CURRENT_TIMESTAMP
		WHERE identity_id=? AND generation=? AND status='rotating'`, registered.ID, registered.Generation)
	if err != nil {
		return identity{}, err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return identity{}, errors.New("Node identity rotation state changed")
	}
	manager.recordEvent(ctx, registered.ID, registered.Generation, "rotate", "succeeded", "")
	registered.Status = "active"
	return registered, nil
}

func (manager *lifecycle) deactivate(ctx context.Context, id string, evict bool) (identity, error) {
	if err := manager.ensureSchema(ctx); err != nil {
		return identity{}, err
	}
	registered, err := manager.identityByID(ctx, id)
	if err != nil {
		return identity{}, err
	}
	targetStatus := "revoked"
	inProgressStatus := "revoking"
	action := "revoke"
	if evict {
		targetStatus = "evicted"
		inProgressStatus = "force-evicting"
		action = "force-evict"
	}
	if registered.Status == targetStatus {
		return registered, nil
	}
	if registered.Status == "evicted" {
		return identity{}, errors.New("an evicted Node identity cannot be changed")
	}
	if registered.Status == "revoked" && !evict {
		return registered, nil
	}
	if registered.Status != inProgressStatus {
		if _, err = manager.db.ExecContext(ctx, `UPDATE node_identity SET status=?,update_time=CURRENT_TIMESTAMP WHERE identity_id=?`, inProgressStatus, id); err != nil {
			return identity{}, err
		}
		registered.Status = inProgressStatus
	}
	account := fmt.Sprintf("`%s`@'%%'", registered.MariaDBUsername)
	if _, err = manager.db.ExecContext(ctx, "DROP USER IF EXISTS "+account); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, action, "failed", "mariadb_revocation_failed")
		return identity{}, err
	}
	if _, err = manager.redis.Do("ACL", "DELUSER", registered.RedisUsername); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, action, "failed", "redis_revocation_failed")
		return identity{}, err
	}
	if _, err = manager.db.ExecContext(ctx, "DELETE FROM node_server WHERE id=?", registered.NodeServerID); err != nil {
		errorCode := "registration_revocation_failed"
		if evict {
			errorCode = "registration_eviction_failed"
		}
		manager.recordEvent(ctx, registered.ID, registered.Generation, action, "failed", errorCode)
		return identity{}, err
	}
	if _, err = manager.db.ExecContext(ctx, `UPDATE node_identity SET status=?,update_time=CURRENT_TIMESTAMP WHERE identity_id=?`, targetStatus, id); err != nil {
		return identity{}, err
	}
	manager.recordEvent(ctx, registered.ID, registered.Generation, action, "succeeded", "")
	registered.Status = targetStatus
	return registered, nil
}

func (manager *lifecycle) identityByID(ctx context.Context, id string) (identity, error) {
	var registered identity
	err := manager.db.QueryRowContext(ctx, `SELECT identity_id,node_server_id,name,domain,public_ip,generation,mariadb_username,redis_username,credential_path,status
		FROM node_identity WHERE identity_id=?`, id).Scan(
		&registered.ID, &registered.NodeServerID, &registered.Name, &registered.Domain, &registered.PublicIP,
		&registered.Generation, &registered.MariaDBUsername, &registered.RedisUsername, &registered.CredentialPath, &registered.Status,
	)
	return registered, err
}

func (manager *lifecycle) ensureSchema(ctx context.Context) error {
	return dao.EnsureNodeIdentitySchema(ctx, manager.db)
}

func (manager *lifecycle) reserveIdentity(ctx context.Context, name, domain, publicIP, credentialPath string) (identity, error) {
	var existing identity
	err := manager.db.QueryRowContext(ctx, `SELECT identity_id,node_server_id,name,domain,public_ip,generation,mariadb_username,redis_username,credential_path,status
		FROM node_identity WHERE name=? OR domain=?`, name, domain).Scan(
		&existing.ID, &existing.NodeServerID, &existing.Name, &existing.Domain, &existing.PublicIP,
		&existing.Generation, &existing.MariaDBUsername, &existing.RedisUsername, &existing.CredentialPath, &existing.Status,
	)
	if err == nil {
		if existing.Name != name || existing.Domain != domain || existing.PublicIP != publicIP ||
			existing.CredentialPath != credentialPath ||
			(existing.Status != "provisioning" && existing.Status != "active") {
			return identity{}, errors.New("Node identity conflicts with an existing registration")
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return identity{}, err
	}

	id, err := randomUUID()
	if err != nil {
		return identity{}, err
	}
	compactID := strings.ReplaceAll(id, "-", "")
	created := identity{
		ID:              id,
		Name:            name,
		Domain:          domain,
		PublicIP:        publicIP,
		Generation:      1,
		MariaDBUsername: "tpn_" + compactID[:20],
		RedisUsername:   "tpn-" + compactID[:20],
		CredentialPath:  credentialPath,
		Status:          "provisioning",
	}
	tx, err := manager.db.BeginTx(ctx, nil)
	if err != nil {
		return identity{}, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO node_server
		(ip,name,grpc_port,grpc_tls_mode,grpc_tls_server_name,traffic_period,traffic_limit_mode,traffic_total_limit,traffic_upload_limit,traffic_download_limit)
		VALUES (?,?,8100,'mtls',?,'none','combined',0,0,0)`, publicIP, name, domain)
	if err != nil {
		return identity{}, err
	}
	nodeServerID, err := result.LastInsertId()
	if err != nil {
		return identity{}, err
	}
	created.NodeServerID = uint64(nodeServerID)
	_, err = tx.ExecContext(ctx, `INSERT INTO node_identity
		(identity_id,node_server_id,name,domain,public_ip,generation,mariadb_username,redis_username,credential_path,status)
		VALUES (?,?,?,?,?,1,?,?,?,'provisioning')`, created.ID, created.NodeServerID, name, domain, publicIP, created.MariaDBUsername, created.RedisUsername, credentialPath)
	if err != nil {
		return identity{}, err
	}
	if err = tx.Commit(); err != nil {
		return identity{}, err
	}
	return created, nil
}

func credentialsForIdentity(path string, registered identity, allowCreate bool) (credentialFile, error) {
	if contents, err := readCredentialFile(path); err == nil {
		var existing credentialFile
		if json.Unmarshal(contents, &existing) != nil || existing.NodeIdentityID != registered.ID ||
			existing.SchemaVersion != 1 || existing.NodeServerID != registered.NodeServerID ||
			existing.NodeName != registered.Name || existing.NodeDomain != registered.Domain ||
			existing.PublicIP != registered.PublicIP || existing.Generation != registered.Generation ||
			existing.MariaDB.Database != databaseName || existing.MariaDB.Username != registered.MariaDBUsername ||
			existing.MariaDB.Password == "" || existing.Redis.Username != registered.RedisUsername ||
			existing.Redis.Password == "" || !equalStrings(existing.Redis.KeyPatterns, redisKeyPatterns) {
			return credentialFile{}, errors.New("existing credential file does not match the reserved Node identity")
		}
		return existing, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return credentialFile{}, err
	}
	if !allowCreate {
		return credentialFile{}, errors.New("the current credential file is required for an idempotent replay")
	}
	dbPassword, err := randomSecret()
	if err != nil {
		return credentialFile{}, err
	}
	redisPassword, err := randomSecret()
	if err != nil {
		return credentialFile{}, err
	}
	created := credentialFile{
		SchemaVersion:  1,
		NodeIdentityID: registered.ID,
		NodeServerID:   registered.NodeServerID,
		NodeName:       registered.Name,
		NodeDomain:     registered.Domain,
		PublicIP:       registered.PublicIP,
		Generation:     registered.Generation,
		MariaDB:        databaseCredential{Database: databaseName, Username: registered.MariaDBUsername, Password: dbPassword},
		Redis: redisCredential{
			Username:    registered.RedisUsername,
			Password:    redisPassword,
			KeyPatterns: append([]string(nil), redisKeyPatterns...),
		},
	}
	if err = writeCredentialFile(path, created); err != nil {
		return credentialFile{}, err
	}
	return created, nil
}

func equalStrings(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func writeCredentialFile(path string, credentials credentialFile) error {
	contents, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return err
	}
	return createCredentialFile(path, append(contents, '\n'))
}

func (manager *lifecycle) provisionMariaDB(ctx context.Context, username, password string) error {
	account := fmt.Sprintf("`%s`@'%%'", username)
	statements := []string{
		fmt.Sprintf("CREATE USER IF NOT EXISTS %s IDENTIFIED BY '%s'", account, password),
		fmt.Sprintf("ALTER USER %s IDENTIFIED BY '%s'", account, password),
		fmt.Sprintf("REVOKE ALL PRIVILEGES, GRANT OPTION FROM %s", account),
		fmt.Sprintf("GRANT SELECT ON `%s`.`account` TO %s", databaseName, account),
		fmt.Sprintf("GRANT UPDATE (`download`,`upload`) ON `%s`.`account` TO %s", databaseName, account),
		fmt.Sprintf("GRANT SELECT ON `%s`.`node_server` TO %s", databaseName, account),
		fmt.Sprintf("GRANT SELECT,INSERT,UPDATE ON `%s`.`account_traffic_total` TO %s", databaseName, account),
		fmt.Sprintf("GRANT SELECT,INSERT,UPDATE ON `%s`.`account_traffic_daily` TO %s", databaseName, account),
		fmt.Sprintf("GRANT SELECT,INSERT,UPDATE ON `%s`.`account_server_traffic_daily` TO %s", databaseName, account),
	}
	for _, statement := range statements {
		if _, err := manager.db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func (manager *lifecycle) provisionRedis(username, password string) error {
	_, err := manager.redis.Do("ACL", "SETUSER", username,
		"reset", "on", ">"+password,
		"~trojan-panel-core:*", "~trojan-panel:jwt-key", "~trojan-panel:token:*",
		"+ping", "+get", "+set", "+del", "+eval", "+evalsha", "+pttl",
	)
	return err
}

func (manager *lifecycle) verifyCredentials(ctx context.Context, credentials credentialFile) error {
	config := manager.mysqlConfig
	config.User = credentials.MariaDB.Username
	config.Passwd = credentials.MariaDB.Password
	testDB, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		return err
	}
	defer testDB.Close()
	if err = testDB.PingContext(ctx); err != nil {
		return err
	}
	var count int
	if err = testDB.QueryRowContext(ctx, "SELECT COUNT(1) FROM account").Scan(&count); err != nil {
		return err
	}
	redisAddress := fmt.Sprintf("%s:%d", core.Config.RedisConfig.Host, core.Config.RedisConfig.Port)
	conn, err := redigo.Dial("tcp", redisAddress,
		redigo.DialUsername(credentials.Redis.Username), redigo.DialPassword(credentials.Redis.Password),
		redigo.DialDatabase(core.Config.RedisConfig.Db), redigo.DialConnectTimeout(3*time.Second))
	if err != nil {
		return err
	}
	defer conn.Close()
	reply, err := redigo.String(conn.Do("PING"))
	if err != nil || reply != "PONG" {
		return errors.New("Redis credential verification failed")
	}
	return nil
}

func (manager *lifecycle) recordEvent(ctx context.Context, id string, generation uint64, action, result, errorCode string) {
	_, _ = manager.db.ExecContext(ctx, `INSERT INTO node_identity_event
		(identity_id,generation,action,result,error_code) VALUES (?,?,?,?,?)`, id, generation, action, result, errorCode)
}

func randomUUID() (string, error) {
	value := make([]byte, 16)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	value[6] = (value[6] & 0x0f) | 0x40
	value[8] = (value[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(value)
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

func randomSecret() (string, error) {
	value := make([]byte, 32)
	if _, err := rand.Read(value); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(value), nil
}

func isUUID(value string) bool {
	if len(value) != 36 || value[8] != '-' || value[13] != '-' || value[18] != '-' || value[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}

func commandArguments(args []string) []string {
	for index, argument := range args {
		if argument == "node-identity" {
			return args[index+1:]
		}
	}
	return nil
}

func writeUsage(output io.Writer) {
	fmt.Fprintln(output, `TrojanPanel Next Node identity lifecycle
Usage:
  trojan-panel node-identity register --name <name> --domain <domain> --public-ip <ip> --credential-file <0600-file>
  trojan-panel node-identity rotate --id <node-identity-id> --credential-file <0600-file>
  trojan-panel node-identity revoke --id <node-identity-id>
  trojan-panel node-identity force-evict --id <node-identity-id>
  trojan-panel node-identity status --id <node-identity-id>`)
}

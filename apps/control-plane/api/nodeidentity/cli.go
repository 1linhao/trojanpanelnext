package nodeidentity

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
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
	namePattern           = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._ -]{0,63}$`)
	domainPattern         = regexp.MustCompile(`^(?i:[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?)+)$`)
	redisCacheKeyPatterns = []string{"trojan-panel-core:*"}
	redisAuthKeyPatterns  = []string{"trojan-panel:jwt-key", "trojan-panel:token:*"}
)

type IdentityStatus string

const (
	statusProvisioning IdentityStatus = "provisioning"
	statusActive       IdentityStatus = "active"
	statusRotating     IdentityStatus = "rotating"
	statusRevoking     IdentityStatus = "revoking"
	statusRevoked      IdentityStatus = "revoked"
	statusEvicting     IdentityStatus = "force-evicting"
	statusEvicted      IdentityStatus = "evicted"
)

type LifecycleAction string

const (
	actionRegister   LifecycleAction = "register"
	actionRotate     LifecycleAction = "rotate"
	actionRevoke     LifecycleAction = "revoke"
	actionForceEvict LifecycleAction = "force-evict"
)

type EventResult string

const (
	resultSucceeded EventResult = "succeeded"
	resultFailed    EventResult = "failed"
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
	RedisAuth      redisCredential    `json:"redis_auth"`
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
	ID                string
	NodeServerID      uint64
	Name              string
	Domain            string
	PublicIP          string
	Generation        uint64
	MariaDBUsername   string
	RedisUsername     string
	RedisAuthUsername string
	CredentialPath    string
	CredentialSHA256  string
	Status            IdentityStatus
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
		return runRevoke(commandArgs[1:], stdout, stderr)
	case "force-evict":
		return runForceEvict(commandArgs[1:], stdout, stderr)
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
	}{registered.ID, registered.NodeServerID, registered.Name, registered.Domain, registered.PublicIP, registered.Generation, string(registered.Status)}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err = encoder.Encode(result); err != nil {
		fmt.Fprintln(stderr, "node identity: could not write status")
		return 1
	}
	return 0
}

func runRevoke(args []string, stdout io.Writer, stderr io.Writer) int {
	return runRemoval(args, actionRevoke, stdout, stderr)
}

func runForceEvict(args []string, stdout io.Writer, stderr io.Writer) int {
	return runRemoval(args, actionForceEvict, stdout, stderr)
}

func runRemoval(args []string, action LifecycleAction, stdout io.Writer, stderr io.Writer) int {
	command := string(action)
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
	var deactivated identity
	if action == actionRevoke {
		deactivated, err = manager.revoke(ctx, id)
	} else {
		deactivated, err = manager.forceEvict(ctx, id)
	}
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

func (manager *lifecycle) withLifecycleLock(ctx context.Context, name string) (func(), error) {
	connection, err := manager.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	var acquired sql.NullInt64
	if err = connection.QueryRowContext(ctx, "SELECT GET_LOCK(?, 10)", name).Scan(&acquired); err != nil || !acquired.Valid || acquired.Int64 != 1 {
		_ = connection.Close()
		if err != nil {
			return nil, err
		}
		return nil, errors.New("timed out waiting for the Node identity lifecycle lock")
	}
	return func() {
		releaseContext, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_, _ = connection.ExecContext(releaseContext, "SELECT RELEASE_LOCK(?)", name)
		_ = connection.Close()
	}, nil
}

func identityLockName(id string) string {
	return "tpn-node-identity:" + id
}

func registrationLockName(name, domain string) string {
	digest := sha256.Sum256([]byte(name + "\x00" + domain))
	return "tpn-node-register:" + hex.EncodeToString(digest[:20])
}

func (manager *lifecycle) register(ctx context.Context, name, domain, publicIP, path string) (identity, error) {
	if err := validateCredentialPath(path); err != nil {
		return identity{}, err
	}
	if err := manager.ensureSchema(ctx); err != nil {
		return identity{}, err
	}
	release, err := manager.withLifecycleLock(ctx, registrationLockName(name, domain))
	if err != nil {
		return identity{}, err
	}
	defer release()
	registered, created, err := manager.reserveIdentity(ctx, name, domain, publicIP, filepath.Clean(path))
	if err != nil {
		return identity{}, err
	}
	identityRelease, err := manager.withLifecycleLock(ctx, identityLockName(registered.ID))
	if err != nil {
		return identity{}, err
	}
	defer identityRelease()
	registered, err = manager.identityByID(ctx, registered.ID)
	if err != nil {
		return identity{}, err
	}
	credentials, digest, err := credentialsForIdentity(path, registered, created)
	if err != nil {
		if created {
			_ = manager.rollbackReservation(ctx, registered)
		}
		return identity{}, err
	}
	if registered.CredentialSHA256 == "" {
		result, updateErr := manager.db.ExecContext(ctx, `UPDATE node_identity SET credential_sha256=? WHERE identity_id=? AND generation=? AND status='provisioning' AND credential_sha256=''`, digest, registered.ID, registered.Generation)
		if updateErr != nil {
			return identity{}, updateErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return identity{}, errors.New("credential commitment changed during registration")
		}
		registered.CredentialSHA256 = digest
	}
	if registered.Status == statusActive {
		if err = manager.verifyCredentials(ctx, credentials); err != nil {
			manager.recordEvent(ctx, registered.ID, registered.Generation, actionRegister, resultFailed, "credential_replay_failed")
			return identity{}, err
		}
		manager.recordEvent(ctx, registered.ID, registered.Generation, actionRegister, resultSucceeded, "")
		return registered, nil
	}
	if err = manager.provisionMariaDB(ctx, registered.MariaDBUsername, credentials.MariaDB.Password); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, actionRegister, resultFailed, "mariadb_provision_failed")
		return identity{}, err
	}
	if err = manager.provisionRedis(credentials); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, actionRegister, resultFailed, "redis_provision_failed")
		return identity{}, err
	}
	if err := manager.verifyCredentials(ctx, credentials); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, actionRegister, resultFailed, "credential_verification_failed")
		return identity{}, err
	}
	result, err := manager.db.ExecContext(ctx, `UPDATE node_identity SET status='active', update_time=CURRENT_TIMESTAMP WHERE identity_id=? AND generation=? AND status='provisioning'`, registered.ID, registered.Generation)
	if err != nil {
		return identity{}, err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return identity{}, errors.New("Node identity state changed while registration completed")
	}
	manager.recordEvent(ctx, registered.ID, registered.Generation, actionRegister, resultSucceeded, "")
	registered.Status = statusActive
	return registered, nil
}

func (manager *lifecycle) rollbackReservation(ctx context.Context, registered identity) error {
	tx, err := manager.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM node_identity WHERE identity_id=? AND generation=1 AND status='provisioning' AND credential_sha256=''`, registered.ID)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return errors.New("Node identity reservation changed before rollback")
	}
	if _, err = tx.ExecContext(ctx, "DELETE FROM node_server WHERE id=?", registered.NodeServerID); err != nil {
		return err
	}
	return tx.Commit()
}

func (manager *lifecycle) rotate(ctx context.Context, id, path string) (identity, error) {
	if err := manager.ensureSchema(ctx); err != nil {
		return identity{}, err
	}
	release, err := manager.withLifecycleLock(ctx, identityLockName(id))
	if err != nil {
		return identity{}, err
	}
	defer release()
	registered, err := manager.identityByID(ctx, id)
	if err != nil {
		return identity{}, err
	}
	switch registered.Status {
	case statusActive:
		if err = manager.preflightRedis(); err != nil {
			return identity{}, err
		}
		registered.Generation++
		registered.CredentialSHA256 = ""
		credentials, digest, credentialErr := credentialsForIdentity(path, registered, true)
		if credentialErr != nil {
			return identity{}, credentialErr
		}
		result, updateErr := manager.db.ExecContext(ctx, `UPDATE node_identity
			SET generation=?,credential_path=?,credential_sha256=?,status='rotating',update_time=CURRENT_TIMESTAMP
			WHERE identity_id=? AND generation=? AND status='active'`, registered.Generation, filepath.Clean(path), digest, registered.ID, registered.Generation-1)
		if updateErr != nil {
			return identity{}, updateErr
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil || rows != 1 {
			return identity{}, errors.New("Node identity changed during rotation")
		}
		registered.Status = statusRotating
		registered.CredentialPath = filepath.Clean(path)
		registered.CredentialSHA256 = digest
		if err = manager.completeRotation(ctx, registered, credentials); err != nil {
			return identity{}, err
		}
		return registered, nil
	case statusRotating:
		if filepath.Clean(path) != registered.CredentialPath {
			return identity{}, errors.New("an interrupted rotation must reuse its credential file")
		}
	default:
		return identity{}, errors.New("only an active Node identity can be rotated")
	}
	credentials, _, err := credentialsForIdentity(path, registered, false)
	if err != nil {
		return identity{}, err
	}
	if err = manager.completeRotation(ctx, registered, credentials); err != nil {
		return identity{}, err
	}
	return registered, nil
}

func (manager *lifecycle) completeRotation(ctx context.Context, registered identity, credentials credentialFile) error {
	// Redis is rotated atomically first. If a later MariaDB or verification step
	// fails, the old Redis passwords are already unusable and the committed
	// credential file can be retried at the same generation.
	if err := manager.provisionRedis(credentials); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, actionRotate, resultFailed, "redis_rotation_failed")
		return err
	}
	if err := manager.provisionMariaDB(ctx, registered.MariaDBUsername, credentials.MariaDB.Password); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, actionRotate, resultFailed, "mariadb_rotation_failed")
		return err
	}
	if err := manager.verifyCredentials(ctx, credentials); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, actionRotate, resultFailed, "credential_verification_failed")
		return err
	}
	result, err := manager.db.ExecContext(ctx, `UPDATE node_identity SET status='active',update_time=CURRENT_TIMESTAMP
		WHERE identity_id=? AND generation=? AND status='rotating'`, registered.ID, registered.Generation)
	if err != nil {
		return err
	}
	rows, err := result.RowsAffected()
	if err != nil || rows != 1 {
		return errors.New("Node identity rotation state changed")
	}
	manager.recordEvent(ctx, registered.ID, registered.Generation, actionRotate, resultSucceeded, "")
	return nil
}

func (manager *lifecycle) revoke(ctx context.Context, id string) (identity, error) {
	return manager.removeIdentity(ctx, id, actionRevoke)
}

func (manager *lifecycle) forceEvict(ctx context.Context, id string) (identity, error) {
	return manager.removeIdentity(ctx, id, actionForceEvict)
}

func (manager *lifecycle) removeIdentity(ctx context.Context, id string, action LifecycleAction) (identity, error) {
	if err := manager.ensureSchema(ctx); err != nil {
		return identity{}, err
	}
	release, err := manager.withLifecycleLock(ctx, identityLockName(id))
	if err != nil {
		return identity{}, err
	}
	defer release()
	registered, err := manager.identityByID(ctx, id)
	if err != nil {
		return identity{}, err
	}
	targetStatus, inProgressStatus, err := removalTransition(registered.Status, action)
	if err != nil {
		return identity{}, err
	}
	if registered.Status == targetStatus {
		return registered, nil
	}
	if registered.Status != inProgressStatus {
		result, updateErr := manager.db.ExecContext(ctx, `UPDATE node_identity SET status=?,update_time=CURRENT_TIMESTAMP WHERE identity_id=? AND status=?`, inProgressStatus, id, registered.Status)
		if updateErr != nil {
			return identity{}, updateErr
		}
		if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
			return identity{}, errors.New("Node identity state changed during removal")
		}
		registered.Status = inProgressStatus
	}
	if err = manager.revokeCredentials(ctx, registered, action); err != nil {
		return identity{}, err
	}
	if action == actionForceEvict {
		if _, err = manager.db.ExecContext(ctx, "DELETE FROM node_server WHERE id=?", registered.NodeServerID); err != nil {
			manager.recordEvent(ctx, registered.ID, registered.Generation, action, resultFailed, "registration_eviction_failed")
			return identity{}, err
		}
	}
	result, err := manager.db.ExecContext(ctx, `UPDATE node_identity SET status=?,update_time=CURRENT_TIMESTAMP WHERE identity_id=? AND status=?`, targetStatus, id, inProgressStatus)
	if err != nil {
		return identity{}, err
	}
	if rows, rowsErr := result.RowsAffected(); rowsErr != nil || rows != 1 {
		return identity{}, errors.New("Node identity state changed while removal completed")
	}
	manager.recordEvent(ctx, registered.ID, registered.Generation, action, resultSucceeded, "")
	registered.Status = targetStatus
	return registered, nil
}

func removalTransition(status IdentityStatus, action LifecycleAction) (IdentityStatus, IdentityStatus, error) {
	if status == statusEvicted {
		if action == actionForceEvict {
			return statusEvicted, statusEvicting, nil
		}
		return "", "", errors.New("an evicted Node identity cannot be changed")
	}
	if action == actionRevoke {
		if status == statusRevoked {
			return statusRevoked, statusRevoking, nil
		}
		if status != statusActive && status != statusRotating && status != statusRevoking {
			return "", "", errors.New("Node identity cannot be revoked from its current state")
		}
		return statusRevoked, statusRevoking, nil
	}
	if status != statusActive && status != statusRotating && status != statusRevoked && status != statusRevoking && status != statusEvicting {
		return "", "", errors.New("Node identity cannot be force-evicted from its current state")
	}
	return statusEvicted, statusEvicting, nil
}

func (manager *lifecycle) revokeCredentials(ctx context.Context, registered identity, action LifecycleAction) error {
	account := fmt.Sprintf("`%s`@'%%'", registered.MariaDBUsername)
	if _, err := manager.db.ExecContext(ctx, "DROP USER IF EXISTS "+account); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, action, resultFailed, "mariadb_revocation_failed")
		return err
	}
	if err := manager.deleteRedisUsers(registered.RedisUsername, registered.RedisAuthUsername); err != nil {
		manager.recordEvent(ctx, registered.ID, registered.Generation, action, resultFailed, "redis_revocation_failed")
		return err
	}
	return nil
}

func (manager *lifecycle) identityByID(ctx context.Context, id string) (identity, error) {
	var registered identity
	err := scanIdentity(manager.db.QueryRowContext(ctx, identitySelect+` WHERE identity_id=?`, id), &registered)
	return registered, err
}

const identitySelect = `SELECT identity_id,node_server_id,name,domain,public_ip,generation,mariadb_username,redis_username,redis_auth_username,credential_path,credential_sha256,status FROM node_identity`

type rowScanner interface {
	Scan(...interface{}) error
}

func scanIdentity(row rowScanner, registered *identity) error {
	return row.Scan(
		&registered.ID, &registered.NodeServerID, &registered.Name, &registered.Domain, &registered.PublicIP,
		&registered.Generation, &registered.MariaDBUsername, &registered.RedisUsername, &registered.RedisAuthUsername,
		&registered.CredentialPath, &registered.CredentialSHA256, &registered.Status,
	)
}

func (manager *lifecycle) ensureSchema(ctx context.Context) error {
	return dao.EnsureNodeIdentitySchema(ctx, manager.db)
}

func (manager *lifecycle) reserveIdentity(ctx context.Context, name, domain, publicIP, credentialPath string) (identity, bool, error) {
	var existing identity
	err := scanIdentity(manager.db.QueryRowContext(ctx, identitySelect+` WHERE name=? OR domain=?`, name, domain), &existing)
	if err == nil {
		if existing.Name != name || existing.Domain != domain || existing.PublicIP != publicIP ||
			existing.CredentialPath != credentialPath ||
			(existing.Status != statusProvisioning && existing.Status != statusActive) {
			return identity{}, false, errors.New("Node identity conflicts with an existing registration")
		}
		return existing, false, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return identity{}, false, err
	}

	id, err := randomUUID()
	if err != nil {
		return identity{}, false, err
	}
	compactID := strings.ReplaceAll(id, "-", "")
	created := identity{
		ID:                id,
		Name:              name,
		Domain:            domain,
		PublicIP:          publicIP,
		Generation:        1,
		MariaDBUsername:   "tpn_" + compactID[:20],
		RedisUsername:     "tpn-" + compactID[:20],
		RedisAuthUsername: "tpn-auth-" + compactID[:20],
		CredentialPath:    credentialPath,
		Status:            statusProvisioning,
	}
	tx, err := manager.db.BeginTx(ctx, nil)
	if err != nil {
		return identity{}, false, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `INSERT INTO node_server
		(ip,name,grpc_port,grpc_tls_mode,grpc_tls_server_name,traffic_period,traffic_limit_mode,traffic_total_limit,traffic_upload_limit,traffic_download_limit)
		VALUES (?,?,8100,'mtls',?,'none','combined',0,0,0)`, publicIP, name, domain)
	if err != nil {
		return identity{}, false, err
	}
	nodeServerID, err := result.LastInsertId()
	if err != nil {
		return identity{}, false, err
	}
	created.NodeServerID = uint64(nodeServerID)
	_, err = tx.ExecContext(ctx, `INSERT INTO node_identity
		(identity_id,node_server_id,name,domain,public_ip,generation,mariadb_username,redis_username,redis_auth_username,credential_path,credential_sha256,status)
		VALUES (?,?,?,?,?,1,?,?,?,?,?,'provisioning')`, created.ID, created.NodeServerID, name, domain, publicIP, created.MariaDBUsername, created.RedisUsername, created.RedisAuthUsername, credentialPath, "")
	if err != nil {
		return identity{}, false, err
	}
	if err = tx.Commit(); err != nil {
		return identity{}, false, err
	}
	return created, true, nil
}

func credentialsForIdentity(path string, registered identity, allowCreate bool) (credentialFile, string, error) {
	if contents, err := readCredentialFile(path); err == nil {
		digest := credentialDigest(contents)
		if registered.CredentialSHA256 == "" || digest != registered.CredentialSHA256 {
			return credentialFile{}, "", errors.New("credential file does not match its control-plane commitment")
		}
		var existing credentialFile
		if json.Unmarshal(contents, &existing) != nil || existing.NodeIdentityID != registered.ID ||
			existing.SchemaVersion != 2 || existing.NodeServerID != registered.NodeServerID ||
			existing.NodeName != registered.Name || existing.NodeDomain != registered.Domain ||
			existing.PublicIP != registered.PublicIP || existing.Generation != registered.Generation ||
			existing.MariaDB.Database != databaseName || existing.MariaDB.Username != registered.MariaDBUsername ||
			existing.MariaDB.Password == "" || existing.Redis.Username != registered.RedisUsername ||
			existing.Redis.Password == "" || !equalStrings(existing.Redis.KeyPatterns, redisCacheKeyPatterns) ||
			existing.RedisAuth.Username != registered.RedisAuthUsername || existing.RedisAuth.Password == "" ||
			!equalStrings(existing.RedisAuth.KeyPatterns, redisAuthKeyPatterns) {
			return credentialFile{}, "", errors.New("existing credential file does not match the reserved Node identity")
		}
		return existing, digest, nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return credentialFile{}, "", err
	}
	if !allowCreate {
		return credentialFile{}, "", errors.New("the committed credential file is required for an idempotent replay")
	}
	dbPassword, err := randomSecret()
	if err != nil {
		return credentialFile{}, "", err
	}
	redisPassword, err := randomSecret()
	if err != nil {
		return credentialFile{}, "", err
	}
	redisAuthPassword, err := randomSecret()
	if err != nil {
		return credentialFile{}, "", err
	}
	created := credentialFile{
		SchemaVersion:  2,
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
			KeyPatterns: append([]string(nil), redisCacheKeyPatterns...),
		},
		RedisAuth: redisCredential{
			Username:    registered.RedisAuthUsername,
			Password:    redisAuthPassword,
			KeyPatterns: append([]string(nil), redisAuthKeyPatterns...),
		},
	}
	contents, err := marshalCredentialFile(created)
	if err != nil {
		return credentialFile{}, "", err
	}
	if err = createCredentialFile(path, contents); err != nil {
		return credentialFile{}, "", err
	}
	return created, credentialDigest(contents), nil
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

func marshalCredentialFile(credentials credentialFile) ([]byte, error) {
	contents, err := json.MarshalIndent(credentials, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(contents, '\n'), nil
}

func credentialDigest(contents []byte) string {
	digest := sha256.Sum256(contents)
	return hex.EncodeToString(digest[:])
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
			var databaseError *mysql.MySQLError
			if strings.HasPrefix(statement, "REVOKE ALL PRIVILEGES") && errors.As(err, &databaseError) && databaseError.Number == 1141 {
				continue
			}
			return err
		}
	}
	return nil
}

func (manager *lifecycle) preflightRedis() error {
	_, err := redigo.String(manager.redis.Do("ACL", "WHOAMI"))
	return err
}

func (manager *lifecycle) provisionRedis(credentials credentialFile) error {
	if err := manager.redis.Send("MULTI"); err != nil {
		return err
	}
	if err := manager.redis.Send("ACL", "SETUSER", credentials.Redis.Username,
		"reset", "on", ">"+credentials.Redis.Password,
		"~trojan-panel-core:*", "+ping", "+get", "+set", "+del", "+eval", "+evalsha", "+pttl"); err != nil {
		return err
	}
	if err := manager.redis.Send("ACL", "SETUSER", credentials.RedisAuth.Username,
		"reset", "on", ">"+credentials.RedisAuth.Password,
		"~trojan-panel:jwt-key", "~trojan-panel:token:*", "+ping", "+get"); err != nil {
		return err
	}
	replies, err := redigo.Values(manager.redis.Do("EXEC"))
	if err != nil {
		return err
	}
	return redisTransactionErrors(replies)
}

func (manager *lifecycle) deleteRedisUsers(usernames ...string) error {
	if err := manager.redis.Send("MULTI"); err != nil {
		return err
	}
	args := redigo.Args{}.Add("DELUSER")
	for _, username := range usernames {
		args = args.Add(username)
	}
	if err := manager.redis.Send("ACL", args...); err != nil {
		return err
	}
	replies, err := redigo.Values(manager.redis.Do("EXEC"))
	if err != nil {
		return err
	}
	return redisTransactionErrors(replies)
}

func redisTransactionErrors(replies []interface{}) error {
	for _, reply := range replies {
		if transactionError, ok := reply.(redigo.Error); ok {
			return transactionError
		}
	}
	return nil
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
	cacheConn, err := redigo.Dial("tcp", redisAddress,
		redigo.DialUsername(credentials.Redis.Username), redigo.DialPassword(credentials.Redis.Password),
		redigo.DialDatabase(core.Config.RedisConfig.Db), redigo.DialConnectTimeout(3*time.Second))
	if err != nil {
		return err
	}
	defer cacheConn.Close()
	reply, err := redigo.String(cacheConn.Do("PING"))
	if err != nil || reply != "PONG" {
		return errors.New("Redis cache credential verification failed")
	}
	authConn, err := redigo.Dial("tcp", redisAddress,
		redigo.DialUsername(credentials.RedisAuth.Username), redigo.DialPassword(credentials.RedisAuth.Password),
		redigo.DialDatabase(core.Config.RedisConfig.Db), redigo.DialConnectTimeout(3*time.Second))
	if err != nil {
		return err
	}
	defer authConn.Close()
	reply, err = redigo.String(authConn.Do("PING"))
	if err != nil || reply != "PONG" {
		return errors.New("Redis auth credential verification failed")
	}
	return nil
}

func (manager *lifecycle) recordEvent(ctx context.Context, id string, generation uint64, action LifecycleAction, result EventResult, errorCode string) {
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

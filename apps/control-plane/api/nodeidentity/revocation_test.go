package nodeidentity

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"database/sql"
	"database/sql/driver"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"trojanpanelnext/revocationreceipt"
)

const receiptTestIdentity = "12345678-1234-1234-1234-123456789abc"

func TestRevocationKeyInitializationAndReceiptPublication(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	fingerprint, err := initializeRevocationKey(root)
	if err != nil {
		t.Fatal(err)
	}
	privatePath, publicPath := revocationKeyPaths(root)
	private, err := loadRevocationSigningKey(root)
	if err != nil {
		t.Fatal(err)
	}
	if second, err := initializeRevocationKey(root); err != nil || second != fingerprint {
		t.Fatalf("key reinitialization changed trust root: %q %v", second, err)
	}
	for name, mode := range map[string]os.FileMode{filepath.Dir(privatePath): 0700, privatePath: 0600, publicPath: 0600} {
		info, err := os.Lstat(name)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("unsafe mode for %s: %v %v", name, info, err)
		}
	}
	receiptDir := filepath.Join(root, "receipts")
	if err := os.Mkdir(receiptDir, 0700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(receiptDir, "receipt.json")
	if err := validateNewReceiptPath(output); err != nil {
		t.Fatal(err)
	}
	identity := identity{ID: receiptTestIdentity, NodeServerID: 42, Generation: 7, Status: statusRevoked}
	if err := issueRevocationReceipt(private, identity, output); err != nil {
		t.Fatal(err)
	}
	encodedPublic, err := readCredentialFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	public, err := revocationreceipt.ParsePublicKey(encodedPublic)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := readCredentialFile(output)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := revocationreceipt.Verify(contents, public, revocationreceipt.Node{IdentityID: identity.ID, ServerID: 42, Generation: 6})
	if err != nil || claims.Status != revocationreceipt.Revoked || claims.IssuerFingerprint != fingerprint {
		t.Fatalf("receipt not verifiable: %+v %v", claims, err)
	}
	if err := validateNewReceiptPath(output); err == nil {
		t.Fatal("accepted existing receipt output")
	}
	if err := issueRevocationReceipt(private, identity, output); err == nil {
		t.Fatal("overwrote existing receipt")
	}
	unchanged, _ := os.ReadFile(output)
	if !bytes.Equal(unchanged, contents) {
		t.Fatal("receipt changed after rejected overwrite")
	}
	link := filepath.Join(receiptDir, "link.json")
	if err := os.Symlink(output, link); err != nil {
		t.Fatal(err)
	}
	if err := validateNewReceiptPath(link); err == nil {
		t.Fatal("accepted symlinked receipt output")
	}
	looseParent := filepath.Join(root, "loose")
	if err := os.Mkdir(looseParent, 0755); err != nil {
		t.Fatal(err)
	}
	if err := validateNewReceiptPath(filepath.Join(looseParent, "receipt.json")); err == nil {
		t.Fatal("accepted receipt output under non-private directory")
	}
}

func TestRevocationKeyFailsClosedOnMismatchedPublicFile(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := initializeRevocationKey(root); err != nil {
		t.Fatal(err)
	}
	_, publicPath := revocationKeyPaths(root)
	if err := os.WriteFile(publicPath, []byte("different public key\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadRevocationSigningKey(root); err == nil {
		t.Fatal("loaded key despite mismatched public file")
	}
	if _, err := initializeRevocationKey(root); err == nil {
		t.Fatal("overwrote mismatched public file")
	}
}

func TestRevocationKeyInitDoesNotRotateWhenPrivateFileIsMissing(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	if _, err := initializeRevocationKey(root); err != nil {
		t.Fatal(err)
	}
	privatePath, publicPath := revocationKeyPaths(root)
	publicBefore, err := os.ReadFile(publicPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(privatePath); err != nil {
		t.Fatal(err)
	}
	if _, err := initializeRevocationKey(root); err == nil {
		t.Fatal("recreated signing key while a pinned public key remained")
	}
	if _, err := os.Lstat(privatePath); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("created a replacement private key")
	}
	publicAfter, _ := os.ReadFile(publicPath)
	if !bytes.Equal(publicBefore, publicAfter) {
		t.Fatal("existing public key changed")
	}
}

func TestRevocationKeyInitCLIIsRepeatable(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "config"), 0700); err != nil {
		t.Fatal(err)
	}
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	var first, second, stderr bytes.Buffer
	for _, output := range []*bytes.Buffer{&first, &second} {
		if status := Run([]string{"trojan-panel", "node-identity", "revocation-key-init"}, output, &stderr); status != 0 {
			t.Fatalf("key init CLI status=%d stderr=%q", status, stderr.String())
		}
	}
	if first.String() != second.String() || !strings.Contains(first.String(), "Issuer fingerprint: sha256:") {
		t.Fatalf("key init CLI changed trust root: %q / %q", first.String(), second.String())
	}
}

func TestRevocationAuditFailureAndTerminalRetry(t *testing.T) {
	state := &receiptDatabase{status: statusActive, auditFails: true}
	manager := testRemovalLifecycle(state)
	deactivated, err := manager.revoke(context.Background(), receiptTestIdentity)
	if err == nil || deactivated.ID != "" || state.status != statusRevoked || state.successEvents != 0 || state.credentialDrops != 1 {
		t.Fatalf("audit failure was not fail-closed: identity=%+v err=%v state=%+v", deactivated, err, state)
	}
	state.auditFails = false
	deactivated, err = manager.revoke(context.Background(), receiptTestIdentity)
	if err != nil || deactivated.Status != statusRevoked || state.successEvents != 1 || state.credentialDrops != 2 {
		t.Fatalf("terminal retry did not revalidate and audit: identity=%+v err=%v state=%+v", deactivated, err, state)
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{3}, ed25519.SeedSize))
	directory := filepath.Join(t.TempDir(), "receipts")
	if err := os.Mkdir(directory, 0700); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(directory, "revoke-receipt.json")
	if err := issueRevocationReceipt(private, deactivated, output); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(output)
	if _, err := revocationreceipt.Verify(data, private.Public().(ed25519.PublicKey), revocationreceipt.Node{IdentityID: receiptTestIdentity, ServerID: 42, Generation: 7}); err != nil {
		t.Fatalf("retry receipt invalid: %v", err)
	}
}

func TestForceEvictTerminalRetryAndCredentialFailure(t *testing.T) {
	state := &receiptDatabase{status: statusEvicted, dropFails: true}
	manager := testRemovalLifecycle(state)
	if result, err := manager.forceEvict(context.Background(), receiptTestIdentity); err == nil || result.ID != "" || state.successEvents != 0 {
		t.Fatalf("failed credential recheck returned success: %+v %v", result, err)
	}
	state.dropFails = false
	result, err := manager.forceEvict(context.Background(), receiptTestIdentity)
	if err != nil || result.Status != statusEvicted || state.successEvents != 1 || state.registrationDeletes != 1 {
		t.Fatalf("terminal force-evict retry failed: %+v %v state=%+v", result, err, state)
	}
}

func TestReceiptCLIRejectsUnsafeOutputBeforeOpeningDataServices(t *testing.T) {
	path := filepath.Join(t.TempDir(), "already-exists.json")
	if err := os.WriteFile(path, []byte("foreign"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	status := Run([]string{"trojan-panel", "node-identity", "revoke", "--id", receiptTestIdentity, "--receipt-file", path}, &stdout, &stderr)
	if status != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "receipt output path") {
		t.Fatalf("unsafe output was not rejected: status=%d stdout=%q stderr=%q", status, stdout.String(), stderr.String())
	}
	data, _ := os.ReadFile(path)
	if string(data) != "foreign" {
		t.Fatal("unsafe output file was modified")
	}
}

type receiptDatabase struct {
	status              IdentityStatus
	auditFails          bool
	dropFails           bool
	credentialDrops     int
	registrationDeletes int
	successEvents       int
}

type receiptConnector struct{ state *receiptDatabase }

func (c receiptConnector) Connect(context.Context) (driver.Conn, error) {
	return receiptConnection{c.state}, nil
}
func (c receiptConnector) Driver() driver.Driver { return receiptDriver{} }

type receiptDriver struct{}

func (receiptDriver) Open(string) (driver.Conn, error) { return nil, errors.New("use connector") }

type receiptConnection struct{ state *receiptDatabase }

func (receiptConnection) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("unexpected prepare")
}
func (receiptConnection) Close() error              { return nil }
func (receiptConnection) Begin() (driver.Tx, error) { return nil, errors.New("unexpected transaction") }

func (c receiptConnection) ExecContext(_ context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	query = strings.TrimSpace(query)
	switch {
	case strings.HasPrefix(query, "CREATE TABLE"), strings.HasPrefix(query, "ALTER TABLE"), strings.HasPrefix(query, "SELECT RELEASE_LOCK"):
		return driver.RowsAffected(1), nil
	case strings.HasPrefix(query, "UPDATE node_identity SET status="):
		if len(args) != 3 || c.state.status != IdentityStatus(args[2].Value.(string)) {
			return driver.RowsAffected(0), nil
		}
		c.state.status = IdentityStatus(args[0].Value.(string))
		return driver.RowsAffected(1), nil
	case strings.HasPrefix(query, "DROP USER IF EXISTS"):
		c.state.credentialDrops++
		if c.state.dropFails {
			return nil, errors.New("test credential failure")
		}
		return driver.RowsAffected(1), nil
	case strings.HasPrefix(query, "DELETE FROM node_server"):
		c.state.registrationDeletes++
		return driver.RowsAffected(1), nil
	case strings.HasPrefix(query, "INSERT INTO node_identity_event"):
		if c.state.auditFails {
			return nil, errors.New("test audit failure")
		}
		if args[3].Value == string(resultSucceeded) {
			c.state.successEvents++
		}
		return driver.RowsAffected(1), nil
	default:
		return nil, errors.New("unexpected SQL: " + query)
	}
}

func (c receiptConnection) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	switch {
	case strings.HasPrefix(query, "SELECT GET_LOCK"):
		return &receiptRows{columns: []string{"locked"}, values: []driver.Value{int64(1)}}, nil
	case strings.HasPrefix(query, "SELECT identity_id,node_server_id"):
		return &receiptRows{columns: []string{"identity_id", "node_server_id", "name", "domain", "public_ip", "generation", "mariadb_username", "redis_username", "redis_auth_username", "credential_path", "credential_nonce", "credential_sha256", "status"}, values: []driver.Value{
			receiptTestIdentity, int64(42), "node-a", "node-a.example.com", "203.0.113.10", int64(7), "node_db", "node_cache", "node_auth", "/unused", "nonce", "digest", string(c.state.status),
		}}, nil
	default:
		return nil, errors.New("unexpected SQL query: " + query)
	}
}

type receiptRows struct {
	columns []string
	values  []driver.Value
	done    bool
}

func (r *receiptRows) Columns() []string { return r.columns }
func (r *receiptRows) Close() error      { return nil }
func (r *receiptRows) Next(dest []driver.Value) error {
	if r.done {
		return io.EOF
	}
	copy(dest, r.values)
	r.done = true
	return nil
}

type receiptRedis struct{}

func (receiptRedis) Close() error { return nil }
func (receiptRedis) Err() error   { return nil }
func (receiptRedis) Do(command string, _ ...interface{}) (interface{}, error) {
	if command == "EXEC" {
		return []interface{}{int64(1)}, nil
	}
	return nil, errors.New("unexpected Redis command")
}
func (receiptRedis) Send(string, ...interface{}) error { return nil }
func (receiptRedis) Flush() error                      { return nil }
func (receiptRedis) Receive() (interface{}, error)     { return nil, errors.New("unexpected receive") }

func testRemovalLifecycle(state *receiptDatabase) *lifecycle {
	db := sql.OpenDB(receiptConnector{state})
	return &lifecycle{db: db, redis: receiptRedis{}}
}

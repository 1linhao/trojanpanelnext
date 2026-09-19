package dao

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"trojan-panel/util"
)

type initialAdminDBState struct {
	passHash   string
	lookupHash string
	updates    int
}

type initialAdminConnector struct{ state *initialAdminDBState }

func (c initialAdminConnector) Connect(context.Context) (driver.Conn, error) {
	return &initialAdminConn{state: c.state}, nil
}

func (initialAdminConnector) Driver() driver.Driver { return initialAdminDriver{} }

type initialAdminDriver struct{}

func (initialAdminDriver) Open(string) (driver.Conn, error) { return nil, driver.ErrSkip }

type initialAdminConn struct{ state *initialAdminDBState }

func (*initialAdminConn) Prepare(string) (driver.Stmt, error) { return nil, driver.ErrSkip }
func (*initialAdminConn) Close() error                        { return nil }
func (*initialAdminConn) Begin() (driver.Tx, error)           { return nil, driver.ErrSkip }

func (c *initialAdminConn) QueryContext(context.Context, string, []driver.NamedValue) (driver.Rows, error) {
	return &initialAdminRows{values: []driver.Value{c.state.passHash, c.state.lookupHash}}, nil
}

func (c *initialAdminConn) ExecContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Result, error) {
	c.state.passHash = args[0].Value.(string)
	c.state.lookupHash = args[1].Value.(string)
	c.state.updates++
	return driver.RowsAffected(1), nil
}

type initialAdminRows struct {
	values []driver.Value
	done   bool
}

func (*initialAdminRows) Columns() []string { return []string{"pass", "hash"} }
func (*initialAdminRows) Close() error      { return nil }
func (r *initialAdminRows) Next(values []driver.Value) error {
	if r.done {
		return io.EOF
	}
	copy(values, r.values)
	r.done = true
	return nil
}

func useInitialAdminTestDB(t *testing.T, state *initialAdminDBState) {
	t.Helper()
	previous := db
	db = sql.OpenDB(initialAdminConnector{state: state})
	t.Cleanup(func() {
		_ = db.Close()
		db = previous
	})
}

func TestFreshDatabaseSeedCannotAuthenticateBeforeInitialization(t *testing.T) {
	sqlFile, err := os.ReadFile(filepath.Join("..", "resource", "sql", "trojan_panel_db_v2.3.0.sql"))
	if err != nil {
		t.Fatal(err)
	}
	for name, seed := range map[string]string{
		"embedded seed": string(sqlInitStr),
		"SQL asset":     string(sqlFile),
	} {
		if !strings.Contains(seed, "VALUES (1,'sysadmin','',''") {
			t.Fatalf("%s contains a usable sysadmin credential", name)
		}
	}
}

func TestInitialSysadminCredentialsFromRestrictedFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "initial-admin-password")
	const password = "A1B2C3D4E5F6G7H8I9J0"
	if err := os.WriteFile(path, []byte(password+"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	passHash, lookupHash, err := initialSysadminCredentials(path)
	if err != nil {
		t.Fatalf("initialSysadminCredentials() error = %v", err)
	}
	if !util.Sha1Match(passHash, "sysadmin"+password) {
		t.Fatal("generated pass hash does not authenticate the configured sysadmin password")
	}
	if lookupHash != util.SHA224String(passHash) {
		t.Fatal("generated lookup hash does not match the account storage contract")
	}
}

func TestPendingSysadminPasswordIsInitializedOnce(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "initial-admin-password")
	const password = "A1B2C3D4E5F6G7H8I9J0"
	if err := os.WriteFile(path, []byte(password+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(initialSysadminPasswordFileEnv, path)
	state := &initialAdminDBState{}
	useInitialAdminTestDB(t, state)

	if err := initializePendingSysadminPassword(); err != nil {
		t.Fatalf("initializePendingSysadminPassword() error = %v", err)
	}
	if state.updates != 1 {
		t.Fatalf("credential updates = %d, want 1", state.updates)
	}
	if !util.Sha1Match(state.passHash, "sysadmin"+password) {
		t.Fatal("persisted sysadmin hash does not authenticate the configured password")
	}
	if state.lookupHash != util.SHA224String(state.passHash) {
		t.Fatal("persisted sysadmin lookup hash is invalid")
	}

	if err := initializePendingSysadminPassword(); err != nil {
		t.Fatalf("second initializePendingSysadminPassword() error = %v", err)
	}
	if state.updates != 1 {
		t.Fatalf("credential updates after replay = %d, want 1", state.updates)
	}
}

func TestInitialSysadminCredentialsRejectsUnsafeInput(t *testing.T) {
	dir := t.TempDir()
	valid := filepath.Join(dir, "valid")
	if err := os.WriteFile(valid, []byte("A1B2C3D4E5F6G7H8I9J0\n"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		prepare func(string) error
	}{
		"group-readable file": {prepare: func(path string) error {
			if err := os.WriteFile(path, []byte("A1B2C3D4E5F6G7H8I9J0\n"), 0600); err != nil {
				return err
			}
			return os.Chmod(path, 0640)
		}},
		"symlink": {prepare: func(path string) error { return os.Symlink(valid, path) }},
		"short password": {prepare: func(path string) error {
			return os.WriteFile(path, []byte("short1\n"), 0600)
		}},
		"punctuation": {prepare: func(path string) error {
			return os.WriteFile(path, []byte("A1B2C3D4E5F6G7H8!J0\n"), 0600)
		}},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(dir, name)
			if err := tt.prepare(path); err != nil {
				t.Fatal(err)
			}
			if _, _, err := initialSysadminCredentials(path); err == nil {
				t.Fatal("unsafe initial sysadmin password input was accepted")
			}
		})
	}
}

package dao

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
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
	roleID     int64
	deleted    int64
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

func (c *initialAdminConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if strings.Contains(query, "`role_id`") {
		return &initialAdminRows{
			columns: []string{"pass", "role_id", "deleted"},
			values:  []driver.Value{c.state.passHash, c.state.roleID, c.state.deleted},
		}, nil
	}
	return &initialAdminRows{
		columns: []string{"pass", "hash"},
		values:  []driver.Value{c.state.passHash, c.state.lookupHash},
	}, nil
}

func (c *initialAdminConn) ExecContext(_ context.Context, _ string, args []driver.NamedValue) (driver.Result, error) {
	c.state.passHash = args[0].Value.(string)
	c.state.lookupHash = args[1].Value.(string)
	c.state.updates++
	return driver.RowsAffected(1), nil
}

type initialAdminRows struct {
	columns []string
	values  []driver.Value
	done    bool
}

func (r *initialAdminRows) Columns() []string { return r.columns }
func (*initialAdminRows) Close() error        { return nil }
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

func TestOpenedInitialSysadminPasswordDoesNotFollowReplacement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "initial-admin-password")
	const original = "A1B2C3D4E5F6G7H8I9J0\n"
	if err := os.WriteFile(path, []byte(original), 0600); err != nil {
		t.Fatal(err)
	}

	file, err := openInitialSysadminPasswordFile(path)
	if err != nil {
		t.Fatalf("openInitialSysadminPasswordFile() error = %v", err)
	}
	defer file.Close()

	moved := filepath.Join(dir, "opened-password")
	if err := os.Rename(path, moved); err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(dir, "replacement")
	if err := os.WriteFile(replacement, []byte("Z9Y8X7W6V5U4T3S2R1Q0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(replacement, path); err != nil {
		t.Fatal(err)
	}

	contents, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != original {
		t.Fatalf("opened credential changed after path replacement: %q", contents)
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

func TestVerifySysadminPasswordIsReadOnlyAcrossRepeatedFailures(t *testing.T) {
	const password = "A1B2C3D4E5F6G7H8I9J0"
	dir := t.TempDir()
	path := filepath.Join(dir, "initial-admin-password")
	if err := os.WriteFile(path, []byte("Z9Y8X7W6V5U4T3S2R1Q0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(initialSysadminPasswordFileEnv, path)
	state := &initialAdminDBState{
		passHash: util.Sha1String("sysadmin" + password),
		roleID:   1,
	}
	useInitialAdminTestDB(t, state)

	for attempt := 0; attempt < 6; attempt++ {
		if err := VerifyInitialSysadminCredential(); !errors.Is(err, ErrSysadminCredentialUnhealthy) {
			t.Fatalf("wrong sysadmin credential error = %v, want ErrSysadminCredentialUnhealthy", err)
		}
	}
	if state.updates != 0 {
		t.Fatalf("read-only health check performed %d updates", state.updates)
	}
	if err := os.WriteFile(path, []byte(password+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyInitialSysadminCredential(); err != nil {
		t.Fatalf("valid sysadmin credential failed after repeated mismatches: %v", err)
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
		"owner-executable file": {prepare: func(path string) error {
			return os.WriteFile(path, []byte("A1B2C3D4E5F6G7H8I9J0\n"), 0700)
		}},
		"symlink": {prepare: func(path string) error { return os.Symlink(valid, path) }},
		"symlink parent": {prepare: func(path string) error {
			realParent := filepath.Join(dir, "real-parent")
			if err := os.Mkdir(realParent, 0700); err != nil && !os.IsExist(err) {
				return err
			}
			if err := os.WriteFile(filepath.Join(realParent, "password"), []byte("A1B2C3D4E5F6G7H8I9J0\n"), 0600); err != nil {
				return err
			}
			if err := os.Symlink(realParent, path); err != nil {
				return err
			}
			return nil
		}},
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
			credentialPath := path
			if name == "symlink parent" {
				credentialPath = filepath.Join(path, "password")
			}
			if _, _, err := initialSysadminCredentials(credentialPath); err == nil {
				t.Fatal("unsafe initial sysadmin password input was accepted")
			}
		})
	}
}

package dao

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"strings"

	"github.com/go-sql-driver/mysql"
	"trojan-panel/util"
)

var errSharedCredential = errors.New("connection credential is already assigned or retired")

// Old backups could assign one pass to multiple rows. Every holder knows that
// bearer token, so none may keep it. Retain the verifier separately for login.
// Empty legacy rows have no connection token and stay outside the unique index.
func migrateAccountCredentials() error {
	for _, query := range []string{
		"ALTER TABLE account ADD COLUMN IF NOT EXISTS login_verifier varchar(64) NOT NULL DEFAULT ''",
		"CREATE TABLE IF NOT EXISTS account_retired_credential (pass varchar(64) NOT NULL PRIMARY KEY) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4",
		"ALTER TABLE account ADD COLUMN IF NOT EXISTS connection_key varchar(64) AS (NULLIF(pass,'')) VIRTUAL",
	} {
		if _, err := db.Exec(query); err != nil {
			return err
		}
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("INSERT IGNORE INTO account_retired_credential(pass) SELECT pass FROM account WHERE pass<>'' GROUP BY pass HAVING COUNT(*)>1"); err != nil {
		return err
	}
	rows, err := tx.Query("SELECT a.id FROM account a JOIN account_retired_credential r ON r.pass=a.pass ORDER BY a.id FOR UPDATE")
	if err != nil {
		return err
	}
	var ids []uint
	for rows.Next() {
		var id uint
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	if err = rows.Close(); err != nil {
		return err
	}
	for _, id := range ids {
		var secret [36]byte
		if _, err = rand.Read(secret[:]); err != nil {
			return err
		}
		pass := base64.RawURLEncoding.EncodeToString(secret[:])
		if _, err = tx.Exec("UPDATE account SET login_verifier=IF(login_verifier='',pass,login_verifier),pass=?,hash=? WHERE id=?", pass, util.SHA224String(pass), id); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	_, err = db.Exec("CREATE UNIQUE INDEX IF NOT EXISTS account_connection_unique ON account(connection_key)")
	return err
}

func checkImportedCredential(pass string, id uint) error {
	var claimed int
	err := db.QueryRow("SELECT 1 FROM account WHERE pass=? AND id<>? UNION ALL SELECT 1 FROM account_retired_credential WHERE pass=? LIMIT 1", pass, id, pass).Scan(&claimed)
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	if err != nil {
		return err
	}
	return errSharedCredential
}

func insertCredentialError(err error) error {
	var failure *mysql.MySQLError
	if errors.As(err, &failure) && failure.Number == 1062 && strings.Contains(failure.Message, "account_connection_unique") {
		return errSharedCredential
	}
	return err
}

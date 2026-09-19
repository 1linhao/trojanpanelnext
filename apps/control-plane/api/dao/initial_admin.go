package dao

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
	"trojan-panel/util"
)

const initialSysadminPasswordFileEnv = "TP_INITIAL_SYSADMIN_PASSWORD_FILE"

var initialSysadminPasswordPattern = regexp.MustCompile(`^[A-Za-z0-9]{16,20}$`)

func initialSysadminCredentials(path string) (string, string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", "", fmt.Errorf("read initial sysadmin password metadata: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", "", errors.New("initial sysadmin password must be a regular non-symlink file")
	}
	if info.Mode().Perm()&0077 != 0 {
		return "", "", errors.New("initial sysadmin password file must not be accessible by group or other users")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return "", "", fmt.Errorf("read initial sysadmin password: %w", err)
	}
	password := strings.TrimSuffix(string(contents), "\n")
	if !initialSysadminPasswordPattern.MatchString(password) {
		return "", "", errors.New("initial sysadmin password must contain 16 to 20 ASCII letters or digits")
	}
	passHash := util.Sha1String("sysadmin" + password)
	return passHash, util.SHA224String(passHash), nil
}

func validateInitialSysadminPasswordFile() error {
	path := os.Getenv(initialSysadminPasswordFileEnv)
	if path == "" {
		return fmt.Errorf("%s is required for a fresh database", initialSysadminPasswordFileEnv)
	}
	_, _, err := initialSysadminCredentials(path)
	return err
}

func initializePendingSysadminPassword() error {
	var passHash string
	var lookupHash string
	if err := db.QueryRow("SELECT `pass`, `hash` FROM `account` WHERE `username` = 'sysadmin'").Scan(&passHash, &lookupHash); err != nil {
		return fmt.Errorf("read sysadmin bootstrap state: %w", err)
	}
	if passHash != "" || lookupHash != "" {
		if passHash == "" || lookupHash == "" {
			return errors.New("sysadmin bootstrap state is inconsistent")
		}
		return nil
	}

	path := os.Getenv(initialSysadminPasswordFileEnv)
	if path == "" {
		return fmt.Errorf("%s is required while sysadmin initialization is pending", initialSysadminPasswordFileEnv)
	}
	passHash, lookupHash, err := initialSysadminCredentials(path)
	if err != nil {
		return err
	}
	result, err := db.Exec(
		"UPDATE `account` SET `pass` = ?, `hash` = ? WHERE `username` = 'sysadmin' AND `pass` = '' AND `hash` = ''",
		passHash,
		lookupHash,
	)
	if err != nil {
		return fmt.Errorf("initialize sysadmin password: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("verify sysadmin password initialization: %w", err)
	}
	if rows != 1 {
		return fmt.Errorf("initialize sysadmin password: expected one pending account, updated %d", rows)
	}
	return nil
}

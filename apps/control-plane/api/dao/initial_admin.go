package dao

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"trojan-panel/util"
)

const initialSysadminPasswordFileEnv = "TP_INITIAL_SYSADMIN_PASSWORD_FILE"

var initialSysadminPasswordPattern = regexp.MustCompile(`^[A-Za-z0-9]{16,20}$`)

func openInitialSysadminPasswordFile(path string) (*os.File, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve initial sysadmin password path: %w", err)
	}
	parts := strings.Split(strings.TrimPrefix(filepath.Clean(absolute), string(filepath.Separator)), string(filepath.Separator))
	if len(parts) == 0 || parts[0] == "" {
		return nil, errors.New("initial sysadmin password path is invalid")
	}

	directoryFD, err := syscall.Open(string(filepath.Separator), syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open initial sysadmin password root: %w", err)
	}
	defer func() { _ = syscall.Close(directoryFD) }()

	for _, component := range parts[:len(parts)-1] {
		nextFD, openErr := syscall.Openat(directoryFD, component, syscall.O_RDONLY|syscall.O_DIRECTORY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
		if openErr != nil {
			return nil, fmt.Errorf("open initial sysadmin password parent without symlinks: %w", openErr)
		}
		_ = syscall.Close(directoryFD)
		directoryFD = nextFD
	}

	fileFD, err := syscall.Openat(directoryFD, parts[len(parts)-1], syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("open initial sysadmin password without symlinks: %w", err)
	}
	file := os.NewFile(uintptr(fileFD), absolute)
	if file == nil {
		_ = syscall.Close(fileFD)
		return nil, errors.New("open initial sysadmin password: invalid file descriptor")
	}
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, fmt.Errorf("read initial sysadmin password metadata: %w", err)
	}
	if !info.Mode().IsRegular() {
		_ = file.Close()
		return nil, errors.New("initial sysadmin password must be a regular non-symlink file")
	}
	if info.Mode().Perm() != 0600 {
		_ = file.Close()
		return nil, errors.New("initial sysadmin password file permissions must be exactly 0600")
	}
	return file, nil
}

func initialSysadminCredentials(path string) (string, string, error) {
	file, err := openInitialSysadminPasswordFile(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()
	contents, err := io.ReadAll(file)
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

func VerifySysadminPassword(password string) error {
	var passHash string
	var roleID uint
	var deleted uint
	if err := db.QueryRow(
		"SELECT `pass`, `role_id`, `deleted` FROM `account` WHERE `username` = 'sysadmin'",
	).Scan(&passHash, &roleID, &deleted); err != nil {
		return fmt.Errorf("read sysadmin health state: %w", err)
	}
	if roleID != 1 || deleted != 0 || !util.Sha1Match(passHash, "sysadmin"+password) {
		return errors.New("sysadmin credential is not healthy")
	}
	return nil
}

package revocationreceipt

import (
	"bytes"
	"crypto/ed25519"
	"crypto/subtle"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const maxPrivateFileSize = 1024

// StorePrivateKey creates a dedicated Web signing key file once. Its private
// directory is 0700 and its PKCS#8 file is 0600. Existing files, including
// symlinks, are never replaced. The caller generates the key separately.
func StorePrivateKey(path string, private ed25519.PrivateKey) error {
	if err := validatePrivateKey(private); err != nil {
		return err
	}
	if err := checkPrivateDirectory(path, true); err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(private)
	if err != nil {
		return errors.New("invalid revocation private key")
	}
	contents := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	if len(contents) > maxPrivateFileSize {
		return errors.New("invalid revocation private key")
	}
	if _, err := os.Lstat(path); err == nil {
		return errors.New("revocation private key path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("revocation private key path is unavailable")
	}
	dir := filepath.Dir(path)
	temporary, err := os.CreateTemp(dir, ".revocation-key-*")
	if err != nil {
		return errors.New("cannot create revocation private key")
	}
	defer os.Remove(temporary.Name())
	if err := temporary.Chmod(0600); err != nil {
		temporary.Close()
		return errors.New("cannot secure revocation private key")
	}
	if _, err := temporary.Write(contents); err != nil {
		temporary.Close()
		return errors.New("cannot write revocation private key")
	}
	if err := temporary.Sync(); err != nil {
		temporary.Close()
		return errors.New("cannot sync revocation private key")
	}
	if err := temporary.Close(); err != nil {
		return errors.New("cannot close revocation private key")
	}
	// Link is atomic and fails if another file appeared at the target. Rename
	// would silently replace it, which is unsafe for a pinned signing key.
	if err := os.Link(temporary.Name(), path); err != nil {
		return errors.New("revocation private key path already exists or is unavailable")
	}
	if err := os.Remove(temporary.Name()); err != nil {
		return errors.New("cannot finalize revocation private key")
	}
	parent, err := os.Open(dir)
	if err != nil {
		return errors.New("cannot sync revocation private key directory")
	}
	defer parent.Close()
	if err := parent.Sync(); err != nil {
		return errors.New("cannot sync revocation private key directory")
	}
	return nil
}

// LoadPrivateKey rejects links, foreign ownership and relaxed permissions.
func LoadPrivateKey(path string) (ed25519.PrivateKey, error) {
	if err := checkPrivateDirectory(path, false); err != nil {
		return nil, err
	}
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, errors.New("revocation private key is unavailable")
	}
	file := os.NewFile(uintptr(fd), path)
	defer file.Close()
	info, err := file.Stat()
	if err != nil || info.Mode() != 0600 || info.Size() <= 0 || info.Size() > maxPrivateFileSize ||
		!ownedByCurrentUser(info) || !singleLink(info) {
		return nil, errors.New("revocation private key is not a safe regular file")
	}
	contents, err := io.ReadAll(io.LimitReader(file, maxPrivateFileSize+1))
	if err != nil || len(contents) > maxPrivateFileSize {
		return nil, errors.New("revocation private key is invalid")
	}
	block, rest := pem.Decode(contents)
	if block == nil || block.Type != "PRIVATE KEY" || len(block.Headers) != 0 || len(rest) != 0 ||
		!bytes.Equal(contents, pem.EncodeToMemory(block)) {
		return nil, errors.New("revocation private key is invalid")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	private, ok := parsed.(ed25519.PrivateKey)
	if err != nil || !ok || validatePrivateKey(private) != nil {
		return nil, errors.New("revocation private key is invalid")
	}
	return private, nil
}

func validatePrivateKey(private ed25519.PrivateKey) error {
	if len(private) != ed25519.PrivateKeySize {
		return errors.New("invalid revocation private key")
	}
	derived := ed25519.NewKeyFromSeed(private.Seed())
	if subtle.ConstantTimeCompare(private, derived) != 1 {
		return errors.New("invalid revocation private key")
	}
	return nil
}

func checkPrivateDirectory(path string, create bool) error {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || filepath.Base(path) == "." || filepath.Base(path) == string(os.PathSeparator) {
		return errors.New("revocation private key path must be absolute and clean")
	}
	dir := filepath.Dir(path)
	if dir == string(os.PathSeparator) {
		return errors.New("revocation private key directory is unsafe")
	}
	parts := strings.Split(strings.TrimPrefix(dir, string(os.PathSeparator)), string(os.PathSeparator))
	current := string(os.PathSeparator)
	for index, part := range parts {
		if part == "" {
			continue
		}
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if errors.Is(err, os.ErrNotExist) && create && index == len(parts)-1 {
			if err := os.Mkdir(current, 0700); err != nil {
				return errors.New("cannot create revocation private key directory")
			}
			info, err = os.Lstat(current)
		}
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("revocation private key directory is unsafe")
		}
		if index == len(parts)-1 && (info.Mode() != os.ModeDir|0700 || !ownedByCurrentUser(info)) {
			return errors.New("revocation private key directory is unsafe")
		}
	}
	return nil
}

func ownedByCurrentUser(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Uid == uint32(os.Geteuid())
}

func singleLink(info os.FileInfo) bool {
	stat, ok := info.Sys().(*syscall.Stat_t)
	return ok && stat.Nlink == 1
}

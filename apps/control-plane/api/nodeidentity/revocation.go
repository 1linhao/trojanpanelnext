package nodeidentity

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"trojanpanelnext/revocationreceipt"
)

const (
	revocationDirectory = "revocation"
	revocationPrivate   = "signing-key.pem"
	revocationPublic    = "public-key.txt"
)

func revocationKeyPaths(root string) (string, string) {
	dir := filepath.Join(root, "config", revocationDirectory)
	return filepath.Join(dir, revocationPrivate), filepath.Join(dir, revocationPublic)
}

// initializeRevocationKey is an explicit, once-only Web operation. The public
// file is the one later embedded in Node bootstrap bundles. A failed or
// concurrent initialization never replaces a previously published key.
func initializeRevocationKey(root string) (string, error) {
	privatePath, publicPath := revocationKeyPaths(root)
	_, err := os.Lstat(privatePath)
	if errors.Is(err, os.ErrNotExist) {
		if _, publicErr := os.Lstat(publicPath); publicErr == nil || !errors.Is(publicErr, os.ErrNotExist) {
			return "", errors.New("revocation public key exists without its signing key")
		}
		_, candidate, generateErr := ed25519.GenerateKey(rand.Reader)
		if generateErr != nil {
			return "", errors.New("cannot generate revocation signing key")
		}
		if storeErr := revocationreceipt.StorePrivateKey(privatePath, candidate); storeErr != nil {
			// An error after publishing but before syncing is uncertain. A
			// retry may load a complete key, but this invocation must fail.
			return "", errors.New("cannot store revocation signing key")
		}
	} else if err != nil {
		return "", errors.New("revocation signing key path is unavailable")
	}
	private, err := revocationreceipt.LoadPrivateKey(privatePath)
	if err != nil {
		return "", errors.New("revocation signing key is unavailable or unsafe")
	}
	public := private.Public().(ed25519.PublicKey)
	encoded, err := revocationreceipt.EncodePublicKey(public)
	if err != nil {
		return "", errors.New("revocation public key is invalid")
	}
	if existing, readErr := readCredentialFile(publicPath); errors.Is(readErr, os.ErrNotExist) {
		if createErr := createCredentialFile(publicPath, encoded); createErr != nil {
			// A concurrent initialization may have published the same key.
			existing, readErr = readCredentialFile(publicPath)
			if readErr != nil || !bytes.Equal(existing, encoded) {
				return "", errors.New("cannot publish revocation public key")
			}
		}
	} else if readErr != nil || !bytes.Equal(existing, encoded) {
		return "", errors.New("revocation public key differs from the signing key")
	}
	fingerprint, _ := revocationreceipt.Fingerprint(public)
	return fingerprint, nil
}

func loadRevocationSigningKey(root string) (ed25519.PrivateKey, error) {
	privatePath, publicPath := revocationKeyPaths(root)
	private, err := revocationreceipt.LoadPrivateKey(privatePath)
	if err != nil {
		return nil, errors.New("revocation signing key is unavailable or unsafe")
	}
	encoded, err := readCredentialFile(publicPath)
	if err != nil {
		return nil, errors.New("revocation public key is unavailable or unsafe")
	}
	public, err := revocationreceipt.ParsePublicKey(encoded)
	if err != nil || !bytes.Equal(public, private.Public().(ed25519.PublicKey)) {
		return nil, errors.New("revocation public key differs from the signing key")
	}
	return private, nil
}

func runRevocationKeyInit(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		fmt.Fprintln(stderr, "node identity: revocation-key-init takes no arguments")
		return 2
	}
	root, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "node identity: Web data directory is unavailable")
		return 1
	}
	fingerprint, err := initializeRevocationKey(root)
	if err != nil {
		fmt.Fprintln(stderr, "node identity: revocation key initialization failed")
		return 1
	}
	_, publicPath := revocationKeyPaths(root)
	fmt.Fprintf(stdout, "Node revocation public key: %s\nIssuer fingerprint: %s\n", publicPath, fingerprint)
	return 0
}

func validateNewReceiptPath(path string) error {
	if path == "" || !filepath.IsAbs(path) || filepath.Clean(path) != path ||
		strings.IndexFunc(path, func(character rune) bool { return character < 32 || character == 127 }) >= 0 {
		return errors.New("receipt path must be absolute and clean")
	}
	if err := validateCredentialPath(path); err != nil {
		return errors.New("receipt path is unsafe")
	}
	parent, err := os.Lstat(filepath.Dir(path))
	if err != nil {
		return errors.New("receipt output directory must be private")
	}
	owner, ok := parent.Sys().(*syscall.Stat_t)
	if !ok || parent.Mode() != os.ModeDir|0700 || owner.Uid != uint32(os.Geteuid()) {
		return errors.New("receipt output directory must be private")
	}
	if _, err := os.Lstat(path); err == nil {
		return errors.New("receipt path already exists")
	} else if !errors.Is(err, os.ErrNotExist) {
		return errors.New("receipt path is unavailable")
	}
	return nil
}

func issueRevocationReceipt(private ed25519.PrivateKey, deactivated identity, outputPath string) error {
	if err := validateNewReceiptPath(outputPath); err != nil {
		return err
	}
	var status revocationreceipt.Status
	switch deactivated.Status {
	case statusRevoked:
		status = revocationreceipt.Revoked
	case statusEvicted:
		status = revocationreceipt.Evicted
	default:
		return errors.New("Node identity is not terminal")
	}
	receipt, err := revocationreceipt.Issue(private, revocationreceipt.Claims{
		IdentityID: deactivated.ID, ServerID: deactivated.NodeServerID,
		RevokedThroughGeneration: deactivated.Generation, Status: status,
	})
	if err != nil {
		return errors.New("cannot sign Node revocation receipt")
	}
	if err := createCredentialFile(outputPath, receipt); err != nil {
		return errors.New("cannot publish Node revocation receipt")
	}
	return nil
}

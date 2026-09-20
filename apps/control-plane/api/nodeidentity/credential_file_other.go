//go:build !linux

package nodeidentity

import "errors"

var errSecureCredentialFilesUnsupported = errors.New("secure Node credential files require a Linux control-plane host")

// The supported deployment platform is Linux, where descriptor-relative
// traversal, O_NOFOLLOW and renameat2 provide the complete secure-file
// contract. Other platforms fail closed instead of weakening that contract
// with path-based checks that are vulnerable to TOCTOU replacement.
func validateCredentialPath(string) error {
	return errSecureCredentialFilesUnsupported
}

func readCredentialFile(string) ([]byte, error) {
	return nil, errSecureCredentialFilesUnsupported
}

func createCredentialFile(string, []byte) error {
	return errSecureCredentialFilesUnsupported
}

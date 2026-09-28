package revocationreceipt

import (
	"bytes"
	"crypto/ed25519"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrivateKeyStoreAndLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "revocation", "signing-key.pem")
	private := testKey()
	if err := StorePrivateKey(path, private); err != nil {
		t.Fatal(err)
	}
	for name, mode := range map[string]os.FileMode{filepath.Dir(path): 0700, path: 0600} {
		info, err := os.Lstat(name)
		if err != nil || info.Mode().Perm() != mode {
			t.Fatalf("%s mode: %v, %v", name, info, err)
		}
	}
	loaded, err := LoadPrivateKey(path)
	if err != nil || !bytes.Equal(loaded, private) {
		t.Fatalf("load: %v", err)
	}
	if err := StorePrivateKey(path, ed25519.NewKeyFromSeed(bytes.Repeat([]byte{5}, 32))); err == nil {
		t.Fatal("overwrote existing private key")
	}
	loaded, err = LoadPrivateKey(path)
	if err != nil || !bytes.Equal(loaded, private) {
		t.Fatal("existing private key changed")
	}
}

func TestPrivateKeyFilesRejectLinksAndUnsafeModes(t *testing.T) {
	private := testKey()
	root := t.TempDir()
	safe := filepath.Join(root, "safe")
	if err := os.Mkdir(safe, 0700); err != nil {
		t.Fatal(err)
	}
	realPath := filepath.Join(safe, "real.pem")
	if err := StorePrivateKey(realPath, private); err != nil {
		t.Fatal(err)
	}
	linkPath := filepath.Join(safe, "link.pem")
	if err := os.Symlink(realPath, linkPath); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrivateKey(linkPath); err == nil {
		t.Fatal("loaded private key symlink")
	}
	if err := StorePrivateKey(linkPath, private); err == nil {
		t.Fatal("replaced private key symlink")
	}
	hardLink := filepath.Join(safe, "hardlink.pem")
	if err := os.Link(realPath, hardLink); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrivateKey(realPath); err == nil {
		t.Fatal("loaded multiply linked private key")
	}
	if err := os.Remove(hardLink); err != nil {
		t.Fatal(err)
	}
	dirLink := filepath.Join(root, "linked-directory")
	if err := os.Symlink(safe, dirLink); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrivateKey(filepath.Join(dirLink, "real.pem")); err == nil {
		t.Fatal("loaded through directory symlink")
	}
	if err := StorePrivateKey(filepath.Join(dirLink, "new.pem"), private); err == nil {
		t.Fatal("stored through directory symlink")
	}
	if err := os.Chmod(realPath, 0644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrivateKey(realPath); err == nil {
		t.Fatal("loaded world-readable key")
	}
	if err := os.Chmod(realPath, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(safe, 0750); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPrivateKey(realPath); err == nil {
		t.Fatal("loaded key under non-private directory")
	}
	if err := StorePrivateKey(filepath.Join(safe, "new.pem"), private); err == nil {
		t.Fatal("stored key under non-private directory")
	}
}

func TestPrivateKeyStoreNeverOverwritesForeignFile(t *testing.T) {
	private := testKey()
	root := t.TempDir()
	dir := filepath.Join(root, "revocation")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "signing-key.pem")
	original := []byte("foreign-data-must-survive")
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := StorePrivateKey(path, private); err == nil {
		t.Fatal("overwrote foreign file")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(after, original) {
		t.Fatal("foreign data changed")
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temporary file leaked: %v %v", entries, err)
	}
}

func TestPrivateKeyFileRejectsMalformedAndUncleanPaths(t *testing.T) {
	private := testKey()
	root := t.TempDir()
	for _, path := range []string{"relative.pem", filepath.Join(root, "missing", "nested", "key.pem"), filepath.Join(root, "..", "key.pem") + "/../key.pem"} {
		if err := StorePrivateKey(path, private); err == nil {
			t.Fatalf("stored at unsafe path %q", path)
		}
	}
	if err := StorePrivateKey("/key.pem", private); err == nil {
		t.Fatal("stored key directly under root")
	}
	dir := filepath.Join(root, "revocation")
	if err := os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "invalid.pem")
	if err := os.WriteFile(path, []byte("secret-material"), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := LoadPrivateKey(path)
	if err == nil || strings.Contains(err.Error(), "secret-material") {
		t.Fatalf("unsafe private key error: %v", err)
	}
}

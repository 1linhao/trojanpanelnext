package pki

import (
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestImportLegacyInstallerCAWithoutKeyUsage(t *testing.T) {
	m, i := seedAuthority(t, time.Now())
	m.Close()
	i.ca.KeyUsage = 0
	der, err := x509.CreateCertificate(rand.Reader, i.ca, i.ca, i.caKey.Public(), i.caKey)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(m.dir, "client-ca.crt"), pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0644); err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(filepath.Join(m.dir, "state.json")); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(m.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if err = reopened.Prepare(time.Now()); err != nil {
		t.Fatal(err)
	}
}

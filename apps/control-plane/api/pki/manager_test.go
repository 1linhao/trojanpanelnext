package pki

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func seedAuthority(t *testing.T, issued time.Time) (*Manager, *identity) {
	t.Helper()
	i, err := createIdentity(issued)
	if err != nil {
		t.Fatal(err)
	}
	if err = issueClient(i, time.Now()); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, data := range map[string][]byte{"client-ca.crt": i.caPEM, "client-ca.key": i.keyPEM, "client.crt": i.certPEM, "client.key": i.clientKeyPEM} {
		if err = os.WriteFile(filepath.Join(dir, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	m, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { m.Close() })
	return m, i
}

func certificateRequest(ca *x509.Certificate) *tls.CertificateRequestInfo {
	return &tls.CertificateRequestInfo{Version: tls.VersionTLS13, AcceptableCAs: [][]byte{ca.RawSubject}, SignatureSchemes: []tls.SignatureScheme{tls.ECDSAWithP256AndSHA256, tls.PSSWithSHA256}}
}

func TestClientRenewalPreservesCAAndPublishesCompleteIdentity(t *testing.T) {
	now := time.Now()
	m, original := seedAuthority(t, now)
	if err := m.Prepare(now.Add(750 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	current, err := m.read(m.state.Current)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(current.ca.Raw, original.ca.Raw) || bytes.Equal(current.pair.Leaf.Raw, original.pair.Leaf.Raw) {
		t.Fatal("client was not renewed under the existing CA")
	}
	roots := x509.NewCertPool()
	roots.AddCert(original.ca)
	if _, err = current.pair.Leaf.Verify(x509.VerifyOptions{Roots: roots, CurrentTime: now.Add(750 * 24 * time.Hour), KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		t.Fatal(err)
	}
	if _, err = tls.LoadX509KeyPair(filepath.Join(m.dir, "client.crt"), filepath.Join(m.dir, "client.key")); err != nil {
		t.Fatal(err)
	}
	stat, err := os.Stat(filepath.Join(m.dir, "client.key"))
	if err != nil || stat.Mode().Perm() != 0600 {
		t.Fatal("client key permissions")
	}
	id := m.state.Current
	if err = m.Prepare(now.Add(751 * 24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	if id != m.state.Current {
		t.Fatal("unnecessary renewal")
	}
}

func TestCARotationWaitsForAllNodesAndSurvivesRestart(t *testing.T) {
	now := time.Now()
	m, old := seedAuthority(t, now.Add(-3450*24*time.Hour))
	if err := m.Prepare(now); err != nil {
		t.Fatal(err)
	}
	oldID, pendingID := m.state.Current, m.state.Pending
	if pendingID == "" {
		t.Fatal("new CA not staged")
	}
	if err := m.Reconcile(now, func(bundle []byte) error {
		if countCerts(bundle) != 2 {
			t.Fatal("expected both CAs before activation")
		}
		if _, err := m.GetClientCertificate(certificateRequest(old.ca)); err != nil {
			t.Fatal(err)
		}
		return errors.New("one node offline")
	}); err == nil {
		t.Fatal("offline node must block rotation")
	}
	if m.state.Current != oldID {
		t.Fatal("identity changed before acknowledgement")
	}
	m.Close()
	reopened, err := Open(m.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.state.Pending != pendingID {
		t.Fatal("pending rotation lost on restart")
	}
	if err = reopened.Reconcile(now, func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	current, err := reopened.read(reopened.state.Current)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(current.ca.Raw, old.ca.Raw) || reopened.state.Previous != oldID {
		t.Fatal("new CA not activated")
	}
	// A rolled-back/offline node can still request the previous identity.
	if pair, err := reopened.GetClientCertificate(certificateRequest(old.ca)); err != nil || !bytes.Equal(pair.Leaf.Raw, old.pair.Leaf.Raw) {
		t.Fatalf("previous identity unavailable: %v", err)
	}
	if _, err = reopened.GetClientCertificate(certificateRequest(current.ca)); err != nil {
		t.Fatal(err)
	}
	if err = reopened.Reconcile(now.Add(RotationGrace), func(bundle []byte) error {
		if countCerts(bundle) == 2 {
			return nil
		}
		if countCerts(bundle) != 1 {
			t.Fatal("invalid retirement bundle")
		}
		return errors.New("another node unavailable")
	}); err == nil {
		t.Fatal("failed retirement must preserve fallback")
	}
	if reopened.state.Previous == "" {
		t.Fatal("fallback retired too early")
	}
	if err = reopened.Reconcile(now.Add(RotationGrace), func([]byte) error { return nil }); err != nil {
		t.Fatal(err)
	}
	if reopened.state.Previous != "" {
		t.Fatal("previous CA not retired")
	}
	if _, err = reopened.GetClientCertificate(certificateRequest(old.ca)); err == nil {
		t.Fatal("retired identity still accepted")
	}
	if _, err = os.Stat(filepath.Join(m.dir, "generations", oldID)); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("retired private keys retained")
	}
}

func TestAuthorityRejectsSecondWriterAndInvalidIdentity(t *testing.T) {
	m, _ := seedAuthority(t, time.Now())
	if other, err := Open(m.dir); err == nil {
		other.Close()
		t.Fatal("second writer acquired authority")
	}
	if err := os.WriteFile(filepath.Join(m.dir, "generations", m.state.Current, "client.key"), []byte("invalid"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := m.Prepare(time.Now()); err == nil {
		t.Fatal("corrupt identity accepted")
	}
}

func countCerts(data []byte) int {
	count := 0
	for {
		block, rest := pem.Decode(data)
		if block == nil {
			return count
		}
		count++
		data = rest
	}
}

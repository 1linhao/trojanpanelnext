// Package pki owns the control plane's persisted mTLS identity. CA keys never
// leave this directory; nodes receive only public trust bundles over mTLS.
package pki

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	ClientRenewBefore = 90 * 24 * time.Hour
	CARenewBefore     = 365 * 24 * time.Hour
	RotationGrace     = 24 * time.Hour
)

type state struct {
	Current     string    `json:"current"`
	Pending     string    `json:"pending,omitempty"`
	Previous    string    `json:"previous,omitempty"`
	ActivatedAt time.Time `json:"activated_at,omitempty"`
}

type identity struct {
	ca                                   *x509.Certificate
	caKey                                crypto.Signer
	caPEM, keyPEM, certPEM, clientKeyPEM []byte
	pair                                 tls.Certificate
}

type Manager struct {
	mu    sync.Mutex
	op    sync.Mutex
	dir   string
	state state
	lock  *os.File
}

func Open(dir string) (*Manager, error) {
	if !filepath.IsAbs(dir) {
		return nil, errors.New("PKI authority directory must be absolute")
	}
	if err := os.MkdirAll(filepath.Join(dir, "generations"), 0700); err != nil {
		return nil, err
	}
	lock, err := os.OpenFile(filepath.Join(dir, ".authority.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, fmt.Errorf("PKI authority already in use: %w", err)
	}
	m := &Manager{dir: dir, lock: lock}
	if err = m.open(); err != nil {
		m.Close()
		return nil, err
	}
	return m, nil
}

func (m *Manager) Close() error { return m.lock.Close() }

func (m *Manager) open() error {
	content, err := os.ReadFile(filepath.Join(m.dir, "state.json"))
	if errors.Is(err, os.ErrNotExist) {
		old, readErr := readIdentity(m.dir)
		if readErr != nil {
			return fmt.Errorf("import installer PKI: %w", readErr)
		}
		id, writeErr := m.store(old)
		if writeErr != nil {
			return writeErr
		}
		return m.commit(state{Current: id})
	}
	if err != nil {
		return err
	}
	if err = json.Unmarshal(content, &m.state); err != nil {
		return err
	}
	if m.state.Current == "" || (m.state.Pending != "" && m.state.Previous != "") {
		return errors.New("invalid PKI rotation state")
	}
	for _, id := range []string{m.state.Current, m.state.Pending, m.state.Previous} {
		if id == "" {
			continue
		}
		if len(id) != 32 || strings.ContainsAny(id, "/\\") {
			return errors.New("invalid PKI generation")
		}
		if _, err = hex.DecodeString(id); err != nil {
			return err
		}
		if _, err = m.read(id); err != nil {
			return err
		}
	}
	return m.export()
}

// Prepare renews the client identity and stages a new CA well before expiry.
// It does not activate a CA: every registered mTLS node must acknowledge first.
func (m *Manager) Prepare(now time.Time) error {
	m.op.Lock()
	defer m.op.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	current, err := m.read(m.state.Current)
	if err != nil {
		return err
	}
	if !now.Before(current.ca.NotAfter) {
		return errors.New("control CA expired; restore trust manually")
	}
	if current.pair.Leaf.NotAfter.After(current.ca.NotAfter) || (current.pair.Leaf.NotAfter.Sub(now) <= ClientRenewBefore && current.pair.Leaf.NotAfter.Before(current.ca.NotAfter)) {
		if err = issueClient(current, now); err != nil {
			return err
		}
		id, err := m.store(current)
		if err != nil {
			return err
		}
		next := m.state
		next.Current = id
		if err = m.commit(next); err != nil {
			return err
		}
	}
	if current.ca.NotAfter.Sub(now) <= CARenewBefore && m.state.Pending == "" && m.state.Previous == "" {
		candidate, err := createIdentity(now)
		if err != nil {
			return err
		}
		id, err := m.store(candidate)
		if err != nil {
			return err
		}
		next := m.state
		next.Pending = id
		return m.commit(next)
	}
	return m.export()
}

// Reconcile sends the bundle to ALL registered mTLS nodes. Any failed or old
// node blocks the transition. Repeating the operation after a crash is safe.
func (m *Manager) Reconcile(now time.Time, distribute func([]byte) error) error {
	m.op.Lock()
	defer m.op.Unlock()
	m.mu.Lock()
	defer m.mu.Unlock()
	prune := m.state.Previous != "" && !now.Before(m.state.ActivatedAt.Add(RotationGrace))
	bundle, err := m.bundle(true)
	if err != nil {
		return err
	}
	// Network calls use GetClientCertificate, which takes the same mutex.
	m.mu.Unlock()
	err = distribute(bundle)
	m.mu.Lock()
	if err != nil {
		return err
	}
	if prune {
		// Restore dual trust first for nodes rolled back to an old backup. The
		// next connection selects the current identity before removing old trust.
		bundle, err = m.bundle(false)
		if err != nil {
			return err
		}
		m.mu.Unlock()
		err = distribute(bundle)
		m.mu.Lock()
		if err != nil {
			return err
		}
	}
	if m.state.Pending != "" {
		candidate, err := m.read(m.state.Pending)
		if err != nil {
			return err
		}
		if !now.Before(candidate.ca.NotAfter) {
			return errors.New("pending CA expired; cannot activate")
		}
		pending := m.state.Pending
		// An offline node can delay activation for months; renew the candidate.
		if candidate.pair.Leaf.NotAfter.Sub(now) <= ClientRenewBefore {
			if err = issueClient(candidate, now); err != nil {
				return err
			}
			id, err := m.store(candidate)
			if err != nil {
				return err
			}
			pending = id
		}
		return m.commit(state{Current: pending, Previous: m.state.Current, ActivatedAt: now})
	}
	if prune {
		next := m.state
		next.Previous = ""
		next.ActivatedAt = time.Time{}
		return m.commit(next)
	}
	return nil
}

func (m *Manager) GetClientCertificate(request *tls.CertificateRequestInfo) (*tls.Certificate, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, id := range []string{m.state.Current, m.state.Previous} {
		if id == "" {
			continue
		}
		entry, err := m.read(id)
		if err != nil {
			return nil, err
		}
		if err = request.SupportsCertificate(&entry.pair); err == nil {
			return &entry.pair, nil
		}
	}
	return nil, errors.New("node does not trust a managed control-plane identity")
}

func (m *Manager) read(id string) (*identity, error) {
	return readIdentity(filepath.Join(m.dir, "generations", id))
}

func readIdentity(dir string) (*identity, error) {
	i := &identity{}
	var err error
	for name, target := range map[string]*[]byte{"client-ca.crt": &i.caPEM, "client-ca.key": &i.keyPEM, "client.crt": &i.certPEM, "client.key": &i.clientKeyPEM} {
		*target, err = os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, err
		}
	}
	i.ca, err = parseCertificate(i.caPEM)
	if err != nil {
		return nil, err
	}
	if !i.ca.IsCA || !i.ca.BasicConstraintsValid || (i.ca.KeyUsage != 0 && i.ca.KeyUsage&x509.KeyUsageCertSign == 0) {
		return nil, errors.New("invalid control CA")
	}
	caPair, err := tls.X509KeyPair(i.caPEM, i.keyPEM)
	if err != nil {
		return nil, err
	}
	var ok bool
	i.caKey, ok = caPair.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, errors.New("unsupported CA private key")
	}
	i.pair, err = tls.X509KeyPair(i.certPEM, i.clientKeyPEM)
	if err != nil {
		return nil, err
	}
	i.pair.Leaf, err = x509.ParseCertificate(i.pair.Certificate[0])
	if err != nil {
		return nil, err
	}
	if err = i.pair.Leaf.CheckSignatureFrom(i.ca); err != nil {
		return nil, err
	}
	return i, nil
}

func parseCertificate(data []byte) (*x509.Certificate, error) {
	block, _ := pem.Decode(data)
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, errors.New("invalid certificate PEM")
	}
	return x509.ParseCertificate(block.Bytes)
}

func createIdentity(now time.Time) (*identity, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	serial, err := randomSerial()
	if err != nil {
		return nil, err
	}
	ca := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "TrojanPanel Next Control CA " + serial.Text(16)}, NotBefore: now.Add(-5 * time.Minute), NotAfter: now.Add(3650 * 24 * time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign}
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		return nil, err
	}
	ca, err = x509.ParseCertificate(der)
	if err != nil {
		return nil, err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return nil, err
	}
	i := &identity{ca: ca, caKey: key, caPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), keyPEM: pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})}
	if err = issueClient(i, now); err != nil {
		return nil, err
	}
	return i, nil
}

func issueClient(i *identity, now time.Time) error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, err := randomSerial()
	if err != nil {
		return err
	}
	expires := now.Add(825 * 24 * time.Hour)
	if expires.After(i.ca.NotAfter) {
		expires = i.ca.NotAfter
	}
	leaf := &x509.Certificate{SerialNumber: serial, Subject: pkix.Name{CommonName: "trojanpanelnext-control-plane"}, NotBefore: now.Add(-5 * time.Minute), NotAfter: expires, KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, leaf, i.ca, &key.PublicKey, i.caKey)
	if err != nil {
		return err
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	i.certPEM = pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	i.clientKeyPEM = pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	i.pair, err = tls.X509KeyPair(i.certPEM, i.clientKeyPEM)
	if err != nil {
		return err
	}
	i.pair.Leaf, err = x509.ParseCertificate(der)
	return err
}

func randomSerial() (*big.Int, error) {
	return rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
}

func (m *Manager) store(i *identity) (string, error) {
	idBytes := make([]byte, 16)
	if _, err := rand.Read(idBytes); err != nil {
		return "", err
	}
	id := hex.EncodeToString(idBytes)
	dir := filepath.Join(m.dir, "generations", id)
	if err := os.Mkdir(dir, 0700); err != nil {
		return "", err
	}
	for name, data := range map[string][]byte{"client-ca.crt": i.caPEM, "client-ca.key": i.keyPEM, "client.crt": i.certPEM, "client.key": i.clientKeyPEM} {
		if err := atomicWrite(filepath.Join(dir, name), data, 0600); err != nil {
			return "", err
		}
	}
	parent, err := os.Open(filepath.Dir(dir))
	if err != nil {
		return "", err
	}
	err = parent.Sync()
	parent.Close()
	if err != nil {
		return "", err
	}
	return id, nil
}

func (m *Manager) bundle(includePrevious bool) ([]byte, error) {
	var bundle []byte
	for _, id := range []string{m.state.Current, m.state.Pending} {
		if id == "" {
			continue
		}
		i, err := m.read(id)
		if err != nil {
			return nil, err
		}
		bundle = append(bundle, i.caPEM...)
	}
	if includePrevious && m.state.Previous != "" {
		i, err := m.read(m.state.Previous)
		if err != nil {
			return nil, err
		}
		bundle = append(bundle, i.caPEM...)
	}
	return bundle, nil
}

func (m *Manager) commit(next state) error {
	content, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(m.dir, "state.json"), content, 0600); err != nil {
		return err
	}
	m.state = next
	if err = m.export(); err != nil {
		return err
	}
	entries, err := os.ReadDir(filepath.Join(m.dir, "generations"))
	if err != nil {
		return err
	}
	for _, entry := range entries {
		id := entry.Name()
		if id != next.Current && id != next.Pending && id != next.Previous {
			if err = os.RemoveAll(filepath.Join(m.dir, "generations", id)); err != nil {
				return err
			}
		}
	}
	return nil
}

// Compatibility exports keep installer backups and bootstrap CA copies current.
func (m *Manager) export() error {
	bundle, err := m.bundle(true)
	if err != nil {
		return err
	}
	if err = atomicWrite(filepath.Join(m.dir, "client-ca.crt"), bundle, 0644); err != nil {
		return err
	}
	for name, target := range map[string]string{"current": filepath.Join("generations", m.state.Current), "client-ca.key": "current/client-ca.key", "client.crt": "current/client.crt", "client.key": "current/client.key"} {
		if err = atomicSymlink(filepath.Join(m.dir, name), target); err != nil {
			return err
		}
	}
	return nil
}

func atomicSymlink(path, target string) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".link-*")
	if err != nil {
		return err
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	defer os.Remove(name)
	if err = os.Symlink(target, name); err != nil {
		return err
	}
	return os.Rename(name, path)
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".pki-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(mode); err == nil {
		_, err = f.Write(content)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(f.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

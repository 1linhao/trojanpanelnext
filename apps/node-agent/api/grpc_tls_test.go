package api

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"
	"trojan-panel-core/core"
)

func TestGRPCMTLSAcceptsOnlyTrustedClientAndServerName(t *testing.T) {
	dir := t.TempDir()
	caCert, caKey, caPEM := createTestCA(t, "controller-ca")
	serverCert, serverKey := issueTestCertificate(t, caCert, caKey, "node.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCert, clientKey := issueTestCertificate(t, caCert, caKey, "controller", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	otherCA, otherKey, _ := createTestCA(t, "other-ca")
	untrustedCert, untrustedKey := issueTestCertificate(t, otherCA, otherKey, "untrusted", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})

	serverCertPath, serverKeyPath := writeTestPair(t, dir, "server", serverCert, serverKey)
	caPath := filepath.Join(dir, "ca.crt")
	if err := os.WriteFile(caPath, caPEM, 0600); err != nil {
		t.Fatal(err)
	}

	oldCert := core.Config.CertConfig
	oldGRPC := core.Config.GrpcConfig
	t.Cleanup(func() {
		core.Config.CertConfig = oldCert
		core.Config.GrpcConfig = oldGRPC
	})
	core.Config.CertConfig.CrtPath = serverCertPath
	core.Config.CertConfig.KeyPath = serverKeyPath
	core.Config.GrpcConfig.ClientCAPath = caPath
	serverConfig, err := grpcTLSConfig()
	if err != nil {
		t.Fatal(err)
	}

	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(caPEM)
	trustedPair := tls.Certificate{Certificate: [][]byte{clientCert.Raw}, PrivateKey: clientKey}
	untrustedPair := tls.Certificate{Certificate: [][]byte{untrustedCert.Raw}, PrivateKey: untrustedKey}

	assertTLSHandshake(t, serverConfig, &tls.Config{
		MinVersion: tls.VersionTLS12, ServerName: "node.test",
		RootCAs: roots, Certificates: []tls.Certificate{trustedPair},
	}, true)
	assertTLSHandshake(t, serverConfig, &tls.Config{
		MinVersion: tls.VersionTLS12, ServerName: "wrong.test",
		RootCAs: roots, Certificates: []tls.Certificate{trustedPair},
	}, false)
	assertTLSHandshake(t, serverConfig, &tls.Config{
		MinVersion: tls.VersionTLS12, ServerName: "node.test", RootCAs: roots,
	}, false)
	assertTLSHandshake(t, serverConfig, &tls.Config{
		MinVersion: tls.VersionTLS12, ServerName: "node.test",
		RootCAs: roots, Certificates: []tls.Certificate{untrustedPair},
	}, false)
}

func assertTLSHandshake(t *testing.T, serverConfig, clientConfig *tls.Config, success bool) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	serverResult := make(chan error, 1)
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			serverResult <- acceptErr
			return
		}
		defer connection.Close()
		serverResult <- tls.Server(connection, serverConfig).Handshake()
	}()
	client, clientErr := tls.Dial("tcp", listener.Addr().String(), clientConfig)
	if client != nil {
		_ = client.Close()
	}
	serverErr := <-serverResult
	if success && (clientErr != nil || serverErr != nil) {
		t.Fatalf("trusted mTLS handshake failed: client=%v server=%v", clientErr, serverErr)
	}
	if !success && clientErr == nil && serverErr == nil {
		t.Fatal("invalid mTLS handshake unexpectedly succeeded")
	}
}

func createTestCA(t *testing.T, commonName string) (*x509.Certificate, *rsa.PrivateKey, []byte) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		NotBefore:    time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, key, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func issueTestCertificate(t *testing.T, ca *x509.Certificate, caKey *rsa.PrivateKey, commonName string, usage []x509.ExtKeyUsage) (*x509.Certificate, *rsa.PrivateKey) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: commonName},
		DNSNames:     []string{commonName},
		NotBefore:    time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		KeyUsage:    x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage: usage,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, ca, &key.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return certificate, key
}

func writeTestPair(t *testing.T, dir, name string, certificate *x509.Certificate, key *rsa.PrivateKey) (string, string) {
	t.Helper()
	certPath := filepath.Join(dir, name+".crt")
	keyPath := filepath.Join(dir, name+".key")
	if err := os.WriteFile(certPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)}), 0600); err != nil {
		t.Fatal(err)
	}
	return certPath, keyPath
}

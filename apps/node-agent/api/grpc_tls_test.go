package api

import (
	"context"
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

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"trojan-panel-core/bootstrap"
	"trojan-panel-core/core"
)

func TestTrustedWebMTLSStateProbeMarksBootstrapReady(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TP_NODE_BOOTSTRAP_MARKER", filepath.Join(dir, "bootstrap-ready.json"))
	caCert, caKey, caPEM := createTestCA(t, "controller-ca")
	serverCert, serverKey := issueTestCertificate(t, caCert, caKey, "node.test", []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth})
	clientCert, clientKey := issueTestCertificate(t, caCert, caKey, "controller", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	serverCertPath, serverKeyPath := writeTestPair(t, dir, "server", serverCert, serverKey)
	caPath := filepath.Join(dir, "client-ca.crt")
	if err := os.WriteFile(caPath, caPEM, 0600); err != nil {
		t.Fatal(err)
	}

	oldCert := core.Config.CertConfig
	oldGRPC := core.Config.GrpcConfig
	oldNode := core.Config.NodeConfig
	t.Cleanup(func() {
		core.Config.CertConfig = oldCert
		core.Config.GrpcConfig = oldGRPC
		core.Config.NodeConfig = oldNode
	})
	core.Config.CertConfig.CrtPath = serverCertPath
	core.Config.CertConfig.KeyPath = serverKeyPath
	core.Config.GrpcConfig.TLSMode = "mtls"
	core.Config.GrpcConfig.ClientCAPath = caPath
	core.Config.NodeConfig = core.NodeConfig{
		ServerID: 42, IdentityID: "11111111-2222-4333-8444-555555555555", IdentityGeneration: 7,
	}
	serverTLS, err := grpcTLSConfig()
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := grpc.NewServer(grpc.Creds(credentials.NewTLS(serverTLS)))
	RegisterApiStateServiceServer(server, new(StateApiServer))
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() {
		server.Stop()
		_ = listener.Close()
	})

	roots := x509.NewCertPool()
	roots.AppendCertsFromPEM(caPEM)
	clientTLS := &tls.Config{
		MinVersion: tls.VersionTLS12, ServerName: "node.test", RootCAs: roots,
		Certificates: []tls.Certificate{{Certificate: [][]byte{clientCert.Raw}, PrivateKey: clientKey}},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	connection, err := grpc.DialContext(ctx, listener.Addr().String(),
		grpc.WithTransportCredentials(credentials.NewTLS(clientTLS)), grpc.WithBlock())
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	response, err := NewApiStateServiceClient(connection).GetNodeServerState(ctx, &NodeServerStateDto{})
	if err != nil || !response.Success {
		t.Fatalf("trusted Web state probe failed: response=%v error=%v", response, err)
	}
	if !bootstrap.Ready() {
		t.Fatal("trusted Web mTLS/gRPC state probe did not mark the Node bootstrap ready")
	}
}

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

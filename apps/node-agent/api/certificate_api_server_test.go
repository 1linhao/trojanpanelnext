package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"os"
	"path/filepath"
	"testing"
	"time"
	"trojan-panel-core/core"
)

func TestTrustUpdateRequiresMTLSAndPreservesAuthenticatedIssuer(t *testing.T) {
	ca, key, bundle := createTestCA(t, "old")
	leaf, _ := issueTestCertificate(t, ca, key, "controller", []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth})
	_, _, next := createTestCA(t, "new")
	old := core.Config.GrpcConfig
	t.Cleanup(func() { core.Config.GrpcConfig = old })
	core.Config.GrpcConfig.TLSMode = "mtls"
	path := filepath.Join(t.TempDir(), "client-ca.crt")
	core.Config.GrpcConfig.ClientCAPath = path
	bootstrap := filepath.Join(t.TempDir(), "client-ca.crt")
	t.Setenv("TP_PKI_BOOTSTRAP_CA_PATH", bootstrap)
	if err := os.WriteFile(path, bundle, 0644); err != nil {
		t.Fatal(err)
	}
	server := new(CertificateApiServer)
	request := &ClientTrustRequest{CaBundle: next}
	response, _ := server.UpdateClientTrust(context.Background(), request)
	if response.Success {
		t.Fatal("unauthenticated update accepted")
	}
	ctx := peer.NewContext(context.Background(), &peer.Peer{AuthInfo: credentials.TLSInfo{State: tls.ConnectionState{PeerCertificates: []*x509.Certificate{leaf}, VerifiedChains: [][]*x509.Certificate{{leaf, ca}}}}})
	response, _ = server.UpdateClientTrust(ctx, request)
	if response.Success {
		t.Fatal("update removed the caller's issuer")
	}
	request.CaBundle = append(append([]byte{}, bundle...), next...)
	response, _ = server.UpdateClientTrust(ctx, request)
	if !response.Success {
		t.Fatal(response.Msg)
	}
	got, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(got, request.CaBundle) {
		t.Fatal("bundle not saved")
	}
	if copy, err := os.ReadFile(bootstrap); err != nil || !bytes.Equal(copy, got) {
		t.Fatal("bootstrap CA copy not updated")
	}
	request.CaBundle = append(request.CaBundle, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("invalid")})...)
	response, _ = server.UpdateClientTrust(ctx, request)
	if response.Success {
		t.Fatal("private key accepted in public trust bundle")
	}
	if _, err = clientTrustPool([]byte("invalid"), time.Now()); err == nil {
		t.Fatal("invalid bundle accepted")
	}
	ca.KeyUsage = 0
	der, err := x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = clientTrustPool(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), time.Now()); err != nil {
		t.Fatalf("legacy installer CA rejected: %v", err)
	}
	ca.NotAfter = time.Now().Add(-time.Minute)
	der, err = x509.CreateCertificate(rand.Reader, ca, ca, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	if roots, err := clientTrustPool(append(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), next...), time.Now()); err != nil || len(roots.Subjects()) != 1 {
		t.Fatalf("expired previous CA disabled current trust: %v", err)
	}
}

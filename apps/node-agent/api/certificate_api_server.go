package api

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"trojan-panel-core/core"
)

var trustUpdate sync.Mutex

type CertificateApiServer struct {
	UnimplementedApiCertificateServiceServer
}

func (s *CertificateApiServer) UpdateClientTrust(ctx context.Context, request *ClientTrustRequest) (*Response, error) {
	if !strings.EqualFold(core.Config.GrpcConfig.TLSMode, "mtls") {
		return &Response{Msg: "trust updates require mTLS"}, nil
	}
	if err := authRequest(ctx); err != nil {
		return &Response{Msg: err.Error()}, nil
	}
	pool, err := clientTrustPool(request.CaBundle, time.Now())
	if err != nil {
		return &Response{Msg: err.Error()}, nil
	}
	remote, _ := peer.FromContext(ctx)
	info := remote.AuthInfo.(credentials.TLSInfo)
	intermediates := x509.NewCertPool()
	for _, cert := range info.State.PeerCertificates[1:] {
		intermediates.AddCert(cert)
	}
	// A caller cannot remove its own trust before the new identity is in use.
	if _, err = info.State.PeerCertificates[0].Verify(x509.VerifyOptions{Roots: pool, Intermediates: intermediates, KeyUsages: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}); err != nil {
		return &Response{Msg: "bundle does not trust the authenticated client"}, nil
	}
	trustUpdate.Lock()
	defer trustUpdate.Unlock()
	path := core.Config.GrpcConfig.ClientCAPath
	old, err := os.ReadFile(path)
	if err != nil {
		return &Response{Msg: err.Error()}, nil
	}
	if !bytes.Equal(old, request.CaBundle) {
		if err = writeTrustBundle(path, request.CaBundle); err != nil {
			return &Response{Msg: err.Error()}, nil
		}
	}
	if bootstrap := os.Getenv("TP_PKI_BOOTSTRAP_CA_PATH"); bootstrap != "" && bootstrap != path {
		data, readErr := os.ReadFile(bootstrap)
		if readErr != nil && !errors.Is(readErr, os.ErrNotExist) {
			return &Response{Msg: readErr.Error()}, nil
		}
		if !bytes.Equal(data, request.CaBundle) {
			if err = writeTrustBundle(bootstrap, request.CaBundle); err != nil {
				return &Response{Msg: err.Error()}, nil
			}
		}
	}
	return &Response{Success: true}, nil
}

func clientTrustPool(data []byte, now time.Time) (*x509.CertPool, error) {
	if len(data) == 0 || len(data) > 64*1024 {
		return nil, errors.New("invalid CA bundle size")
	}
	pool := x509.NewCertPool()
	count := 0
	valid := 0
	for len(bytes.TrimSpace(data)) > 0 {
		if !bytes.HasPrefix(bytes.TrimSpace(data), []byte("-----BEGIN CERTIFICATE-----")) {
			return nil, errors.New("CA bundle must contain only certificates")
		}
		block, rest := pem.Decode(data)
		if block == nil || block.Type != "CERTIFICATE" {
			return nil, errors.New("CA bundle must contain only certificates")
		}
		cert, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		if !cert.IsCA || !cert.BasicConstraintsValid || (cert.KeyUsage != 0 && cert.KeyUsage&x509.KeyUsageCertSign == 0) {
			return nil, errors.New("CA bundle contains an invalid CA")
		}
		if err = cert.CheckSignatureFrom(cert); err != nil {
			return nil, fmt.Errorf("control CA must be self-signed: %w", err)
		}
		// An expired previous CA must not disable a still-valid current CA.
		if !now.Before(cert.NotBefore) && now.Before(cert.NotAfter) {
			pool.AddCert(cert)
			valid++
		}
		count++
		if count > 4 {
			return nil, errors.New("too many control CAs")
		}
		data = rest
	}
	if valid == 0 {
		return nil, errors.New("CA bundle contains no currently valid certificates")
	}
	return pool, nil
}

func writeTrustBundle(path string, data []byte) error {
	file, err := os.CreateTemp(filepath.Dir(path), ".client-ca-*")
	if err != nil {
		return err
	}
	defer os.Remove(file.Name())
	if err = file.Chmod(0644); err == nil {
		_, err = file.Write(data)
	}
	if err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err = os.Rename(file.Name(), path); err != nil {
		return err
	}
	dir, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}

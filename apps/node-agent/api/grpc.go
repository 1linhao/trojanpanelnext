package api

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"net"
	"os"
	"strings"
	"trojan-panel-core/core"
)

func InitGrpcServer() {
	if err := initKernelManager(); err != nil {
		panic(fmt.Sprintf("kernel manager init err: %v", err))
	}
	go func() {
		grpcConfig := core.Config.GrpcConfig
		options := make([]grpc.ServerOption, 0, 1)
		switch strings.ToLower(grpcConfig.TLSMode) {
		case "mtls":
			tlsConfig, err := grpcTLSConfig()
			if err != nil {
				panic(fmt.Sprintf("gRPC mTLS init err: %v", err))
			}
			options = append(options, grpc.Creds(credentials.NewTLS(tlsConfig)))
		case "", "legacy":
		default:
			panic(fmt.Sprintf("unsupported gRPC TLS mode %q", grpcConfig.TLSMode))
		}
		rpcServer := grpc.NewServer(options...)
		RegisterApiNodeServiceServer(rpcServer, new(NodeApiServer))
		RegisterApiAccountServiceServer(rpcServer, new(AccountApiServer))
		RegisterApiStateServiceServer(rpcServer, new(StateApiServer))
		RegisterApiNodeServerServiceServer(rpcServer, new(NodeServerApiServer))
		RegisterApiKernelServiceServer(rpcServer, new(KernelApiServer))
		listener, err := net.Listen("tcp", fmt.Sprintf(":%s", grpcConfig.Port))
		if err != nil {
			panic(fmt.Sprintf("gRPC service listening port err: %v", err))
		}
		_ = rpcServer.Serve(listener)
	}()
}

func grpcTLSConfig() (*tls.Config, error) {
	certConfig := core.Config.CertConfig
	clientCAPath := core.Config.GrpcConfig.ClientCAPath
	clientCA, err := os.ReadFile(clientCAPath)
	if err != nil {
		return nil, fmt.Errorf("read client CA: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(clientCA) {
		return nil, fmt.Errorf("client CA contains no certificates")
	}
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  pool,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			certificate, loadErr := tls.LoadX509KeyPair(certConfig.CrtPath, certConfig.KeyPath)
			if loadErr != nil {
				return nil, loadErr
			}
			return &certificate, nil
		},
	}, nil
}

package core

import (
	"fmt"
	"time"
)

func UpdateClientTrust(ip string, port uint, transport NodeTransport, bundle []byte) error {
	if transport.Mode != "mtls" {
		return fmt.Errorf("trust updates require mTLS")
	}
	connection, ctx, closeConnection, err := newGrpcInstance("", ip, port, 10*time.Second, transport)
	defer closeConnection()
	if err != nil {
		return err
	}
	response, err := NewApiCertificateServiceClient(connection).UpdateClientTrust(ctx, &ClientTrustRequest{CaBundle: bundle})
	if err != nil {
		return err
	}
	if !response.Success {
		return fmt.Errorf("node rejected trust update: %s", response.Msg)
	}
	return nil
}

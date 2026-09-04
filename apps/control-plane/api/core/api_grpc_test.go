package core

import (
	"testing"
	"time"
)

func TestGRPCTransportNeverDowngradesUnknownMode(t *testing.T) {
	_, _, closeConnection, err := newGrpcInstance(
		"", "127.0.0.1", 1, time.Millisecond,
		NodeTransport{Mode: "unexpected"},
	)
	closeConnection()
	if err == nil {
		t.Fatal("unknown transport mode was silently downgraded")
	}
}

func TestGRPCMTLSRequiresServerName(t *testing.T) {
	_, _, closeConnection, err := newGrpcInstance(
		"", "127.0.0.1", 1, time.Millisecond,
		NodeTransport{Mode: "mtls"},
	)
	closeConnection()
	if err == nil {
		t.Fatal("mTLS without a certificate server name was accepted")
	}
}

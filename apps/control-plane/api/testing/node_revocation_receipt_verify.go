package main

import (
	"fmt"
	"os"
	"strconv"

	"trojanpanelnext/revocationreceipt"
)

// This test-only command verifies CLI output through the same public offline
// verifier used by the standalone Node removal path.
func main() {
	if len(os.Args) != 7 {
		fmt.Fprintln(os.Stderr, "usage: verify public-key receipt identity-id server-id generation expected-status")
		os.Exit(2)
	}
	serverID, err := strconv.ParseUint(os.Args[4], 10, 64)
	if err != nil {
		fail()
	}
	generation, err := strconv.ParseUint(os.Args[5], 10, 64)
	if err != nil {
		fail()
	}
	publicData, err := os.ReadFile(os.Args[1])
	if err != nil {
		fail()
	}
	public, err := revocationreceipt.ParsePublicKey(publicData)
	if err != nil {
		fail()
	}
	receipt, err := os.ReadFile(os.Args[2])
	if err != nil {
		fail()
	}
	claims, err := revocationreceipt.Verify(receipt, public, revocationreceipt.Node{
		IdentityID: os.Args[3], ServerID: serverID, Generation: generation,
	})
	if err != nil || string(claims.Status) != os.Args[6] {
		fail()
	}
}

func fail() {
	fmt.Fprintln(os.Stderr, "revocation receipt verification failed")
	os.Exit(1)
}

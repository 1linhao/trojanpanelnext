// Command receipt_fixture creates deterministic test-only Ed25519 receipts.
package main

import (
	"bytes"
	"crypto/ed25519"
	"flag"
	"fmt"
	"os"

	"trojanpanelnext/revocationreceipt"
)

func main() {
	var publicPath, receiptPath, identity, status string
	var server, generation, seed int
	flag.StringVar(&publicPath, "public", "", "public key output")
	flag.StringVar(&receiptPath, "receipt", "", "receipt output")
	flag.StringVar(&identity, "identity", "11111111-2222-4333-8444-555555555555", "identity")
	flag.StringVar(&status, "status", "revoked", "terminal status")
	flag.IntVar(&server, "server", 42, "server ID")
	flag.IntVar(&generation, "through", 7, "revocation generation bound")
	flag.IntVar(&seed, "seed", 7, "test signing seed")
	flag.Parse()
	if seed < 0 || seed > 255 || publicPath == "" && receiptPath == "" {
		panic("invalid fixture arguments")
	}
	private := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{byte(seed)}, ed25519.SeedSize))
	if publicPath != "" {
		encoded, err := revocationreceipt.EncodePublicKey(private.Public().(ed25519.PublicKey))
		must(err)
		must(os.WriteFile(publicPath, encoded, 0600))
	}
	if receiptPath != "" {
		receipt, err := revocationreceipt.Issue(private, revocationreceipt.Claims{
			IdentityID: identity, ServerID: uint64(server),
			RevokedThroughGeneration: uint64(generation), Status: revocationreceipt.Status(status),
		})
		must(err)
		must(os.WriteFile(receiptPath, receipt, 0600))
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

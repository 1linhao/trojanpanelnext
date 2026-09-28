package revocationreceipt

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"
)

const testIdentity = "12345678-1234-1234-1234-123456789abc"

func testKey() ed25519.PrivateKey {
	seed := make([]byte, ed25519.SeedSize)
	for i := range seed {
		seed[i] = byte(i)
	}
	return ed25519.NewKeyFromSeed(seed)
}

func testClaims() Claims {
	return Claims{IdentityID: testIdentity, ServerID: 42, RevokedThroughGeneration: 7, Status: Revoked}
}

func testNode() Node {
	return Node{IdentityID: testIdentity, ServerID: 42, Generation: 7}
}

func TestGoldenRoundTripAndGenerationCeiling(t *testing.T) {
	private := testKey()
	public := private.Public().(ed25519.PublicKey)
	encodedPublic, err := EncodePublicKey(public)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParsePublicKey(encodedPublic)
	if err != nil || !bytes.Equal(parsed, public) {
		t.Fatalf("public key round trip: %v", err)
	}
	receipt, err := Issue(private, testClaims())
	if err != nil {
		t.Fatal(err)
	}
	// Changes to the wire format or protocol domain require an explicit review.
	sum := sha256.Sum256(receipt)
	const goldenReceiptSHA256 = "fc53ed6350a4e6903f64f56156428c06ee11431243fd53a5f2badd4c8ea349aa"
	if got := hex.EncodeToString(sum[:]); got != goldenReceiptSHA256 {
		t.Fatalf("receipt wire format changed: %s", got)
	}
	for _, generation := range []uint64{1, 6, 7} {
		local := testNode()
		local.Generation = generation
		claims, err := Verify(receipt, parsed, local)
		if err != nil || claims.Status != Revoked || claims.RevokedThroughGeneration != 7 || claims.Version != Version {
			t.Fatalf("generation %d: claims=%+v err=%v", generation, claims, err)
		}
	}
	local := testNode()
	local.Generation = 8
	assertRejected(t, receipt, parsed, local)
	assertRejected(t, receipt, parsed, Node{IdentityID: testIdentity, ServerID: 42, Generation: 0})
	assertRejected(t, receipt, parsed, Node{IdentityID: testIdentity, ServerID: 0, Generation: 7})
}

func TestWrongTrustAndTampering(t *testing.T) {
	private := testKey()
	public := private.Public().(ed25519.PublicKey)
	receipt, err := Issue(private, testClaims())
	if err != nil {
		t.Fatal(err)
	}
	otherSeed := bytes.Repeat([]byte{9}, ed25519.SeedSize)
	otherPublic := ed25519.NewKeyFromSeed(otherSeed).Public().(ed25519.PublicKey)
	assertRejected(t, receipt, otherPublic, testNode())
	assertRejected(t, receipt, public, Node{IdentityID: "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa", ServerID: 42, Generation: 7})
	assertRejected(t, receipt, public, Node{IdentityID: testIdentity, ServerID: 43, Generation: 7})
	tampered := append([]byte(nil), receipt...)
	for i, character := range tampered {
		if character == 'A' {
			tampered[i] = 'B'
			break
		}
	}
	assertRejected(t, tampered, public, testNode())
	assertRejected(t, append(receipt, 'x'), public, testNode())
	assertRejected(t, bytes.Repeat([]byte{'x'}, MaxReceiptSize+1), public, testNode())
	assertRejected(t, []byte(`{"payload":"","signature":""}`), public, testNode())
	assertRejected(t, []byte(`{"payload":"x","payload":"x","signature":"x"}`), public, testNode())
	assertRejected(t, []byte(`{"payload":"x","signature":"x","public_key":"attacker"}`), public, testNode())
	assertRejected(t, signedRaw(private, append(bytes.Repeat([]byte{' '}, maxPayloadSize), ' '), domain), public, testNode())
}

func TestSignedInvalidClaimsAndProtocolIsolation(t *testing.T) {
	private := testKey()
	public := private.Public().(ed25519.PublicKey)
	base := testClaims()
	base.Version = Version
	base.IssuerFingerprint, _ = Fingerprint(public)
	cases := map[string]func(*Claims){
		"wrong issuer":       func(c *Claims) { c.IssuerFingerprint = "sha256:" + strings.Repeat("0", 64) },
		"wrong version":      func(c *Claims) { c.Version = Version + 1 },
		"wrong identity":     func(c *Claims) { c.IdentityID = "AAAAAAAA-AAAA-AAAA-AAAA-AAAAAAAAAAAA" },
		"zero server":        func(c *Claims) { c.ServerID = 0 },
		"zero generation":    func(c *Claims) { c.RevokedThroughGeneration = 0 },
		"nonterminal status": func(c *Claims) { c.Status = "active" },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			claims := base
			change(&claims)
			payload, _ := json.Marshal(claims)
			assertRejected(t, signedRaw(private, payload, domain), public, testNode())
		})
	}
	for _, status := range []Status{Revoked, Evicted} {
		claims := testClaims()
		claims.Status = status
		if _, err := Issue(private, claims); err != nil {
			t.Fatalf("terminal status %s: %v", status, err)
		}
	}
	for _, invalid := range []string{
		`{"version":1,"version":1,"issuer_fingerprint":"x","identity_id":"x","server_id":1,"revoked_through_generation":1,"status":"revoked"}`,
		`{"version":1,"issuer_fingerprint":"x","identity_id":"x","server_id":1,"revoked_through_generation":1,"status":"revoked","extra":true}`,
		`{"version":1,"issuer_fingerprint":"x","identity_id":"x","server_id":1,"revoked_through_generation":1}`,
		`[]`,
	} {
		assertRejected(t, signedRaw(private, []byte(invalid), domain), public, testNode())
	}
	validPayload, _ := json.Marshal(base)
	assertRejected(t, signedRaw(private, validPayload, "trojanpanelnext/other-purpose/v1\x00"), public, testNode())
	assertRejected(t, signedRaw(private, validPayload, ""), public, testNode())
	if _, err := Issue(private, Claims{IdentityID: testIdentity, ServerID: 42, RevokedThroughGeneration: 7, Status: "active"}); err == nil {
		t.Fatal("Issue accepted nonterminal status")
	}
}

func TestMalformedKeysAndErrorsAreNonSensitive(t *testing.T) {
	private := testKey()
	receipt, _ := Issue(private, testClaims())
	badPrivate := append(ed25519.PrivateKey(nil), private...)
	badPrivate[len(badPrivate)-1] ^= 1
	if _, err := Issue(badPrivate, testClaims()); err == nil {
		t.Fatal("Issue accepted inconsistent private key")
	}
	if _, err := ParsePublicKey([]byte("secret-material")); err == nil {
		t.Fatal("parsed malformed public key")
	}
	encoded, _ := EncodePublicKey(private.Public().(ed25519.PublicKey))
	if _, err := ParsePublicKey(append(encoded, '\n')); err == nil {
		t.Fatal("accepted noncanonical public key")
	}
	_, err := Verify(receipt, ed25519.PublicKey([]byte("secret-material")), testNode())
	if err == nil || strings.Contains(err.Error(), "secret-material") || strings.Contains(err.Error(), testIdentity) {
		t.Fatalf("unsafe verification error: %v", err)
	}
}

func signedRaw(private ed25519.PrivateKey, payload []byte, prefix string) []byte {
	encoded, _ := json.Marshal(envelope{
		Payload:   base64.RawURLEncoding.EncodeToString(payload),
		Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, append([]byte(prefix), payload...))),
	})
	return encoded
}

func assertRejected(t *testing.T, receipt []byte, public ed25519.PublicKey, local Node) {
	t.Helper()
	if _, err := Verify(receipt, public, local); err == nil {
		t.Fatalf("accepted invalid receipt %q", receipt)
	}
}

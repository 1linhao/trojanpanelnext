// Package revocationreceipt implements the offline Web-issued Node revocation
// proof shared by the Web signer and the standalone Node removal verifier.
package revocationreceipt

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const (
	Version        = 1
	MaxReceiptSize = 4096
	maxPayloadSize = 1024
	domain         = "trojanpanelnext/node-revocation-receipt/v1\x00"
	publicPrefix   = "TPNEXT-REVOCATION-ED25519-V1:"
)

type Status string

const (
	Revoked Status = "revoked"
	Evicted Status = "evicted"
)

// Claims are the signed facts. IssuerFingerprint is set from the signing key
// by Issue; callers must not choose it. A receipt grants removal only.
type Claims struct {
	Version                  int    `json:"version"`
	IssuerFingerprint        string `json:"issuer_fingerprint"`
	IdentityID               string `json:"identity_id"`
	ServerID                 uint64 `json:"server_id"`
	RevokedThroughGeneration uint64 `json:"revoked_through_generation"`
	Status                   Status `json:"status"`
}

// Node identifies the installation requesting local removal.
type Node struct {
	IdentityID string
	ServerID   uint64
	Generation uint64
}

type envelope struct {
	Payload   string `json:"payload"`
	Signature string `json:"signature"`
}

var errInvalid = errors.New("invalid Node revocation receipt")

// Fingerprint identifies a trusted Ed25519 public key, not a TLS certificate.
func Fingerprint(public ed25519.PublicKey) (string, error) {
	if len(public) != ed25519.PublicKeySize {
		return "", errors.New("invalid revocation public key")
	}
	sum := sha256.Sum256(public)
	return "sha256:" + hex.EncodeToString(sum[:]), nil
}

// EncodePublicKey serializes the public key for the encrypted bootstrap bundle.
func EncodePublicKey(public ed25519.PublicKey) ([]byte, error) {
	if _, err := Fingerprint(public); err != nil {
		return nil, err
	}
	return []byte(publicPrefix + base64.RawURLEncoding.EncodeToString(public) + "\n"), nil
}

// ParsePublicKey accepts exactly the public key format distributed at install.
func ParsePublicKey(data []byte) (ed25519.PublicKey, error) {
	if len(data) != len(publicPrefix)+base64.RawURLEncoding.EncodedLen(ed25519.PublicKeySize)+1 ||
		!bytes.HasPrefix(data, []byte(publicPrefix)) || data[len(data)-1] != '\n' {
		return nil, errors.New("invalid revocation public key")
	}
	encoded := string(data[len(publicPrefix) : len(data)-1])
	public, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(public) != ed25519.PublicKeySize || base64.RawURLEncoding.EncodeToString(public) != encoded {
		return nil, errors.New("invalid revocation public key")
	}
	return ed25519.PublicKey(public), nil
}

// Issue signs a terminal identity state only after the caller has verified
// successful credential revocation, registration removal, and audit recording.
func Issue(private ed25519.PrivateKey, claims Claims) ([]byte, error) {
	if err := validatePrivateKey(private); err != nil {
		return nil, err
	}
	public := private.Public().(ed25519.PublicKey)
	claims.Version = Version
	claims.IssuerFingerprint, _ = Fingerprint(public)
	if !validClaims(claims) {
		return nil, errInvalid
	}
	payload, err := json.Marshal(claims)
	if err != nil || len(payload) > maxPayloadSize {
		return nil, errInvalid
	}
	signed := append([]byte(domain), payload...)
	result, err := json.Marshal(envelope{
		Payload:   base64.RawURLEncoding.EncodeToString(payload),
		Signature: base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, signed)),
	})
	if err != nil || len(result) > MaxReceiptSize {
		return nil, errInvalid
	}
	return append(result, '\n'), nil
}

// Verify uses only the public key fixed at installation. The receipt may be
// reused for retries and for older local generations of the same identity.
func Verify(data []byte, trustedPublic ed25519.PublicKey, local Node) (Claims, error) {
	var zero Claims
	if len(trustedPublic) != ed25519.PublicKeySize || !validNode(local) || len(data) == 0 || len(data) > MaxReceiptSize {
		return zero, errInvalid
	}
	parts, err := decodeObject(bytes.TrimSuffix(data, []byte("\n")), []string{"payload", "signature"})
	if err != nil {
		return zero, errInvalid
	}
	var encodedPayload, encodedSignature string
	if json.Unmarshal(parts["payload"], &encodedPayload) != nil || json.Unmarshal(parts["signature"], &encodedSignature) != nil {
		return zero, errInvalid
	}
	payload, err := base64.RawURLEncoding.DecodeString(encodedPayload)
	if err != nil || len(payload) == 0 || len(payload) > maxPayloadSize || base64.RawURLEncoding.EncodeToString(payload) != encodedPayload {
		return zero, errInvalid
	}
	signature, err := base64.RawURLEncoding.DecodeString(encodedSignature)
	if err != nil || len(signature) != ed25519.SignatureSize || base64.RawURLEncoding.EncodeToString(signature) != encodedSignature {
		return zero, errInvalid
	}
	if !ed25519.Verify(trustedPublic, append([]byte(domain), payload...), signature) {
		return zero, errInvalid
	}
	fields, err := decodeObject(payload, []string{"version", "issuer_fingerprint", "identity_id", "server_id", "revoked_through_generation", "status"})
	if err != nil {
		return zero, errInvalid
	}
	var claims Claims
	if json.Unmarshal(fields["version"], &claims.Version) != nil ||
		json.Unmarshal(fields["issuer_fingerprint"], &claims.IssuerFingerprint) != nil ||
		json.Unmarshal(fields["identity_id"], &claims.IdentityID) != nil ||
		json.Unmarshal(fields["server_id"], &claims.ServerID) != nil ||
		json.Unmarshal(fields["revoked_through_generation"], &claims.RevokedThroughGeneration) != nil ||
		json.Unmarshal(fields["status"], &claims.Status) != nil || !validClaims(claims) {
		return zero, errInvalid
	}
	fingerprint, _ := Fingerprint(trustedPublic)
	if claims.IssuerFingerprint != fingerprint || claims.IdentityID != local.IdentityID ||
		claims.ServerID != local.ServerID || local.Generation > claims.RevokedThroughGeneration {
		return zero, errInvalid
	}
	return claims, nil
}

func validClaims(claims Claims) bool {
	return claims.Version == Version && len(claims.IssuerFingerprint) == 71 &&
		strings.HasPrefix(claims.IssuerFingerprint, "sha256:") && lowerHex(claims.IssuerFingerprint[7:]) &&
		validUUID(claims.IdentityID) && claims.ServerID > 0 && claims.RevokedThroughGeneration > 0 &&
		(claims.Status == Revoked || claims.Status == Evicted)
}

func validNode(node Node) bool {
	return validUUID(node.IdentityID) && node.ServerID > 0 && node.Generation > 0
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i := 0; i < len(value); i++ {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if value[i] != '-' {
				return false
			}
		} else if !((value[i] >= '0' && value[i] <= '9') || (value[i] >= 'a' && value[i] <= 'f')) {
			return false
		}
	}
	return true
}

func lowerHex(value string) bool {
	for i := 0; i < len(value); i++ {
		if !((value[i] >= '0' && value[i] <= '9') || (value[i] >= 'a' && value[i] <= 'f')) {
			return false
		}
	}
	return true
}

// decodeObject rejects missing, duplicate, and unknown fields at each level.
func decodeObject(data []byte, allowed []string) (map[string]json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	first, err := decoder.Token()
	if err != nil || first != json.Delim('{') {
		return nil, errInvalid
	}
	fields := make(map[string]json.RawMessage, len(allowed))
	for decoder.More() {
		token, err := decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || !contains(allowed, key) {
			return nil, errInvalid
		}
		if _, duplicate := fields[key]; duplicate {
			return nil, errInvalid
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, errInvalid
		}
		fields[key] = value
	}
	end, err := decoder.Token()
	if err != nil || end != json.Delim('}') || len(fields) != len(allowed) {
		return nil, errInvalid
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return nil, errInvalid
	}
	return fields, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

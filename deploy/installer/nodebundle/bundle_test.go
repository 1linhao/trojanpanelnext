package main

import (
	"archive/tar"
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"filippo.io/age"
)

const testPassword = "correct horse battery staple"

func TestCreateAndExtractBundleHasFixedSafeInventory(t *testing.T) {
	root := t.TempDir()
	credentialPath := writeTestCredential(t, root)
	configPath := writeTestConfig(t, root)
	caPath := filepath.Join(root, "client-ca.crt")
	if err := os.WriteFile(caPath, testCAPEM(t), 0644); err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(root, "node.age")
	if err := createBundle(createOptions{
		CredentialPath: credentialPath,
		ConfigPath:     configPath,
		ClientCAPath:   caPath,
		OutputPath:     bundlePath,
	}, []byte(testPassword)); err != nil {
		t.Fatalf("createBundle: %v", err)
	}
	info, err := os.Stat(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0600 {
		t.Fatalf("bundle mode = %o, want 0600", info.Mode().Perm())
	}
	header, err := os.ReadFile(bundlePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(header[:min(len(header), 512)], []byte("-> scrypt ")) {
		t.Fatal("bundle is not an age scrypt envelope")
	}

	contents, manifest, err := decryptAndValidate(bundlePath, []byte(testPassword))
	if err != nil {
		t.Fatalf("decryptAndValidate: %v", err)
	}
	wantNames := []string{"config-node.yaml", "manifest.json", "pki/client-ca.crt"}
	if strings.Join(sortedKeys(contents), ",") != strings.Join(wantNames, ",") {
		t.Fatalf("bundle entries = %v, want %v", sortedKeys(contents), wantNames)
	}
	if manifest.NodeIdentityID != "11111111-2222-4333-8444-555555555555" || manifest.Generation != 7 {
		t.Fatalf("manifest identity = %s generation %d", manifest.NodeIdentityID, manifest.Generation)
	}
	for name := range contents {
		lower := strings.ToLower(name)
		if strings.Contains(lower, "client.key") || strings.Contains(lower, "ca.key") {
			t.Fatalf("private key entry leaked: %s", name)
		}
	}
	config := string(contents["config-node.yaml"])
	for _, expected := range []string{
		"node_identity_id: 11111111-2222-4333-8444-555555555555",
		"node_identity_generation: 7",
		"mariadb_user: tpn_example",
		"redis_username: tpn-cache-example",
		"redis_auth_username: tpn-auth-example",
	} {
		if !strings.Contains(config, expected) {
			t.Fatalf("generated config missing %q:\n%s", expected, config)
		}
	}
}

func TestWrongPasswordDoesNotExtract(t *testing.T) {
	root := t.TempDir()
	bundlePath := createTestBundle(t, root)
	output := filepath.Join(root, "plain")
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	if err := extractBundle(bundlePath, output, []byte("this password is wrong")); err == nil {
		t.Fatal("wrong password unexpectedly extracted bundle")
	}
	entries, err := os.ReadDir(output)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("wrong password left plaintext entries: %v", entries)
	}
}

func TestDecryptRejectsUnexpectedOrUnsafeArchiveEntry(t *testing.T) {
	for _, name := range []string{"client.key", "../escape", "/absolute", "pki/client-ca.key"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			bundlePath := filepath.Join(root, "unsafe.age")
			if err := writeEncryptedArchive(bundlePath, testPassword, map[string][]byte{
				"manifest.json": []byte(`{"schema_version":1}`),
				name:            []byte("PRIVATE"),
			}); err != nil {
				t.Fatal(err)
			}
			if _, _, err := decryptAndValidate(bundlePath, []byte(testPassword)); err == nil {
				t.Fatalf("unsafe entry %q was accepted", name)
			}
		})
	}
}

func TestDecryptRejectsDataAfterTarEndOfArchive(t *testing.T) {
	root := t.TempDir()
	valid := createTestBundle(t, root)
	forged := filepath.Join(root, "trailing-private-key.age")
	rewriteBundleWithTail(t, valid, forged, []byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n"))
	if _, _, err := decryptAndValidate(forged, []byte(testPassword)); err == nil {
		t.Fatal("bundle with private-key data after tar end-of-archive was accepted")
	}
}

func TestDecryptRevalidatesCompleteNodeConfigurationSchema(t *testing.T) {
	tests := map[string]func([]byte) []byte{
		"absolute client CA path": func(config []byte) []byte {
			return bytes.ReplaceAll(config, []byte(nodeClientCAPath), []byte("/tmp/attacker/client-ca.crt"))
		},
		"absolute kernel runtime path": func(config []byte) []byte {
			return bytes.ReplaceAll(config, []byte("/tpdata/trojan-panel-core/runtime"), []byte("/root/attacker-runtime"))
		},
		"unknown key": func(config []byte) []byte {
			return append(config, []byte("  attacker_output_path: /root/owned\n")...)
		},
		"duplicate root": func(config []byte) []byte {
			return append(config, []byte("trojanpanelnext:\n  deployment_mode: node\n")...)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			forged := createForgedBundle(t, root, mutate)
			if _, _, err := decryptAndValidate(forged, []byte(testPassword)); err == nil {
				t.Fatalf("forged Node configuration %q was accepted", name)
			}
		})
	}
}

func TestCreateRejectsUnknownNodeConfigKey(t *testing.T) {
	root := t.TempDir()
	credentialPath := writeTestCredential(t, root)
	configPath := writeTestConfig(t, root)
	file, err := os.OpenFile(configPath, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = io.WriteString(file, "  grpc_client_key_path: /web/client.key\n"); err != nil {
		t.Fatal(err)
	}
	if err = file.Close(); err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(root, "client-ca.crt")
	if err = os.WriteFile(caPath, testCAPEM(t), 0644); err != nil {
		t.Fatal(err)
	}
	err = createBundle(createOptions{
		CredentialPath: credentialPath,
		ConfigPath:     configPath,
		ClientCAPath:   caPath,
		OutputPath:     filepath.Join(root, "node.age"),
	}, []byte(testPassword))
	if err == nil || !strings.Contains(err.Error(), "unsupported node configuration key") {
		t.Fatalf("createBundle error = %v, want unsupported-key error", err)
	}
}

func TestCreateRejectsUnknownCredentialField(t *testing.T) {
	root := t.TempDir()
	credentialPath := writeTestCredential(t, root)
	contents, err := os.ReadFile(credentialPath)
	if err != nil {
		t.Fatal(err)
	}
	contents = append(contents[:len(contents)-1], []byte(`,"client_key":"must-not-be-accepted"}`)...)
	if err = os.WriteFile(credentialPath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	configPath := writeTestConfig(t, root)
	caPath := filepath.Join(root, "client-ca.crt")
	if err = os.WriteFile(caPath, testCAPEM(t), 0644); err != nil {
		t.Fatal(err)
	}
	err = createBundle(createOptions{
		CredentialPath: credentialPath,
		ConfigPath:     configPath,
		ClientCAPath:   caPath,
		OutputPath:     filepath.Join(root, "node.age"),
	}, []byte(testPassword))
	if err == nil || !strings.Contains(err.Error(), "parse Node credential") {
		t.Fatalf("createBundle error = %v, want unknown-field rejection", err)
	}
}

func TestCreateRejectsPublicCABundleContainingPrivateKey(t *testing.T) {
	root := t.TempDir()
	credentialPath := writeTestCredential(t, root)
	configPath := writeTestConfig(t, root)
	caPath := filepath.Join(root, "client-ca.crt")
	contents := append(testCAPEM(t), []byte("-----BEGIN PRIVATE KEY-----\nAAAA\n-----END PRIVATE KEY-----\n")...)
	if err := os.WriteFile(caPath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	err := createBundle(createOptions{
		CredentialPath: credentialPath,
		ConfigPath:     configPath,
		ClientCAPath:   caPath,
		OutputPath:     filepath.Join(root, "node.age"),
	}, []byte(testPassword))
	if err == nil || !strings.Contains(err.Error(), "only PEM certificates") {
		t.Fatalf("createBundle error = %v, want private-key rejection", err)
	}
}

func TestCreatePinsNodePKIPaths(t *testing.T) {
	root := t.TempDir()
	credentialPath := writeTestCredential(t, root)
	configPath := writeTestConfig(t, root)
	contents, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	contents = bytes.ReplaceAll(contents,
		[]byte("grpc_client_ca_path: /tpdata/trojan-panel-core/pki/client-ca.crt"),
		[]byte("grpc_client_ca_path: /tmp/attacker-controlled.crt"))
	if err = os.WriteFile(configPath, contents, 0600); err != nil {
		t.Fatal(err)
	}
	caPath := filepath.Join(root, "client-ca.crt")
	if err = os.WriteFile(caPath, testCAPEM(t), 0644); err != nil {
		t.Fatal(err)
	}
	err = createBundle(createOptions{
		CredentialPath: credentialPath,
		ConfigPath:     configPath,
		ClientCAPath:   caPath,
		OutputPath:     filepath.Join(root, "node.age"),
	}, []byte(testPassword))
	if err == nil || !strings.Contains(err.Error(), "grpc_client_ca_path") {
		t.Fatalf("createBundle error = %v, want pinned-path rejection", err)
	}
}

func createTestBundle(t *testing.T, root string) string {
	t.Helper()
	credentialPath := writeTestCredential(t, root)
	configPath := writeTestConfig(t, root)
	caPath := filepath.Join(root, "client-ca.crt")
	if err := os.WriteFile(caPath, testCAPEM(t), 0644); err != nil {
		t.Fatal(err)
	}
	bundlePath := filepath.Join(root, "node.age")
	if err := createBundle(createOptions{credentialPath, configPath, caPath, bundlePath}, []byte(testPassword)); err != nil {
		t.Fatal(err)
	}
	return bundlePath
}

func writeTestCredential(t *testing.T, root string) string {
	t.Helper()
	credential := credentialFile{
		SchemaVersion:  2,
		NodeIdentityID: "11111111-2222-4333-8444-555555555555",
		NodeServerID:   42,
		NodeName:       "node-sg",
		NodeDomain:     "node.example.com",
		PublicIP:       "203.0.113.42",
		Generation:     7,
		MariaDB:        databaseCredential{Database: "trojan_panel_db", Username: "tpn_example", Password: "db-secret"},
		Redis:          redisCredential{Username: "tpn-cache-example", Password: "cache-secret", KeyPatterns: []string{"trojan-panel-core:*"}},
		RedisAuth:      redisCredential{Username: "tpn-auth-example", Password: "auth-secret", KeyPatterns: []string{"trojan-panel:jwt-key", "trojan-panel:token:*"}},
	}
	contents, err := json.Marshal(credential)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "credential.json")
	if err = os.WriteFile(path, contents, 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeTestConfig(t *testing.T, root string) string {
	t.Helper()
	path := filepath.Join(root, "config-node.yaml")
	contents := `trojanpanelnext:
  schema_version: 1
  asset_version: 1.2.3
  deployment_mode: node
  hostname: node.example.com
  email: admin@example.com
  caddy_image: caddy@sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
  node_agent_image: example/node@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
  mariadb_host: panel.example.com
  mariadb_port: 9507
  mariadb_user: replace
  mariadb_password: replace
  database: trojan_panel_db
  account_table: account
  redis_host: panel.example.com
  redis_port: 6378
  redis_username: replace
  redis_password: replace
  redis_auth_username: replace
  redis_auth_password: replace
  grpc_port: 8100
  core_port: 8082
  node_server_id: 1
  grpc_tls_mode: mtls
  grpc_tls_server_name: node.example.com
  grpc_client_ca_path: /tpdata/trojan-panel-core/pki/client-ca.crt
  pki_bundle_dir: /tpdata/trojanpanelnext-pki
  kernel_runtime_path: /tpdata/trojan-panel-core/runtime
  force: 0
  purge_data: 0
`
	if err := os.WriteFile(path, []byte(contents), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeEncryptedArchive(path, password string, entries map[string][]byte) error {
	return writeEncryptedBytes(path, password, canonicalTestArchive(entries))
}

func writeEncryptedBytes(path, password string, archive []byte) error {
	var encrypted bytes.Buffer
	recipient, err := age.NewScryptRecipient(password)
	if err != nil {
		return err
	}
	writer, err := age.Encrypt(&encrypted, recipient)
	if err != nil {
		return err
	}
	if _, err = writer.Write(archive); err != nil {
		return err
	}
	if err = writer.Close(); err != nil {
		return err
	}
	return os.WriteFile(path, encrypted.Bytes(), 0600)
}

func canonicalTestArchive(entries map[string][]byte) []byte {
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	written := make(map[string]bool)
	ordered := append([]string(nil), bundleInventory...)
	for _, name := range sortedKeys(entries) {
		if !contains(ordered, name) {
			ordered = append(ordered, name)
		}
	}
	for _, name := range ordered {
		contents, exists := entries[name]
		if !exists || written[name] {
			continue
		}
		written[name] = true
		mode := int64(0600)
		if name == clientCAPath {
			mode = 0644
		}
		_ = writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(contents)), Typeflag: tar.TypeReg})
		_, _ = writer.Write(contents)
	}
	_ = writer.Close()
	return archive.Bytes()
}

func rewriteBundleWithTail(t *testing.T, source, destination string, tail []byte) {
	t.Helper()
	file, err := os.Open(source)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := age.NewScryptIdentity(testPassword)
	if err != nil {
		t.Fatal(err)
	}
	reader, err := age.Decrypt(file, identity)
	if err != nil {
		t.Fatal(err)
	}
	archive, err := io.ReadAll(reader)
	_ = file.Close()
	if err != nil {
		t.Fatal(err)
	}
	archive = append(archive, tail...)
	if err = writeEncryptedBytes(destination, testPassword, archive); err != nil {
		t.Fatal(err)
	}
}

func createForgedBundle(t *testing.T, root string, mutate func([]byte) []byte) string {
	t.Helper()
	valid := createTestBundle(t, root)
	entries, manifest, err := decryptAndValidate(valid, []byte(testPassword))
	if err != nil {
		t.Fatal(err)
	}
	entries[configPath] = mutate(entries[configPath])
	for index := range manifest.Files {
		if manifest.Files[index].Path == configPath {
			manifest.Files[index].SHA256 = digest(entries[configPath])
		}
	}
	manifestContents, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	entries[manifestPath] = append(manifestContents, '\n')
	forged := filepath.Join(root, "forged.age")
	if err = writeEncryptedArchive(forged, testPassword, entries); err != nil {
		t.Fatal(err)
	}
	return forged
}

func min(left, right int) int {
	if left < right {
		return left
	}
	return right
}

func testCAPEM(t *testing.T) []byte {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "control-plane-ca"},
		NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

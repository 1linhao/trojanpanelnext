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
	privateKeyTail := strings.Join([]string{
		"-----BEGIN", " PRIVATE", " KEY-----\n",
		"AAAA\n",
		"-----END", " PRIVATE", " KEY-----\n",
	}, "")
	rewriteBundleWithTail(t, valid, forged, []byte(privateKeyTail))
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

func TestCreateRejectsNodeConfigWithoutSchemaVersion(t *testing.T) {
	root := t.TempDir()
	credentialPath := writeTestCredential(t, root)
	configPath := writeTestConfig(t, root)
	removeConfigLine(t, configPath, "  schema_version: 1\n")
	caPath := filepath.Join(root, "client-ca.crt")
	if err := os.WriteFile(caPath, testCAPEM(t), 0644); err != nil {
		t.Fatal(err)
	}
	err := createBundle(createOptions{
		CredentialPath: credentialPath,
		ConfigPath:     configPath,
		ClientCAPath:   caPath,
		OutputPath:     filepath.Join(root, "node.age"),
	}, []byte(testPassword))
	if err == nil {
		t.Fatal("create accepted a Node configuration without schema_version")
	}
}

func TestDecryptRejectsNodeConfigWithoutSchemaVersion(t *testing.T) {
	root := t.TempDir()
	forged := createForgedBundle(t, root, func(config []byte) []byte {
		return bytes.Replace(config, []byte("  schema_version: 1\n"), nil, 1)
	})
	if _, _, err := decryptAndValidate(forged, []byte(testPassword)); err == nil {
		t.Fatal("extract accepted a Node configuration without schema_version")
	}
}

func TestCreateRejectsEveryOtherMissingRequiredNodeConfigField(t *testing.T) {
	required := []string{
		"asset_version", "deployment_mode", "hostname", "email",
		"caddy_image", "mariadb_image", "redis_image", "api_image", "web_image", "node_agent_image",
		"node_caddy_http_port", "node_caddy_https_port",
		"mariadb_host", "mariadb_port", "mariadb_user", "mariadb_password", "database", "account_table",
		"redis_host", "redis_port", "redis_username", "redis_password", "redis_auth_username", "redis_auth_password",
		"grpc_port", "core_port", "node_server_id", "node_identity_id", "node_identity_generation",
		"grpc_tls_mode", "grpc_tls_server_name", "grpc_client_ca_path", "pki_bundle_dir", "kernel_runtime_path",
		"force", "purge_data",
	}
	root := t.TempDir()
	credentialPath := writeTestCredential(t, root)
	caPath := filepath.Join(root, "client-ca.crt")
	if err := os.WriteFile(caPath, testCAPEM(t), 0644); err != nil {
		t.Fatal(err)
	}
	for _, key := range required {
		t.Run(key, func(t *testing.T) {
			caseRoot := t.TempDir()
			configPath := writeTestConfig(t, caseRoot)
			removeConfigKey(t, configPath, key)
			err := createBundle(createOptions{
				CredentialPath: credentialPath,
				ConfigPath:     configPath,
				ClientCAPath:   caPath,
				OutputPath:     filepath.Join(caseRoot, "node.age"),
			}, []byte(testPassword))
			if err == nil {
				t.Fatalf("create accepted a Node configuration without %s", key)
			}
		})
	}
}

func TestDecryptRejectsOtherMissingRequiredNodeConfigFields(t *testing.T) {
	for _, key := range []string{"asset_version", "caddy_image", "mariadb_port", "database", "core_port", "force"} {
		t.Run(key, func(t *testing.T) {
			root := t.TempDir()
			forged := createForgedBundle(t, root, func(config []byte) []byte {
				return removeConfigKeyBytes(t, config, key)
			})
			if _, _, err := decryptAndValidate(forged, []byte(testPassword)); err == nil {
				t.Fatalf("extract accepted a Node configuration without %s", key)
			}
		})
	}
}

func TestCreateRejectsWrongNodeConfigTypesAndValues(t *testing.T) {
	tests := map[string]func(*testing.T, []byte) []byte{
		"wrong schema version": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "schema_version", "999")
		},
		"string schema version": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "schema_version", `"1"`)
		},
		"numeric asset version": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "asset_version", "123")
		},
		"boolean deployment mode": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "deployment_mode", "true")
		},
		"numeric email": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "email", "7")
		},
		"numeric image": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "caddy_image", "7")
		},
		"string port": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "mariadb_port", `"9507"`)
		},
		"zero port": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "mariadb_port", "0")
		},
		"oversized port": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "core_port", "65536")
		},
		"boolean force": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "force", "true")
		},
		"force outside enum": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "force", "2")
		},
		"wrong database": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "database", "other")
		},
		"wrong account table": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "account_table", "other")
		},
		"malformed identity": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "node_identity_id", "not-a-uuid")
		},
		"mismatched grpc server name": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "grpc_tls_server_name", "other.example.com")
		},
		"relative image bundle directory": func(t *testing.T, config []byte) []byte {
			return append(config, []byte("  image_bundle_dir: ../images\n")...)
		},
		"relative external TLS directory": func(t *testing.T, config []byte) []byte {
			return append(config, []byte("  tls_mode: external\n  tls_cert_dir: ../certs\n")...)
		},
		"certificate file path": func(t *testing.T, config []byte) []byte {
			return append(config, []byte("  tls_cert_file: ../server.crt\n")...)
		},
		"invalid bind address": func(t *testing.T, config []byte) []byte {
			return append(config, []byte("  bind_address: every-interface\n")...)
		},
		"wrong managed certificate directory": func(t *testing.T, config []byte) []byte {
			return append(config, []byte("  managed_cert_dir: /tmp/cert\n")...)
		},
		"wrong external managed directory": func(t *testing.T, config []byte) []byte {
			return append(config, []byte("  external_managed_dir: /tmp/external\n")...)
		},
		"wrong external routes directory": func(t *testing.T, config []byte) []byte {
			return append(config, []byte("  external_routes_dir: /tmp/routes\n")...)
		},
	}
	root := t.TempDir()
	credentialPath := writeTestCredential(t, root)
	caPath := filepath.Join(root, "client-ca.crt")
	if err := os.WriteFile(caPath, testCAPEM(t), 0644); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			caseRoot := t.TempDir()
			configPath := writeTestConfig(t, caseRoot)
			contents, err := os.ReadFile(configPath)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.WriteFile(configPath, mutate(t, contents), 0600); err != nil {
				t.Fatal(err)
			}
			err = createBundle(createOptions{
				CredentialPath: credentialPath, ConfigPath: configPath, ClientCAPath: caPath,
				OutputPath: filepath.Join(caseRoot, "node.age"),
			}, []byte(testPassword))
			if err == nil {
				t.Fatalf("create accepted invalid Node configuration: %s", name)
			}
		})
	}
}

func TestDecryptRejectsWrongNodeConfigTypesAndValues(t *testing.T) {
	tests := map[string]func(*testing.T, []byte) []byte{
		"wrong schema version": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "schema_version", "999")
		},
		"string port": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "redis_port", `"6378"`)
		},
		"force outside enum": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "force", "2")
		},
		"wrong database": func(t *testing.T, config []byte) []byte {
			return setConfigValueBytes(t, config, "database", "other")
		},
		"unsafe optional host path": func(t *testing.T, config []byte) []byte {
			return append(config, []byte("  image_bundle_dir: ../images\n")...)
		},
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			forged := createForgedBundle(t, root, func(config []byte) []byte {
				return mutate(t, config)
			})
			if _, _, err := decryptAndValidate(forged, []byte(testPassword)); err == nil {
				t.Fatalf("extract accepted invalid Node configuration: %s", name)
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

func TestCreateRejectsInvalidCredentialValuesAndTypes(t *testing.T) {
	tests := map[string]struct {
		old string
		new string
	}{
		"invalid identity UUID": {
			old: `"node_identity_id":"11111111-2222-4333-8444-555555555555"`,
			new: `"node_identity_id":"not-a-uuid"`,
		},
		"invalid node domain": {
			old: `"node_domain":"node.example.com"`,
			new: `"node_domain":"not a domain"`,
		},
		"loopback public IP": {
			old: `"public_ip":"203.0.113.42"`,
			new: `"public_ip":"127.0.0.1"`,
		},
		"private public IP": {
			old: `"public_ip":"203.0.113.42"`,
			new: `"public_ip":"10.0.0.7"`,
		},
		"public IP wrong type": {
			old: `"public_ip":"203.0.113.42"`,
			new: `"public_ip":203`,
		},
		"node server ID wrong type": {
			old: `"node_server_id":42`,
			new: `"node_server_id":"42"`,
		},
		"wrong Redis cache key pattern": {
			old: `"key_patterns":["trojan-panel-core:*"]`,
			new: `"key_patterns":["trojan-panel:*"]`,
		},
		"wrong Redis auth key pattern": {
			old: `"key_patterns":["trojan-panel:jwt-key","trojan-panel:token:*"]`,
			new: `"key_patterns":["trojan-panel-core:*"]`,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			credentialPath := writeTestCredential(t, root)
			contents, err := os.ReadFile(credentialPath)
			if err != nil {
				t.Fatal(err)
			}
			updated := bytes.Replace(contents, []byte(test.old), []byte(test.new), 1)
			if bytes.Equal(updated, contents) {
				t.Fatalf("test fixture does not contain %q", test.old)
			}
			if err = os.WriteFile(credentialPath, updated, 0600); err != nil {
				t.Fatal(err)
			}
			configPath := writeTestConfig(t, root)
			caPath := filepath.Join(root, "client-ca.crt")
			if err = os.WriteFile(caPath, testCAPEM(t), 0644); err != nil {
				t.Fatal(err)
			}
			outputPath := filepath.Join(root, "node.age")
			if err = createBundle(createOptions{
				CredentialPath: credentialPath,
				ConfigPath:     configPath,
				ClientCAPath:   caPath,
				OutputPath:     outputPath,
			}, []byte(testPassword)); err == nil {
				t.Fatalf("create accepted invalid credential: %s", name)
			}
			if _, statErr := os.Stat(outputPath); !os.IsNotExist(statErr) {
				t.Fatalf("invalid credential left output bundle, stat error = %v", statErr)
			}
		})
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
  mariadb_image: mariadb@sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc
  redis_image: redis@sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd
  api_image: example/api@sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee
  web_image: example/web@sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff
  node_agent_image: example/node@sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb
  node_caddy_http_port: 80
  node_caddy_https_port: 8863
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
  node_identity_id: 11111111-2222-4333-8444-555555555555
  node_identity_generation: 1
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

func removeConfigLine(t *testing.T, path, line string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := bytes.Replace(contents, []byte(line), nil, 1)
	if bytes.Equal(updated, contents) {
		t.Fatalf("test fixture does not contain %q", line)
	}
	if err = os.WriteFile(path, updated, 0600); err != nil {
		t.Fatal(err)
	}
}

func removeConfigKey(t *testing.T, path, key string) {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := removeConfigKeyBytes(t, contents, key)
	if err = os.WriteFile(path, updated, 0600); err != nil {
		t.Fatal(err)
	}
}

func removeConfigKeyBytes(t *testing.T, contents []byte, key string) []byte {
	t.Helper()
	prefix := []byte("  " + key + ":")
	lines := bytes.SplitAfter(contents, []byte("\n"))
	for index, line := range lines {
		if bytes.HasPrefix(line, prefix) {
			return bytes.Join(append(lines[:index], lines[index+1:]...), nil)
		}
	}
	t.Fatalf("test fixture does not contain key %q", key)
	return nil
}

func setConfigValueBytes(t *testing.T, contents []byte, key, value string) []byte {
	t.Helper()
	prefix := []byte("  " + key + ":")
	lines := bytes.SplitAfter(contents, []byte("\n"))
	for index, line := range lines {
		if bytes.HasPrefix(line, prefix) {
			lines[index] = []byte("  " + key + ": " + value + "\n")
			return bytes.Join(lines, nil)
		}
	}
	t.Fatalf("test fixture does not contain key %q", key)
	return nil
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

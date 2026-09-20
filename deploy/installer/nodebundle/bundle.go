package main

import (
	"archive/tar"
	"bytes"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"filippo.io/age"
	"gopkg.in/yaml.v3"
)

const (
	bundleSchemaVersion = 1
	maxBundleSize       = 4 << 20
	manifestPath        = "manifest.json"
	configPath          = "config-node.yaml"
	clientCAPath        = "pki/client-ca.crt"
	nodeClientCAPath    = "/tpdata/trojan-panel-core/pki/client-ca.crt"
	nodePKIBundleDir    = "/tpdata/trojanpanelnext-pki"
	nodeKernelRuntime   = "/tpdata/trojan-panel-core/runtime"
	nodeManagedCertDir  = "/tpdata/trojan-panel-core/cert"
	nodeExternalDir     = "/tpdata/trojanpanelnext-external"
	nodeExternalRoutes  = "/tpdata/trojan-panel-core/external"
)

var bundleInventory = []string{configPath, manifestPath, clientCAPath}

type createOptions struct {
	CredentialPath string
	ConfigPath     string
	ClientCAPath   string
	OutputPath     string
}

type credentialFile struct {
	SchemaVersion  int                `json:"schema_version"`
	NodeIdentityID string             `json:"node_identity_id"`
	NodeServerID   uint64             `json:"node_server_id"`
	NodeName       string             `json:"node_name"`
	NodeDomain     string             `json:"node_domain"`
	PublicIP       string             `json:"public_ip"`
	Generation     uint64             `json:"generation"`
	MariaDB        databaseCredential `json:"mariadb"`
	Redis          redisCredential    `json:"redis"`
	RedisAuth      redisCredential    `json:"redis_auth"`
}

type databaseCredential struct {
	Database string `json:"database"`
	Username string `json:"username"`
	Password string `json:"password"`
}

type redisCredential struct {
	Username    string   `json:"username"`
	Password    string   `json:"password"`
	KeyPatterns []string `json:"key_patterns"`
}

type bundleManifest struct {
	SchemaVersion  int            `json:"schema_version"`
	NodeIdentityID string         `json:"node_identity_id"`
	NodeServerID   uint64         `json:"node_server_id"`
	NodeName       string         `json:"node_name"`
	NodeDomain     string         `json:"node_domain"`
	PublicIP       string         `json:"public_ip"`
	Generation     uint64         `json:"generation"`
	Inventory      []string       `json:"inventory"`
	Files          []manifestFile `json:"files"`
}

type manifestFile struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

var allowedNodeConfigKeys = map[string]bool{
	"schema_version": true, "asset_version": true, "deployment_mode": true,
	"hostname": true, "email": true, "caddy_image": true, "mariadb_image": true,
	"redis_image": true, "api_image": true, "web_image": true, "node_agent_image": true,
	"image_bundle_dir": true, "node_caddy_http_port": true, "node_caddy_https_port": true,
	"mariadb_host": true, "mariadb_port": true, "mariadb_user": true,
	"mariadb_password": true, "database": true, "account_table": true,
	"redis_host": true, "redis_port": true, "redis_username": true,
	"redis_password": true, "redis_auth_username": true, "redis_auth_password": true,
	"grpc_port": true, "core_port": true, "node_server_id": true,
	"node_identity_id": true, "node_identity_generation": true,
	"grpc_tls_mode": true, "grpc_tls_server_name": true, "grpc_client_ca_path": true,
	"pki_bundle_dir": true, "kernel_runtime_path": true, "tls_mode": true,
	"tls_cert_dir": true, "tls_cert_file": true, "tls_key_file": true,
	"bind_address": true, "managed_cert_dir": true, "external_managed_dir": true,
	"external_routes_dir": true, "force": true, "purge_data": true,
}

func createBundle(options createOptions, password []byte) error {
	if err := validatePassword(password); err != nil {
		return err
	}
	credentialContents, err := readRegularFile(options.CredentialPath, true)
	if err != nil {
		return fmt.Errorf("read Node credential: %w", err)
	}
	var credential credentialFile
	if err = decodeStrictJSON(credentialContents, &credential); err != nil {
		return fmt.Errorf("parse Node credential: %w", err)
	}
	if err = validateCredential(credential); err != nil {
		return err
	}
	template, err := readRegularFile(options.ConfigPath, true)
	if err != nil {
		return fmt.Errorf("read Node configuration: %w", err)
	}
	config, err := renderNodeConfig(template, credential)
	if err != nil {
		return err
	}
	clientCA, err := readRegularFile(options.ClientCAPath, false)
	if err != nil {
		return fmt.Errorf("read public control-plane CA: %w", err)
	}
	if err = validatePublicCA(clientCA); err != nil {
		return err
	}

	manifest := bundleManifest{
		SchemaVersion: bundleSchemaVersion, NodeIdentityID: credential.NodeIdentityID,
		NodeServerID: credential.NodeServerID, NodeName: credential.NodeName,
		NodeDomain: credential.NodeDomain, PublicIP: credential.PublicIP,
		Generation: credential.Generation, Inventory: append([]string(nil), bundleInventory...),
		Files: []manifestFile{{Path: configPath, SHA256: digest(config)}, {Path: clientCAPath, SHA256: digest(clientCA)}},
	}
	manifestContents, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	manifestContents = append(manifestContents, '\n')
	entries := map[string][]byte{manifestPath: manifestContents, configPath: config, clientCAPath: clientCA}

	archive, err := encodeCanonicalArchive(entries)
	if err != nil {
		return err
	}
	var encrypted bytes.Buffer
	recipient, err := age.NewScryptRecipient(string(password))
	if err != nil {
		return fmt.Errorf("create age scrypt recipient: %w", err)
	}
	ageWriter, err := age.Encrypt(&encrypted, recipient)
	if err != nil {
		return fmt.Errorf("start age encryption: %w", err)
	}
	if _, err = ageWriter.Write(archive); err != nil {
		return err
	}
	if err = ageWriter.Close(); err != nil {
		return err
	}
	if encrypted.Len() > maxBundleSize {
		return errors.New("encrypted Node bootstrap bundle exceeds the size limit")
	}
	if err = createRestrictedFile(options.OutputPath, encrypted.Bytes()); err != nil {
		return fmt.Errorf("write encrypted Node bootstrap bundle: %w", err)
	}
	return nil
}

func renderNodeConfig(template []byte, credential credentialFile) ([]byte, error) {
	root, config, err := parseAndValidateNodeConfig(template, nil)
	if err != nil {
		return nil, err
	}
	if scalarString(config["hostname"]) != credential.NodeDomain {
		return nil, errors.New("Node configuration hostname must match the registered Node domain")
	}
	config["hostname"] = credential.NodeDomain
	config["node_server_id"] = credential.NodeServerID
	config["node_identity_id"] = credential.NodeIdentityID
	config["node_identity_generation"] = credential.Generation
	config["mariadb_user"] = credential.MariaDB.Username
	config["mariadb_password"] = credential.MariaDB.Password
	config["database"] = credential.MariaDB.Database
	config["redis_username"] = credential.Redis.Username
	config["redis_password"] = credential.Redis.Password
	config["redis_auth_username"] = credential.RedisAuth.Username
	config["redis_auth_password"] = credential.RedisAuth.Password
	config["grpc_tls_mode"] = "mtls"
	config["grpc_tls_server_name"] = credential.NodeDomain
	var encoded bytes.Buffer
	encoder := yaml.NewEncoder(&encoded)
	encoder.SetIndent(2)
	if err = encoder.Encode(root); err != nil {
		return nil, err
	}
	if err = encoder.Close(); err != nil {
		return nil, err
	}
	return encoded.Bytes(), nil
}

func decryptAndValidate(path string, password []byte) (map[string][]byte, bundleManifest, error) {
	if err := validatePassword(password); err != nil {
		return nil, bundleManifest{}, err
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, bundleManifest{}, err
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxBundleSize {
		return nil, bundleManifest{}, errors.New("encrypted Node bootstrap bundle is not a safe regular file")
	}
	identity, err := age.NewScryptIdentity(string(password))
	if err != nil {
		return nil, bundleManifest{}, err
	}
	reader, err := age.Decrypt(file, identity)
	if err != nil {
		return nil, bundleManifest{}, errors.New("Node bootstrap bundle password or envelope is invalid")
	}
	limited := io.LimitReader(reader, maxBundleSize+1)
	archive, err := io.ReadAll(limited)
	if err != nil || len(archive) > maxBundleSize {
		return nil, bundleManifest{}, errors.New("decrypted Node bootstrap bundle exceeds the size limit")
	}
	entries := make(map[string][]byte, len(bundleInventory))
	tarReader := tar.NewReader(bytes.NewReader(archive))
	for {
		header, nextErr := tarReader.Next()
		if errors.Is(nextErr, io.EOF) {
			break
		}
		if nextErr != nil {
			return nil, bundleManifest{}, fmt.Errorf("read Node bootstrap archive: %w", nextErr)
		}
		if !contains(bundleInventory, header.Name) || header.Typeflag != tar.TypeReg || header.Size < 0 || header.Size > maxBundleSize {
			return nil, bundleManifest{}, fmt.Errorf("Node bootstrap archive contains unsafe or unexpected entry %q", header.Name)
		}
		if _, duplicate := entries[header.Name]; duplicate {
			return nil, bundleManifest{}, fmt.Errorf("Node bootstrap archive repeats entry %q", header.Name)
		}
		contents, readErr := io.ReadAll(io.LimitReader(tarReader, header.Size+1))
		if readErr != nil || int64(len(contents)) != header.Size {
			return nil, bundleManifest{}, fmt.Errorf("read Node bootstrap entry %q", header.Name)
		}
		entries[header.Name] = contents
	}
	if strings.Join(sortedKeys(entries), "\x00") != strings.Join(bundleInventory, "\x00") {
		return nil, bundleManifest{}, errors.New("Node bootstrap archive inventory is incomplete")
	}
	canonical, canonicalErr := encodeCanonicalArchive(entries)
	if canonicalErr != nil || !bytes.Equal(archive, canonical) {
		return nil, bundleManifest{}, errors.New("Node bootstrap archive is not in the canonical fixed-inventory format")
	}
	var manifest bundleManifest
	if err = decodeStrictJSON(entries[manifestPath], &manifest); err != nil || manifest.SchemaVersion != bundleSchemaVersion {
		return nil, bundleManifest{}, errors.New("Node bootstrap manifest is invalid")
	}
	if strings.Join(manifest.Inventory, "\x00") != strings.Join(bundleInventory, "\x00") || len(manifest.Files) != 2 {
		return nil, bundleManifest{}, errors.New("Node bootstrap manifest inventory is invalid")
	}
	wantHashes := map[string]string{configPath: digest(entries[configPath]), clientCAPath: digest(entries[clientCAPath])}
	for _, item := range manifest.Files {
		if wantHashes[item.Path] == "" || wantHashes[item.Path] != item.SHA256 {
			return nil, bundleManifest{}, fmt.Errorf("Node bootstrap entry digest mismatch: %s", item.Path)
		}
		delete(wantHashes, item.Path)
	}
	if len(wantHashes) != 0 {
		return nil, bundleManifest{}, errors.New("Node bootstrap manifest omits a required digest")
	}
	if err = validatePublicCA(entries[clientCAPath]); err != nil {
		return nil, bundleManifest{}, err
	}
	if _, _, err = parseAndValidateNodeConfig(entries[configPath], &manifest); err != nil {
		return nil, bundleManifest{}, err
	}
	return entries, manifest, nil
}

func encodeCanonicalArchive(entries map[string][]byte) ([]byte, error) {
	var archive bytes.Buffer
	writer := tar.NewWriter(&archive)
	for _, name := range bundleInventory {
		contents, exists := entries[name]
		if !exists {
			return nil, fmt.Errorf("Node bootstrap archive is missing %s", name)
		}
		mode := int64(0600)
		if name == clientCAPath {
			mode = 0644
		}
		if err := writer.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(contents)), Typeflag: tar.TypeReg}); err != nil {
			return nil, err
		}
		if _, err := writer.Write(contents); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	return archive.Bytes(), nil
}

func parseAndValidateNodeConfig(contents []byte, expected *bundleManifest) (map[string]interface{}, map[string]interface{}, error) {
	var root map[string]interface{}
	if err := yaml.Unmarshal(contents, &root); err != nil {
		return nil, nil, fmt.Errorf("parse Node configuration: %w", err)
	}
	if len(root) != 1 {
		return nil, nil, errors.New("Node configuration must contain only trojanpanelnext")
	}
	raw, ok := root["trojanpanelnext"]
	if !ok {
		return nil, nil, errors.New("Node configuration is missing trojanpanelnext")
	}
	config, ok := raw.(map[string]interface{})
	if !ok {
		return nil, nil, errors.New("trojanpanelnext must be a mapping")
	}
	for key := range config {
		if !allowedNodeConfigKeys[key] {
			return nil, nil, fmt.Errorf("unsupported node configuration key %q", key)
		}
	}
	if scalarString(config["deployment_mode"]) != "node" {
		return nil, nil, errors.New("Node configuration deployment_mode must be node")
	}
	if scalarString(config["grpc_tls_mode"]) != "mtls" {
		return nil, nil, errors.New("Node configuration must keep grpc_tls_mode mtls")
	}
	if uint64Value(config["grpc_port"]) != 8100 {
		return nil, nil, errors.New("Node configuration grpc_port must match the registered port 8100")
	}
	fixedPaths := map[string]string{
		"grpc_client_ca_path": nodeClientCAPath,
		"pki_bundle_dir":      nodePKIBundleDir,
		"kernel_runtime_path": nodeKernelRuntime,
	}
	for key, expectedPath := range fixedPaths {
		if scalarString(config[key]) != expectedPath {
			return nil, nil, fmt.Errorf("Node configuration %s must be %s", key, expectedPath)
		}
	}
	optionalFixedPaths := map[string]string{
		"managed_cert_dir":     nodeManagedCertDir,
		"external_managed_dir": nodeExternalDir,
		"external_routes_dir":  nodeExternalRoutes,
	}
	for key, expectedPath := range optionalFixedPaths {
		if value := strings.TrimSpace(scalarString(config[key])); value != "" && value != expectedPath {
			return nil, nil, fmt.Errorf("Node configuration %s must be %s", key, expectedPath)
		}
	}
	for _, key := range []string{"tls_cert_file", "tls_key_file"} {
		if value := strings.TrimSpace(scalarString(config[key])); value != "" && (filepath.Base(value) != value || value == "." || value == "..") {
			return nil, nil, fmt.Errorf("Node configuration %s must be a file name without path components", key)
		}
	}
	for _, key := range []string{"hostname", "mariadb_host", "redis_host", "node_agent_image", "grpc_client_ca_path"} {
		if strings.TrimSpace(scalarString(config[key])) == "" {
			return nil, nil, fmt.Errorf("Node configuration requires %s", key)
		}
	}
	if expected != nil && (scalarString(config["node_identity_id"]) != expected.NodeIdentityID ||
		uint64Value(config["node_identity_generation"]) != expected.Generation ||
		uint64Value(config["node_server_id"]) != expected.NodeServerID ||
		scalarString(config["hostname"]) != expected.NodeDomain ||
		scalarString(config["grpc_tls_server_name"]) != expected.NodeDomain) {
		return nil, nil, errors.New("Node bootstrap manifest and configuration identity disagree")
	}
	return root, config, nil
}

func decodeStrictJSON(contents []byte, destination interface{}) error {
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var trailing interface{}
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("JSON contains trailing data")
		}
		return err
	}
	return nil
}

func validatePublicCA(contents []byte) error {
	rest := contents
	found := false
	for len(bytes.TrimSpace(rest)) > 0 {
		block, remainder := pem.Decode(rest)
		if block == nil || block.Type != "CERTIFICATE" {
			return errors.New("public control-plane CA must contain only PEM certificates")
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !certificate.IsCA {
			return errors.New("public control-plane CA contains an invalid or non-CA certificate")
		}
		found = true
		rest = remainder
	}
	if !found {
		return errors.New("public control-plane CA contains no certificates")
	}
	return nil
}

func extractBundle(path, outputDir string, password []byte) (err error) {
	entries, _, err := decryptAndValidate(path, password)
	if err != nil {
		return err
	}
	info, err := os.Lstat(outputDir)
	if err != nil || !info.IsDir() || info.Mode()&0077 != 0 {
		return errors.New("Node bootstrap extraction directory must be an existing private directory")
	}
	existing, err := os.ReadDir(outputDir)
	if err != nil || len(existing) != 0 {
		return errors.New("Node bootstrap extraction directory must be empty")
	}
	if err = os.Mkdir(filepath.Join(outputDir, "pki"), 0700); err != nil {
		return err
	}
	complete := false
	defer func() {
		if complete {
			return
		}
		for _, name := range bundleInventory {
			_ = os.Remove(filepath.Join(outputDir, filepath.FromSlash(name)))
		}
		_ = os.Remove(filepath.Join(outputDir, "pki"))
	}()
	for _, name := range bundleInventory {
		mode := os.FileMode(0600)
		if name == clientCAPath {
			mode = 0644
		}
		if err = writeExclusive(filepath.Join(outputDir, filepath.FromSlash(name)), entries[name], mode); err != nil {
			return err
		}
	}
	complete = true
	return nil
}

func validateCredential(credential credentialFile) error {
	if credential.SchemaVersion != 2 || credential.NodeIdentityID == "" || credential.NodeServerID == 0 ||
		credential.NodeName == "" || credential.NodeDomain == "" || credential.Generation == 0 ||
		credential.MariaDB.Database != "trojan_panel_db" || credential.MariaDB.Username == "" || credential.MariaDB.Password == "" ||
		strings.EqualFold(credential.MariaDB.Username, "root") || credential.Redis.Username == "" ||
		credential.Redis.Password == "" || strings.EqualFold(credential.Redis.Username, "default") ||
		credential.RedisAuth.Username == "" || credential.RedisAuth.Password == "" ||
		strings.EqualFold(credential.RedisAuth.Username, "default") || credential.RedisAuth.Username == credential.Redis.Username {
		return errors.New("Node credential file is incomplete or does not contain dedicated identities")
	}
	return nil
}

func validatePassword(password []byte) error {
	if len(password) < 12 {
		return errors.New("Node bootstrap bundle password must contain at least 12 bytes")
	}
	if bytes.IndexByte(password, 0) >= 0 || bytes.IndexByte(password, '\n') >= 0 || bytes.IndexByte(password, '\r') >= 0 {
		return errors.New("Node bootstrap bundle password contains unsupported bytes")
	}
	return nil
}

func readRegularFile(path string, restricted bool) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() {
		return nil, errors.New("path must name a regular non-symlink file")
	}
	if restricted && info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("sensitive input file must not be accessible by group or other users")
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(contents) > maxBundleSize {
		return nil, errors.New("input file exceeds the size limit")
	}
	return contents, nil
}

func createRestrictedFile(path string, contents []byte) error {
	if path == "" {
		return errors.New("output path is required")
	}
	if info, err := os.Lstat(path); err == nil {
		return fmt.Errorf("output already exists with mode %s", info.Mode())
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return writeExclusive(path, contents, 0600)
}

func writeExclusive(path string, contents []byte, mode os.FileMode) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	complete := false
	defer func() {
		_ = file.Close()
		if !complete {
			_ = os.Remove(path)
		}
	}()
	if _, err = file.Write(contents); err != nil {
		return err
	}
	if err = file.Sync(); err != nil {
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	complete = true
	return nil
}

func digest(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func sortedKeys(values map[string][]byte) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func scalarString(value interface{}) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func uint64Value(value interface{}) uint64 {
	switch typed := value.(type) {
	case int:
		if typed > 0 {
			return uint64(typed)
		}
	case uint64:
		return typed
	}
	return 0
}

package nodeidentity

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var catalogKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{0,62}$`)

// The binding is published before registration. A crash after either the
// database transaction or credential publication can therefore be replayed
// without guessing which YAML entry owns the identity.
type catalogBinding struct {
	SchemaVersion   int    `json:"schema_version"`
	NodeKey         string `json:"node_key"`
	HostID          string `json:"host_id"`
	Name            string `json:"name"`
	Domain          string `json:"domain"`
	PublicIP        string `json:"public_ip"`
	GRPCPort        uint   `json:"grpc_port"`
	CredentialPath  string `json:"credential_path"`
	Generation      uint64 `json:"generation"`
}

type catalogStatus struct {
	NodeKey              string `json:"node_key"`
	HostID               string `json:"host_id"`
	NodeIdentityID       string `json:"node_identity_id"`
	NodeServerID         uint64 `json:"node_server_id"`
	Name                 string `json:"name"`
	Domain               string `json:"domain"`
	PublicIP             string `json:"public_ip"`
	GRPCPort             uint   `json:"grpc_port"`
	Generation           uint64 `json:"generation"`
	Status               string `json:"status"`
	CredentialPath       string `json:"credential_path"`
	MariaDBUsername      string `json:"mariadb_username"`
	RedisUsername        string `json:"redis_username"`
	RedisAuthUsername    string `json:"redis_auth_username"`
	CredentialsVerified  bool   `json:"credentials_verified"`
	NodeHealth           string `json:"node_health"`
}

func runCatalog(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "lookup" && args[0] != "reconcile") {
		fmt.Fprintln(stderr, "node catalog: expected lookup or reconcile")
		return 2
	}
	mode := args[0]
	set := flag.NewFlagSet("node-identity catalog "+mode, flag.ContinueOnError)
	set.SetOutput(stderr)
	var binding catalogBinding
	var credentialDir string
	set.StringVar(&binding.NodeKey, "node-key", "", "stable YAML Node key")
	set.StringVar(&binding.HostID, "host-id", "", "stable YAML host id")
	set.StringVar(&binding.Name, "name", "", "registered Node name")
	set.StringVar(&binding.Domain, "domain", "", "registered Node domain")
	set.StringVar(&binding.PublicIP, "public-ip", "", "registered public IP")
	set.UintVar(&binding.GRPCPort, "grpc-port", 0, "registered gRPC port")
	set.StringVar(&credentialDir, "credential-dir", "", "existing restricted Web credential directory")
	if err := set.Parse(args[1:]); err != nil || len(set.Args()) != 0 ||
		!catalogKeyPattern.MatchString(binding.NodeKey) || !catalogKeyPattern.MatchString(binding.HostID) ||
		binding.GRPCPort == 0 || binding.GRPCPort > 65535 ||
		credentialDir == "" || !filepath.IsAbs(credentialDir) || filepath.Clean(credentialDir) != credentialDir {
		fmt.Fprintln(stderr, "node catalog: invalid or missing Node key, host, port, or credential directory")
		return 2
	}
	binding.Domain = strings.ToLower(binding.Domain)
	binding.SchemaVersion = 1
	binding.Generation = 1
	binding.CredentialPath = filepath.Join(credentialDir, binding.NodeKey+".g1.json")
	if err := validateRegisterInput(binding.Name, binding.Domain, binding.PublicIP, binding.CredentialPath, nil); err != nil {
		fmt.Fprintln(stderr, "node catalog: invalid Node registration fields")
		return 2
	}
	bindingPath := filepath.Join(credentialDir, binding.NodeKey+".binding.json")
	if err := validateCatalogDirectory(credentialDir, bindingPath); err != nil {
		fmt.Fprintf(stderr, "node catalog %s: restricted credential directory is unavailable\n", binding.NodeKey)
		return 1
	}
	manager, err := openLifecycle()
	if err != nil {
		fmt.Fprintf(stderr, "node catalog %s: control-plane data services are unavailable\n", binding.NodeKey)
		return 1
	}
	defer manager.close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	if mode == "reconcile" {
		if err = manager.ensureSchema(ctx); err != nil {
			fmt.Fprintf(stderr, "node catalog %s: identity schema is unavailable\n", binding.NodeKey)
			return 1
		}
	}
	registered, err := manager.catalogIdentity(ctx, binding)
	if err != nil {
		fmt.Fprintf(stderr, "node catalog %s: conflicting or incomplete database registration\n", binding.NodeKey)
		return 1
	}
	contents, err := readCredentialFile(bindingPath)
	if errors.Is(err, os.ErrNotExist) && mode == "reconcile" && registered == nil {
		contents, err = json.MarshalIndent(binding, "", "  ")
		if err == nil {
			err = createCredentialFile(bindingPath, append(contents, '\n'))
		}
		if err == nil {
			contents, err = readCredentialFile(bindingPath)
		}
	} else if errors.Is(err, os.ErrNotExist) && registered != nil {
		fmt.Fprintf(stderr, "node catalog %s: existing identity has no provable node_key binding\n", binding.NodeKey)
		return 1
	}
	var committed catalogBinding
	if err != nil || json.Unmarshal(contents, &committed) != nil || committed != binding {
		fmt.Fprintf(stderr, "node catalog %s: committed node_key or host fields differ\n", binding.NodeKey)
		return 1
	}
	if registered == nil && mode == "lookup" {
		fmt.Fprintf(stderr, "node catalog %s: identity is not registered\n", binding.NodeKey)
		return 1
	}
	if mode == "reconcile" {
		if _, err = manager.register(ctx, binding.Name, binding.Domain, binding.PublicIP, binding.CredentialPath, binding.GRPCPort); err != nil {
			fmt.Fprintf(stderr, "node catalog %s: registration or credential verification failed\n", binding.NodeKey)
			return 1
		}
		registered, err = manager.catalogIdentity(ctx, binding)
		if err != nil {
			fmt.Fprintf(stderr, "node catalog %s: post-registration database verification failed\n", binding.NodeKey)
			return 1
		}
	} else {
		credentials, _, credentialErr := manager.credentialsForIdentity(binding.CredentialPath, *registered, false, "catalog")
		if credentialErr != nil || manager.verifyCredentials(ctx, credentials) != nil {
			fmt.Fprintf(stderr, "node catalog %s: committed credentials or independent ACL verification failed\n", binding.NodeKey)
			return 1
		}
	}
	if registered == nil || registered.Status != statusActive || registered.Generation != binding.Generation {
		fmt.Fprintf(stderr, "node catalog %s: identity is inactive or generation changed\n", binding.NodeKey)
		return 1
	}
	result := catalogStatus{
		NodeKey: binding.NodeKey, HostID: binding.HostID,
		NodeIdentityID: registered.ID, NodeServerID: registered.NodeServerID,
		Name: registered.Name, Domain: registered.Domain, PublicIP: registered.PublicIP,
		GRPCPort: binding.GRPCPort, Generation: registered.Generation, Status: string(registered.Status),
		CredentialPath: registered.CredentialPath,
		MariaDBUsername: registered.MariaDBUsername, RedisUsername: registered.RedisUsername,
		RedisAuthUsername: registered.RedisAuthUsername,
		CredentialsVerified: true, NodeHealth: "unverified; use node-identity verify with an install challenge",
	}
	if err = json.NewEncoder(stdout).Encode(result); err != nil {
		fmt.Fprintf(stderr, "node catalog %s: could not write status\n", binding.NodeKey)
		return 1
	}
	return 0
}

func validateCatalogDirectory(directory, bindingPath string) error {
	if err := validateCredentialPath(bindingPath); err != nil {
		return err
	}
	info, err := os.Lstat(directory)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		return errors.New("catalog directory must be mode 0700")
	}
	return nil
}

// A missing row is valid only before first registration. Any partial or
// conflicting join fails before register can touch credentials or ACLs.
func (manager *lifecycle) catalogIdentity(ctx context.Context, expected catalogBinding) (*identity, error) {
	rows, err := manager.db.QueryContext(ctx, identitySelect+` WHERE name=? OR domain=? OR credential_path=?`,
		expected.Name, expected.Domain, expected.CredentialPath)
	if err != nil {
		return nil, err
	}
	var matches []identity
	for rows.Next() {
		var item identity
		if err = scanIdentity(rows, &item); err != nil {
			break
		}
		matches = append(matches, item)
	}
	if err == nil {
		err = rows.Err()
	}
	_ = rows.Close()
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	if len(matches) != 1 {
		return nil, errors.New("multiple registrations match the requested Node")
	}
	item := matches[0]
	if item.Name != expected.Name || item.Domain != expected.Domain || item.PublicIP != expected.PublicIP ||
		item.CredentialPath != expected.CredentialPath || item.Generation != expected.Generation || item.Status != statusActive {
		// Provisioning is a safe replay only when it has the same commitment.
		if item.Status != statusProvisioning || item.Generation != expected.Generation ||
			item.Name != expected.Name || item.Domain != expected.Domain || item.PublicIP != expected.PublicIP ||
			item.CredentialPath != expected.CredentialPath {
			return nil, errors.New("identity fields or lifecycle state differ")
		}
	}
	var serverID uint64
	var serverName, serverIP, tlsMode, tlsName, period, limitMode string
	var grpcPort uint
	var total, upload, download uint64
	err = manager.db.QueryRowContext(ctx, `SELECT id,name,ip,grpc_port,grpc_tls_mode,grpc_tls_server_name,
		traffic_period,traffic_limit_mode,traffic_total_limit,traffic_upload_limit,traffic_download_limit
		FROM node_server WHERE id=?`, item.NodeServerID).Scan(
		&serverID, &serverName, &serverIP, &grpcPort, &tlsMode, &tlsName,
		&period, &limitMode, &total, &upload, &download)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("node_server registration is missing")
		}
		return nil, err
	}
	if serverID != item.NodeServerID || serverName != expected.Name || serverIP != expected.PublicIP ||
		grpcPort != expected.GRPCPort || tlsMode != "mtls" || tlsName != expected.Domain ||
		period != "none" || limitMode != "combined" || total != 0 || upload != 0 || download != 0 {
		return nil, errors.New("node_server connection or policy fields differ")
	}
	return &item, nil
}

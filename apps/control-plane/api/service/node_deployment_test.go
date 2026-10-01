package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
	"trojan-panel/core"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
)

func deploymentFixture() (*model.NodeServer, dto.NodeDeploymentDownloadDto, core.AppConfig) {
	id, port := uint(42), uint(8102)
	name, ip, tlsName, mode := "offline fixture", "203.0.113.42", "node.example.test", "mtls"
	return &model.NodeServer{Id: &id, Name: &name, Ip: &ip, GrpcPort: &port, GrpcTLSServerName: &tlsName, GrpcTLSMode: &mode}, dto.NodeDeploymentDownloadDto{Id: id, WebHost: "panel.example.test", Email: "test@example.test", CertificateMode: "caddy"}, core.AppConfig{
		MySQLConfig: core.MySQLConfig{Host: "127.0.0.1", Port: 9507, User: "root", Password: "p:a'=$()\"`\\!"},
		RedisConfig: core.RedisConfig{Host: "localhost", Port: 6378, Password: "r:#=\\\"'`$"},
	}
}

func deploymentCAFixture(t *testing.T, serial int64, isCA bool, expired bool) []byte {
	return deploymentCAWithUsage(t, serial, isCA, expired, x509.KeyUsageCertSign|x509.KeyUsageDigitalSignature)
}

func deploymentCAWithUsage(t *testing.T, serial int64, isCA bool, expired bool, usage x509.KeyUsage) []byte {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	cert := &x509.Certificate{SerialNumber: big.NewInt(serial), Subject: pkix.Name{CommonName: "fixture CA"}, NotBefore: now.Add(-time.Hour), NotAfter: now.Add(time.Hour), IsCA: isCA, BasicConstraintsValid: true, KeyUsage: usage}
	if expired {
		cert.NotAfter = now.Add(-time.Minute)
	}
	der, err := x509.CreateCertificate(rand.Reader, cert, cert, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
}

func deploymentArchiveFiles(t *testing.T, data []byte) (map[string][]byte, map[string]int64) {
	t.Helper()
	compressed, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer compressed.Close()
	archive := tar.NewReader(compressed)
	files, modes := map[string][]byte{}, map[string]int64{}
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if header.Typeflag != tar.TypeReg || filepath.Base(header.Name) != header.Name {
			t.Fatalf("unsafe archive member: %#v", header)
		}
		if _, exists := files[header.Name]; exists {
			t.Fatal("duplicate archive member")
		}
		files[header.Name], err = io.ReadAll(archive)
		if err != nil {
			t.Fatal(err)
		}
		modes[header.Name] = header.Mode
	}
	return files, modes
}

func TestNodeDeploymentArchiveAndSafeMetadata(t *testing.T) {
	server, request, config := deploymentFixture()
	ca := append(deploymentCAFixture(t, 1, true, false), deploymentCAFixture(t, 2, true, false)...)
	for _, mode := range []string{"caddy", "external"} {
		t.Run(mode, func(t *testing.T) {
			selected := request
			selected.CertificateMode = mode
			if mode == "external" {
				selected.Email = ""
				selected.CertificatePath = "/etc/letsencrypt/live/node.example.test/fullchain.pem"
				selected.PrivateKeyPath = "/etc/letsencrypt/live/node.example.test/privkey.pem"
			}
			archive, err := buildNodeDeploymentArchive(server, selected, config, ca)
			if err != nil {
				t.Fatal(err)
			}
			files, modes := deploymentArchiveFiles(t, archive)
			if !reflect.DeepEqual(modes, map[string]int64{"node.yaml": 0600, "client-ca.crt": 0644, "install-node.sh": 0700, "README.md": 0600}) {
				t.Fatalf("archive files or modes changed: %v", modes)
			}
			var document map[string]map[string]interface{}
			if err := yaml.Unmarshal(files["node.yaml"], &document); err != nil {
				t.Fatal(err)
			}
			node := document["trojanpanelnext"]
			for key, want := range map[string]interface{}{"release": strings.TrimPrefix(constant.TrojanPanelVersion, "v"), "purpose": "node", "node_server_id": 42, "grpc_port": 8102, "hostname": *server.GrpcTLSServerName, "grpc_tls_server_name": *server.GrpcTLSServerName, "mariadb_host": request.WebHost, "redis_host": request.WebHost, "mariadb_user": config.MySQLConfig.User, "mariadb_password": config.MySQLConfig.Password, "redis_password": config.RedisConfig.Password, "node_certificate_mode": mode, "database": "trojan_panel_db", "account_table": "account", "force": 0, "purge_data": 0} {
				if node[key] != want {
					t.Fatalf("YAML %s changed: got=%#v want=%#v", key, node[key], want)
				}
			}
			if !bytes.Equal(files["client-ca.crt"], ca) {
				t.Fatal("rotation CA bundle changed")
			}
			metadata, err := deploymentMetadata(server, request.WebHost, config)
			if err != nil {
				t.Fatal(err)
			}
			jsonData, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			for _, data := range [][]byte{jsonData, files["README.md"], files["install-node.sh"], files["client-ca.crt"]} {
				for _, secret := range []string{config.MySQLConfig.Password, config.RedisConfig.Password} {
					if bytes.Contains(data, []byte(secret)) {
						t.Fatal("credentials escaped node.yaml")
					}
				}
			}
			if strings.Contains(string(jsonData), "password") || strings.Contains(string(jsonData), "privateKey") {
				t.Fatal("metadata includes credential fields")
			}
			if bytes.Contains(files["client-ca.crt"], []byte("PRIVATE KEY")) || bytes.Contains(files["install-node.sh"], []byte("--force")) {
				t.Fatal("deployment bundle exports a key or forces installation")
			}
		})
	}
	config.MySQLConfig.Host = "db.example.test"
	config.RedisConfig.Host = "192.0.2.10"
	metadata, err := deploymentMetadata(server, request.WebHost, config)
	if err != nil {
		t.Fatal(err)
	}
	if metadata.MariaDBHost != config.MySQLConfig.Host || metadata.RedisHost != config.RedisConfig.Host {
		t.Fatal("nonloopback connection hosts were replaced")
	}
}

func TestNodeDeploymentConnectionHostFlagsMatchArchive(t *testing.T) {
	server, request, config := deploymentFixture()
	ca := deploymentCAFixture(t, 1, true, false)
	for _, test := range []struct {
		name                                 string
		mariadbHost, redisHost               string
		mariadbUsesWebHost, redisUsesWebHost bool
	}{
		{"same_public_domain", request.WebHost, request.WebHost, false, false},
		{"other_public_hosts", "db.example.test", "192.0.2.10", false, false},
		{"localhost", "localhost", "LOCALHOST.", true, true},
		{"ipv4_loopback", "127.0.0.1", "127.2.3.4", true, true},
		{"ipv6_loopback", "::1", "::1", true, true},
		{"unspecified", "0.0.0.0", "::", true, true},
		{"mysql_only", "localhost", request.WebHost, true, false},
		{"redis_only", request.WebHost, "::1", false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			selectedConfig := config
			selectedConfig.MySQLConfig.Host, selectedConfig.RedisConfig.Host = test.mariadbHost, test.redisHost
			metadata, err := deploymentMetadata(server, request.WebHost, selectedConfig)
			if err != nil {
				t.Fatal(err)
			}
			if metadata.MariaDBUsesWebHost != test.mariadbUsesWebHost || metadata.RedisUsesWebHost != test.redisUsesWebHost {
				t.Fatalf("connection source flags changed: %#v", metadata)
			}
			data, err := json.Marshal(metadata)
			if err != nil {
				t.Fatal(err)
			}
			var response map[string]interface{}
			if err := json.Unmarshal(data, &response); err != nil {
				t.Fatal(err)
			}
			if response["mariadbUsesWebHost"] != test.mariadbUsesWebHost || response["redisUsesWebHost"] != test.redisUsesWebHost {
				t.Fatal("metadata JSON must explicitly include both boolean flags")
			}
			selectedRequest := request
			selectedRequest.WebHost = "new-panel.example.test"
			archive, err := buildNodeDeploymentArchive(server, selectedRequest, selectedConfig, ca)
			if err != nil {
				t.Fatal(err)
			}
			files, _ := deploymentArchiveFiles(t, archive)
			var document map[string]map[string]interface{}
			if err := yaml.Unmarshal(files["node.yaml"], &document); err != nil {
				t.Fatal(err)
			}
			mysqlPreview, redisPreview := metadata.MariaDBHost, metadata.RedisHost
			if metadata.MariaDBUsesWebHost {
				mysqlPreview = selectedRequest.WebHost
			}
			if metadata.RedisUsesWebHost {
				redisPreview = selectedRequest.WebHost
			}
			if document["trojanpanelnext"]["mariadb_host"] != mysqlPreview || document["trojanpanelnext"]["redis_host"] != redisPreview {
				t.Fatalf("edited Web hostname preview differs from YAML: %v", document["trojanpanelnext"])
			}
		})
	}
}

func TestNodeDeploymentRejectsUnsafeInputAndCA(t *testing.T) {
	_, request, _ := deploymentFixture()
	for _, host := range []string{"", "localhost", "127.0.0.1", "::1", "0.0.0.0", "https://panel.example.test", "panel.example.test:443", "user@panel.example.test", "panel.example.test\nother"} {
		selected := request
		selected.WebHost = host
		if validateDeploymentRequest(selected) == nil {
			t.Fatalf("unsafe webHost accepted: %q", host)
		}
	}
	for _, host := range []string{"panel.example.test", "192.0.2.1", "2001:db8::1"} {
		selected := request
		selected.WebHost = host
		if err := validateDeploymentRequest(selected); err != nil {
			t.Fatalf("host %q: %v", host, err)
		}
	}
	for _, email := range []string{"", "invalid", "Someone <test@example.test>", "test@example.test\n"} {
		selected := request
		selected.Email = email
		if validateDeploymentRequest(selected) == nil {
			t.Fatal("invalid Caddy email accepted")
		}
	}
	request.CertificateMode = "external"
	request.Email = ""
	for _, path := range []string{"", "relative.pem", "/", "/etc/test,a.pem", "/etc/test\n.pem"} {
		request.CertificatePath = path
		request.PrivateKeyPath = "/etc/private.pem"
		if validateDeploymentRequest(request) == nil {
			t.Fatal("unsafe external path accepted")
		}
	}
	valid := deploymentCAFixture(t, 1, true, false)
	if err := validateDeploymentCA(deploymentCAWithUsage(t, 4, true, false, 0), time.Now()); err != nil {
		t.Fatalf("valid installer CA with no KeyUsage was rejected: %v", err)
	}
	if validateDeploymentCA(deploymentCAWithUsage(t, 5, true, false, x509.KeyUsageDigitalSignature), time.Now()) == nil {
		t.Fatal("CA with explicit non-signing KeyUsage was accepted")
	}
	private := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("fixture")})
	for _, data := range [][]byte{nil, []byte("not PEM"), append([]byte("garbage\n"), valid...), append(append([]byte{}, valid...), private...), deploymentCAFixture(t, 2, false, false), deploymentCAFixture(t, 3, true, true)} {
		if validateDeploymentCA(data, time.Now()) == nil {
			t.Fatal("invalid, leaf or private CA material accepted")
		}
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "client-ca.crt")
	if err := os.WriteFile(path, valid, 0644); err != nil {
		t.Fatal(err)
	}
	if data, err := readDeploymentCA(dir); err != nil || !bytes.Equal(data, valid) {
		t.Fatalf("valid public CA refused: %v", err)
	}
	if err := os.WriteFile(path, bytes.Repeat([]byte{'x'}, publicCALimit+1), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readDeploymentCA(dir); err == nil {
		t.Fatal("oversized CA accepted")
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(dir, "client-ca.key"), path); err != nil {
		t.Fatal(err)
	}
	if _, err := readDeploymentCA(dir); err == nil {
		t.Fatal("CA symlink accepted")
	}
	t.Setenv("TP_HOST_REMOVAL_CALLBACK_URL", "https://panel.example.test/api/nodeServer/completeHostRemoval")
	if host, err := deploymentWebHost(); err != nil || host != "panel.example.test" {
		t.Fatalf("trusted callback host not used: %q %v", host, err)
	}
	t.Setenv("TP_HOST_REMOVAL_CALLBACK_URL", "")
	if _, err := deploymentWebHost(); err == nil {
		t.Fatal("missing trusted hostname accepted")
	}
}

func TestNodeDeploymentInstallScriptPreservesTrust(t *testing.T) {
	for _, scenario := range []string{"new", "same", "different", "symlink", "validate_failure"} {
		t.Run(scenario, func(t *testing.T) {
			server, request, config := deploymentFixture()
			ca := deploymentCAFixture(t, 1, true, false)
			data, err := buildNodeDeploymentArchive(server, request, config, ca)
			if err != nil {
				t.Fatal(err)
			}
			files, _ := deploymentArchiveFiles(t, data)
			dir := t.TempDir()
			pkg := filepath.Join(dir, "bundle")
			bin := filepath.Join(dir, "bin")
			pki := filepath.Join(dir, "custom-pki")
			for _, path := range []string{pkg, bin} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			for name, body := range files {
				if err := os.WriteFile(filepath.Join(pkg, name), body, 0644); err != nil {
					t.Fatal(err)
				}
			}
			version := strings.TrimPrefix(constant.TrojanPanelVersion, "v")
			entry := filepath.Join(dir, "entry.sh")
			events := filepath.Join(dir, "events")
			fakeEntry := `#!/usr/bin/env bash
if [[ "$1" == --entry-version ]]; then printf '%s\n' "$TEST_VERSION"; exit; fi
printf '%s\n' "$3" >> "$TEST_EVENTS"
if [[ "$3" == validate && "$TEST_VALIDATE_FAIL" == 1 ]]; then exit 1; fi
`
			if err := os.WriteFile(entry, []byte(fakeEntry), 0700); err != nil {
				t.Fatal(err)
			}
			stubs := map[string]string{
				"id":     "#!/usr/bin/env bash\nprintf '0\\n'\n",
				"docker": "#!/usr/bin/env bash\nexit 0\n",
				"curl": `#!/usr/bin/env bash
while (($#)); do if [[ "$1" == -o ]]; then cp -- "$TEST_ENTRY" "$2"; exit; fi; shift; done
exit 1
`,
				"yq": `#!/usr/bin/env bash
if [[ "$1" == --version ]]; then printf '%s\n' 'yq mikefarah/yq version v4.53.6'; exit; fi
case "$2" in
  *.purpose) printf 'node\n';;
  *.release) printf '%s\n' "$TEST_VERSION";;
  *.pki_bundle_dir) printf '%s\n' "$TEST_PKI";;
  *) exit 1;;
esac
`,
			}
			for name, body := range stubs {
				if err := os.WriteFile(filepath.Join(bin, name), []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			trust := filepath.Join(pki, "client-ca.crt")
			if scenario != "new" && scenario != "validate_failure" {
				if err := os.Mkdir(pki, 0700); err != nil {
					t.Fatal(err)
				}
				old := ca
				if scenario == "different" {
					old = deploymentCAFixture(t, 2, true, false)
				}
				if scenario == "symlink" {
					if err := os.Symlink(filepath.Join(pkg, "client-ca.crt"), trust); err != nil {
						t.Fatal(err)
					}
				} else if err := os.WriteFile(trust, old, 0644); err != nil {
					t.Fatal(err)
				}
			}
			validationFail := "0"
			if scenario == "validate_failure" {
				validationFail = "1"
			}
			cmd := exec.Command("bash", filepath.Join(pkg, "install-node.sh"))
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TEST_VERSION="+version, "TEST_ENTRY="+entry, "TEST_EVENTS="+events, "TEST_PKI="+pki, "TEST_VALIDATE_FAIL="+validationFail)
			output, err := cmd.CombinedOutput()
			shouldFail := scenario == "different" || scenario == "symlink" || scenario == "validate_failure"
			if (err != nil) != shouldFail {
				t.Fatalf("script %s: %v %s", scenario, err, output)
			}
			eventData, _ := os.ReadFile(events)
			if shouldFail {
				if strings.Contains(string(eventData), "install") {
					t.Fatal("unsafe bundle reached installation")
				}
				if scenario == "different" {
					current, _ := os.ReadFile(trust)
					if bytes.Equal(current, ca) {
						t.Fatal("different existing trust was overwritten")
					}
				}
				if scenario == "validate_failure" {
					if _, err := os.Stat(pki); !os.IsNotExist(err) {
						t.Fatal("CA directory written before configuration validation")
					}
				}
			} else {
				current, err := os.ReadFile(trust)
				if err != nil || !bytes.Equal(current, ca) {
					t.Fatal("CA not copied to the configured custom directory")
				}
				if string(eventData) != "validate\ninstall\n" {
					t.Fatalf("wrong action order: %q", eventData)
				}
			}
			info, err := os.Stat(filepath.Join(pkg, "node.yaml"))
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatal("configuration permissions are not private")
			}
		})
	}
}

func TestNodeDeploymentRequiresAvailableMTLSRegistration(t *testing.T) {
	server, _, _ := deploymentFixture()
	if err := validateNodeDeploymentServer(server); err != nil {
		t.Fatal(err)
	}
	legacy := "legacy"
	server.GrpcTLSMode = &legacy
	if err := validateNodeDeploymentServer(server); err == nil || !strings.Contains(err.Error(), "mTLS") {
		t.Fatalf("legacy registration lacks a useful error: %v", err)
	}
	mtls := "mtls"
	server.GrpcTLSMode = &mtls
	server.GrpcTLSServerName = nil
	if err := validateNodeDeploymentServer(server); err == nil || !strings.Contains(err.Error(), "TLS domain") {
		t.Fatalf("missing TLS name lacks a useful error: %v", err)
	}
	removing := uint(1)
	server.Removing = &removing
	if err := validateNodeDeploymentServer(server); err == nil || !strings.Contains(err.Error(), "being removed") {
		t.Fatal("uninstalling server can receive a deployment bundle")
	}
	if err := validateNodeDeploymentServer(nil); err == nil {
		t.Fatal("unknown server can receive a deployment bundle")
	}
}

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
	// Exact paths reject absolute names, traversal, entries outside the package,
	// and unexpected nested files before a test extracts anything to disk.
	members := map[string]byte{
		"tpnext/":          tar.TypeDir,
		"tpnext/node.yaml": tar.TypeReg, "tpnext/client-ca.crt": tar.TypeReg,
		"tpnext/install-node.sh": tar.TypeReg, "tpnext/README.md": tar.TypeReg,
		"tpnext/install-dependencies.sh": tar.TypeReg,
	}
	for {
		header, err := archive.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		wantType, expected := members[header.Name]
		if !expected || header.Typeflag != wantType || header.Linkname != "" {
			t.Fatalf("unsafe or unexpected archive member: %#v", header)
		}
		if _, exists := modes[header.Name]; exists {
			t.Fatalf("duplicate archive member: %q", header.Name)
		}
		if header.Name == "tpnext/" {
			if len(modes) != 0 || header.Size != 0 {
				t.Fatal("package directory must be the first, empty archive member")
			}
		} else {
			if _, exists := modes["tpnext/"]; !exists {
				t.Fatal("archive file precedes its package directory")
			}
			files[header.Name], err = io.ReadAll(archive)
			if err != nil {
				t.Fatal(err)
			}
		}
		modes[header.Name] = header.Mode
	}
	if len(modes) != len(members) {
		t.Fatalf("incomplete archive: %v", modes)
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
			if !reflect.DeepEqual(modes, map[string]int64{"tpnext/": 0700, "tpnext/node.yaml": 0600, "tpnext/client-ca.crt": 0644, "tpnext/install-node.sh": 0700, "tpnext/install-dependencies.sh": 0700, "tpnext/README.md": 0600}) {
				t.Fatalf("archive files or modes changed: %v", modes)
			}
			var document map[string]map[string]interface{}
			if err := yaml.Unmarshal(files["tpnext/node.yaml"], &document); err != nil {
				t.Fatal(err)
			}
			node := document["trojanpanelnext"]
			for key, want := range map[string]interface{}{"release": strings.TrimPrefix(constant.TrojanPanelVersion, "v"), "purpose": "node", "node_server_id": 42, "grpc_port": 8102, "hostname": *server.GrpcTLSServerName, "grpc_tls_server_name": *server.GrpcTLSServerName, "mariadb_host": request.WebHost, "redis_host": request.WebHost, "mariadb_user": config.MySQLConfig.User, "mariadb_password": config.MySQLConfig.Password, "redis_password": config.RedisConfig.Password, "node_certificate_mode": mode, "database": "trojan_panel_db", "account_table": "account", "force": 0, "purge_data": 0} {
				if node[key] != want {
					t.Fatalf("YAML %s changed: got=%#v want=%#v", key, node[key], want)
				}
			}
			if !bytes.Equal(files["tpnext/client-ca.crt"], ca) {
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
			for _, data := range [][]byte{jsonData, files["tpnext/README.md"], files["tpnext/install-node.sh"], files["tpnext/install-dependencies.sh"], files["tpnext/client-ca.crt"]} {
				for _, secret := range []string{config.MySQLConfig.Password, config.RedisConfig.Password} {
					if bytes.Contains(data, []byte(secret)) {
						t.Fatal("credentials escaped node.yaml")
					}
				}
			}
			if strings.Contains(string(jsonData), "password") || strings.Contains(string(jsonData), "privateKey") {
				t.Fatal("metadata includes credential fields")
			}
			if bytes.Contains(files["tpnext/client-ca.crt"], []byte("PRIVATE KEY")) || bytes.Contains(files["tpnext/install-node.sh"], []byte("--force")) {
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

func TestNodeDeploymentReadmeProvidesBootstrapAndOrderedInstallation(t *testing.T) {
	server, request, config := deploymentFixture()
	data, err := buildNodeDeploymentArchive(server, request, config, deploymentCAFixture(t, 1, true, false))
	if err != nil {
		t.Fatal(err)
	}
	files, _ := deploymentArchiveFiles(t, data)
	readme := string(files["tpnext/README.md"])
	sections := strings.Split(readme, "## English")
	if len(sections) != 2 {
		t.Fatal("bundle lacks separate Chinese and English installation instructions")
	}
	for _, section := range sections {
		for _, requirement := range []string{"root", "Bash", "Debian 12/13", "Ubuntu 22.04/24.04", "amd64/arm64", "systemd", "Docker", "yq", "openssl", "findutils", "awk"} {
			if !strings.Contains(section, requirement) {
				t.Fatalf("bundle instructions lack prerequisite %q", requirement)
			}
		}
		previous := -1
		for _, command := range []string{
			"apt-get update",
			"apt-get install -y --no-install-recommends bash curl ca-certificates grep coreutils util-linux tar gzip",
			"umask 077",
			"mkdir -m 700 ./node-deployment",
			"tar --extract --gzip",
			"bash ./tpnext/install-dependencies.sh",
			"bash ./tpnext/install-node.sh",
		} {
			position := strings.Index(section, command)
			if position == -1 || position <= previous {
				t.Fatalf("bundle installation command missing or unordered: %q", command)
			}
			previous = position
		}
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
			if err := yaml.Unmarshal(files["tpnext/node.yaml"], &document); err != nil {
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

func TestNodeDeploymentInstallScriptDelegatesTrustImport(t *testing.T) {
	for _, scenario := range []string{"new", "new_from_package", "same", "different", "symlink_source", "missing_source", "directory_source", "symlink_config", "validate_failure", "install_failure"} {
		t.Run(scenario, func(t *testing.T) {
			server, request, config := deploymentFixture()
			ca := deploymentCAFixture(t, 1, true, false)
			data, err := buildNodeDeploymentArchive(server, request, config, ca)
			if err != nil {
				t.Fatal(err)
			}
			_, modes := deploymentArchiveFiles(t, data)
			dir := t.TempDir()
			extractRoot := filepath.Join(dir, "bundle with spaces")
			pkg := filepath.Join(extractRoot, "tpnext")
			bin := filepath.Join(dir, "bin")
			pki := filepath.Join(dir, "custom-pki")
			for _, path := range []string{extractRoot, bin} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			archivePath := filepath.Join(dir, "node.tar.gz")
			if err := os.WriteFile(archivePath, data, 0600); err != nil {
				t.Fatal(err)
			}
			extract := exec.Command("tar", "--extract", "--gzip", "--file", archivePath, "--directory", extractRoot, "--no-same-owner", "--same-permissions")
			if output, err := extract.CombinedOutput(); err != nil {
				t.Fatalf("extract deployment archive: %v %s", err, output)
			}
			entries, err := os.ReadDir(extractRoot)
			if err != nil || len(entries) != 1 || entries[0].Name() != "tpnext" || !entries[0].IsDir() {
				t.Fatal("deployment files were scattered outside the package directory")
			}
			for name, mode := range modes {
				info, err := os.Stat(filepath.Join(extractRoot, filepath.FromSlash(name)))
				if err != nil || int64(info.Mode().Perm()) != mode {
					t.Fatalf("extracted package permissions changed for %s", name)
				}
			}
			version := strings.TrimPrefix(constant.TrojanPanelVersion, "v")
			entry := filepath.Join(dir, "entry.sh")
			events := filepath.Join(dir, "events")
			args := filepath.Join(dir, "install-args")
			// This boundary records the installer's actual authorization. It deliberately
			// never imports a CA: replacement and backup belong to the real installer.
			fakeEntry := `#!/usr/bin/env bash
if [[ "$1" == --entry-version ]]; then printf '%s\n' "$TEST_VERSION"; exit; fi
printf '%s\n' "$3" >> "$TEST_EVENTS"
case "$3" in
  validate) [[ "$TEST_VALIDATE_FAIL" == 0 ]];;
  install) printf '%s\0' "$@" > "$TEST_INSTALL_ARGS"; exit "$TEST_INSTALL_EXIT";;
  *) exit 2;;
esac
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
			var old []byte
			if scenario == "same" || scenario == "different" || scenario == "install_failure" {
				if err := os.Mkdir(pki, 0700); err != nil {
					t.Fatal(err)
				}
				old = ca
				if scenario != "same" {
					old = deploymentCAFixture(t, 2, true, false)
				}
				if err := os.WriteFile(trust, old, 0600); err != nil {
					t.Fatal(err)
				}
			}
			caSource := filepath.Join(pkg, "client-ca.crt")
			configPath := filepath.Join(pkg, "node.yaml")
			if scenario == "symlink_source" || scenario == "symlink_config" {
				unsafePath := caSource
				if scenario == "symlink_config" {
					unsafePath = configPath
				}
				target := unsafePath + ".original"
				if err := os.Rename(unsafePath, target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(target, unsafePath); err != nil {
					t.Fatal(err)
				}
			} else if scenario == "missing_source" || scenario == "directory_source" {
				if err := os.Remove(caSource); err != nil {
					t.Fatal(err)
				}
				if scenario == "directory_source" {
					if err := os.Mkdir(caSource, 0700); err != nil {
						t.Fatal(err)
					}
				}
			}
			validationFail, installExit := "0", "0"
			if scenario == "validate_failure" {
				validationFail = "1"
			}
			if scenario == "install_failure" {
				installExit = "1"
			}
			cmd := exec.Command("bash", "./tpnext/install-node.sh")
			cmd.Dir = extractRoot
			if scenario == "new_from_package" {
				cmd = exec.Command("bash", "./install-node.sh")
				cmd.Dir = pkg
			}
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "TEST_VERSION="+version, "TEST_ENTRY="+entry, "TEST_EVENTS="+events, "TEST_PKI="+pki, "TEST_VALIDATE_FAIL="+validationFail, "TEST_INSTALL_EXIT="+installExit, "TEST_INSTALL_ARGS="+args)
			output, err := cmd.CombinedOutput()
			unsafeSource := scenario == "symlink_source" || scenario == "missing_source" || scenario == "directory_source" || scenario == "symlink_config"
			shouldFail := unsafeSource || scenario == "validate_failure" || scenario == "install_failure"
			if (err != nil) != shouldFail {
				t.Fatalf("script %s: %v %s", scenario, err, output)
			}
			eventData, _ := os.ReadFile(events)
			if unsafeSource {
				if len(eventData) != 0 {
					t.Fatalf("unsafe bundle reached the release entry: %q", eventData)
				}
			} else if scenario == "validate_failure" {
				if string(eventData) != "validate\n" {
					t.Fatalf("invalid configuration reached installation: %q", eventData)
				}
			} else {
				if string(eventData) != "validate\ninstall\n" {
					t.Fatalf("wrong action order: %q", eventData)
				}
				actualArgs, err := os.ReadFile(args)
				if err != nil {
					t.Fatal(err)
				}
				wantArgs := []string{"--version", "v" + version, "install", "--config", configPath, "--client-ca", caSource}
				if got := strings.Split(strings.TrimSuffix(string(actualArgs), "\x00"), "\x00"); !reflect.DeepEqual(got, wantArgs) {
					t.Fatalf("installer lacks explicit bundle trust authorization: got=%q want=%q", got, wantArgs)
				}
			}
			if old == nil {
				if _, err := os.Stat(pki); !os.IsNotExist(err) {
					t.Fatal("bundle wrote a CA directory instead of delegating import to the installer")
				}
			} else {
				current, err := os.ReadFile(trust)
				if err != nil || !bytes.Equal(current, old) {
					t.Fatal("bundle modified retained trust before the installer accepted the import")
				}
				info, err := os.Stat(trust)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatal("bundle modified retained trust permissions")
				}
			}
			if !unsafeSource {
				info, err := os.Stat(configPath)
				if err != nil || info.Mode().Perm() != 0600 {
					t.Fatal("configuration permissions are not private")
				}
			}
		})
	}
}

func TestNodeDeploymentDependencyScriptUsesBoundReleaseOnly(t *testing.T) {
	for _, scenario := range []string{"without_deployment_dependencies", "from_package_directory", "arm64", "degraded_systemd", "download_failure", "invalid_script", "wrong_version", "version_probe_failure", "deps_failure", "version_override", "not_root", "unsupported_os", "unsupported_arch", "inactive_systemd", "missing_curl", "missing_flock"} {
		t.Run(scenario, func(t *testing.T) {
			server, request, config := deploymentFixture()
			data, err := buildNodeDeploymentArchive(server, request, config, deploymentCAFixture(t, 1, true, false))
			if err != nil {
				t.Fatal(err)
			}
			deploymentArchiveFiles(t, data)
			dir := t.TempDir()
			extractRoot := filepath.Join(dir, "bundle with spaces")
			bin := filepath.Join(dir, "bootstrap tools")
			tmp := filepath.Join(dir, "temporary downloads")
			for _, path := range []string{extractRoot, bin, tmp} {
				if err := os.Mkdir(path, 0700); err != nil {
					t.Fatal(err)
				}
			}
			archivePath := filepath.Join(dir, "node.tar.gz")
			if err := os.WriteFile(archivePath, data, 0600); err != nil {
				t.Fatal(err)
			}
			extract := exec.Command("tar", "--extract", "--gzip", "--file", archivePath, "--directory", extractRoot, "--no-same-owner", "--same-permissions")
			if output, err := extract.CombinedOutput(); err != nil {
				t.Fatalf("extract deployment archive: %v %s", err, output)
			}
			pkg := filepath.Join(extractRoot, "tpnext")
			// Dependency preparation must work before credentials/configuration are present.
			for _, name := range []string{"node.yaml", "client-ca.crt"} {
				if err := os.Remove(filepath.Join(pkg, name)); err != nil {
					t.Fatal(err)
				}
			}
			write := func(path, body string) {
				t.Helper()
				if err := os.WriteFile(path, []byte(body), 0700); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(pkg, "install-node.sh"), "#!/usr/bin/env bash\nprintf 'deployment started\\n' > \"$TEST_DEPLOYED\"\n")
			// Expose only bootstrap tools. Docker, yq, OpenSSL, find and awk are absent.
			for _, name := range []string{"bash", "grep", "mktemp", "chmod", "rm", "stat", "install", "sha256sum", "mkdir", "cp", "ln", "readlink", "dirname", "cat", "flock"} {
				tool, err := exec.LookPath(name)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(tool, filepath.Join(bin, name)); err != nil {
					t.Fatal(err)
				}
			}
			write(filepath.Join(bin, "id"), "#!/usr/bin/env bash\nprintf '%s\\n' \"$TEST_UID\"\n")
			write(filepath.Join(bin, "dpkg"), "#!/usr/bin/env bash\n[[ \"$*\" == --print-architecture ]] || exit 9\nprintf '%s\\n' \"$TEST_ARCH\"\n")
			write(filepath.Join(bin, "systemctl"), "#!/usr/bin/env bash\n[[ \"$*\" == is-system-running ]] || exit 9\nprintf '%s\\n' \"$TEST_SYSTEMD\"\n[[ \"$TEST_SYSTEMD\" == running ]]\n")
			for _, name := range []string{"apt-get", "dpkg-query"} {
				write(filepath.Join(bin, name), "#!/usr/bin/env bash\nprintf 'unexpected system mutation\\n' > \"$TEST_DEPLOYED\"\nexit 9\n")
			}
			write(filepath.Join(bin, "curl"), `#!/usr/bin/env bash
printf '%s\0' "$@" > "$TEST_CURL_ARGS"
[[ "$TEST_DOWNLOAD_FAIL" == 0 ]] || exit 22
while (($#)); do
  if [[ "$1" == -o ]]; then cp -- "$TEST_ENTRY" "$2"; exit; fi
  shift
done
exit 9
`)
			// This replaces the host OS file read at the shell boundary without any
			// production environment override or modifications to /etc/os-release.
			bashEnv := filepath.Join(dir, "os-release boundary.sh")
			write(bashEnv, `source() {
  if [[ "$1" == /etc/os-release ]]; then ID="$TEST_OS"; VERSION_ID="$TEST_OS_VERSION";
  else builtin source "$@"; fi
}
`)
			entry := filepath.Join(dir, "downloaded-entry.sh")
			write(entry, `#!/usr/bin/env bash
for tool in docker yq openssl find awk; do command -v "$tool" >/dev/null 2>&1 && exit 9; done
printf '%s\0' "$@" >> "$TEST_ENTRY_ARGS"
printf '\0' >> "$TEST_ENTRY_ARGS"
if [[ "$1" == --entry-version ]]; then
  printf '%s\n' "$TEST_ENTRY_VERSION"
  exit "$TEST_VERSION_EXIT"
fi
[[ "$#" == 4 && "$1" == --version && "$2" == "v$TEST_VERSION" && "$3" == deps && "$4" == install ]] || {
  printf 'unexpected deployment command\n' > "$TEST_DEPLOYED"; exit 9;
}
[[ "$(stat -c %a "$0")" == 600 && "$(stat -c %a "${0%/*}")" == 700 ]] || exit 9
exit "$TEST_DEPS_EXIT"
`)
			version := strings.TrimPrefix(constant.TrojanPanelVersion, "v")
			entryVersion, uid, arch, osID, osVersion, systemd := version, "0", "amd64", "debian", "12", "running"
			downloadFail, versionExit, depsExit := "0", "0", "0"
			args := []string{"./tpnext/install-dependencies.sh"}
			switch scenario {
			case "arm64":
				arch, osID, osVersion = "arm64", "ubuntu", "24.04"
			case "degraded_systemd":
				systemd, osVersion = "degraded", "13"
			case "download_failure":
				downloadFail = "1"
			case "invalid_script":
				write(entry, "#!/usr/bin/env bash\nif (; then\n")
			case "wrong_version":
				entryVersion = "0.0.0"
			case "version_probe_failure":
				versionExit = "1"
			case "deps_failure":
				depsExit = "17"
			case "version_override":
				args = append(args, "--version", "v0.0.0")
			case "not_root":
				uid = "1000"
			case "unsupported_os":
				osID, osVersion = "arch", "rolling"
			case "unsupported_arch":
				arch = "i386"
			case "inactive_systemd":
				systemd = "offline"
			case "missing_flock":
				if err := os.Remove(filepath.Join(bin, "flock")); err != nil {
					t.Fatal(err)
				}
			case "missing_curl":
				if err := os.Remove(filepath.Join(bin, "curl")); err != nil {
					t.Fatal(err)
				}
			}
			bash, err := exec.LookPath("bash")
			if err != nil {
				t.Fatal(err)
			}
			cmd := exec.Command(bash, args...)
			cmd.Dir = extractRoot
			if scenario == "from_package_directory" {
				cmd.Args[1] = "./install-dependencies.sh"
				cmd.Dir = pkg
			}
			curlArgsPath, entryArgsPath, deployed := filepath.Join(dir, "curl-args"), filepath.Join(dir, "entry-args"), filepath.Join(dir, "deployed")
			cmd.Env = append(os.Environ(), "PATH="+bin, "TMPDIR="+tmp, "BASH_ENV="+bashEnv, "TEST_VERSION="+version, "TEST_ENTRY="+entry, "TEST_ENTRY_VERSION="+entryVersion, "TEST_VERSION_EXIT="+versionExit, "TEST_UID="+uid, "TEST_ARCH="+arch, "TEST_OS="+osID, "TEST_OS_VERSION="+osVersion, "TEST_SYSTEMD="+systemd, "TEST_DOWNLOAD_FAIL="+downloadFail, "TEST_DEPS_EXIT="+depsExit, "TEST_CURL_ARGS="+curlArgsPath, "TEST_ENTRY_ARGS="+entryArgsPath, "TEST_DEPLOYED="+deployed)
			output, err := cmd.CombinedOutput()
			shouldSucceed := scenario == "without_deployment_dependencies" || scenario == "from_package_directory" || scenario == "arm64" || scenario == "degraded_systemd"
			if (err == nil) != shouldSucceed {
				t.Fatalf("dependency preparation %s: %v %s", scenario, err, output)
			}
			if scenario == "deps_failure" {
				if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 17 {
					t.Fatalf("dependency error status lost: %v %s", err, output)
				}
			}
			if _, err := os.Stat(deployed); !os.IsNotExist(err) {
				t.Fatal("dependency script started deployment or directly invoked APT")
			}
			entries, err := os.ReadDir(tmp)
			if err != nil || len(entries) != 0 {
				t.Fatalf("temporary release downloads leaked: %v %v", entries, err)
			}
			actualCurl, _ := os.ReadFile(curlArgsPath)
			beforeDownloadFailure := scenario == "version_override" || scenario == "not_root" || scenario == "unsupported_os" || scenario == "unsupported_arch" || scenario == "inactive_systemd" || scenario == "missing_curl" || scenario == "missing_flock"
			if beforeDownloadFailure {
				if len(actualCurl) != 0 {
					t.Fatalf("unsupported prerequisites reached download: %q", actualCurl)
				}
			} else {
				curlArgs := strings.Split(strings.TrimSuffix(string(actualCurl), "\x00"), "\x00")
				want := []string{"--fail", "--location", "--silent", "--show-error", "--proto", "=https", "--proto-redir", "=https", "--retry", "2", "--retry-max-time", "180", "--connect-timeout", "10", "--max-time", "60", "--max-filesize", "5242880", "https://raw.githubusercontent.com/1linhao/trojanpanelnext/v" + version + "/scripts/tp.sh"}
				if len(curlArgs) != len(want)+2 || !reflect.DeepEqual(curlArgs[:len(want)], want) || curlArgs[len(want)] != "-o" || filepath.Dir(filepath.Dir(curlArgs[len(want)+1])) != tmp {
					t.Fatalf("entry download changed release/security boundary: %q", curlArgs)
				}
			}
			actualEntry, _ := os.ReadFile(entryArgsPath)
			wantCalls := ""
			if shouldSucceed || scenario == "deps_failure" {
				wantCalls = "--entry-version\x00\x00--version\x00v" + version + "\x00deps\x00install\x00\x00"
			} else if scenario == "wrong_version" || scenario == "version_probe_failure" {
				wantCalls = "--entry-version\x00\x00"
			}
			if string(actualEntry) != wantCalls {
				t.Fatalf("release entry received unexpected actions: %q", actualEntry)
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

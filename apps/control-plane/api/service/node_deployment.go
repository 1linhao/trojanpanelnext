package service

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net"
	"net/mail"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/go-playground/validator/v10"
	"gopkg.in/yaml.v3"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
)

const publicCALimit = 1024 * 1024

func NodeServerDeployment(id uint) (*vo.NodeDeploymentVo, error) {
	nodeLifecycle.RLock()
	defer nodeLifecycle.RUnlock()
	server, err := deploymentServer(id)
	if err != nil {
		return nil, err
	}
	webHost, err := deploymentWebHost()
	if err != nil {
		return nil, err
	}
	return deploymentMetadata(server, webHost, *core.Config)
}

func DownloadNodeDeployment(request dto.NodeDeploymentDownloadDto) ([]byte, error) {
	nodeLifecycle.RLock()
	defer nodeLifecycle.RUnlock()
	server, err := deploymentServer(request.Id)
	if err != nil {
		return nil, err
	}
	if err = validateDeploymentRequest(request); err != nil {
		return nil, err
	}
	ca, err := readDeploymentCA(os.Getenv("TP_PKI_AUTHORITY_DIR"))
	if err != nil {
		return nil, err
	}
	return buildNodeDeploymentArchive(server, request, *core.Config, ca)
}

func deploymentServer(id uint) (*model.NodeServer, error) {
	if id == 0 {
		return nil, errors.New("a valid node server ID is required")
	}
	server, err := dao.SelectNodeServer(map[string]interface{}{"id": id})
	if err != nil {
		return nil, err
	}
	if err := validateNodeDeploymentServer(server); err != nil {
		return nil, err
	}
	return server, nil
}

func validateNodeDeploymentServer(server *model.NodeServer) error {
	if server == nil {
		return errors.New("node server does not exist")
	}
	if server.Removing != nil && *server.Removing != 0 {
		return errors.New("node server is being removed; deployment is unavailable")
	}
	if server.Id == nil || *server.Id == 0 || server.Name == nil || server.Ip == nil || server.GrpcPort == nil || *server.GrpcPort == 0 || *server.GrpcPort > 65534 {
		return errors.New("node server registration must include a valid server ID, address and gRPC port")
	}
	if server.GrpcTLSMode == nil || *server.GrpcTLSMode != "mtls" || server.GrpcTLSServerName == nil || validator.New().Var(*server.GrpcTLSServerName, "fqdn") != nil {
		return errors.New("register this server with mTLS and a valid TLS domain before requesting its deployment bundle")
	}
	return nil
}

func deploymentWebHost() (string, error) {
	callback, err := core.HostRemovalCallbackURL()
	if err != nil {
		return "", errors.New("Web public hostname is unavailable; configure TP_HOST_REMOVAL_CALLBACK_URL")
	}
	parsed, err := url.Parse(callback)
	if err != nil || !validDeploymentHost(parsed.Hostname()) {
		return "", errors.New("Web public hostname is invalid; configure TP_HOST_REMOVAL_CALLBACK_URL")
	}
	return parsed.Hostname(), nil
}

func validDeploymentHost(host string) bool {
	if host == "" || len(host) > 253 || strings.ContainsAny(host, "\x00\r\n\t /\\@?#") {
		return false
	}
	if ip := net.ParseIP(host); ip != nil {
		return !ip.IsLoopback() && !ip.IsUnspecified() && !ip.IsMulticast()
	}
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return false
	}
	return validator.New().Var(host, "fqdn") == nil
}

func deploymentConnectionHost(host, webHost string) (string, bool) {
	if strings.EqualFold(strings.TrimSuffix(host, "."), "localhost") {
		return webHost, true
	}
	if ip := net.ParseIP(host); ip != nil && (ip.IsLoopback() || ip.IsUnspecified()) {
		return webHost, true
	}
	return host, false
}

func deploymentMetadata(server *model.NodeServer, webHost string, config core.AppConfig) (*vo.NodeDeploymentVo, error) {
	version := strings.TrimPrefix(constant.TrojanPanelVersion, "v")
	if !regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?(?:-[0-9A-Za-z]+(?:[.-][0-9A-Za-z]+)*)?$`).MatchString(version) {
		return nil, errors.New("deployment release is invalid")
	}
	if !validDeploymentHost(webHost) {
		return nil, errors.New("webHost must be a reachable hostname or IP without a scheme or port")
	}
	if config.MySQLConfig.Host == "" || config.RedisConfig.Host == "" || config.MySQLConfig.Port < 1 || config.MySQLConfig.Port > 65535 || config.RedisConfig.Port < 1 || config.RedisConfig.Port > 65535 {
		return nil, errors.New("Web database connection configuration is unavailable")
	}
	mariadbHost, mariadbUsesWebHost := deploymentConnectionHost(config.MySQLConfig.Host, webHost)
	redisHost, redisUsesWebHost := deploymentConnectionHost(config.RedisConfig.Host, webHost)
	return &vo.NodeDeploymentVo{
		NodeServerRegistrationVo: vo.NodeServerRegistrationVo{Id: *server.Id, Name: *server.Name, Ip: *server.Ip, GrpcPort: *server.GrpcPort, GrpcTLSServerName: *server.GrpcTLSServerName},
		Version:                  version, WebHost: webHost, MariaDBHost: mariadbHost, MariaDBPort: config.MySQLConfig.Port, MariaDBUsesWebHost: mariadbUsesWebHost,
		RedisHost: redisHost, RedisPort: config.RedisConfig.Port, RedisUsesWebHost: redisUsesWebHost,
		DocsURL: "https://github.com/1linhao/trojanpanelnext/blob/v" + version + "/docs/deployment.md#node-deployment-package",
	}, nil
}

func validateDeploymentRequest(request dto.NodeDeploymentDownloadDto) error {
	if request.Id == 0 || !validDeploymentHost(request.WebHost) {
		return errors.New("a valid server ID and reachable webHost are required")
	}
	switch request.CertificateMode {
	case "caddy":
		address, err := mail.ParseAddress(request.Email)
		if err != nil || address.Address != request.Email || validator.New().Var(request.Email, "email") != nil {
			return errors.New("a valid email is required in Caddy mode")
		}
		if request.CertificatePath != "" || request.PrivateKeyPath != "" {
			return errors.New("certificate paths apply only to external mode")
		}
	case "external":
		for _, path := range []string{request.CertificatePath, request.PrivateKeyPath} {
			if !filepath.IsAbs(path) || filepath.Clean(path) == "/" || len(path) > 4096 || strings.ContainsAny(path, "\x00\r\n,") {
				return errors.New("external certificate and private key paths must be absolute and contain no commas or line breaks")
			}
		}
	default:
		return errors.New("certificateMode must be caddy or external")
	}
	return nil
}

func readDeploymentCA(authorityDir string) ([]byte, error) {
	if !filepath.IsAbs(authorityDir) {
		return nil, errors.New("Web mTLS authority is unavailable")
	}
	path := filepath.Join(authorityDir, "client-ca.crt")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Size() > publicCALimit {
		return nil, errors.New("Web public client CA is unavailable or invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return nil, errors.New("Web public client CA is unavailable or invalid")
	}
	defer file.Close()
	ca, err := io.ReadAll(io.LimitReader(file, publicCALimit+1))
	if err != nil || len(ca) > publicCALimit || validateDeploymentCA(ca, time.Now()) != nil {
		return nil, errors.New("Web public client CA is unavailable or invalid")
	}
	return ca, nil
}

func validateDeploymentCA(data []byte, now time.Time) error {
	remaining := bytes.TrimSpace(data)
	count := 0
	for len(remaining) > 0 {
		if !bytes.HasPrefix(remaining, []byte("-----BEGIN CERTIFICATE-----")) {
			return errors.New("CA bundle contains non-certificate material")
		}
		block, rest := pem.Decode(remaining)
		if block == nil || block.Type != "CERTIFICATE" || len(block.Headers) != 0 {
			return errors.New("CA bundle contains invalid PEM")
		}
		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil || !certificate.IsCA || !certificate.BasicConstraintsValid || (certificate.KeyUsage != 0 && certificate.KeyUsage&x509.KeyUsageCertSign == 0) || now.Before(certificate.NotBefore) || !now.Before(certificate.NotAfter) {
			return errors.New("CA bundle contains an invalid CA certificate")
		}
		remaining = bytes.TrimSpace(rest)
		count++
	}
	if count == 0 {
		return errors.New("CA bundle is empty")
	}
	return nil
}

func buildNodeDeploymentArchive(server *model.NodeServer, request dto.NodeDeploymentDownloadDto, config core.AppConfig, ca []byte) ([]byte, error) {
	if err := validateDeploymentRequest(request); err != nil {
		return nil, err
	}
	metadata, err := deploymentMetadata(server, request.WebHost, config)
	if err != nil {
		return nil, err
	}
	if config.MySQLConfig.User == "" || config.MySQLConfig.Password == "" || config.RedisConfig.Password == "" || config.RedisConfig.Db != 0 {
		return nil, errors.New("Web deployment credentials are unavailable or its Redis database is unsupported")
	}
	if err = validateDeploymentCA(ca, time.Now()); err != nil {
		return nil, errors.New("Web public client CA is invalid")
	}
	settings := map[string]interface{}{
		"release": metadata.Version, "schema_version": 1, "purpose": "node", "hostname": metadata.GrpcTLSServerName, "email": request.Email,
		"caddy_image": "caddy:2.8.4", "core_image": "ghcr.io/1linhao/trojanpanelnext-node-agent:" + metadata.Version,
		"node_certificate_mode": request.CertificateMode, "node_certificate_path": request.CertificatePath, "node_private_key_path": request.PrivateKeyPath,
		"node_caddy_http_port": 80, "node_caddy_https_port": 8863,
		"mariadb_host": metadata.MariaDBHost, "mariadb_port": metadata.MariaDBPort, "mariadb_user": config.MySQLConfig.User, "mariadb_password": config.MySQLConfig.Password,
		"database": "trojan_panel_db", "account_table": "account", "redis_host": metadata.RedisHost, "redis_port": metadata.RedisPort, "redis_password": config.RedisConfig.Password,
		"grpc_port": metadata.GrpcPort, "core_port": 8082, "node_server_id": metadata.Id, "grpc_tls_mode": "mtls", "grpc_tls_server_name": metadata.GrpcTLSServerName,
		"grpc_client_ca_path": "/tpdata/trojan-panel-core/pki/client-ca.crt", "pki_bundle_dir": "/tpdata/trojanpanelnext-pki", "kernel_runtime_path": "/tpdata/trojan-panel-core/runtime", "force": 0, "purge_data": 0,
	}
	configYAML, err := yaml.Marshal(map[string]interface{}{"trojanpanelnext": settings})
	if err != nil {
		return nil, errors.New("cannot create Node deployment configuration")
	}
	readme := fmt.Sprintf(`# Node 部署包

此包用于部署已登记的 Node 服务器 ID %[1]d，版本 v%[2]s。

## 保护部署文件

归档和 node.yaml 含 MariaDB、Redis 的真实连接凭据。请将归档和配置权限设为 600，使用安全方式传输到 Node 主机，并存放在私有目录中；不要公开或上传到仓库。包中仅包含公开的客户端 CA，不导出 Web 客户端私钥、CA 私钥或外部证书文件。

## 安装

自动依赖准备支持 Debian 12/13、Ubuntu 22.04/24.04，架构为 amd64/arm64，需要以 root 使用 Bash，宿主机运行 systemd。其他 Linux 发行版需手动准备部署依赖后直接运行安装入口。请先在 Node 主机上使用 APT 安装最小引导包：

    apt-get update
    apt-get install -y --no-install-recommends bash curl ca-certificates grep coreutils util-linux tar gzip

其中 util-linux 提供 flock，tar 和 gzip 用于解包。以 root 安全解压已下载的归档；请使用新建的私有目录，避免覆盖已有文件：

    chmod 600 ./tpnext-node-%[1]d.tar.gz
    umask 077
    mkdir -m 700 ./node-deployment
    tar --extract --gzip --file ./tpnext-node-%[1]d.tar.gz --directory ./node-deployment --no-same-owner --same-permissions
    cd ./node-deployment

归档会生成权限为 700 的 tpnext/ 目录。先准备部署依赖：

    bash ./tpnext/install-dependencies.sh

依赖脚本固定调用 v%[2]s 的脚本库，为本版本准备 Docker、mikefarah/yq v4、openssl、findutils、awk 等工具；无需预先安装这些工具。它不读取 node.yaml 或客户端 CA，也不会自动部署 Node。依赖准备成功后检查 tpnext/node.yaml，再安装 Node：

    bash ./tpnext/install-node.sh

也可以先 cd tpnext，再依次运行 bash ./install-dependencies.sh 和 bash ./install-node.sh。外部证书模式要求该主机上已存在配置指向的完整证书链和私钥文件。安装脚本先验证当前版本配置，再将包内当前 Web 的客户端 CA 交给安装器。安装器校验该 CA，并在重新部署时先备份、再替换不同的旧 CA，避免继续信任旧控制端。安装不默认启用 force。

完整说明：%[3]s

## English instructions

This bundle deploys the registered Node server ID %[1]d using release v%[2]s. Keep the credential-bearing archive and node.yaml private (chmod 600) and transfer them securely. Only the public client CA is included; Web client/CA private keys and external certificate files are excluded.

Automatic dependency preparation supports Debian 12/13 or Ubuntu 22.04/24.04 on amd64/arm64. Run Bash as root on a host running systemd. Other Linux distributions require manual dependency preparation before running the Node installer. First install the minimum bootstrap packages on the Node host:

    apt-get update
    apt-get install -y --no-install-recommends bash curl ca-certificates grep coreutils util-linux tar gzip

util-linux provides flock; tar and gzip extract the archive. Extract as root into a new private directory to avoid overwriting existing files:

    chmod 600 ./tpnext-node-%[1]d.tar.gz
    umask 077
    mkdir -m 700 ./node-deployment
    tar --extract --gzip --file ./tpnext-node-%[1]d.tar.gz --directory ./node-deployment --no-same-owner --same-permissions
    cd ./node-deployment

The archive creates a mode-700 tpnext/ directory. Prepare deployment dependencies first:

    bash ./tpnext/install-dependencies.sh

This script uses only the v%[2]s script library to prepare Docker, mikefarah/yq v4, openssl, findutils, awk and other deployment tools. These tools are not required beforehand. It does not read node.yaml or the client CA and never starts Node deployment. After it succeeds, review tpnext/node.yaml and install Node:

    bash ./tpnext/install-node.sh

Alternatively, cd tpnext and run bash ./install-dependencies.sh followed by bash ./install-node.sh. External certificate mode requires the configured full certificate chain and private key to already exist on the Node host. The installer validates the bundle's current Web public client CA and backs up any different retained CA before replacing it. Installation does not enable force.

Full guide: %[3]s
`, metadata.Id, metadata.Version, metadata.DocsURL)

	files := []struct {
		name string
		mode int64
		data []byte
	}{
		{"node.yaml", 0600, configYAML}, {"client-ca.crt", 0644, ca},
		{"install-dependencies.sh", 0700, []byte(strings.ReplaceAll(nodeDeploymentDependencyScript, "@@VERSION@@", metadata.Version))},
		{"install-node.sh", 0700, []byte(strings.ReplaceAll(nodeDeploymentInstallScript, "@@VERSION@@", metadata.Version))},
		{"README.md", 0600, []byte(readme)},
	}
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	archive := tar.NewWriter(compressed)
	if err = archive.WriteHeader(&tar.Header{Name: "tpnext/", Mode: 0700, Typeflag: tar.TypeDir}); err != nil {
		return nil, errors.New("cannot create Node deployment archive")
	}
	for _, file := range files {
		if err = archive.WriteHeader(&tar.Header{Name: "tpnext/" + file.name, Mode: file.mode, Size: int64(len(file.data)), Typeflag: tar.TypeReg}); err != nil {
			return nil, errors.New("cannot create Node deployment archive")
		}
		if _, err = archive.Write(file.data); err != nil {
			return nil, errors.New("cannot create Node deployment archive")
		}
	}
	if err = archive.Close(); err != nil {
		return nil, errors.New("cannot create Node deployment archive")
	}
	if err = compressed.Close(); err != nil {
		return nil, errors.New("cannot create Node deployment archive")
	}
	return buffer.Bytes(), nil
}

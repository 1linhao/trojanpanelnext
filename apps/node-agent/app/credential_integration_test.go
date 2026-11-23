package app

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"trojan-panel-core/app/xray"
	"trojan-panel-core/core"
	"trojan-panel-core/dao"
	"trojan-panel-core/model/constant"
)

// The control-plane integration test owns the disposable database and sends sync
// requests over stdin. Both inbounds and all test traffic are loopback only.
func TestCredentialNodeFixture(t *testing.T) {
	if os.Getenv("TP_CREDENTIAL_NODE_FIXTURE") != "1" {
		t.Skip("invoked by the credential integration test")
	}
	cfg, err := mysql.ParseDSN(os.Getenv("TP_CREDENTIAL_TEST_DSN"))
	if err != nil {
		t.Fatal(err)
	}
	host, port, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		t.Fatal(err)
	}
	dbPort, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	servicePort, err := strconv.Atoi(os.Getenv("TP_CREDENTIAL_SOCKS_PORT"))
	if err != nil {
		t.Fatal(err)
	}
	apiPort := servicePort + 30000
	core.Config.MySQLConfig = core.MySQLConfig{Host: host, Port: dbPort, User: cfg.User, Password: cfg.Passwd, Database: "trojan_panel_db", AccountTable: "account"}
	core.Config.NodeConfig.ServerID = 7
	dao.InitMySQL()
	defer dao.CloseDb()
	t.Setenv("TP_KERNEL_RUNTIME", filepath.Join(t.TempDir(), "runtime"))
	if err = os.MkdirAll(constant.XrayPath, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Symlink(os.Getenv("TP_CREDENTIAL_XRAY"), filepath.Join(constant.XrayBinPath, "xray")); err != nil {
		t.Fatal(err)
	}
	config := fmt.Sprintf(`{"inbounds":[{"listen":"127.0.0.1","port":%d,"protocol":"dokodemo-door","settings":{"address":"127.0.0.1"},"tag":"api"},{"listen":"127.0.0.1","port":%d,"protocol":"socks","tag":"custom-socks","settings":{"auth":"password","accounts":[{"user":"fixture","pass":"fixture-shared"}],"udp":false}}],"outbounds":[{"protocol":"freedom"}],"api":{"tag":"api","services":["HandlerService","StatsService"]},"stats":{},"routing":{"rules":[{"inboundTag":["api"],"outboundTag":"api","type":"field"}]}}`, apiPort, servicePort)
	path := filepath.Join(constant.XrayPath, fmt.Sprintf("config-%d-socks.json", apiPort))
	if err = os.WriteFile(path, []byte(config), 0600); err != nil {
		t.Fatal(err)
	}
	if err = xray.InitXrayApp(); err != nil {
		t.Fatal(err)
	}
	defer xray.StopXray(uint(apiPort), true)
	deadline := time.Now().Add(4 * time.Second)
	for {
		c, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", apiPort), 100*time.Millisecond)
		if err == nil {
			c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("Xray API not ready")
		}
		time.Sleep(20 * time.Millisecond)
	}
	fmt.Println("CREDENTIAL_FIXTURE_READY")
	input := bufio.NewScanner(os.Stdin)
	for input.Scan() {
		CronHandlerUser()
		fmt.Println("CREDENTIAL_FIXTURE_SYNCED")
	}
}

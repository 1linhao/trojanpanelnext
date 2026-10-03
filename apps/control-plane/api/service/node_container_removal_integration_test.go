package service

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"database/sql"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
)

// Requires an unused disposable MariaDB instance. Exclusive creation of the
// initializer's fixed DB name prevents modifying an existing deployment.
func containerRemovalIntegrationDB(t *testing.T) *sql.DB {
	t.Helper()
	dsn := os.Getenv("TP_NODE_CONTAINER_TEST_DSN")
	if dsn == "" {
		t.Skip("TP_NODE_CONTAINER_TEST_DSN is not set")
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil {
		t.Fatal(err)
	}
	host, portText, err := net.SplitHostPort(config.Addr)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	config.DBName = ""
	admin, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { admin.Close() })
	if _, err := admin.Exec("CREATE DATABASE trojan_panel_db CHARACTER SET utf8mb4"); err != nil {
		t.Fatalf("requires unused disposable database: %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec("DROP DATABASE trojan_panel_db"); err != nil {
			t.Error(err)
		}
	})
	previous := *core.Config
	core.Config.MySQLConfig = core.MySQLConfig{Host: host, Port: port, User: config.User, Password: config.Passwd}
	t.Cleanup(func() { *core.Config = previous })
	dao.InitMySQL()
	t.Cleanup(dao.CloseDb)
	config.DBName = "trojan_panel_db"
	testDB, err := sql.Open("mysql", config.FormatDSN())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { testDB.Close() })
	t.Setenv("TP_HOST_REMOVAL_CALLBACK_URL", "https://panel.example.test/api/nodeServer/completeHostRemoval")
	return testDB
}

func containerRemovalIntegrationHost(t *testing.T, handler http.HandlerFunc) (string, uint) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "removal fixture"}, DNSNames: []string{"node.example.test"}, NotBefore: time.Now().Add(-time.Hour), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER})
	pair, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(certPEM)
	directory := t.TempDir()
	for name, data := range map[string][]byte{"client.crt": certPEM, "client.key": keyPEM, "ca.crt": certPEM} {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	previous, managed := core.Config.GrpcConfig, core.ManagedClientCertificate
	core.Config.GrpcConfig.ClientCertPath = filepath.Join(directory, "client.crt")
	core.Config.GrpcConfig.ClientKeyPath = filepath.Join(directory, "client.key")
	core.Config.GrpcConfig.ServerCAPath = filepath.Join(directory, "ca.crt")
	core.ManagedClientCertificate = nil
	t.Cleanup(func() { core.Config.GrpcConfig = previous; core.ManagedClientCertificate = managed })
	server := httptest.NewUnstartedServer(handler)
	server.TLS = &tls.Config{Certificates: []tls.Certificate{pair}, ClientCAs: pool, ClientAuth: tls.RequireAndVerifyClientCert, MinVersion: tls.VersionTLS12}
	server.StartTLS()
	t.Cleanup(server.Close)
	host, portText, err := net.SplitHostPort(server.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	return host, uint(port)
}

func TestContainerUpdateBlocksUninstallWithoutChangingRegistration(t *testing.T) {
	testDB := containerRemovalIntegrationDB(t)
	var status atomic.Value
	status.Store("queued")
	var removals atomic.Int32
	host, port := containerRemovalIntegrationHost(t, func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/container/inventory":
			job := &core.HostContainerJob{ID: strings.Repeat("a", 64), FromVersion: "1.0.2-rc.11", TargetVersion: "1.0.2-rc.12", Status: status.Load().(string)}
			if job.Status == "running" {
				job.StartedAt = "2026-10-03T08:00:00Z"
			}
			json.NewEncoder(w).Encode(core.HostContainerInventory{NodeID: 7, CurrentVersion: "1.0.2-rc.11", Image: "ghcr.io/1linhao/trojanpanelnext-node-agent:1.0.2-rc.11", UpdateSupported: true, Job: job})
		case "/remove":
			removals.Add(1)
			http.Error(w, "container_update_active", http.StatusConflict)
		default:
			t.Error("unexpected host path")
		}
	})
	if _, err := testDB.Exec(`INSERT INTO node_server(id,name,ip,grpc_port,grpc_tls_mode,grpc_tls_server_name) VALUES(7,'fixture',?,?,'mtls','node.example.test')`, host, port-1); err != nil {
		t.Fatal(err)
	}
	for _, current := range []string{"queued", "running"} {
		t.Run(current, func(t *testing.T) {
			status.Store(current)
			removals.Store(0)
			if _, err := testDB.Exec(`UPDATE node_server SET removing=0 WHERE id=7`); err != nil {
				t.Fatal(err)
			}
			id := uint(7)
			if result, err := UninstallNodeServerById(&id, false); err == nil || result != nil {
				t.Fatalf("busy uninstall accepted: %+v %v", result, err)
			}
			var flag int
			if err := testDB.QueryRow(`SELECT removing FROM node_server WHERE id=7`).Scan(&flag); err != nil || flag != 0 {
				t.Errorf("busy uninstall polluted flag: %d %v", flag, err)
			}
			if removals.Load() != 0 {
				t.Error("busy uninstall reached host removal instead of rejecting before Begin")
			}
		})
	}
	t.Run("kernel_create_and_retry_do_not_persist_under_active_container", func(t *testing.T) {
		if _, err := testDB.Exec(`INSERT INTO kernel_upgrade_task(id,operator_id,operator_name,status) VALUES(501,1,'fixture','failed')`); err != nil {
			t.Fatal(err)
		}
		if _, err := testDB.Exec(`INSERT INTO kernel_upgrade_task_item(id,task_id,node_server_id,node_server_name,kernel_name,target_version,channel_name,stage,result,idempotency_key) VALUES(601,501,7,'fixture','xray','1.0.0','stable','failed','failed','fixture-retry')`); err != nil {
			t.Fatal(err)
		}
		request := dto.KernelTaskCreateDto{NodeServerIds: []uint{7}, Targets: []dto.KernelTargetDto{{Kernel: "xray", Version: "1.0.0", Channel: "stable"}}}
		if _, err := CreateKernelTask(request, vo.AccountVo{Id: 1, Username: "fixture"}, ""); !errors.Is(err, core.ErrHostContainerUpdateActive) {
			t.Fatalf("active container allowed kernel creation: %v", err)
		}
		if err := RetryKernelTask(dto.KernelTaskRetryDto{Id: 501}, ""); !errors.Is(err, core.ErrHostContainerUpdateActive) {
			t.Fatalf("active container allowed kernel retry: %v", err)
		}
		var tasks, attempt int
		var stage, result string
		if err := testDB.QueryRow(`SELECT COUNT(*) FROM kernel_upgrade_task`).Scan(&tasks); err != nil || tasks != 1 {
			t.Fatalf("blocked creation persisted task: %d %v", tasks, err)
		}
		if err := testDB.QueryRow(`SELECT stage,result,attempt FROM kernel_upgrade_task_item WHERE id=601`).Scan(&stage, &result, &attempt); err != nil || stage != "failed" || result != "failed" || attempt != 1 {
			t.Fatalf("blocked retry mutated task: %s %s %d %v", stage, result, attempt, err)
		}
	})
}

func TestPendingUninstallRetrySkipsUnavailableContainerInventory(t *testing.T) {
	testDB := containerRemovalIntegrationDB(t)
	var inventories atomic.Int32
	host, port := containerRemovalIntegrationHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/container/inventory" {
			inventories.Add(1)
			http.Error(w, "container_inventory_unavailable", 503)
			return
		}
		if r.URL.Path != "/remove" && r.URL.Path != "/finalize" {
			t.Error("unexpected host request")
		}
		json.NewEncoder(w).Encode(core.HostRemoval{NodeID: 7, Receipt: strings.Repeat("a", 64), Success: true})
	})
	if _, err := testDB.Exec(`INSERT INTO node_server(id,name,ip,grpc_port,grpc_tls_mode,grpc_tls_server_name,removing) VALUES(7,'fixture',?,?,'mtls','node.example.test',1)`, host, port-1); err != nil {
		t.Fatal(err)
	}
	id := uint(7)
	result, err := UninstallNodeServerById(&id, false)
	if err != nil || result == nil || !result.CleanupPending {
		t.Fatalf("completed host uninstall cannot recover: %+v %v", result, err)
	}
	var nodes, receipts int
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM node_server WHERE id=7`).Scan(&nodes); err != nil || nodes != 0 {
		t.Fatalf("completed uninstall registration retained: %d %v", nodes, err)
	}
	if err := testDB.QueryRow(`SELECT COUNT(*) FROM node_removal_cleanup WHERE node_server_id=7`).Scan(&receipts); err != nil || receipts != 1 {
		t.Fatalf("uninstall cleanup receipt lost: %d %v", receipts, err)
	}
	if inventories.Load() != 0 {
		t.Fatal("pending uninstall retry required a removed container inventory")
	}
}

func TestUninstallContainerRaceResetsOnlyConfirmedUnstartedAttempt(t *testing.T) {
	testDB := containerRemovalIntegrationDB(t)
	var refusal atomic.Value
	refusal.Store("container_update_active")
	host, port := containerRemovalIntegrationHost(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/container/inventory" {
			json.NewEncoder(w).Encode(core.HostContainerInventory{NodeID: 7, CurrentVersion: "1.0.2-rc.11", Image: "ghcr.io/1linhao/trojanpanelnext-node-agent:1.0.2-rc.11", UpdateSupported: true})
			return
		}
		if r.URL.Path != "/remove" {
			t.Error("unexpected host request")
		}
		switch refusal.Load().(string) {
		case "connection_lost":
			connection, _, err := w.(http.Hijacker).Hijack()
			if err != nil {
				t.Error(err)
			} else {
				connection.Close()
			}
		case "uninstall_failed":
			http.Error(w, "host uninstall failed; registration retained", 500)
		default:
			http.Error(w, refusal.Load().(string), 409)
		}
	})
	if _, err := testDB.Exec(`INSERT INTO node_server(id,name,ip,grpc_port,grpc_tls_mode,grpc_tls_server_name) VALUES(7,'fixture',?,?,'mtls','node.example.test')`, host, port-1); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		name, body    string
		initial, want int
		outbox        bool
	}{
		{"new_unstarted", "container_update_active", 0, 0, false},
		{"already_pending", "container_update_active", 1, 1, false},
		{"unknown_conflict", "host_removal_pending", 0, 1, false},
		{"execution_failure", "uninstall_failed", 0, 1, false},
		{"network_unknown", "connection_lost", 0, 1, false},
		{"saved_receipt", "container_update_active", 0, 1, true},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			refusal.Store(scenario.body)
			if _, err := testDB.Exec(`DELETE FROM node_removal_cleanup WHERE node_server_id=7`); err != nil {
				t.Fatal(err)
			}
			if _, err := testDB.Exec(`UPDATE node_server SET removing=? WHERE id=7`, scenario.initial); err != nil {
				t.Fatal(err)
			}
			if scenario.outbox {
				if _, err := testDB.Exec(`INSERT INTO node_removal_cleanup(node_server_id,ip,port,server_name,purge_data,receipt) VALUES(7,?,?,'node.example.test',0,?)`, host, port, strings.Repeat("b", 64)); err != nil {
					t.Fatal(err)
				}
			}
			id := uint(7)
			if result, err := UninstallNodeServerById(&id, false); err == nil || result != nil {
				t.Fatalf("uninstall refusal accepted: %+v %v", result, err)
			}
			var flag int
			if err := testDB.QueryRow(`SELECT removing FROM node_server WHERE id=7`).Scan(&flag); err != nil || flag != scenario.want {
				t.Fatalf("unsafe marker reset: %d want=%d err=%v", flag, scenario.want, err)
			}
		})
	}
}

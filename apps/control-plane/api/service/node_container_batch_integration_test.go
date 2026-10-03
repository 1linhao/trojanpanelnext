package service

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"sync"
	"testing"
	"time"

	"trojan-panel/core"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
)

func seedKernelBatchAdmission(t *testing.T, db *sql.DB, host string, port uint) {
	t.Helper()
	for _, id := range []uint{7, 8} {
		if _, err := db.Exec(`INSERT INTO node_server(id,name,ip,grpc_port,grpc_tls_mode,grpc_tls_server_name) VALUES(?,'fixture',?,?,'mtls','node.example.test') ON DUPLICATE KEY UPDATE ip=VALUES(ip),grpc_port=VALUES(grpc_port),grpc_tls_mode='mtls',grpc_tls_server_name='node.example.test',removing=0`, id, host, port-1); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.Exec(`INSERT INTO kernel_upgrade_task(id,operator_id,operator_name,status) VALUES(501,1,'fixture','failed') ON DUPLICATE KEY UPDATE status='failed'`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO kernel_upgrade_task_item(id,task_id,node_server_id,node_server_name,kernel_name,target_version,channel_name,stage,result,idempotency_key) VALUES(601,501,7,'fixture','xray','1.0.0','stable','failed','failed','fixture-retry') ON DUPLICATE KEY UPDATE stage='failed',result='failed',attempt=1`); err != nil {
		t.Fatal(err)
	}
}

func kernelBatchAdmissionAttempt(kind string) error {
	if kind == "retry" {
		return RetryKernelTask(dto.KernelTaskRetryDto{Id: 501}, "")
	}
	_, err := CreateKernelTask(dto.KernelTaskCreateDto{NodeServerIds: []uint{7}, Targets: []dto.KernelTargetDto{{Kernel: "xray", Version: "1.0.0", Channel: "stable"}}}, vo.AccountVo{Id: 1, Username: "fixture"}, "")
	return err
}

func TestKernelPreflightDoesNotBlockOtherNodeWebDeletion(t *testing.T) {
	testDB := containerRemovalIntegrationDB(t)
	for _, kind := range []string{"create", "retry"} {
		t.Run(kind, func(t *testing.T) {
			entered, release := make(chan struct{}), make(chan struct{})
			var once sync.Once
			defer once.Do(func() { close(release) })
			host, port := containerRemovalIntegrationHost(t, func(w http.ResponseWriter, r *http.Request) {
				close(entered)
				<-release
				http.Error(w, "container_inventory_unavailable", 503)
			})
			seedKernelBatchAdmission(t, testDB, host, port)
			admissionDone := make(chan error, 1)
			go func() { admissionDone <- kernelBatchAdmissionAttempt(kind) }()
			select {
			case <-entered:
			case <-time.After(2 * time.Second):
				t.Fatal("kernel admission did not query host")
			}
			deleted := make(chan error, 1)
			go func() { id := uint(8); deleted <- DeleteNodeServerById(&id) }()
			var deletionErr error
			blocked := false
			select {
			case deletionErr = <-deleted:
			case <-time.After(time.Second):
				blocked = true
				t.Error("blocked remote inventory prevented another Node's Web-only deletion")
			}
			once.Do(func() { close(release) })
			select {
			case err := <-admissionDone:
				if err == nil {
					t.Error("failed preflight persisted kernel task")
				}
			case <-time.After(2 * time.Second):
				t.Fatal("cancelled admission did not finish")
			}
			if blocked {
				select {
				case deletionErr = <-deleted:
				case <-time.After(2 * time.Second):
					t.Fatal("Web-only deletion remained blocked")
				}
			}
			if deletionErr != nil {
				t.Fatal(deletionErr)
			}
		})
	}
}

func writeBatchAdmissionInventory(w http.ResponseWriter) {
	json.NewEncoder(w).Encode(core.HostContainerInventory{NodeID: 7, CurrentVersion: "1.0.2-rc.11", Image: "ghcr.io/1linhao/trojanpanelnext-node-agent:1.0.2-rc.11", UpdateSupported: true})
}

type kernelBatchAudit struct {
	tasks, attempt                 int
	taskStatus, stage, result, key string
}

func readKernelBatchAudit(t *testing.T, db *sql.DB) kernelBatchAudit {
	t.Helper()
	var state kernelBatchAudit
	if err := db.QueryRow(`SELECT COUNT(*) FROM kernel_upgrade_task`).Scan(&state.tasks); err != nil {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT status FROM kernel_upgrade_task WHERE id=501`).Scan(&state.taskStatus); err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	if err := db.QueryRow(`SELECT stage,result,attempt,idempotency_key FROM kernel_upgrade_task_item WHERE id=601`).Scan(&state.stage, &state.result, &state.attempt, &state.key); err != nil && err != sql.ErrNoRows {
		t.Fatal(err)
	}
	return state
}

func TestKernelPreflightRejectsChangedOrDeletedNodeSnapshot(t *testing.T) {
	testDB := containerRemovalIntegrationDB(t)
	for _, kind := range []string{"create", "retry"} {
		for _, mutation := range []string{"deleted", "endpoint", "tls", "removing"} {
			t.Run(kind+"_"+mutation, func(t *testing.T) {
				entered, release := make(chan struct{}), make(chan struct{})
				var once sync.Once
				defer once.Do(func() { close(release) })
				host, port := containerRemovalIntegrationHost(t, func(w http.ResponseWriter, r *http.Request) {
					close(entered)
					<-release
					writeBatchAdmissionInventory(w)
				})
				seedKernelBatchAdmission(t, testDB, host, port)
				done := make(chan error, 1)
				go func() { done <- kernelBatchAdmissionAttempt(kind) }()
				select {
				case <-entered:
				case <-time.After(2 * time.Second):
					t.Fatal("kernel admission did not query host")
				}
				if mutation == "deleted" {
					id := uint(7)
					if err := DeleteNodeServerById(&id); err != nil {
						t.Fatal(err)
					}
				} else {
					query := map[string]string{
						"endpoint": `UPDATE node_server SET ip='127.0.0.2' WHERE id=7`,
						"tls":      `UPDATE node_server SET grpc_tls_server_name='changed.example.test' WHERE id=7`,
						"removing": `UPDATE node_server SET removing=1 WHERE id=7`,
					}[mutation]
					// Match an ordinary management edit's lifecycle read lock.
					nodeLifecycle.RLock()
					_, err := testDB.Exec(query)
					nodeLifecycle.RUnlock()
					if err != nil {
						t.Fatal(err)
					}
				}
				before := readKernelBatchAudit(t, testDB)
				once.Do(func() { close(release) })
				select {
				case err := <-done:
					if err == nil {
						t.Fatal("changed or deleted snapshot was admitted")
					}
				case <-time.After(2 * time.Second):
					t.Fatal("changed snapshot did not finish")
				}
				if after := readKernelBatchAudit(t, testDB); after != before {
					t.Fatalf("changed snapshot persisted or reset task: before=%+v after=%+v", before, after)
				}
			})
		}
	}
}

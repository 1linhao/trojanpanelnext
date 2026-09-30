package service

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
	"sync"
	"time"
	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/pki"
)

var certificateAuthority *pki.Manager
var certificateMaintenance sync.Mutex

func InitCertificateAuthority() error {
	dir := os.Getenv("TP_PKI_AUTHORITY_DIR")
	if dir == "" {
		return nil
	}
	manager, err := pki.Open(dir)
	if err != nil {
		return err
	}
	certificateAuthority = manager
	core.ManagedClientCertificate = manager.GetClientCertificate
	return nil
}

func MaintainCertificates() {
	if certificateAuthority == nil || !certificateMaintenance.TryLock() {
		return
	}
	defer certificateMaintenance.Unlock()
	now := time.Now()
	if err := certificateAuthority.Prepare(now); err != nil {
		logrus.Errorf("mTLS identity maintenance failed: %v", err)
		return
	}
	// Include servers without proxy instances: they also need the new CA.
	nodes, err := dao.SelectNodeServersForCertificates()
	if err != nil {
		logrus.Errorf("mTLS node inventory failed: %v", err)
		return
	}
	err = certificateAuthority.Reconcile(now, func(bundle []byte) error {
		var failures []string
		for _, node := range nodes {
			if node.GrpcTLSMode == nil || *node.GrpcTLSMode != "mtls" {
				continue
			}
			if node.Ip == nil || node.GrpcPort == nil || node.GrpcTLSServerName == nil {
				failures = append(failures, "invalid node transport")
				continue
			}
			if err := core.UpdateClientTrust(*node.Ip, *node.GrpcPort, nodeTransport(&node), bundle); err != nil {
				failures = append(failures, fmt.Sprintf("node %d: %v", *node.Id, err))
			}
		}
		if len(failures) > 0 {
			return fmt.Errorf("rotation waiting for nodes: %s", strings.Join(failures, "; "))
		}
		return nil
	})
	if err != nil {
		logrus.Errorf("mTLS trust maintenance failed: %v", err)
	}
}

package nodeidentity

import (
	"bytes"
	"strings"
	"testing"
)

func TestVerifyRequiresThisInstallationChallengeBeforeOpeningDataServices(t *testing.T) {
	var stdout, stderr bytes.Buffer
	status := Run([]string{"trojan-panel", "node-identity", "verify", "--id", "11111111-2222-4333-8444-555555555555"}, &stdout, &stderr)
	if status != 2 || !strings.Contains(stderr.String(), "--challenge") {
		t.Fatalf("verify without challenge status=%d stderr=%q, want usage rejection", status, stderr.String())
	}
}

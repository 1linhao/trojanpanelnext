package api

import (
	"encoding/json"
	"strings"
	"testing"
	"trojan-panel/model/constant"
)

func TestNodeContainerUpdateRejectsBrowserOverridesAndInvalidIdentity(t *testing.T) {
	for _, body := range []string{`{}`, `{"nodeServerId":0}`, `{"nodeServerId":-1}`, `{"nodeServerId":null}`, `{"nodeServerId":"7"}`, `{"nodeServerId":7,"version":"1.0.2-rc.12"}`, `{"nodeServerId":7,"image":"attacker"}`, `{"nodeServerId":7,"path":"/tmp"}`, `{"nodeServerId":7,"shell":"whoami"}`, `{"nodeServerId":7}{}`, strings.Repeat(" ", 1025) + `{"nodeServerId":7}`} {
		ctx, writer := remarkContext("POST", "/api/container/update", body, "")
		UpdateNodeContainer(ctx)
		var response struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil || response.Type != "error" || response.Message != constant.ValidateFailed {
			t.Fatalf("invalid update accepted: %s", writer.Body.String())
		}
	}
}

func TestNodeContainerInventoryRejectsAmbiguousOrExtraQuery(t *testing.T) {
	for _, query := range []string{"", "nodeServerId=0", "nodeServerId=-1", "nodeServerId=7&nodeServerId=8", "nodeServerId=7&ip=attacker"} {
		ctx, writer := remarkContext("GET", "/api/container/inventory?"+query, "", "")
		NodeContainerInventory(ctx)
		if got := remarkResponse(t, writer)["message"]; got != constant.ValidateFailed {
			t.Fatalf("invalid inventory query accepted: %s", writer.Body.String())
		}
	}
}

package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
)

func TestWebOnlyNodeDeleteRejectsRemoteOptionsBeforePersistence(t *testing.T) {
	InitValidator()
	for _, body := range []string{
		`{"id":11,"purge":false}`,
		`{"id":11,"purge":true}`,
		`{"id":11,"action":"uninstall"}`,
		`{"id":11} {"id":12}`,
		`{"id":0}`,
		`{}`,
		`{"id":null}`,
	} {
		t.Run(body, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			ctx, _ := gin.CreateTestContext(recorder)
			ctx.Request = httptest.NewRequest("POST", "/api/nodeServer/deleteNodeServerById", strings.NewReader(body))
			// No database or Node client is initialized. Validation must reject
			// old uninstall payloads without attempting either kind of deletion.
			DeleteNodeServerById(ctx)
			var result struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
			}
			if err := json.Unmarshal(recorder.Body.Bytes(), &result); err != nil {
				t.Fatal(err)
			}
			if result.Code == constant.CodeSuccess || result.Message != constant.ValidateFailed {
				t.Fatalf("unexpected response: %s", recorder.Body.String())
			}
		})
	}
}

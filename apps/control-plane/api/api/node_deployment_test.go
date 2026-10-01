package api

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"trojan-panel/model/constant"
)

func TestDeploymentDownloadRejectsUnknownJSONAndDisablesCaching(t *testing.T) {
	for _, body := range []string{`{"id":1,"webHost":"panel.example.test","password":"mock"}`, `{"id":0} {"id":1}`, `{"id":0}`, `{`, strings.Repeat(" ", 16*1024) + `{"id":1}`} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest("POST", "/api/nodeServer/downloadDeployment", strings.NewReader(body))
		// No persistence or Node client is initialized. These requests must
		// fail before exporting credentials or writing an attachment response.
		DownloadNodeDeployment(ctx)
		var response struct {
			Code int `json:"code"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Code == constant.CodeSuccess {
			t.Fatal("invalid deployment download accepted")
		}
		if recorder.Header().Get("Cache-Control") != "private, no-store" || recorder.Header().Get("Pragma") != "no-cache" {
			t.Fatal("credential endpoint permits caching")
		}
		if recorder.Header().Get("Content-Disposition") != "" || !strings.HasPrefix(recorder.Header().Get("Content-Type"), "application/json") {
			t.Fatal("failure was returned as an install archive")
		}
	}
}

func TestDeploymentMetadataRejectsInvalidIdentityWithoutNodeContact(t *testing.T) {
	for _, id := range []string{"", "0", "-1", "invalid", "18446744073709551616"} {
		recorder := httptest.NewRecorder()
		ctx, _ := gin.CreateTestContext(recorder)
		ctx.Request = httptest.NewRequest("GET", "/api/nodeServer/deployment?id="+id, nil)
		NodeServerDeployment(ctx)
		var response struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Code == constant.CodeSuccess || response.Message != constant.ValidateFailed {
			t.Fatal("invalid identity reached deployment metadata")
		}
		if recorder.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatal("metadata cache policy missing")
		}
	}
}

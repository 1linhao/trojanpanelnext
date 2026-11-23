package router

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"trojan-panel/model"
	"trojan-panel/model/constant"

	"github.com/gin-gonic/gin"
)

func TestBackupRoutesAndImportTemplates(t *testing.T) {
	engine := gin.New()
	initApiRouter(engine)
	for _, route := range engine.Routes() {
		if strings.HasPrefix(route.Path, "/api/fileTask/") || strings.HasPrefix(route.Path, "/api/blackList/") {
			t.Fatalf("obsolete API is still registered: %s", route.Path)
		}
	}

	tests := []struct {
		path, filename string
		valid          func([]byte) bool
	}{
		{"/api/account/downloadImportTemplate", "AccountTemplate.json", func(data []byte) bool {
			var accounts []model.Account
			return json.Unmarshal(data, &accounts) == nil && len(accounts) == 1 &&
				accounts[0].RoleId != nil && *accounts[0].RoleId == constant.USER &&
				accounts[0].Username != nil && *accounts[0].Username != ""
		}},
		{"/api/nodeServer/downloadImportTemplate", "NodeServerTemplate.json", func(data []byte) bool {
			var servers []model.NodeServer
			return json.Unmarshal(data, &servers) == nil && len(servers) == 1 &&
				servers[0].GrpcTLSServerName != nil && *servers[0].GrpcTLSServerName != "" &&
				servers[0].GrpcPort != nil && *servers[0].GrpcPort == 8100
		}},
	}
	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			engine.ServeHTTP(recorder, httptest.NewRequest(http.MethodPost, tc.path, nil))
			if recorder.Code != http.StatusOK || recorder.Header().Get("Content-Disposition") != "attachment; filename="+tc.filename ||
				recorder.Header().Get("Cache-Control") != "no-store" || !tc.valid(recorder.Body.Bytes()) {
				t.Fatalf("invalid template attachment: status=%d headers=%v data=%s", recorder.Code, recorder.Header(), recorder.Body)
			}
		})
	}
}

func TestSynchronousImportRejectsInvalidFiles(t *testing.T) {
	engine := gin.New()
	initApiRouter(engine)
	for _, path := range []string{"/api/account/importAccount", "/api/nodeServer/importNodeServer"} {
		for _, tc := range []struct{ cover, data, message string }{
			{"bad", "[]", constant.ValidateFailed},
			{"0", "[]", constant.RowNotEnough},
			{"1", "[{}]{}", constant.FileFormatError},
		} {
			t.Run(path+"/"+tc.message, func(t *testing.T) {
				var body bytes.Buffer
				form := multipart.NewWriter(&body)
				if err := form.WriteField("cover", tc.cover); err != nil {
					t.Fatal(err)
				}
				part, err := form.CreateFormFile("file", "test.json")
				if err != nil {
					t.Fatal(err)
				}
				if _, err = part.Write([]byte(tc.data)); err != nil {
					t.Fatal(err)
				}
				if err = form.Close(); err != nil {
					t.Fatal(err)
				}
				request := httptest.NewRequest(http.MethodPost, path, &body)
				request.Header.Set("Content-Type", form.FormDataContentType())
				recorder := httptest.NewRecorder()
				engine.ServeHTTP(recorder, request)
				var response struct{ Message string }
				if err = json.Unmarshal(recorder.Body.Bytes(), &response); err != nil || response.Message != tc.message {
					t.Fatalf("expected %q, got %s (decode error: %v)", tc.message, recorder.Body, err)
				}
			})
		}
	}
}

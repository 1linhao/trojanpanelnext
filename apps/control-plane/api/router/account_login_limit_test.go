package router

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
	"trojan-panel/api"
	"trojan-panel/model/constant"
)

func TestAccountLoginLimitResetRouteValidatesIdentity(t *testing.T) {
	api.InitValidator()
	engine := gin.New()
	initAccountRouter(engine.Group("/api"))
	writer := httptest.NewRecorder()
	request := httptest.NewRequest("POST", "/api/account/resetAccountLoginLimit", strings.NewReader(`{}`))
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(writer, request)
	if writer.Code != 200 {
		t.Fatalf("login-limit reset route is unavailable: HTTP %d", writer.Code)
	}
	var response struct {
		Message string `json:"message"`
	}
	if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Message != constant.ValidateFailed {
		t.Fatalf("missing identity reached reset: %s", writer.Body.String())
	}
}

package router

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"trojan-panel-core/bootstrap"
	"trojan-panel-core/core"
	"trojan-panel-core/middleware"
)

func TestNodeAPIHealthWaitsForWebMTLSVerification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	t.Setenv("TP_NODE_BOOTSTRAP_MARKER", filepath.Join(t.TempDir(), "bootstrap.json"))
	oldNode := core.Config.NodeConfig
	t.Cleanup(func() { core.Config.NodeConfig = oldNode })
	core.Config.NodeConfig = core.NodeConfig{
		ServerID: 42, IdentityID: "11111111-2222-4333-8444-555555555555", IdentityGeneration: 7,
		BootstrapChallenge: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	middleware.InitRateLimiter()
	router := gin.New()
	Router(router)

	request := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusServiceUnavailable {
		t.Fatalf("health before mTLS verification = %d, want 503", response.Code)
	}
	if err := bootstrap.MarkVerified(); err != nil {
		t.Fatal(err)
	}
	response = httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("health after mTLS verification = %d, want 200", response.Code)
	}
}

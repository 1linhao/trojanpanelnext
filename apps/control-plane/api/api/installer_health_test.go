package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestInstallerHealthRejectsNonLoopbackCaller(t *testing.T) {
	gin.SetMode(gin.TestMode)
	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodPost, "/api/auth/installer-health", nil)
	context.Request.RemoteAddr = "192.0.2.10:43210"

	InstallerHealth(context)

	if recorder.Code != http.StatusNotFound {
		t.Fatalf("remote installer health status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}

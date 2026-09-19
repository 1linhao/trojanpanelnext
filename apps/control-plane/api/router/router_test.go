package router

import (
	"testing"

	"github.com/gin-gonic/gin"
)

func TestRouterDoesNotExposeInstallerCredentialVerification(t *testing.T) {
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	Router(engine)

	for _, route := range engine.Routes() {
		if route.Path == "/api/auth/installer-health" {
			t.Fatal("router exposes the installer administrator credential oracle")
		}
	}
}

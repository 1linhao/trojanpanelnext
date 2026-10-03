package router

import (
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestNodeContainerRoutesUseDedicatedMethods(t *testing.T) {
	engine := gin.New()
	initNodeContainerRouter(engine.Group("/api"))
	for _, test := range []struct {
		method, path string
		code         int
	}{{"GET", "/api/container/inventory", 200}, {"POST", "/api/container/update", 200}, {"GET", "/api/container/update", 404}, {"POST", "/api/container/inventory", 404}} {
		writer := httptest.NewRecorder()
		engine.ServeHTTP(writer, httptest.NewRequest(test.method, test.path, strings.NewReader(`{}`)))
		if writer.Code != test.code {
			t.Fatalf("%s %s status %d", test.method, test.path, writer.Code)
		}
	}
}

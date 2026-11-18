package router

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"

	"trojan-panel/middleware"
	"trojan-panel/model/constant"
)

func TestServerTrafficDisplayRoutesRequireJWT(t *testing.T) {
	engine := gin.New()
	initNodeRouter(engine.Group("/api", middleware.JWTHandler()))
	for _, request := range []struct{ method, path string }{
		{"GET", "/api/node/serverTrafficSummaries"},
		{"GET", "/api/node/serverTrafficDisplay"},
		{"POST", "/api/node/serverTrafficDisplay"},
	} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, nil))
		var response struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Message != constant.UnauthorizedError || response.Code == constant.CodeSuccess {
			t.Fatalf("%s %s allowed a request without a JWT", request.method, request.path)
		}
	}
}

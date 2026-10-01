package router

import (
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"trojan-panel/middleware"
	"trojan-panel/model/constant"
)

func TestNodeDeploymentRoutesRequireJWT(t *testing.T) {
	engine := gin.New()
	initNodeServerRouter(engine.Group("/api", middleware.JWTHandler()))
	for _, request := range []struct{ method, path string }{{"GET", "/api/nodeServer/deployment?id=42"}, {"POST", "/api/nodeServer/downloadDeployment"}} {
		recorder := httptest.NewRecorder()
		engine.ServeHTTP(recorder, httptest.NewRequest(request.method, request.path, strings.NewReader(`{"id":42}`)))
		var response struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatal(err)
		}
		if response.Message != constant.UnauthorizedError || response.Code == constant.CodeSuccess {
			t.Fatal("deployment route allowed a request without a JWT")
		}
		if recorder.Header().Get("Content-Disposition") != "" {
			t.Fatal("unauthenticated request exported a deployment package")
		}
	}
}

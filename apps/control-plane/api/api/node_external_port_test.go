package api

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"net/http/httptest"
	"strings"
	"testing"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
)

const forwardedNodeJSON = `{"id":1,"nodeSubId":1,"nodeServerId":7,"nodeTypeId":1,"name":"forwarded","domain":"node.example.test","port":445,"priority":0,"trojanGoMuxEnable":0,"trojanGoWebsocketEnable":0,"trojanGoSsEnable":0,"hysteriaUpMbps":100,"hysteriaDownMbps":100,"hysteria2UpMbps":100,"hysteria2DownMbps":100`

func TestNodeExternalPortDTO(t *testing.T) {
	InitValidator()
	for _, tc := range []struct {
		name, suffix string
		want         *uint
	}{
		{"omitted", "}", nil},
		{"disabled", `,"externalPort":0}`, externalPortTestPointer(0)},
		{"minimum", `,"externalPort":1}`, externalPortTestPointer(1)},
		{"public_tls", `,"externalPort":443}`, externalPortTestPointer(443)},
		{"maximum", `,"externalPort":65535}`, externalPortTestPointer(65535)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var create dto.NodeCreateDto
			var update dto.NodeUpdateDto
			body := []byte(forwardedNodeJSON + tc.suffix)
			if err := json.Unmarshal(body, &create); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(body, &update); err != nil {
				t.Fatal(err)
			}
			for _, value := range []interface{}{create, update} {
				if err := validate.Struct(value); err != nil {
					t.Fatal(err)
				}
			}
			for _, actual := range []*uint{create.ExternalPort, update.ExternalPort} {
				if tc.want == nil {
					if actual != nil {
						t.Fatal("omitted external port must remain nil for update preservation")
					}
				} else if actual == nil || *actual != *tc.want {
					t.Fatalf("externalPort=%v, want %d", actual, *tc.want)
				}
			}
		})
	}
}

func TestNodeExternalPortInvalidHTTPInput(t *testing.T) {
	InitValidator()
	for _, handler := range []struct {
		name string
		run  gin.HandlerFunc
	}{{"create", CreateNode}, {"update", UpdateNodeById}} {
		for _, invalid := range []string{"65536", "-1", "1.5", `"443"`} {
			t.Run(handler.name+"/"+invalid, func(t *testing.T) {
				writer := httptest.NewRecorder()
				ctx, _ := gin.CreateTestContext(writer)
				ctx.Request = httptest.NewRequest("POST", "/node", strings.NewReader(forwardedNodeJSON+`,"externalPort":`+invalid+"}"))
				ctx.Request.Header.Set("Content-Type", "application/json")
				handler.run(ctx)
				var response struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				}
				if err := json.Unmarshal(writer.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response.Type != "error" || response.Message != constant.ValidateFailed {
					t.Fatalf("invalid external port reached service: %s", writer.Body.String())
				}
			})
		}
	}
}

func externalPortTestPointer(value uint) *uint { return &value }

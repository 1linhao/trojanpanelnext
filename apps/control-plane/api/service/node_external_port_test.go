package service

import (
	"testing"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
)

func TestForwardedNaiveOutboundUsesPublicPort(t *testing.T) {
	node := model.Node{NodeTypeId: uintPtr(constant.NaiveProxy), Name: stringPtr("forwarded"), Domain: stringPtr("node.example.test"), Port: uintPtr(445), ExternalPort: uintPtr(443)}
	outbound, err := buildSingBoxOutbound(node, "fixture-pass", "fixture-user")
	if err != nil {
		t.Fatal(err)
	}
	if got := outbound["server_port"]; got != uint(443) {
		t.Fatalf("client connects to internal listener: got=%v want=443", got)
	}
	if *node.Port != 445 {
		t.Fatal("export changed the actual listener port")
	}
}

func TestNodePortRangesBeforePersistence(t *testing.T) {
	for _, tc := range []struct {
		name             string
		actual, external uint
		want             string
	}{
		{"actual_below_range", 100, 443, constant.PortRangeError},
		{"actual_above_range", 30000, 443, constant.PortRangeError},
		{"public_above_range", 445, 65536, "externalPort must be between 0 and 65535"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			create := dto.NodeCreateDto{NodeTypeId: uintPtr(constant.Xray), Port: uintPtr(tc.actual), ExternalPort: uintPtr(tc.external)}
			update := dto.NodeUpdateDto{NodeTypeId: uintPtr(constant.Xray), Port: uintPtr(tc.actual), ExternalPort: uintPtr(tc.external)}
			for _, err := range []error{CreateNode("", create), UpdateNodeById("", &update)} {
				if err == nil || err.Error() != tc.want {
					t.Fatalf("invalid port reached persistence: %v", err)
				}
			}
		})
	}
}

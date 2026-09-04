package service

import (
	"testing"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
)

func TestRetiredNodeTypesAreRejectedBeforePersistence(t *testing.T) {
	for _, nodeType := range []uint{constant.TrojanGo, constant.Hysteria} {
		nodeType := nodeType
		if !isRetiredNodeType(nodeType) {
			t.Fatalf("type %d was not recognized as retired", nodeType)
		}
		if err := CreateNode("", dto.NodeCreateDto{NodeTypeId: &nodeType}); err == nil {
			t.Fatalf("create accepted retired node type %d", nodeType)
		}
		if err := UpdateNodeById("", &dto.NodeUpdateDto{NodeTypeId: &nodeType}); err == nil {
			t.Fatalf("update accepted retired node type %d", nodeType)
		}
	}
	for _, nodeType := range []uint{constant.Xray, constant.Hysteria2, constant.NaiveProxy} {
		if isRetiredNodeType(nodeType) {
			t.Fatalf("active type %d was recognized as retired", nodeType)
		}
	}
}

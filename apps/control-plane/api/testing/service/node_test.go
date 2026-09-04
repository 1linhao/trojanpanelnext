package service

import (
	"fmt"
	"testing"
	"trojan-panel/core"
)

func TestGrpcAddNode(t *testing.T) {
	dto := core.NodeAddDto{
		NodeTypeId: 4,
		Port:       883,
		Domain:     "demo.wellveryfunny.xyz",
	}
	if err := core.AddNode("REDACTED_TEST_TOKEN",
		"127.0.0.1", 8100, &dto); err != nil {
		fmt.Println(err.Error())
	}
}

func TestGrpcRemoveNode(t *testing.T) {
	removeDto := core.NodeRemoveDto{NodeTypeId: 4, Port: 883}
	if err := core.RemoveNode("REDACTED_TEST_TOKEN",
		"127.0.0.1", 8100, &removeDto); err != nil {
		fmt.Println(err.Error())
	}
}

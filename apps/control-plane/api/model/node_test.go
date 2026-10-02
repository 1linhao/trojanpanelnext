package model

import "testing"

func TestNodeClientPort(t *testing.T) {
	actual, disabled, public, maximum := uint(445), uint(0), uint(443), uint(65535)
	for _, tc := range []struct {
		name     string
		external *uint
		want     uint
	}{
		{"existing_node", nil, 445},
		{"forwarding_disabled", &disabled, 445},
		{"forwarding_enabled", &public, 443},
		{"highest_public_port", &maximum, 65535},
	} {
		t.Run(tc.name, func(t *testing.T) {
			node := Node{Port: &actual, ExternalPort: tc.external}
			if got := node.ClientPort(); got != tc.want {
				t.Fatalf("ClientPort()=%d, want %d", got, tc.want)
			}
			if *node.Port != 445 {
				t.Fatal("selecting client port modified actual listener")
			}
		})
	}
}

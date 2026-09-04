package service

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	"trojan-panel/model"
	"trojan-panel/model/bo"
)

func uintPtr(value uint) *uint       { return &value }
func stringPtr(value string) *string { return &value }

func TestApplySingBoxXrayClientOptions(t *testing.T) {
	tests := []struct {
		name       string
		protocol   string
		node       model.NodeXray
		wantKey    string
		wantAbsent string
	}{
		{
			name: "shadowsocks uot v1", protocol: "shadowsocks",
			node:    model.NodeXray{UotEnable: uintPtr(1), UotVersion: uintPtr(1)},
			wantKey: "udp_over_tcp", wantAbsent: "packet_encoding",
		},
		{
			name: "vless xudp", protocol: "vless",
			node:    model.NodeXray{XudpEnable: uintPtr(1)},
			wantKey: "packet_encoding", wantAbsent: "udp_over_tcp",
		},
		{
			name: "vmess multiplex", protocol: "vmess",
			node:    model.NodeXray{MuxEnable: uintPtr(1)},
			wantKey: "multiplex", wantAbsent: "udp_over_tcp",
		},
		{
			name: "trojan ignores xudp", protocol: "trojan",
			node:    model.NodeXray{XudpEnable: uintPtr(1), MuxEnable: uintPtr(1)},
			wantKey: "multiplex", wantAbsent: "packet_encoding",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.node.Protocol = stringPtr(test.protocol)
			outbound := map[string]interface{}{}
			applySingBoxXrayClientOptions(outbound, &test.node)
			if _, ok := outbound[test.wantKey]; !ok {
				t.Fatalf("expected %q in outbound: %#v", test.wantKey, outbound)
			}
			if _, ok := outbound[test.wantAbsent]; ok {
				t.Fatalf("did not expect %q in outbound: %#v", test.wantAbsent, outbound)
			}
		})
	}
}

func TestXrayUotVersionDefaultsToTwo(t *testing.T) {
	if got := xrayUotVersion(nil); got != 2 {
		t.Fatalf("got %d, want 2", got)
	}
	if got := xrayUotVersion(uintPtr(9)); got != 2 {
		t.Fatalf("got %d, want 2 for invalid version", got)
	}
}

func TestClashXrayClientOptionsYAML(t *testing.T) {
	encoded, err := yaml.Marshal(bo.Vless{
		Name:           "test",
		PacketEncoding: "xudp",
		Smux:           &bo.Smux{Enabled: true, Protocol: "h2mux"},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"packet-encoding: xudp", "smux:", "protocol: h2mux"}
	for _, value := range want {
		if !strings.Contains(string(encoded), value) {
			t.Fatalf("expected %q in yaml:\n%s", value, encoded)
		}
	}
}

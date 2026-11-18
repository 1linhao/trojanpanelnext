package clientrouting

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func sampleProfile() Profile {
	return Profile{
		Name: "office", Base: Base{MixedPort: 7890, TunMTU: 1500, LogLevel: "info"},
		Traffic: Traffic{
			Groups: []Group{
				{ID: "g1", Name: "工作", ServerID: 4, NodeIDs: []uint{10, 11}, AutoTest: true},
				{ID: "g2", Name: "其他", ServerID: 5, NodeIDs: []uint{12}},
			},
			Rules:   []Rule{{"domain", "API.example.org.", "g1"}, {"suffix", "example.org", "g2"}, {"ip", "2001:db8::1", Direct}, {"ip", "192.0.2.123/24", All}},
			Default: "g2", Empty: Reject,
		},
		DNS: DNS{Default: "1.1.1.1", Rules: []DNSRule{{"example.org", "192.0.2.53"}, {"sub.example.org", "2001:db8::53"}}},
	}
}

func TestCompileValidation(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Profile)
	}{
		{"mixed servers", func(p *Profile) { p.Traffic.Groups[0].NodeIDs = []uint{10, 12} }},
		{"missing node", func(p *Profile) { p.Traffic.Groups[1].NodeIDs = []uint{99} }},
		{"duplicate node", func(p *Profile) { p.Traffic.Groups[0].NodeIDs = []uint{10, 10} }},
		{"duplicate group", func(p *Profile) { p.Traffic.Groups[0].ID = "g2" }},
		{"invalid name", func(p *Profile) { p.Traffic.Groups[0].Name = "x,DIRECT" }},
		{"removed group", func(p *Profile) { p.Traffic.Rules[0].Target = "gone" }},
		{"unknown final", func(p *Profile) { p.Traffic.Default = "" }},
		{"unsafe empty", func(p *Profile) { p.Traffic.Empty = "g1" }},
		{"invalid domain", func(p *Profile) { p.Traffic.Rules[0].Value = "bad/example.org" }},
		{"invalid network", func(p *Profile) { p.Traffic.Rules[2].Value = "192.0.2.0/33" }},
		{"invalid dns", func(p *Profile) { p.DNS.Default = "example.org" }},
		{"duplicate dns", func(p *Profile) { p.DNS.Rules[1].Suffix = "EXAMPLE.ORG." }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := sampleProfile()
			tc.edit(&p)
			if _, err := Compile(p, map[uint]uint{10: 4, 11: 4, 12: 5}); err == nil {
				t.Fatal("accepted invalid profile")
			}
		})
	}
}

func TestSnapshotAccountRendering(t *testing.T) {
	p := sampleProfile()
	raw, err := Compile(p, map[uint]uint{10: 4, 11: 4, 12: 5})
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(raw)
	if err != nil {
		t.Fatal(err)
	}
	var s Snapshot
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}

	// 11 has been revoked or filtered by the client, 12 moved servers.
	members := []Member{{10, 4, "node10"}, {12, 9, "node12"}}
	clashNodes := []interface{}{Object{"name": "node10", "type": "socks5", "server": "192.0.2.1", "port": 1080}, Object{"name": "node12", "type": "socks5", "server": "192.0.2.2", "port": 1080}}
	singNodes := []Object{{"tag": "node10", "type": "socks", "server": "192.0.2.1", "server_port": 1080}, {"tag": "node12", "type": "socks", "server": "192.0.2.2", "server_port": 1080}}
	clash := s.RenderClash(clashNodes, members)
	want := []string{"DOMAIN,api.example.org,组 · 工作", "DOMAIN-SUFFIX,example.org,REJECT", "IP-CIDR6,2001:db8::1/128,DIRECT", "IP-CIDR,192.0.2.0/24,PROXY", "MATCH,REJECT"}
	if !reflect.DeepEqual(clash["rules"], want) {
		t.Fatalf("rules: %v", clash["rules"])
	}
	groups := clash["proxy-groups"].([]Object)
	if len(groups) != 2 || groups[1]["type"] != "url-test" || !reflect.DeepEqual(groups[1]["proxies"], []string{"node10"}) {
		t.Fatal(groups)
	}
	sing := s.RenderSingBox(singNodes, members)
	rules := sing["route"].(Object)["rules"].([]Object)
	if rules[2]["domain"] != "api.example.org" || rules[2]["outbound"] != "组 · 工作" || rules[3]["action"] != "reject" || rules[4]["ip_cidr"] != "2001:db8::1/128" || rules[5]["ip_cidr"] != "192.0.2.0/24" || rules[6]["action"] != "reject" {
		t.Fatal(rules)
	}
	outbounds := sing["outbounds"].([]Object)
	if len(outbounds) != 5 || outbounds[4]["type"] != "urltest" || !reflect.DeepEqual(outbounds[4]["outbounds"], []string{"node10"}) {
		t.Fatal(outbounds)
	}
	checkClientConfig(t, "sing-box", sing)
	checkClientConfig(t, "mihomo", clash)

	s.Empty = Direct
	empty := s.RenderSingBox(nil, nil)
	if empty["route"].(Object)["final"] != Direct || len(empty["outbounds"].([]Object)) != 1 {
		t.Fatal(empty)
	}
	emptyClash := s.RenderClash(nil, nil)
	if !reflect.DeepEqual(emptyClash["proxy-groups"], []Object{}) || emptyClash["rules"].([]string)[0] != "DOMAIN,api.example.org,DIRECT" {
		t.Fatal(emptyClash)
	}
	checkClientConfig(t, "sing-box", empty)
	checkClientConfig(t, "mihomo", emptyClash)
}

func checkClientConfig(t *testing.T, client string, config Object) {
	t.Helper()
	bin, err := exec.LookPath(client)
	if err != nil {
		return
	} // Native validators are optional developer tools.
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	data, err := json.Marshal(config)
	args := []string{"check", "-c", path}
	if client == "mihomo" {
		path = filepath.Join(dir, "config.yaml")
		data, err = yaml.Marshal(config)
		args = []string{"-t", "-d", dir, "-f", path}
	}
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if output, err := exec.CommandContext(ctx, bin, args...).CombinedOutput(); err != nil {
		t.Fatalf("%s: %v\n%s", client, err, output)
	}
}

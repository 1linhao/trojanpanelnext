// Package clientrouting composes reusable, credential-free client profiles.
// Subscription rendering supplies only nodes actually emitted for the account.
package clientrouting

import (
	"fmt"
	"net/netip"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	All      = "PROXY"
	Direct   = "DIRECT"
	Reject   = "REJECT"
	probeURL = "https://www.gstatic.com/generate_204"
)

type Base struct {
	MixedPort int    `json:"mixedPort"`
	TunMTU    int    `json:"tunMtu"`
	LogLevel  string `json:"logLevel"`
}

type Group struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	ServerID uint   `json:"serverId"`
	NodeIDs  []uint `json:"nodeIds"`
	AutoTest bool   `json:"autoTest"`
}

type Rule struct {
	Kind   string `json:"kind"`
	Value  string `json:"value"`
	Target string `json:"target"`
}

type Traffic struct {
	Groups  []Group `json:"groups"`
	Rules   []Rule  `json:"rules"`
	Default string  `json:"default"`
	Empty   string  `json:"empty"`
}

type DNSRule struct {
	Suffix string `json:"suffix"`
	Server string `json:"server"`
}

type DNS struct {
	Default string    `json:"default"`
	Rules   []DNSRule `json:"rules"`
}

type Profile struct {
	Name    string  `json:"name"`
	Base    Base    `json:"base"`
	Traffic Traffic `json:"traffic"`
	DNS     DNS     `json:"dns"`
}

type Object = map[string]interface{}

type Snapshot struct {
	Groups     []Group  `json:"groups"`
	Empty      string   `json:"empty"`
	Clash      Object   `json:"clash"`
	SingBox    Object   `json:"singBox"`
	ClashRules []string `json:"clashRules"`
	SingRules  []Object `json:"singRules"`
}

var groupID = regexp.MustCompile(`^g[a-zA-Z0-9_-]{1,64}$`)
var dnsLabel = regexp.MustCompile(`^[a-z0-9](?:[a-z0-9-]{0,61}[a-z0-9])?$`)

func domain(value string) (string, bool) {
	value = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(value), "."))
	if len(value) > 253 {
		return "", false
	}
	if _, err := netip.ParseAddr(value); err == nil {
		return "", false
	}
	for _, label := range strings.Split(value, ".") {
		if !dnsLabel.MatchString(label) {
			return "", false
		}
	}
	return value, true
}

func addr(value string) (netip.Addr, bool) {
	a, err := netip.ParseAddr(strings.TrimSpace(value))
	return a, err == nil && a.Zone() == ""
}

func title(value string) bool {
	return value == strings.TrimSpace(value) && value != "" && utf8.RuneCountInString(value) <= 64 && strings.IndexFunc(value, unicode.IsControl) < 0
}

func groupTag(g Group) string { return "组 · " + g.Name }

// Compile validates the editable source against the live node catalog and
// materializes the native bases, DNS and ordered destination rules on publish.
func Compile(p Profile, nodeServers map[uint]uint) (*Snapshot, error) {
	if !title(p.Name) {
		return nil, fmt.Errorf("版本名称须为 1–64 个字符")
	}
	if p.Base.MixedPort < 1 || p.Base.MixedPort > 65535 || p.Base.TunMTU < 1280 || p.Base.TunMTU > 9000 {
		return nil, fmt.Errorf("监听端口须为 1–65535，TUN MTU 须为 1280–9000")
	}
	switch p.Base.LogLevel {
	case "debug", "info", "error":
	default:
		return nil, fmt.Errorf("日志级别无效")
	}
	if p.Traffic.Empty != Direct && p.Traffic.Empty != Reject {
		return nil, fmt.Errorf("空组策略须为直连或拒绝")
	}
	if len(p.Traffic.Groups) > 256 || len(p.Traffic.Rules) > 4096 || len(p.DNS.Rules) > 256 {
		return nil, fmt.Errorf("节点组或规则过多")
	}
	targets := map[string]string{All: All, Direct: Direct, Reject: Reject}
	names := map[string]bool{}
	for _, g := range p.Traffic.Groups {
		if !groupID.MatchString(g.ID) || targets[g.ID] != "" || !title(g.Name) || strings.Contains(g.Name, ",") || names[g.Name] {
			return nil, fmt.Errorf("组标识、名称重复或无效（组名不能含逗号）")
		}
		if g.ServerID == 0 || len(g.NodeIDs) == 0 {
			return nil, fmt.Errorf("组 %s 需要选择服务器和节点", g.Name)
		}
		seen := map[uint]bool{}
		for _, id := range g.NodeIDs {
			if id == 0 || nodeServers[id] != g.ServerID || seen[id] {
				return nil, fmt.Errorf("组 %s 的节点不存在、重复或属于其他服务器", g.Name)
			}
			seen[id] = true
		}
		names[g.Name] = true
		targets[g.ID] = groupTag(g)
	}
	if targets[p.Traffic.Default] == "" {
		return nil, fmt.Errorf("默认策略引用的组不存在")
	}

	clashDNS, singDNS, err := compileDNS(p.DNS)
	if err != nil {
		return nil, err
	}
	s := &Snapshot{Groups: p.Traffic.Groups, Empty: p.Traffic.Empty,
		Clash: Object{"mixed-port": p.Base.MixedPort, "mode": "rule", "log-level": p.Base.LogLevel, "dns": clashDNS},
		SingBox: Object{"log": Object{"level": p.Base.LogLevel}, "dns": singDNS,
			"inbounds": []Object{{"type": "tun", "tag": "tun-in", "address": []string{"172.19.0.1/30", "fdfe:dcba:9876::1/126"}, "mtu": p.Base.TunMTU, "auto_route": true, "strict_route": true, "stack": "mixed"}}},
	}
	for i, r := range p.Traffic.Rules {
		target := targets[r.Target]
		if target == "" {
			return nil, fmt.Errorf("第 %d 条规则引用的组不存在", i+1)
		}
		var clashKey, singKey, value string
		switch r.Kind {
		case "domain", "suffix":
			var ok bool
			value, ok = domain(r.Value)
			if !ok {
				return nil, fmt.Errorf("第 %d 条规则域名无效（国际化域名请用 Punycode）", i+1)
			}
			clashKey, singKey = "DOMAIN", "domain"
			if r.Kind == "suffix" {
				clashKey, singKey = "DOMAIN-SUFFIX", "domain_suffix"
			}
		case "ip":
			prefix, err := netip.ParsePrefix(strings.TrimSpace(r.Value))
			if err != nil {
				a, ok := addr(r.Value)
				if !ok {
					return nil, fmt.Errorf("第 %d 条规则 IP 地址或网段无效", i+1)
				}
				prefix = netip.PrefixFrom(a, a.BitLen())
			}
			clashKey, singKey, value = "IP-CIDR", "ip_cidr", prefix.Masked().String()
			if prefix.Addr().Is6() {
				clashKey = "IP-CIDR6"
			}
		default:
			return nil, fmt.Errorf("第 %d 条规则类型无效", i+1)
		}
		s.ClashRules = append(s.ClashRules, strings.Join([]string{clashKey, value, target}, ","))
		s.SingRules = append(s.SingRules, Object{singKey: value, "outbound": target})
	}
	s.ClashRules = append(s.ClashRules, "MATCH,"+targets[p.Traffic.Default])
	s.SingRules = append(s.SingRules, Object{"outbound": targets[p.Traffic.Default]})
	return s, nil
}

func compileDNS(d DNS) (Object, Object, error) {
	defaultAddr, ok := addr(d.Default)
	if !ok {
		return nil, nil, fmt.Errorf("默认 DNS 须为 IP 地址")
	}
	servers := []Object{{"type": "udp", "tag": "default", "server": defaultAddr.String()}}
	policy := map[string]string{}
	rules := []Object{}
	for i, r := range d.Rules {
		suffix, valid := domain(r.Suffix)
		a, ok := addr(r.Server)
		key := "+." + suffix
		if !valid || !ok || policy[key] != "" {
			return nil, nil, fmt.Errorf("第 %d 条 DNS 规则域名重复或地址无效", i+1)
		}
		policy[key] = a.String()
		tag := fmt.Sprintf("dns-%d", i+1)
		servers = append(servers, Object{"type": "udp", "tag": tag, "server": a.String()})
		rules = append(rules, Object{"domain_suffix": suffix, "action": "route", "server": tag})
	}
	// Mihomo uses the most specific suffix for nameserver-policy.
	sort.SliceStable(rules, func(i, j int) bool {
		return len(rules[i]["domain_suffix"].(string)) > len(rules[j]["domain_suffix"].(string))
	})
	return Object{"enable": true, "nameserver": []string{defaultAddr.String()}, "nameserver-policy": policy},
		Object{"servers": servers, "rules": rules, "final": "default", "reverse_mapping": true}, nil
}

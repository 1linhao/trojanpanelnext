package clientrouting

import (
	"fmt"
	"strings"
)

type Member struct {
	ID       uint
	ServerID uint
	Tag      string
}

func NodeTag(id uint, name string) string { return fmt.Sprintf("节点 %d · %s", id, name) }

type selection struct {
	tag   string
	auto  bool
	nodes []string
}

func (s Snapshot) selections(members []Member) ([]selection, map[string]string) {
	options := []selection{}
	targets := map[string]string{Direct: Direct, Reject: Reject, All: s.Empty}
	byID := map[uint]Member{}
	all := make([]string, 0, len(members))
	for _, m := range members {
		byID[m.ID] = m
		all = append(all, m.Tag)
	}
	if len(all) > 0 {
		targets[All] = All
		options = append(options, selection{tag: All, nodes: all})
	}
	for _, g := range s.Groups {
		tag := groupTag(g)
		nodes := []string{}
		for _, id := range g.NodeIDs {
			if m, ok := byID[id]; ok && m.ServerID == g.ServerID {
				nodes = append(nodes, m.Tag)
			}
		}
		targets[tag] = s.Empty
		if len(nodes) != 0 {
			targets[tag] = tag
			options = append(options, selection{tag: tag, auto: g.AutoTest, nodes: nodes})
		}
	}
	return options, targets
}

func copyObject(obj Object) Object {
	out := make(Object, len(obj))
	for k, v := range obj {
		out[k] = v
	}
	return out
}

func (s Snapshot) RenderClash(proxies []interface{}, members []Member) Object {
	cfg := copyObject(s.Clash)
	groups, targets := s.selections(members)
	options := []Object{}
	for _, g := range groups {
		item := Object{"name": g.tag, "type": "select", "proxies": g.nodes}
		if g.auto {
			item["type"], item["url"], item["interval"] = "url-test", probeURL, 300
		}
		options = append(options, item)
	}
	rules := make([]string, 0, len(s.ClashRules))
	for _, rule := range s.ClashRules {
		i := strings.LastIndexByte(rule, ',') + 1
		rules = append(rules, rule[:i]+targets[rule[i:]])
	}
	if proxies == nil {
		proxies = []interface{}{}
	}
	cfg["proxies"], cfg["proxy-groups"], cfg["rules"] = proxies, options, rules
	return cfg
}

func (s Snapshot) RenderSingBox(nodes []Object, members []Member) Object {
	cfg := copyObject(s.SingBox)
	groups, targets := s.selections(members)
	outbounds := append([]Object{{"type": "direct", "tag": Direct}}, nodes...)
	for _, g := range groups {
		item := Object{"type": "selector", "tag": g.tag, "outbounds": g.nodes}
		if g.auto {
			item["type"], item["url"], item["interval"] = "urltest", probeURL, "5m"
		}
		outbounds = append(outbounds, item)
	}
	rules := []Object{{"action": "sniff"}, {"protocol": "dns", "action": "hijack-dns"}}
	route := Object{"auto_detect_interface": true, "default_domain_resolver": "default", "final": Direct}
	for i, template := range s.SingRules {
		target := targets[template["outbound"].(string)]
		if i == len(s.SingRules)-1 && target != Reject {
			route["final"] = target
			break
		}
		rule := copyObject(template)
		rule["action"], rule["outbound"] = "route", target
		if target == Reject {
			delete(rule, "outbound")
			rule["action"] = "reject"
		}
		rules = append(rules, rule)
	}
	route["rules"] = rules
	cfg["outbounds"], cfg["route"] = outbounds, route
	return cfg
}

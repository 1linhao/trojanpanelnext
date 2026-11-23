package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"trojan-panel/core"
	"trojan-panel/dao"
	"trojan-panel/model"
	"trojan-panel/model/constant"
	"trojan-panel/model/dto"
	"trojan-panel/model/vo"
)

func validateXrayShape(content string) error {
	if len(content) > 10240 {
		return errors.New("Xray template exceeds 10 KiB")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal([]byte(content), &fields); err != nil || fields == nil {
		return errors.New("expected an Xray JSON object")
	}
	for k, raw := range fields {
		var allowed string
		switch k {
		case "log", "api", "dns", "routing", "policy", "transport", "stats", "reverse":
			allowed = "{"
		case "inbounds", "outbounds":
			allowed = "["
		case "fakeDns", "fakedns":
			allowed = "[{"
		default:
			return fmt.Errorf("unsupported Xray template field %q", k)
		}
		if string(raw) != "null" && raw[0] != allowed[0] && !(len(allowed) == 2 && raw[0] == allowed[1]) {
			return fmt.Errorf("invalid JSON type for Xray field %q", k)
		}
	}
	return nil
}

// Only exact, resource-independent configurations can be accepted when no
// Node kernel can check them. Other content requires every registered target.
func portableXrayTemplate(content string) bool {
	var current, builtin map[string]interface{}
	if json.Unmarshal([]byte(content), &current) != nil || current == nil {
		return false
	}
	for k, v := range current {
		if v == nil {
			delete(current, k)
		}
	}
	if len(current) == 0 {
		return true
	}
	_ = json.Unmarshal([]byte(constant.DefaultXrayTemplate), &builtin)
	if reflect.DeepEqual(current, builtin) {
		return true
	}
	if len(current) == 1 {
		log, ok := current["log"].(map[string]interface{})
		if ok && len(log) == 1 {
			switch log["loglevel"] {
			case "debug", "info", "warning", "error", "none":
				return true
			}
		}
	}
	return false
}

func changedXrayTemplates(old vo.SystemVo, next dto.SystemUpdateDto) []string {
	library := old.TemplateLibrary
	if next.TemplateLibrary != nil {
		library = *next.TemplateLibrary
	}
	fallback := old.XrayTemplate
	var changed []string
	seen := make(map[string]bool)
	add := func(content string) {
		if !seen[content] {
			changed = append(changed, content)
			seen[content] = true
		}
	}
	if next.XrayTemplate != nil && *next.XrayTemplate != fallback {
		fallback = *next.XrayTemplate
		add(fallback)
	}
	for _, t := range library.Xray {
		oldEntry, found := model.FindTemplate(old.TemplateLibrary.Xray, t.ID)
		if !found || oldEntry.Content != t.Content {
			add(t.Content)
		}
	}
	active := func(id string) string {
		if id == "" {
			return "default"
		}
		return id
	}
	if active(library.ActiveXray) != active(old.TemplateLibrary.ActiveXray) {
		content, _ := activeXrayTemplate(library, fallback)
		add(content)
	}
	return changed
}

func checkSystemTemplates(ctx context.Context, token string, next dto.SystemUpdateDto) error {
	name := constant.SystemName
	old, err := SelectSystemByName(&name)
	if err != nil {
		return err
	}
	if next.TemplateLibrary != nil {
		if err := validateTemplateLibrary(*next.TemplateLibrary, old.TemplateLibrary); err != nil {
			return err
		}
	}
	changed := changedXrayTemplates(old, next)
	if len(changed) == 0 {
		return nil
	}
	portable := true
	for _, content := range changed {
		if err := validateXrayShape(content); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidTemplateLibrary, err)
		}
		if !portableXrayTemplate(content) {
			portable = false
		}
	}
	servers, err := dao.SelectRegisteredNodeServers()
	if err != nil {
		return err
	}
	if len(servers) == 0 && !portable {
		return fmt.Errorf("%w: add an online Node with Xray template checking to verify this custom template", ErrInvalidTemplateLibrary)
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	for _, server := range servers {
		if err := ctx.Err(); err != nil {
			return fmt.Errorf("%w: Xray Node validation timed out", ErrInvalidTemplateLibrary)
		}
		err := core.CheckXrayTemplates(ctx, token, *server.Ip, *server.GrpcPort, nodeTransport(&server), changed)
		if err != nil {
			switch status.Code(err) {
			case codes.Unimplemented, codes.Unavailable, codes.FailedPrecondition, codes.DeadlineExceeded:
				if portable {
					continue
				}
				return fmt.Errorf("%w: Node #%d cannot verify this custom template; check its connection and Node version", ErrInvalidTemplateLibrary, *server.Id)
			}
			return fmt.Errorf("%w: Node #%d: %v", ErrInvalidTemplateLibrary, *server.Id, err)
		}
	}
	return nil
}

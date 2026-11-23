package service

import (
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
	"trojan-panel/model"

	"trojan-panel/model/constant"
	"trojan-panel/model/vo"
)

var ErrInvalidTemplateLibrary = errors.New("invalid template library")
var templateIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9_-]{0,63}$`)

func validateTemplateLibrary(library, previous model.TemplateLibrary) error {
	for _, group := range []struct {
		entries  []model.NamedTemplate
		format   string
		maxBytes int
	}{{library.Clash, "YAML", 102400}, {library.Xray, "JSON", 10240}} {
		if len(group.entries) > 16 {
			return fmt.Errorf("%w: at most 16 additional %s templates", ErrInvalidTemplateLibrary, group.format)
		}
		seen := map[string]bool{"default": true}
		for _, t := range group.entries {
			if seen[t.ID] || !templateIDPattern.MatchString(t.ID) || strings.TrimSpace(t.Name) == "" || utf8.RuneCountInString(t.Name) > 32 || len(t.Content) > group.maxBytes {
				return fmt.Errorf("%w: invalid %s template ID, name or length", ErrInvalidTemplateLibrary, group.format)
			}
			seen[t.ID] = true
			if group.format == "YAML" {
				if _, err := parseClashTemplate(t.Content); err != nil {
					return fmt.Errorf("%w: %s: %v", ErrInvalidTemplateLibrary, t.Name, err)
				}
			} else if old, found := model.FindTemplate(previous.Xray, t.ID); !found || old.Content != t.Content {
				if err := validateXrayShape(t.Content); err != nil {
					return fmt.Errorf("%w: %s: %v", ErrInvalidTemplateLibrary, t.Name, err)
				}
			}
		}
	}
	if _, err := activeXrayTemplate(library, ""); err != nil {
		return fmt.Errorf("%w: unknown active Xray template", ErrInvalidTemplateLibrary)
	}
	return nil
}

func parseClashTemplate(s string) (map[string]interface{}, error) {
	dec := yaml.NewDecoder(strings.NewReader(s))
	var config map[string]interface{}
	if err := dec.Decode(&config); err != nil || config == nil {
		return nil, errors.New("expected a YAML mapping")
	}
	var extra yaml.Node
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, errors.New("expected a single YAML document")
	}
	return config, nil
}

func nodeServerTemplate(nodeType uint) (string, error) {
	if nodeType != constant.Xray {
		return "", nil
	}
	name := constant.SystemName
	system, err := SelectSystemByName(&name)
	if err != nil {
		return "", err
	}
	return activeXrayTemplate(system.TemplateLibrary, system.XrayTemplate)
}

func activeXrayTemplate(library model.TemplateLibrary, fallback string) (string, error) {
	if library.ActiveXray == "" || library.ActiveXray == "default" {
		return fallback, nil
	}
	if t, ok := model.FindTemplate(library.Xray, library.ActiveXray); ok {
		return t.Content, nil
	}
	return "", errors.New(constant.ValidateFailed)
}

func clashTemplateOptions(system vo.SystemVo) []vo.ClientTemplateVo {
	result := []vo.ClientTemplateVo{{Id: "default", Name: system.ClashTemplateName}}
	for _, t := range system.TemplateLibrary.Clash {
		result = append(result, vo.ClientTemplateVo{Id: t.ID, Name: t.Name})
	}
	return result
}

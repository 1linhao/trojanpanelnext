package service

import (
	"errors"
	"strings"
	"testing"
	"trojan-panel/model"
)

func TestTemplateLibraryValidation(t *testing.T) {
	entry := model.NamedTemplate{ID: "commute", Name: "通勤", Content: "rules: []"}
	clash := func(id, name, content string) model.TemplateLibrary {
		return model.TemplateLibrary{Clash: []model.NamedTemplate{{ID: id, Name: name, Content: content}}}
	}
	for _, valid := range []model.TemplateLibrary{{}, clash("one", "One", "rules: []"), {ActiveXray: "same-id", Xray: []model.NamedTemplate{{ID: "same-id", Name: "Work", Content: "{}"}}}} {
		if err := validateTemplateLibrary(valid, model.TemplateLibrary{}); err != nil {
			t.Fatal(err)
		}
	}
	for label, invalid := range map[string]model.TemplateLibrary{
		"reserved": clash("default", "Valid", "{}"), "invalidID": clash("a/b", "Valid", "{}"),
		"blankName": clash("one", "  ", "{}"), "longName": clash("one", strings.Repeat("名", 33), "{}"),
		"multipleDocuments": clash("one", "Valid", "{}\n---\n{}"), "scalar": clash("one", "Valid", "true"),
		"duplicateKeys": clash("one", "Valid", "rules: []\nrules: []"), "long": clash("one", "Valid", strings.Repeat("a", 102401)),
		"duplicate": {Clash: []model.NamedTemplate{entry, entry}}, "dangling": {ActiveXray: "deleted"},
		"logType":   {Xray: []model.NamedTemplate{{ID: "bad", Name: "Bad", Content: `{"log":"warning"}`}}},
		"jsonArray": {Xray: []model.NamedTemplate{{ID: "bad", Name: "Bad", Content: "[]"}}},
		"jsonNull":  {Xray: []model.NamedTemplate{{ID: "bad", Name: "Bad", Content: "null"}}},
	} {
		t.Run(label, func(t *testing.T) {
			if !errors.Is(validateTemplateLibrary(invalid, model.TemplateLibrary{}), ErrInvalidTemplateLibrary) {
				t.Fatal("accepted invalid library")
			}
		})
	}
}

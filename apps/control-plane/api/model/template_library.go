package model

// TemplateLibrary supplements the existing file-backed default templates.
// Stable IDs are used in subscriptions even when administrators rename entries.
type TemplateLibrary struct {
	Clash      []NamedTemplate `json:"clash"`
	Xray       []NamedTemplate `json:"xray"`
	ActiveXray string          `json:"activeXray"`
}

type NamedTemplate struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

func FindTemplate(entries []NamedTemplate, id string) (NamedTemplate, bool) {
	for _, entry := range entries {
		if entry.ID == id {
			return entry, true
		}
	}
	return NamedTemplate{}, false
}

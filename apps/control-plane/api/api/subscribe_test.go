package api

import "testing"

func TestValidExportSelection(t *testing.T) {
	tests := []struct {
		client   string
		template string
		want     bool
	}{
		{client: "shadowrocket", template: "default", want: true},
		{client: "Shadowrocket", template: "default", want: true},
		{client: "shadowrocket", template: "tun", want: false},
		{client: "v2ray", template: "default", want: true},
		{client: "unknown", template: "default", want: false},
	}
	for _, test := range tests {
		if got := validExportSelection(test.client, test.template); got != test.want {
			t.Fatalf("validExportSelection(%q, %q) = %v, want %v", test.client, test.template, got, test.want)
		}
	}
}

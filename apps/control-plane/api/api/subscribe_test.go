package api

import (
	"net/http/httptest"
	"testing"
)

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

func TestSubscriptionQRCodeURL(t *testing.T) {
	tests := []struct {
		name      string
		request   string
		peer      string
		proto     string
		forwarded string
		want      string
	}{
		{"bundled reverse proxy", "http://127.0.0.1:8081/", "127.0.0.1:12345", "https", "panel.example.com", "https://panel.example.com"},
		{"forwarded public port", "http://localhost:8081/", "[::1]:12345", "https", "panel.example.com:8443", "https://panel.example.com:8443"},
		{"direct HTTPS", "https://panel.example.com/", "192.0.2.1:12345", "", "", "https://panel.example.com"},
		{"original host already preserved", "http://panel.example.com/", "127.0.0.1:12345", "https", "foreign.example", "https://panel.example.com"},
		{"ignore forwarded host from nonlocal peer", "http://127.0.0.1:8081/", "192.0.2.1:12345", "", "foreign.example", "http://127.0.0.1:8081"},
		{"ignore malformed forwarded authority", "http://127.0.0.1:8081/", "127.0.0.1:12345", "https", "panel.example.com, foreign.example", "https://127.0.0.1:8081"},
	}
	path := buildSubscribePath("test-token", "v2ray", "default")
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tc.request, nil)
			req.RemoteAddr = tc.peer
			req.Header.Set("X-Forwarded-Proto", tc.proto)
			req.Header.Set("X-Forwarded-Host", tc.forwarded)
			if got := subscriptionQRCodeURL(req, path); got != tc.want+path {
				t.Fatalf("got %q, want %q", got, tc.want+path)
			}
		})
	}
}

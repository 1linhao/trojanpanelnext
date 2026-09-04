package clientcompat

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"strings"
	"testing"
)

func TestV2RaySubscriptionFormat(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		userAgent string
		want      string
	}{
		{name: "v2rayNG released client", userAgent: "v2rayNG/2.2.5", want: V2RaySubscriptionStandard},
		{name: "v2rayNG case insensitive", userAgent: "V2RAYNG/2.2.6", want: V2RaySubscriptionStandard},
		{name: "v2rayN desktop", userAgent: "v2rayN/7.24.2", want: V2RaySubscriptionV2RayN},
		{name: "browser fallback", userAgent: "Mozilla/5.0", want: V2RaySubscriptionStandard},
		{name: "empty fallback", userAgent: "", want: V2RaySubscriptionStandard},
	}
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if got := V2RaySubscriptionFormat(test.userAgent); got != test.want {
				t.Fatalf("V2RaySubscriptionFormat(%q) = %q, want %q", test.userAgent, got, test.want)
			}
		})
	}
}

func TestV2RayNGHysteria2URI(t *testing.T) {
	t.Parallel()

	uri := V2RayNGHysteria2URI(V2RayNHysteria2Config{
		Remarks:        "hy2 test",
		Address:        "example.com",
		Port:           29999,
		Password:       "test-password",
		SNI:            "sni.example.com",
		AllowInsecure:  true,
		SalamanderPass: "obfs-password",
		UpMbps:         30,
		DownMbps:       50,
		Ports:          "20000-30000",
		HopInterval:    "30",
	})
	parsed, err := url.Parse(uri)
	if err != nil {
		t.Fatalf("parse standard URI: %v", err)
	}
	if parsed.Scheme != "hysteria2" || parsed.Hostname() != "example.com" || parsed.Port() != "29999" {
		t.Fatalf("standard URI endpoint = %s://%s, want hysteria2://example.com:29999", parsed.Scheme, parsed.Host)
	}
	if parsed.User.Username() != "test-password" || parsed.Fragment != "hy2 test" {
		t.Fatalf("standard URI auth/remarks were not preserved: %q", uri)
	}
	query := parsed.Query()
	wantQuery := map[string]string{
		"insecure":      "1",
		"upmbps":        "30",
		"downmbps":      "50",
		"mport":         "20000-30000",
		"mportHopInt":   "30",
		"obfs":          "salamander",
		"obfs-password": "obfs-password",
		"sni":           "sni.example.com",
	}
	for key, want := range wantQuery {
		if got := query.Get(key); got != want {
			t.Errorf("standard URI query %q = %q, want %q", key, got, want)
		}
	}
}

func TestV2RayNHysteria2URI(t *testing.T) {
	t.Parallel()

	uri, err := V2RayNHysteria2URI(V2RayNHysteria2Config{
		Remarks:        "hy2-test",
		Address:        "example.com",
		Port:           29999,
		Password:       "test-password",
		SNI:            "sni.example.com",
		AllowInsecure:  true,
		SalamanderPass: "obfs-password",
		UpMbps:         30,
		DownMbps:       50,
		Ports:          "20000-30000",
		HopInterval:    "30",
	})
	if err != nil {
		t.Fatalf("V2RayNHysteria2URI() error = %v", err)
	}
	const prefix = "v2rayn://hysteria2/"
	if !strings.HasPrefix(uri, prefix) {
		t.Fatalf("V2RayNHysteria2URI() = %q, want prefix %q", uri, prefix)
	}
	content, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(uri, prefix))
	if err != nil {
		t.Fatalf("decode inner URI: %v", err)
	}
	var profile v2RayNHysteria2Profile
	if err := json.Unmarshal(content, &profile); err != nil {
		t.Fatalf("unmarshal inner profile: %v", err)
	}
	if profile.ConfigType != v2RayNConfigTypeHysteria2 || profile.ConfigVersion != v2RayNConfigVersion {
		t.Fatalf("profile version/type = %d/%d", profile.ConfigVersion, profile.ConfigType)
	}
	if profile.Address != "example.com" || profile.Port != 29999 || profile.Password != "test-password" {
		t.Fatalf("profile endpoint/auth was not preserved: %+v", profile)
	}
	if profile.AllowInsecure != "true" || profile.SNI != "sni.example.com" {
		t.Fatalf("profile TLS settings were not preserved: %+v", profile)
	}
	if profile.ProtoExtra.UpMbps != 30 || profile.ProtoExtra.DownMbps != 50 {
		t.Fatalf("profile bandwidth = %d/%d", profile.ProtoExtra.UpMbps, profile.ProtoExtra.DownMbps)
	}
	if profile.ProtoExtra.Ports != "20000-30000" || profile.ProtoExtra.HopInterval != "30" {
		t.Fatalf("profile port hopping was not preserved: %+v", profile.ProtoExtra)
	}
}

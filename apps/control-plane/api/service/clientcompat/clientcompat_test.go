package clientcompat

import (
	"reflect"
	"testing"
	"trojan-panel/model/constant"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name    string
		clients *[]string
		want    string
		wantErr bool
	}{
		{name: "missing uses defaults", want: constant.DefaultNodeClientTypes},
		{name: "empty selection", clients: stringSlicePointer([]string{}), want: ""},
		{
			name:    "normalizes order and duplicates",
			clients: stringSlicePointer([]string{constant.ClientShadowrocket, constant.ClientV2Ray, constant.ClientSingBox, constant.ClientV2Ray}),
			want:    constant.ClientSingBox + "," + constant.ClientV2Ray + "," + constant.ClientShadowrocket,
		},
		{name: "rejects unknown client", clients: stringSlicePointer([]string{"unknown"}), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Encode(tt.clients)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("Encode() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDecodeAndIncludes(t *testing.T) {
	if got := Decode(nil); !reflect.DeepEqual(got, DefaultClients()) {
		t.Fatalf("Decode(nil) = %v, want %v", got, DefaultClients())
	}

	empty := ""
	if got := Decode(&empty); len(got) != 0 {
		t.Fatalf("Decode(empty) = %v, want empty", got)
	}

	value := constant.ClientV2Ray + "," + constant.ClientSingBox
	if !Includes(&value, constant.ClientSingBox) {
		t.Fatal("Includes() did not find sing-box")
	}
	if Includes(&value, constant.ClientClashMeta) {
		t.Fatal("Includes() unexpectedly found clash-meta")
	}
}

func TestValidateNode(t *testing.T) {
	allClients := DefaultClients()
	singBoxOnly := []string{constant.ClientSingBox}

	if err := ValidateNode(uintPointer(constant.NaiveProxy), &allClients); err == nil {
		t.Fatal("ValidateNode() accepted NaiveProxy with Clash.Meta")
	}
	if err := ValidateNode(uintPointer(constant.NaiveProxy), nil); err == nil {
		t.Fatal("ValidateNode() accepted NaiveProxy with implicit defaults")
	}
	if err := ValidateNode(uintPointer(constant.NaiveProxy), &singBoxOnly); err != nil {
		t.Fatalf("ValidateNode() rejected NaiveProxy with sing-box: %v", err)
	}
	if err := ValidateNode(uintPointer(constant.Hysteria2), &allClients); err != nil {
		t.Fatalf("ValidateNode() rejected Hysteria2 defaults: %v", err)
	}
}

func uintPointer(value uint) *uint {
	return &value
}

func stringSlicePointer(value []string) *[]string {
	return &value
}

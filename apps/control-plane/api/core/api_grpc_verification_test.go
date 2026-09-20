package core

import "testing"

func TestValidateNodeVerificationResponseRequiresExactEcho(t *testing.T) {
	want := NodeVerification{
		IdentityID: "11111111-2222-4333-8444-555555555555", Generation: 7, ServerID: 42,
		Challenge: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
	}
	matching := func() *NodeServerStateVo {
		return &NodeServerStateVo{
			NodeIdentityId: want.IdentityID, IdentityGeneration: want.Generation,
			NodeServerId: want.ServerID, BootstrapChallenge: want.Challenge,
		}
	}
	if err := validateNodeVerificationResponse(matching(), want); err != nil {
		t.Fatalf("matching response rejected: %v", err)
	}
	tests := map[string]func(*NodeServerStateVo){
		"identity":   func(state *NodeServerStateVo) { state.NodeIdentityId = "other" },
		"generation": func(state *NodeServerStateVo) { state.IdentityGeneration-- },
		"server":     func(state *NodeServerStateVo) { state.NodeServerId++ },
		"challenge":  func(state *NodeServerStateVo) { state.BootstrapChallenge = "stale" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			state := matching()
			mutate(state)
			if err := validateNodeVerificationResponse(state, want); err == nil {
				t.Fatal("mismatched response was accepted")
			}
		})
	}
	if err := validateNodeVerificationResponse(nil, want); err == nil {
		t.Fatal("nil response was accepted")
	}
}

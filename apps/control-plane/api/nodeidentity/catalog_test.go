package nodeidentity

import "testing"

func TestCatalogEndpointLockUsesIPAddress(t *testing.T) {
	compressed := catalogEndpointLockName("2001:db8::77", 8400)
	expanded := catalogEndpointLockName("2001:0db8:0:0:0:0:0:77", 8400)
	if compressed != expanded {
		t.Fatal("equivalent IPv6 endpoints must share the same registration lock")
	}
	if compressed == catalogEndpointLockName("2001:db8::77", 8401) {
		t.Fatal("different gRPC ports must use independent registration locks")
	}
	if catalogEndpointLockName("203.0.113.78", 8500) != catalogEndpointLockName("::ffff:203.0.113.78", 8500) {
		t.Fatal("IPv4 and IPv4-mapped IPv6 must share the same registration lock")
	}
}

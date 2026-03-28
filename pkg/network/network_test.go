package network

import (
	"net"
	"testing"
)

func TestGetLocalIP(t *testing.T) {
	ip, mask, err := getLocalIP()
	if err != nil {
		t.Fatalf("getLocalIP failed: %v", err)
	}

	t.Logf("IP: %s, Mask: %s", ip, mask)

	if net.ParseIP(ip) == nil {
		t.Errorf("Invalid IP returned: %s", ip)
	}

	if net.ParseIP(mask) == nil {
		t.Errorf("Invalid mask returned: %s", mask)
	}

    // Check if the mask is in dotted decimal format for IPv4
    // (Simple check, mostly to ensure we don't get hex or CIDR suffix unless intended,
    // but the original code returned "255.255.255.0", so we expect dotted decimal).
    // Note: net.IP(mask).String() returns dotted decimal.
}

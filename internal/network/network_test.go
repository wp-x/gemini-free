package network

import (
	"net"
	"strings"
	"testing"
)

func TestDescribeScopesAndAddressFamilies(t *testing.T) {
	addresses := []Address{
		{Interface: "wifi", IP: net.ParseIP("192.168.1.24")},
		{Interface: "wifi", IP: net.ParseIP("192.168.1.24")},
		{Interface: "loopback", IP: net.ParseIP("127.0.0.1")},
		{Interface: "link", IP: net.ParseIP("169.254.2.3")},
		{Interface: "wifi", IP: net.ParseIP("fd00::24")},
	}
	tests := []struct {
		host string
		url  string
		lan  string
	}{
		{"127.0.0.1", "http://127.0.0.1:8081", ""},
		{"localhost", "http://localhost:8081", ""},
		{"::1", "http://[::1]:8081", ""},
		{"0.0.0.0", "http://127.0.0.1:8081", "http://192.168.1.24:8081"},
		{"::", "http://[::1]:8081", "http://[fd00::24]:8081"},
		{"192.168.1.24", "http://192.168.1.24:8081", "http://192.168.1.24:8081"},
	}
	for _, item := range tests {
		t.Run(item.host, func(t *testing.T) {
			state := Describe(item.host, 8081, addresses)
			if state.LocalURL != item.url || state.LANEnabled != (item.lan != "") {
				t.Fatalf("incorrect scope: %+v", state)
			}
			if item.lan == "" && len(state.LAN) != 0 {
				t.Fatalf("local-only mode advertised LAN endpoints: %+v", state)
			}
			if item.lan != "" && (len(state.LAN) != 1 || state.LAN[0].URL != item.lan) {
				t.Fatalf("incorrect LAN endpoints: %+v", state)
			}
			if strings.Contains(state.LocalURL, "0.0.0.0") {
				t.Fatal("advertised a wildcard as a client address")
			}
		})
	}
}

func TestDescribeOfflineDoesNotInventLANAddress(t *testing.T) {
	state := Describe("0.0.0.0", 8081, nil)
	if !state.LANEnabled || len(state.LAN) != 0 {
		t.Fatalf("unexpected offline state: %+v", state)
	}
}

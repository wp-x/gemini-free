package app

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"gemini-free/internal/network"
)

func TestLANServicePreservesAuthenticationAndLocalManagement(t *testing.T) {
	c, _ := testController(t)
	c.addresses = func() ([]network.Address, error) {
		return []network.Address{{Interface: "wifi", IP: net.ParseIP("192.168.1.24")}}, nil
	}
	cfg := c.Config()
	cfg.Host, cfg.Port = "0.0.0.0", freePort(t)
	keys := []string{"lan-test-key"}
	if err := c.Configure(SettingsUpdate{Config: cfg, APIKeys: &keys}); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	state := c.State()
	if len(state.Network.LAN) != 1 || !state.Network.LANEnabled {
		t.Fatalf("missing LAN state: %+v", state.Network)
	}
	assertHealth(t, &http.Client{Timeout: time.Second}, state.Network.LocalURL)
	control := testControl(t, c)
	listener, err := control.Listen()
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if !listener.Addr().(*net.TCPAddr).IP.IsLoopback() {
		t.Fatal("LAN mode exposed management outside loopback")
	}
	request := controlRequest(control, http.MethodGet, "/v1/models")
	expectStatus(t, controlResponse(c, request), http.StatusUnauthorized)
	request.Header.Set("Authorization", "Bearer "+keys[0])
	expectStatus(t, controlResponse(c, request), http.StatusOK)
	request = controlRequest(control, http.MethodGet, "/control/state")
	request.Header.Set("Authorization", "Bearer "+keys[0])
	expectStatus(t, controlResponse(c, request), http.StatusNotFound)
}

func TestNetworkDiscoveryErrorIsVisible(t *testing.T) {
	c, _ := testController(t)
	c.addresses = func() ([]network.Address, error) { return nil, fmt.Errorf("interface unavailable") }
	cfg := c.Config()
	cfg.Host = "0.0.0.0"
	if err := c.Configure(SettingsUpdate{Config: cfg}); err != nil {
		t.Fatal(err)
	}
	state := c.State()
	if state.Network.Error == "" || len(state.Network.LAN) != 0 {
		t.Fatalf("network failure was hidden: %+v", state.Network)
	}
}

func TestLANScopeCannotChangeWhileServiceIsRunning(t *testing.T) {
	c, _ := testController(t)
	cfg := c.Config()
	cfg.Port = freePort(t)
	if err := c.Configure(SettingsUpdate{Config: cfg}); err != nil {
		t.Fatal(err)
	}
	if err := c.Start(); err != nil {
		t.Fatal(err)
	}
	cfg.Host = "0.0.0.0"
	if err := c.Configure(SettingsUpdate{Config: cfg}); err == nil {
		t.Fatal("scope change succeeded without stopping the listener")
	}
	if c.State().Network.LANEnabled {
		t.Fatal("rejected scope change mutated the current state")
	}
}

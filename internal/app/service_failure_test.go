package app

import (
	"net"
	"net/http"
	"testing"
)

func TestUnexpectedListenerFailureAllowsServiceRestart(t *testing.T) {
	c, _ := testController(t)
	cfg := c.Config()
	cfg.Port = freePort(t)
	if err := c.Configure(SettingsUpdate{Config: cfg}); err != nil {
		t.Fatal(err)
	}
	listener := closedListener(t)
	server := &http.Server{Handler: c}
	c.server, c.service = server, Service{Running: true}
	c.serve(server, listener)
	state := c.State().Service
	if state.Running || state.Error == "" {
		t.Fatalf("failed listener retained an invalid running state: %+v", state)
	}
	if err := c.Start(); err != nil {
		t.Fatalf("failed listener prevented restart: %v", err)
	}
	if c.State().Service.Error != "" {
		t.Fatal("successful restart retained the previous error")
	}
}

func TestOldListenerFailureDoesNotClearReplacementService(t *testing.T) {
	c, _ := testController(t)
	old := &http.Server{Handler: c}
	replacement := &http.Server{Handler: c}
	c.server, c.service = replacement, Service{Running: true}
	c.serve(old, closedListener(t))
	if c.server != replacement || !c.State().Service.Running || c.State().Service.Error != "" {
		t.Fatal("old listener failure overwrote the replacement service state")
	}
}

func closedListener(t *testing.T) net.Listener {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return listener
}

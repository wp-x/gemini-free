package app

import (
	"net/http"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"gemini-free/internal/config"
)

func TestServiceStopsAndRestartsAtConfiguredAddress(t *testing.T) {
	controller, _ := testController(t)
	cfg := controller.Config()
	cfg.Port = freePort(t)
	if err := controller.Configure(SettingsUpdate{Config: cfg}); err != nil {
		t.Fatal(err)
	}
	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	if err := controller.Start(); err == nil {
		t.Fatal("starting an already running service succeeded")
	}
	oldAddress := controller.State().Service.Address
	client := &http.Client{Timeout: time.Second}
	assertHealth(t, client, oldAddress)
	if err := controller.Stop(); err != nil {
		t.Fatal(err)
	}
	if controller.State().Service.Running {
		t.Fatal("stopped service remains running")
	}
	if response, err := client.Get(oldAddress + "/health"); err == nil {
		response.Body.Close()
		t.Fatal("stopped listener still accepted a request")
	}
	cfg.Port = freePort(t)
	if err := controller.Configure(SettingsUpdate{Config: cfg}); err != nil {
		t.Fatal(err)
	}
	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	newAddress := controller.State().Service.Address
	if newAddress == oldAddress {
		t.Fatal("restart ignored configured address")
	}
	assertHealth(t, client, newAddress)
}

func assertHealth(t *testing.T, client *http.Client, address string) {
	t.Helper()
	response, err := client.Get(address + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("health returned %d", response.StatusCode)
	}
}

func TestConfigureAPIKeysChangesAuthenticationImmediately(t *testing.T) {
	controller, _ := testController(t)
	control := testControl(t, controller)
	keys := []string{"local-only-key"}
	response := controlJSON(control, "/control/config", SettingsUpdate{Config: config.Default(), APIKeys: &keys})
	expectStatus(t, response, http.StatusOK)
	request := controlRequest(control, http.MethodGet, "/v1/models")
	out := controlResponse(controller, request)
	expectStatus(t, out, http.StatusUnauthorized)
	request.Header.Set("Authorization", "Bearer "+keys[0])
	expectStatus(t, controlResponse(controller, request), http.StatusOK)
	empty := []string{}
	if err := controller.Configure(SettingsUpdate{Config: controller.Config(), APIKeys: &empty}); err != nil {
		t.Fatal(err)
	}
	request.Header.Del("Authorization")
	expectStatus(t, controlResponse(controller, request), http.StatusOK)
}

func TestRunningServiceRejectsAddressChangeUntilStopped(t *testing.T) {
	controller, _ := testController(t)
	cfg := controller.Config()
	cfg.Port = freePort(t)
	if err := controller.Configure(SettingsUpdate{Config: cfg}); err != nil {
		t.Fatal(err)
	}
	if err := controller.Start(); err != nil {
		t.Fatal(err)
	}
	address := controller.State().Service.Address
	next := cfg
	next.Port = freePort(t)
	if err := controller.Configure(SettingsUpdate{Config: next}); err == nil {
		t.Fatal("running service accepted an unapplied address change")
	}
	if controller.Config().Port != cfg.Port || controller.State().Service.Address != address {
		t.Fatal("rejected listener change mutated effective configuration")
	}
	if err := controller.Stop(); err != nil {
		t.Fatal(err)
	}
	if err := controller.Configure(SettingsUpdate{Config: next}); err != nil {
		t.Fatal(err)
	}
}

func TestConcurrentServiceStartsAndStopsAreSerialized(t *testing.T) {
	controller, _ := testController(t)
	cfg := controller.Config()
	cfg.Port = freePort(t)
	if err := controller.Configure(SettingsUpdate{Config: cfg}); err != nil {
		t.Fatal(err)
	}
	var group sync.WaitGroup
	var started atomic.Int32
	const workers = 8
	for range workers {
		group.Go(func() {
			if controller.Start() == nil {
				started.Add(1)
			}
		})
	}
	group.Wait()
	if started.Load() != 1 || !controller.State().Service.Running {
		t.Fatalf("concurrent starts produced %d listeners", started.Load())
	}
	for range workers {
		group.Go(func() {
			if err := controller.Stop(); err != nil {
				t.Error(err)
			}
		})
	}
	group.Wait()
	if controller.State().Service.Running {
		t.Fatal("concurrent stops left listener running")
	}
}

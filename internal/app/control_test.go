package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"gemini-free/internal/config"
)

func TestManagementHostAndTokenAuthorization(t *testing.T) {
	controller, _ := testController(t)
	control := testControl(t, controller)
	tests := []struct {
		host   string
		token  string
		status int
	}{
		{control.Host, control.Token, http.StatusOK},
		{control.Host, "", http.StatusUnauthorized},
		{control.Host, "wrong-token", http.StatusUnauthorized},
		{"attacker.invalid", control.Token, http.StatusForbidden},
		{"localhost:23456", control.Token, http.StatusForbidden},
	}
	for _, item := range tests {
		request := controlRequest(control, http.MethodGet, "/control/state")
		request.Host = item.host
		request.Header.Set("X-Desktop-Token", item.token)
		response := httptest.NewRecorder()
		control.ServeHTTP(response, request)
		expectStatus(t, response, item.status)
		if response.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("management state must not be cached")
		}
	}
}

func TestManagementRejectsInvalidConfiguration(t *testing.T) {
	controller, _ := testController(t)
	control := testControl(t, controller)
	tests := []func(*config.Config){
		func(cfg *config.Config) { cfg.Mode = "automatic" },
		func(cfg *config.Config) { cfg.Mode = config.AccountMode; cfg.SelectedAccount = "missing" },
		func(cfg *config.Config) { cfg.Port = 0 },
		func(cfg *config.Config) { cfg.Host = "example.invalid" },
		func(cfg *config.Config) { cfg.DefaultModel = "unknown" },
		func(cfg *config.Config) { cfg.Proxy = "file:///tmp/proxy" },
	}
	before, _ := json.Marshal(controller.Config())
	for _, invalidate := range tests {
		cfg := config.Default()
		invalidate(&cfg)
		response := controlJSON(control, "/control/config", cfg)
		expectStatus(t, response, http.StatusBadRequest)
		after, _ := json.Marshal(controller.Config())
		if string(after) != string(before) {
			t.Fatal("invalid configuration modified current state")
		}
	}
}

func TestManagementRejectsTrailingJSONBeforeMutation(t *testing.T) {
	controller, _ := testController(t)
	control := testControl(t, controller)
	cfg := config.Default()
	cfg.Port = freePort(t)
	body, _ := json.Marshal(cfg)
	request := controlRequest(control, http.MethodPost, "/control/config")
	request.Body = io.NopCloser(strings.NewReader(string(body) + "\n{}"))
	response := httptest.NewRecorder()
	control.ServeHTTP(response, request)
	expectStatus(t, response, http.StatusBadRequest)
	if controller.Config().Port != config.DefaultPort {
		t.Fatal("trailing JSON request mutated configuration")
	}
}

func TestManagementStateDoesNotExposeSecrets(t *testing.T) {
	controller, repo := testController(t)
	control := testControl(t, controller)
	secret := "__Secure-1PSID=private-cookie; SAPISID=private-signing-cookie"
	input := AccountInput{Name: "Test account", Cookie: secret, AuthUser: "2",
		XSRF: "private-xsrf-token", Build: "session-build"}
	if err := controller.AddAccount(input); err != nil {
		t.Fatal(err)
	}
	keys := []string{"private-api-key"}
	if err := controller.Configure(SettingsUpdate{Config: controller.Config(), APIKeys: &keys}); err != nil {
		t.Fatal(err)
	}
	request := controlRequest(control, http.MethodGet, "/control/state")
	response := httptest.NewRecorder()
	control.ServeHTTP(response, request)
	expectStatus(t, response, http.StatusOK)
	for _, forbidden := range []string{secret, "private-cookie", "private-signing-cookie", "private-xsrf-token", keys[0]} {
		if strings.Contains(response.Body.String(), forbidden) {
			t.Fatalf("management state exposed credential %q", forbidden)
		}
	}
	state := controller.State()
	if len(state.Accounts) != 1 || state.APIKeysConfigured != 1 {
		t.Fatalf("missing account/key metadata: %+v", state)
	}
	doc, err := repo.Load(state.Accounts[0].ID)
	if err != nil || doc.Cookie != secret {
		t.Fatal("test credential was not stored in injected repository")
	}
}

func TestFailedConfigurationSaveDoesNotPersistNewKeys(t *testing.T) {
	controller, repo := testController(t)
	validPath := controller.path
	oldKeys := []string{"old-local-key"}
	if err := controller.Configure(SettingsUpdate{Config: controller.Config(), APIKeys: &oldKeys}); err != nil {
		t.Fatal(err)
	}
	blocker := controller.path + ".parent-file"
	if err := os.WriteFile(blocker, []byte("not a directory"), 0600); err != nil {
		t.Fatal(err)
	}
	controller.path = blocker + "/config.json"
	newKeys := []string{"new-local-key"}
	if err := controller.Configure(SettingsUpdate{Config: controller.Config(), APIKeys: &newKeys}); err == nil {
		t.Fatal("configuration write unexpectedly succeeded")
	}
	cfg, err := config.Load(validPath)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := New(Options{Config: cfg, Path: validPath, Credentials: repo,
		Client: controller.client, Addresses: controller.addresses})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	request.Header.Set("Authorization", "Bearer "+oldKeys[0])
	expectStatus(t, controlResponse(restarted, request), http.StatusOK)
	request.Header.Set("Authorization", "Bearer "+newKeys[0])
	expectStatus(t, controlResponse(restarted, request), http.StatusUnauthorized)
	if restarted.State().APIKeysConfigured != 1 {
		t.Fatal("failed configuration save removed persisted API authentication")
	}
}

package app

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"gemini-free/internal/config"
	"gemini-free/internal/gemini"
	"gemini-free/internal/generation"
)

func testWireResponse() *http.Response {
	const textIndex = 4
	const modeIndex = 58
	const thinkingIndex = 59
	inner := make([]any, thinkingIndex+1)
	inner[textIndex] = []any{[]any{"candidate", []string{"42"}}}
	inner[modeIndex], inner[thinkingIndex] = generation.FlashLiteMode, generation.ThinkingLow
	body, _ := json.Marshal(inner)
	envelope, _ := json.Marshal([]any{[]any{"wrb.fr", nil, string(body)}})
	return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header),
		Body: io.NopCloser(strings.NewReader(string(envelope) + "\n"))}
}

func controlResponse(handler http.Handler, request *http.Request) *httptest.ResponseRecorder {
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func TestAPIAndProbeUseSameExplicitMode(t *testing.T) {
	controller, repo := testController(t)
	control := testControl(t, controller)
	input := AccountInput{Name: "Local test", Cookie: "__Secure-1PSID=test-identity; SAPISID=test-signer",
		AuthUser: "2", XSRF: "test-xsrf", Build: "test-build"}
	if err := controller.AddAccount(input); err != nil {
		t.Fatal(err)
	}
	account := controller.Config().Accounts[0]
	doc, err := repo.Load(account.ID)
	if err != nil {
		t.Fatal(err)
	}
	doc.RefreshedAt = time.Now()
	if err := repo.Save(account.ID, doc); err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{config.AccountMode, config.Anonymous} {
		cfg := controller.Config()
		cfg.Mode, cfg.SelectedAccount = mode, account.ID
		if err := controller.Configure(SettingsUpdate{Config: cfg}); err != nil {
			t.Fatal(err)
		}
		checkUpstreamMode(t, controller, mode)
		payload := `{"model":"gemini-flash-lite","messages":[{"role":"user","content":"17+25"}]}`
		request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(payload))
		expectStatus(t, controlResponse(controller, request), http.StatusOK)
		probe := controlJSON(control, "/control/probe", map[string]string{"prompt": "17+25", "model": generation.DefaultModel})
		expectStatus(t, probe, http.StatusOK)
		if !strings.Contains(probe.Body.String(), "event: result") {
			t.Fatalf("probe failed: %s", probe.Body.String())
		}
	}
}

func checkUpstreamMode(t *testing.T, controller *Controller, mode string) {
	t.Helper()
	controller.client = func(cfg config.Config) (gemini.HTTPDoer, error) {
		if cfg.Mode != mode {
			t.Errorf("client received mode %s, want %s", cfg.Mode, mode)
		}
		return doerFunc(func(request *http.Request) (*http.Response, error) {
			body, _ := io.ReadAll(request.Body)
			form, _ := url.ParseQuery(string(body))
			authenticated := request.Header.Get("Cookie") != ""
			if authenticated != (mode == config.AccountMode) {
				t.Errorf("mode %s cookie authentication=%v", mode, authenticated)
			}
			if mode == config.AccountMode {
				if request.Header.Get("X-Goog-AuthUser") != "2" || form.Get("at") != "test-xsrf" {
					t.Error("account mode lost session parameters")
				}
			} else if form.Has("at") || request.Header.Get("Authorization") != "" || strings.Contains(request.URL.Path, "/u/") {
				t.Error("anonymous mode reused account parameters")
			}
			return testWireResponse(), nil
		}), nil
	}
	controller.engine = gemini.New(gemini.EngineOptions{Config: controller.Config,
		Sessions: controller.sessions, Client: controller.client})
}

func TestConcurrentStateGenerationAndConfiguration(t *testing.T) {
	controller, _ := testController(t)
	controller.client = func(config.Config) (gemini.HTTPDoer, error) {
		return doerFunc(func(*http.Request) (*http.Response, error) { return testWireResponse(), nil }), nil
	}
	controller.engine = gemini.New(gemini.EngineOptions{Config: controller.Config,
		Sessions: controller.sessions, Client: controller.client})
	var group sync.WaitGroup
	const workers = 8
	for index := range workers {
		group.Go(func() {
			for range workers {
				if index%2 == 0 {
					update := SettingsUpdate{Config: controller.Config()}
					if err := controller.Configure(update); err != nil {
						t.Error(err)
					}
				} else {
					request := httptest.NewRequest(http.MethodPost, "/v1/chat/completions",
						strings.NewReader(`{"messages":[{"role":"user","content":"hi"}]}`))
					expectStatus(t, controlResponse(controller, request), http.StatusOK)
				}
				_ = controller.State()
			}
		})
	}
	group.Wait()
}

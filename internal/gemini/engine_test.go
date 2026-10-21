package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gemini-free/internal/config"
	"gemini-free/internal/generation"
)

type doFunc func(*http.Request) (*http.Response, error)

func (f doFunc) Do(r *http.Request) (*http.Response, error) { return f(r) }

func wireFrame(text string, mode int, thinking int) string {
	inner := make([]any, responseThinkingIndex+1)
	inner[responseTextIndex] = []any{[]any{"candidate", []string{text}}}
	inner[responseModeIndex], inner[responseThinkingIndex] = mode, thinking
	body, _ := json.Marshal(inner)
	envelope, _ := json.Marshal([]any{[]any{"wrb.fr", nil, string(body)}})
	return string(envelope) + "\n"
}

func TestAnonymousDoesNotUseAccountState(t *testing.T) {
	cfg := config.Default()
	cfg.SelectedAccount = "account-with-secret"
	cfg.Accounts = []config.Account{{ID: cfg.SelectedAccount, AuthUser: "2"}}
	client := doFunc(func(r *http.Request) (*http.Response, error) {
		for _, name := range []string{"Cookie", "Authorization", "X-Goog-AuthUser"} {
			if r.Header.Get(name) != "" {
				t.Errorf("anonymous leaked %s", name)
			}
		}
		if strings.Contains(r.URL.Path, "/u/") {
			t.Error("anonymous used account URL")
		}
		data, _ := io.ReadAll(r.Body)
		form, _ := url.ParseQuery(string(data))
		if form.Has("at") {
			t.Error("anonymous leaked session token")
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(wireFrame("42", 6, 1)))}, nil
	})
	engine := New(EngineOptions{Config: func() config.Config { return cfg },
		Client: func(config.Config) (HTTPDoer, error) { return client, nil }})
	result, err := engine.Generate(context.Background(), generation.Request{Model: generation.DefaultModel, Prompt: "17+25"}, nil)
	if err != nil || result.Text != "42" || result.ActualMode != 6 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}

func TestAnonymousAccountModelsFailBeforeCreatingClient(t *testing.T) {
	cfg := config.Default()
	clientCalls := 0
	engine := New(EngineOptions{Config: func() config.Config { return cfg },
		Client: func(config.Config) (HTTPDoer, error) {
			clientCalls++
			return doFunc(func(*http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("unexpected upstream request")
			}), nil
		}})
	for _, model := range generation.Models() {
		if !model.Account {
			continue
		}
		_, err := engine.Generate(context.Background(), generation.Request{Model: model.ID, Prompt: "hi"}, nil)
		if err == nil || !strings.Contains(err.Error(), "此模型需要登录会话") {
			t.Errorf("%s: expected login requirement, got %v", model.ID, err)
		}
	}
	if clientCalls != 0 {
		t.Errorf("anonymous account requests created %d upstream clients", clientCalls)
	}
}

func TestStreamDeltasAndRouteFailure(t *testing.T) {
	var deltas []string
	response := &http.Response{Body: io.NopCloser(strings.NewReader(wireFrame("你", 6, 1) + wireFrame("你好", 6, 1)))}
	model, _ := generation.Resolve(generation.Request{Model: generation.DefaultModel})
	result, err := readGeneration(response, model, func(event generation.Event) error {
		deltas = append(deltas, event.Text)
		return nil
	})
	if err != nil || result.Text != "你好" || strings.Join(deltas, "|") != "你|好" {
		t.Fatalf("result=%+v deltas=%v err=%v", result, deltas, err)
	}
	argon, _ := generation.Resolve(generation.Request{Model: "gemini-4-argon"})
	if validateRoute(argon, frame{text: "answer", mode: 6, thinking: 2}) == nil {
		t.Error("model downgrade accepted")
	}
	if validateRoute(argon, frame{text: "answer", mode: 8, thinking: 1}) == nil {
		t.Error("thinking downgrade accepted")
	}
}

func TestMalformedAndEmptyStreamsFail(t *testing.T) {
	model, _ := generation.Resolve(generation.Request{Model: generation.DefaultModel})
	for _, input := range []string{`[["wrb.fr",null,"bad"]]`, ")]}'\n0\n"} {
		response := &http.Response{Body: io.NopCloser(strings.NewReader(input))}
		if _, err := readGeneration(response, model, nil); err == nil {
			t.Errorf("accepted malformed or empty stream %s", input)
		}
	}
}

func TestPayloadShape(t *testing.T) {
	model, _ := generation.Resolve(generation.Request{Model: "gemini-4-argon-low"})
	body, err := payload("hello", model, "session-token")
	if err != nil {
		t.Fatal(err)
	}
	values, _ := url.ParseQuery(body)
	var outer []json.RawMessage
	json.Unmarshal([]byte(values.Get("f.req")), &outer)
	var encoded string
	json.Unmarshal(outer[1], &encoded)
	var fields []json.RawMessage
	json.Unmarshal([]byte(encoded), &fields)
	if len(fields) != fieldCount || string(fields[modeIndex]) != "8" || string(fields[generation.ArgonThinking]) != "1" {
		t.Fatalf("incorrect payload fields")
	}
	if values.Get("at") != "session-token" || modelHeaders(model)[metadataHeader] == "" {
		t.Fatal("missing session token or Argon metadata")
	}
}

package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gemini-free/internal/generation"
)

type generatorFunc func(context.Context, generation.Request, func(generation.Event) error) (generation.Result, error)

func (f generatorFunc) Generate(ctx context.Context, input generation.Request, callback func(generation.Event) error) (generation.Result, error) {
	return f(ctx, input, callback)
}

func testHandler(generator generation.Generator) http.Handler {
	return New(Options{Generator: generator, Models: []generation.Model{{ID: "test"}},
		DefaultModel: "test"})
}

func post(handler http.Handler, path string, payload any) *httptest.ResponseRecorder {
	body := serialized(payload)
	if text, ok := payload.(string); ok {
		body = text
	}
	request := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	return response
}

func basicInput() object {
	return object{"model": "test", "messages": []object{{"role": "user", "content": "17+25"}}}
}

func assertStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status %d, want %d: %s", response.Code, status, response.Body.String())
	}
}

func decodeResponse(t *testing.T, response *httptest.ResponseRecorder) object {
	t.Helper()
	var value object
	if err := json.Unmarshal(response.Body.Bytes(), &value); err != nil {
		t.Fatal(err)
	}
	return value
}

func TestTextCompletions(t *testing.T) {
	generator := generatorFunc(func(ctx context.Context, input generation.Request, callback func(generation.Event) error) (generation.Result, error) {
		if input.Model != "test" || !strings.Contains(input.Prompt, "[User]\n17+25") {
			t.Fatalf("incorrect upstream request: %+v", input)
		}
		return generation.Result{Text: "42", RequestedModel: "test", ActualMode: 6}, nil
	})
	for _, path := range []string{"/v1/chat/completions", "/v1/messages"} {
		t.Run(path, func(t *testing.T) {
			response := post(testHandler(generator), path, basicInput())
			assertStatus(t, response, http.StatusOK)
			value := decodeResponse(t, response)
			if !strings.Contains(response.Body.String(), `"42"`) || value["model"] != "test" {
				t.Fatalf("incorrect response: %s", response.Body)
			}
			usage := value["usage"].(map[string]any)
			if usage["estimated"] != true {
				t.Fatal("token estimation not disclosed")
			}
			if response.Header().Get("X-Gemini-Actual-Mode") != "6" ||
				response.Header().Get("X-Gemini-Thinking-Level") != "0" {
				t.Fatal("actual route headers missing")
			}
		})
	}
}

func TestUnsupportedParameterDisclosure(t *testing.T) {
	input := basicInput()
	input["max_tokens"], input["temperature"] = 128, 0.2
	input["stop"] = []string{"END"}
	for _, path := range []string{"/v1/chat/completions", "/v1/messages"} {
		response := post(testHandler(fixedOutput("42")), path, input)
		assertStatus(t, response, http.StatusOK)
		header := response.Header().Get("X-Gemini-Unsupported-Parameters")
		assertContainsAll(t, header, []string{"max_tokens", "temperature", "stop"})
	}
}

func TestRoutingAndAuthentication(t *testing.T) {
	handler := New(Options{Models: []generation.Model{{ID: "test"}}, APIKeys: []string{"local-key"}})
	tests := []struct {
		method string
		path   string
		header string
		value  string
		status int
	}{
		{http.MethodGet, "/health", "", "", http.StatusOK},
		{http.MethodGet, "/v1/models", "", "", http.StatusUnauthorized},
		{http.MethodGet, "/v1/models", "Authorization", "Bearer local-key", http.StatusOK},
		{http.MethodGet, "/v1/models", "x-api-key", "local-key", http.StatusOK},
		{http.MethodGet, "/missing", "x-api-key", "local-key", http.StatusNotFound},
		{http.MethodPost, "/v1/models", "x-api-key", "local-key", http.StatusMethodNotAllowed},
	}
	for _, item := range tests {
		request := httptest.NewRequest(item.method, item.path, nil)
		request.Header.Set(item.header, item.value)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		assertStatus(t, response, item.status)
	}
}

func TestInvalidRequests(t *testing.T) {
	handler := testHandler(generatorFunc(func(context.Context, generation.Request, func(generation.Event) error) (generation.Result, error) {
		t.Fatal("invalid request reached generator")
		return generation.Result{}, nil
	}))
	inputs := []any{
		`{"messages":`,
		`{"messages":[]} {}`,
		object{"model": "unknown", "messages": []object{{"role": "user", "content": "hello"}}},
		object{"model": "test", "messages": []object{}},
		object{"model": "test", "messages": []object{{"role": "user", "content": []object{{"type": "image"}}}}},
		object{"model": "test", "messages": []object{{"role": "tool", "tool_call_id": "unknown", "content": "hello"}}},
	}
	for _, input := range inputs {
		for _, path := range []string{"/v1/chat/completions", "/v1/messages"} {
			response := post(handler, path, input)
			assertStatus(t, response, http.StatusBadRequest)
		}
	}
}

func TestDefaultModelAndTokenEstimate(t *testing.T) {
	input := basicInput()
	delete(input, "model")
	handler := New(Options{Models: []generation.Model{{ID: "first"}, {ID: "test"}}, DefaultModel: "test",
		Generator: generatorFunc(func(_ context.Context, input generation.Request, _ func(generation.Event) error) (generation.Result, error) {
			if input.Model != "test" {
				t.Fatalf("default model ignored: %s", input.Model)
			}
			return generation.Result{Text: "42"}, nil
		})})
	assertStatus(t, post(handler, "/v1/chat/completions", input), http.StatusOK)
	response := post(handler, "/v1/messages/count_tokens", input)
	assertStatus(t, response, http.StatusOK)
	if decodeResponse(t, response)["estimated"] != true {
		t.Fatal("token estimate was presented as exact")
	}
}

func TestUpstreamFailureAndCancellation(t *testing.T) {
	handler := testHandler(generatorFunc(func(ctx context.Context, _ generation.Request, _ func(generation.Event) error) (generation.Result, error) {
		if err := ctx.Err(); err != nil {
			return generation.Result{}, err
		}
		return generation.Result{}, errors.New("upstream route verification failed")
	}))
	for _, path := range []string{"/v1/chat/completions", "/v1/messages"} {
		response := post(handler, path, basicInput())
		assertStatus(t, response, http.StatusBadGateway)
		if !strings.Contains(response.Body.String(), "route verification failed") {
			t.Fatal("upstream error was hidden")
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	request := httptest.NewRequest(http.MethodPost, "/v1/messages", strings.NewReader(serialized(basicInput()))).WithContext(ctx)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if !strings.Contains(response.Body.String(), context.Canceled.Error()) {
		t.Fatal("request cancellation was not propagated")
	}
}

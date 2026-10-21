package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"gemini-free/internal/generation"
)

func TestIncrementalTextStreams(t *testing.T) {
	generator := generatorFunc(func(ctx context.Context, input generation.Request, callback func(generation.Event) error) (generation.Result, error) {
		if callback == nil {
			t.Fatal("stream callback missing")
		}
		for _, delta := range []string{"4", "2"} {
			if err := callback(generation.Event{Text: delta}); err != nil {
				return generation.Result{}, err
			}
		}
		return generation.Result{Text: "42", ActualMode: 6, ThinkingLevel: 4}, nil
	})
	for _, path := range []string{"/v1/chat/completions", "/v1/messages"} {
		input := basicInput()
		input["stream"] = true
		response := post(testHandler(generator), path, input)
		assertStatus(t, response, http.StatusOK)
		if !strings.HasPrefix(response.Header().Get("Content-Type"), "text/event-stream") {
			t.Fatalf("incorrect content type: %s", response.Header().Get("Content-Type"))
		}
		body := response.Body.String()
		assertContainsAll(t, body, []string{`"actual_mode":6`, `"thinking_level":4`})
		if path == "/v1/messages" {
			assertContainsAll(t, body, []string{`"text":"4"`, `"text":"2"`, "event: message_stop", `"stop_reason":"end_turn"`})
		} else {
			assertContainsAll(t, body, []string{`"content":"4"`, `"content":"2"`, "data: [DONE]", `"finish_reason":"stop"`})
		}
	}
}

func assertContainsAll(t *testing.T, text string, fragments []string) {
	t.Helper()
	for _, fragment := range fragments {
		if !strings.Contains(text, fragment) {
			t.Fatalf("missing %q in %s", fragment, text)
		}
	}
}

func TestStreamingToolIndexesAndJSON(t *testing.T) {
	output := callBlock("read_file", object{"path": "/tmp/a"}) + "\n" + callBlock("search", object{"query": "x"})
	for _, anthropic := range []bool{false, true} {
		path := "/v1/chat/completions"
		if anthropic {
			path = "/v1/messages"
		}
		input := toolInput(anthropic)
		input["stream"] = true
		response := post(testHandler(fixedOutput(output)), path, input)
		assertStatus(t, response, http.StatusOK)
		body := response.Body.String()
		if anthropic {
			assertContainsAll(t, body, []string{"event: message_start", `"index":0`, `"index":1`,
				`"type":"input_json_delta"`, `"stop_reason":"tool_use"`, "event: message_stop"})
		} else {
			assertContainsAll(t, body, []string{`"index":0`, `"index":1`, `"name":"read_file"`,
				`"name":"search"`, `"finish_reason":"tool_calls"`, "data: [DONE]"})
		}
		validateSSEJSON(t, body)
	}
}

func validateSSEJSON(t *testing.T, text string) {
	t.Helper()
	for _, line := range strings.Split(text, "\n") {
		if !strings.HasPrefix(line, "data: ") || line == "data: [DONE]" {
			continue
		}
		if !json.Valid([]byte(strings.TrimPrefix(line, "data: "))) {
			t.Fatalf("invalid SSE JSON: %s", line)
		}
	}
}

func TestStreamingErrors(t *testing.T) {
	generator := generatorFunc(func(ctx context.Context, input generation.Request, callback func(generation.Event) error) (generation.Result, error) {
		if err := callback(generation.Event{Text: "partial"}); err != nil {
			return generation.Result{}, err
		}
		return generation.Result{}, errors.New("upstream connection reset")
	})
	for _, path := range []string{"/v1/chat/completions", "/v1/messages"} {
		input := basicInput()
		input["stream"] = true
		response := post(testHandler(generator), path, input)
		body := response.Body.String()
		assertContainsAll(t, body, []string{"partial", "upstream connection reset", `"type":"upstream_error"`})
		if strings.Contains(body, `"finish_reason":"stop"`) || strings.Contains(body, "event: message_stop") {
			t.Fatalf("failed stream emitted success: %s", body)
		}
	}
}

type failingStreamWriter struct {
	header http.Header
}

func (w *failingStreamWriter) Header() http.Header {
	return w.header
}

func (w *failingStreamWriter) WriteHeader(int) {}

func (w *failingStreamWriter) Write([]byte) (int, error) {
	return 0, fmt.Errorf("client write failure")
}

func (w *failingStreamWriter) Flush() {}

func TestStreamWriteFailureStopsBeforeGeneration(t *testing.T) {
	handler := testHandler(generatorFunc(func(context.Context, generation.Request, func(generation.Event) error) (generation.Result, error) {
		t.Fatal("generation started after client write failed")
		return generation.Result{}, nil
	}))
	input := basicInput()
	input["stream"] = true
	for _, path := range []string{"/v1/chat/completions", "/v1/messages"} {
		request, err := http.NewRequest(http.MethodPost, path, strings.NewReader(serialized(input)))
		if err != nil {
			t.Fatal(err)
		}
		handler.ServeHTTP(&failingStreamWriter{header: make(http.Header)}, request)
	}
}

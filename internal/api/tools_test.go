package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"gemini-free/internal/generation"
)

func toolInput(anthropic bool) object {
	input := basicInput()
	if anthropic {
		input["tools"] = []object{{"name": "read_file", "input_schema": object{"type": "object"}},
			{"name": "search", "input_schema": object{"type": "object"}}}
		input["tool_choice"] = object{"type": "any"}
		return input
	}
	input["tools"] = []object{{"type": "function", "function": object{"name": "read_file", "parameters": object{"type": "object"}}},
		{"type": "function", "function": object{"name": "search", "parameters": object{"type": "object"}}}}
	input["tool_choice"] = "required"
	return input
}

func fixedOutput(output string) generation.Generator {
	return generatorFunc(func(context.Context, generation.Request, func(generation.Event) error) (generation.Result, error) {
		return generation.Result{Text: output}, nil
	})
}

func TestMultipleToolCalls(t *testing.T) {
	output := callBlock("read_file", object{"path": "/tmp/a"}) + "\n" + callBlock("search", object{"query": "answer"})
	for _, anthropic := range []bool{false, true} {
		path := "/v1/chat/completions"
		if anthropic {
			path = "/v1/messages"
		}
		response := post(testHandler(fixedOutput(output)), path, toolInput(anthropic))
		assertStatus(t, response, http.StatusOK)
		value := decodeResponse(t, response)
		if anthropic {
			blocks := value["content"].([]any)
			if len(blocks) != 2 || value["stop_reason"] != "tool_use" {
				t.Fatalf("incorrect tool response: %s", response.Body)
			}
		} else {
			choice := value["choices"].([]any)[0].(map[string]any)
			calls := choice["message"].(map[string]any)["tool_calls"].([]any)
			if len(calls) != 2 || choice["finish_reason"] != "tool_calls" {
				t.Fatalf("incorrect tool response: %s", response.Body)
			}
		}
	}
}

func TestToolProtocolFailures(t *testing.T) {
	outputs := []string{
		"no tool call",
		"```tool_call\n{\"name\":\"read_file\"}",
		"```tool_call\n{\"name\":\"read_file\",\"arguments\":[]}\n```",
		"```tool_call\n{\"name\":\"unknown\",\"arguments\":{}}\n```",
		"```tool_call\n{\"name\":\"read_file\",\"arguments\":{}} {}\n```",
	}
	for _, output := range outputs {
		for _, anthropic := range []bool{false, true} {
			path := "/v1/chat/completions"
			if anthropic {
				path = "/v1/messages"
			}
			assertStatus(t, post(testHandler(fixedOutput(output)), path, toolInput(anthropic)), http.StatusBadGateway)
		}
	}
}

func TestInvalidToolChoices(t *testing.T) {
	choices := []any{"invalid", "required", object{"type": "function", "function": object{"name": "missing"}}}
	for _, choice := range choices {
		input := basicInput()
		input["tool_choice"] = choice
		response := post(testHandler(fixedOutput("not reached")), "/v1/chat/completions", input)
		assertStatus(t, response, http.StatusBadRequest)
	}
	input := toolInput(true)
	input["tool_choice"] = object{"type": "tool", "name": "undeclared"}
	assertStatus(t, post(testHandler(fixedOutput("")), "/v1/messages", input), http.StatusBadRequest)
}

func TestNamedAndParallelToolPolicies(t *testing.T) {
	input := toolInput(false)
	input["tool_choice"] = object{"type": "function", "function": object{"name": "read_file"}}
	wrong := post(testHandler(fixedOutput(callBlock("search", object{}))), "/v1/chat/completions", input)
	assertStatus(t, wrong, http.StatusBadGateway)
	input["tool_choice"] = "required"
	input["parallel_tool_calls"] = false
	two := callBlock("search", object{}) + callBlock("read_file", object{})
	assertStatus(t, post(testHandler(fixedOutput(two)), "/v1/chat/completions", input), http.StatusBadGateway)
}

func TestToolHistoryPrompt(t *testing.T) {
	tests := []struct {
		path     string
		messages []object
	}{
		{"/v1/chat/completions", []object{
			{"role": "assistant", "content": nil, "tool_calls": []object{{"id": "call_one", "type": "function",
				"function": object{"name": "read_file", "arguments": `{"path":"/tmp/a"}`}}}},
			{"role": "tool", "tool_call_id": "call_one", "content": "file content"},
		}},
		{"/v1/messages", []object{
			{"role": "assistant", "content": []object{{"type": "tool_use", "id": "call_one", "name": "read_file", "input": object{"path": "/tmp/a"}}}},
			{"role": "user", "content": []object{{"type": "tool_result", "tool_use_id": "call_one", "content": "file content", "is_error": true}}},
		}},
	}
	for _, item := range tests {
		input := basicInput()
		input["messages"] = item.messages
		handler := testHandler(generatorFunc(func(_ context.Context, input generation.Request, _ func(generation.Event) error) (generation.Result, error) {
			if !strings.Contains(input.Prompt, "Tool result for read_file; id=call_one") || !strings.Contains(input.Prompt, "file content") {
				t.Fatalf("tool history lost: %s", input.Prompt)
			}
			return generation.Result{Text: "read"}, nil
		}))
		assertStatus(t, post(handler, item.path, input), http.StatusOK)
	}
}

func TestToolDefinitionValidation(t *testing.T) {
	raw := []string{
		`{"type":"function","function":{"name":"x","parameters":[]}}`,
		`{"type":"web_search","name":"x","parameters":{}}`,
		`{"type":"function","function":{"name":"","parameters":{}}}`,
	}
	for _, value := range raw {
		input := basicInput()
		input["tools"] = []json.RawMessage{json.RawMessage(value)}
		assertStatus(t, post(testHandler(fixedOutput("")), "/v1/chat/completions", input), http.StatusBadRequest)
	}
}

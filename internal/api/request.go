package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gemini-free/internal/generation"
)

type request struct {
	Model             string            `json:"model"`
	Messages          []message         `json:"messages"`
	System            json.RawMessage   `json:"system"`
	Tools             []json.RawMessage `json:"tools"`
	ToolChoice        json.RawMessage   `json:"tool_choice"`
	Stream            bool              `json:"stream"`
	ReasoningEffort   string            `json:"reasoning_effort"`
	ParallelToolCalls *bool             `json:"parallel_tool_calls"`
	unsupported       []string
}

type message struct {
	Role       string          `json:"role"`
	Content    json.RawMessage `json:"content"`
	Name       string          `json:"name"`
	ToolCallID string          `json:"tool_call_id"`
	ToolCalls  []historyCall   `json:"tool_calls"`
}

type historyCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	} `json:"function"`
}

type chatContext struct {
	generation  generation.Request
	policy      toolPolicy
	id          string
	stream      bool
	unsupported []string
}

func (s *server) context(input request, anthropic bool) (chatContext, error) {
	model, err := s.resolveModel(input.Model)
	if err != nil {
		return chatContext{}, err
	}
	if input.ReasoningEffort != "" {
		if _, err := generation.Resolve(generation.Request{Model: model, ReasoningEffort: input.ReasoningEffort}); err != nil {
			return chatContext{}, err
		}
	}
	policy, err := buildPolicy(input, anthropic)
	if err != nil {
		return chatContext{}, err
	}
	prompt, err := buildPrompt(input, policy, anthropic)
	if err != nil {
		return chatContext{}, err
	}
	if strings.TrimSpace(prompt) == "" {
		return chatContext{}, fmt.Errorf("empty prompt")
	}
	prefix := "chatcmpl-"
	if anthropic {
		prefix = "msg_"
	}
	return chatContext{generation: generation.Request{Model: model, Prompt: prompt,
		ReasoningEffort: input.ReasoningEffort}, policy: policy,
		id: newID(prefix), stream: input.Stream, unsupported: append([]string(nil), input.unsupported...)}, nil
}

func buildPrompt(input request, policy toolPolicy, anthropic bool) (string, error) {
	if len(input.Messages) == 0 {
		return "", fmt.Errorf("messages must be a non-empty array")
	}
	if anthropic {
		return anthropicPrompt(input, policy)
	}
	return openAIPrompt(input, policy)
}

func jsonObject(raw json.RawMessage) (object, error) {
	var value object
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&value); err != nil {
		return nil, err
	}
	if value == nil {
		return nil, fmt.Errorf("expected JSON object")
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return nil, fmt.Errorf("expected exactly one JSON object")
	}
	return value, nil
}

func serialized(value any) string {
	data, _ := json.Marshal(value)
	return string(data)
}

func decodeArguments(raw json.RawMessage) (object, error) {
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return jsonObject(json.RawMessage(text))
	}
	return jsonObject(raw)
}

func estimatedTokens(text string) int {
	const approximateBytesPerToken = 4
	count := len(text) / approximateBytesPerToken
	if count == 0 {
		return 1
	}
	return count
}

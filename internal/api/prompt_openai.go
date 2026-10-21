package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

func openAIPrompt(input request, policy toolPolicy) (string, error) {
	parts := []string{policy.instruction()}
	state := conversation{toolNames: make(map[string]string)}
	for _, item := range input.Messages {
		part, err := state.openAIMessage(item)
		if err != nil {
			return "", err
		}
		if part != "" {
			parts = append(parts, part)
		}
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n")), nil
}

func (c *conversation) openAIMessage(item message) (string, error) {
	content, err := textContent(item.Content)
	if err != nil {
		return "", err
	}
	switch item.Role {
	case "system", "developer":
		return "[System instruction]\n" + content, nil
	case "assistant":
		return c.assistantHistory(item, content)
	case "tool":
		if item.ToolCallID == "" {
			return "", fmt.Errorf("tool messages require tool_call_id")
		}
		name := c.toolNames[item.ToolCallID]
		if name == "" {
			return "", fmt.Errorf("tool result references unknown tool_call_id %q", item.ToolCallID)
		}
		return "[Tool result for " + name + "; id=" + item.ToolCallID + "]\n" + content, nil
	case "user":
		if content == "" {
			return "", fmt.Errorf("user content must not be empty")
		}
		return "[User]\n" + content, nil
	default:
		return "", fmt.Errorf("unsupported message role %q", item.Role)
	}
}

func (c *conversation) assistantHistory(item message, content string) (string, error) {
	parts := []string{"[Assistant]\n" + content}
	for _, call := range item.ToolCalls {
		if call.ID == "" || call.Type != "function" || call.Function.Name == "" {
			return "", fmt.Errorf("assistant tool_calls require id, type=function, and function.name")
		}
		arguments, err := decodeArguments(call.Function.Arguments)
		if err != nil {
			return "", fmt.Errorf("invalid arguments for tool %q: %w", call.Function.Name, err)
		}
		if _, exists := c.toolNames[call.ID]; exists {
			return "", fmt.Errorf("duplicate tool call id %q", call.ID)
		}
		c.toolNames[call.ID] = call.Function.Name
		parts = append(parts, callBlock(call.Function.Name, arguments))
	}
	return strings.Join(parts, "\n"), nil
}

func textContent(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	var blocks []object
	if json.Unmarshal(raw, &blocks) != nil {
		return "", fmt.Errorf("content must be a string or text block array")
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		if block["type"] != "text" && block["type"] != "input_text" {
			return "", fmt.Errorf("unsupported content block type %q", block["type"])
		}
		value, ok := block["text"].(string)
		if !ok {
			return "", fmt.Errorf("text content block requires text")
		}
		parts = append(parts, value)
	}
	return strings.Join(parts, "\n"), nil
}

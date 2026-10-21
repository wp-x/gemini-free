package api

import (
	"encoding/json"
	"fmt"
	"strings"
)

type conversation struct {
	toolNames map[string]string
}

func anthropicPrompt(input request, policy toolPolicy) (string, error) {
	system, err := systemText(input.System)
	if err != nil {
		return "", err
	}
	parts := []string{policy.instruction()}
	if system != "" {
		parts = append(parts, "# System Instructions\n\n"+system)
	}
	parts = append(parts, "# Conversation")
	state := conversation{toolNames: make(map[string]string)}
	for _, item := range input.Messages {
		part, err := state.anthropicMessage(item)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return strings.TrimSpace(strings.Join(parts, "\n\n")), nil
}

func systemText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}
	var text string
	if json.Unmarshal(raw, &text) == nil {
		return text, nil
	}
	return textBlocks(raw)
}

func textBlocks(raw json.RawMessage) (string, error) {
	var blocks []object
	if json.Unmarshal(raw, &blocks) != nil || blocks == nil {
		return "", fmt.Errorf("content must be a string or text block array")
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		text, ok := block["text"].(string)
		if block["type"] != "text" || !ok {
			return "", fmt.Errorf("only text blocks are supported here")
		}
		parts = append(parts, text)
	}
	return strings.Join(parts, "\n"), nil
}

func (c *conversation) anthropicMessage(item message) (string, error) {
	if item.Role != "user" && item.Role != "assistant" {
		return "", fmt.Errorf("message role must be user or assistant")
	}
	var text string
	if json.Unmarshal(item.Content, &text) == nil {
		return roleLabel(item.Role) + text, nil
	}
	var blocks []object
	if json.Unmarshal(item.Content, &blocks) != nil || blocks == nil {
		return "", fmt.Errorf("message content must be a string or content block array")
	}
	parts := make([]string, 0, len(blocks))
	for _, block := range blocks {
		part, err := c.anthropicBlock(block, item.Role)
		if err != nil {
			return "", err
		}
		parts = append(parts, part)
	}
	return roleLabel(item.Role) + strings.Join(parts, "\n"), nil
}

func roleLabel(role string) string {
	if role == "assistant" {
		return "[Assistant]\n"
	}
	return "[User]\n"
}

func (c *conversation) anthropicBlock(block object, role string) (string, error) {
	switch block["type"] {
	case "text":
		text, ok := block["text"].(string)
		if !ok {
			return "", fmt.Errorf("text block requires text")
		}
		return text, nil
	case "tool_use":
		if role != "assistant" {
			return "", fmt.Errorf("tool_use blocks must be in assistant messages")
		}
		return c.toolUse(block)
	case "tool_result":
		if role != "user" {
			return "", fmt.Errorf("tool_result blocks must be in user messages")
		}
		return c.toolResult(block)
	case "thinking":
		thinking, ok := block["thinking"].(string)
		if role != "assistant" || !ok {
			return "", fmt.Errorf("thinking blocks require assistant role and thinking text")
		}
		return "[Assistant reasoning]\n" + thinking, nil
	case "redacted_thinking":
		if role != "assistant" {
			return "", fmt.Errorf("redacted_thinking blocks require assistant role")
		}
		return "[Assistant reasoning was redacted]", nil
	default:
		return "", fmt.Errorf("unsupported content block type %q", block["type"])
	}
}

func (c *conversation) toolUse(block object) (string, error) {
	id, _ := block["id"].(string)
	name, _ := block["name"].(string)
	input, ok := block["input"].(map[string]any)
	if id == "" || name == "" || !ok {
		return "", fmt.Errorf("tool_use requires id, name, and input object")
	}
	if _, exists := c.toolNames[id]; exists {
		return "", fmt.Errorf("duplicate tool_use id %q", id)
	}
	c.toolNames[id] = name
	return callBlock(name, input), nil
}

func (c *conversation) toolResult(block object) (string, error) {
	id, _ := block["tool_use_id"].(string)
	name, exists := c.toolNames[id]
	if id == "" || !exists {
		return "", fmt.Errorf("tool_result references unknown tool_use_id %q", id)
	}
	status := "success"
	if block["is_error"] == true {
		status = "error"
	}
	content := ""
	if raw, exists := block["content"]; exists {
		encoded, err := json.Marshal(raw)
		if err != nil {
			return "", err
		}
		if json.Unmarshal(encoded, &content) != nil {
			content, err = textBlocks(encoded)
			if err != nil {
				return "", err
			}
		}
	}
	return "[Tool result for " + name + "; id=" + id + "; status=" + status + "]\n" + content, nil
}

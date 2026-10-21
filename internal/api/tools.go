package api

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

type toolDefinition struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Parameters  object `json:"parameters"`
}

type toolPolicy struct {
	tools        []toolDefinition
	active       bool
	required     bool
	requiredName string
	parallel     bool
}

type parsedCall struct {
	name      string
	arguments object
}

type parsedOutput struct {
	text  string
	calls []parsedCall
}

var toolBlocks = regexp.MustCompile("(?s)```(?:tool_call|function_call)\\s*(.*?)```")

func buildPolicy(input request, anthropic bool) (toolPolicy, error) {
	policy := toolPolicy{parallel: input.ParallelToolCalls == nil || *input.ParallelToolCalls}
	seen := make(map[string]bool)
	for _, raw := range input.Tools {
		tool, err := decodeTool(raw, anthropic)
		if err != nil {
			return policy, err
		}
		if seen[tool.Name] {
			return policy, fmt.Errorf("duplicate tool name %q", tool.Name)
		}
		seen[tool.Name] = true
		policy.tools = append(policy.tools, tool)
	}
	if err := policy.setChoice(input.ToolChoice, anthropic); err != nil {
		return policy, err
	}
	if policy.required && len(policy.tools) == 0 {
		return policy, fmt.Errorf("tool_choice requires at least one declared tool")
	}
	if policy.requiredName != "" && !seen[policy.requiredName] {
		return policy, fmt.Errorf("tool_choice names an undeclared tool: %s", policy.requiredName)
	}
	return policy, nil
}

func decodeTool(raw json.RawMessage, anthropic bool) (toolDefinition, error) {
	value, err := jsonObject(raw)
	if err != nil {
		return toolDefinition{}, fmt.Errorf("invalid tool: %w", err)
	}
	schemaKey := "input_schema"
	if !anthropic {
		if value["type"] != "function" {
			return toolDefinition{}, fmt.Errorf("only function tools are supported")
		}
		value, _ = value["function"].(map[string]any)
		schemaKey = "parameters"
	}
	name, _ := value["name"].(string)
	if strings.TrimSpace(name) == "" {
		return toolDefinition{}, fmt.Errorf("each tool requires a non-empty name")
	}
	schema, ok := value[schemaKey].(map[string]any)
	if !anthropic && value[schemaKey] == nil {
		schema, ok = object{}, true
	}
	if !ok {
		return toolDefinition{}, fmt.Errorf("tool %q requires a %s object", name, schemaKey)
	}
	description, _ := value["description"].(string)
	return toolDefinition{Name: name, Description: description, Parameters: schema}, nil
}

func (p *toolPolicy) setChoice(raw json.RawMessage, anthropic bool) error {
	choice := "auto"
	if len(raw) > 0 {
		if anthropic || json.Unmarshal(raw, &choice) != nil {
			value, err := jsonObject(raw)
			if err != nil {
				return fmt.Errorf("invalid tool_choice")
			}
			return p.setObjectChoice(value, anthropic)
		}
	}
	switch choice {
	case "none":
		p.active = false
	case "auto":
		p.active = len(p.tools) > 0
	case "required":
		p.active, p.required = true, true
	default:
		return fmt.Errorf("unsupported tool_choice %q", choice)
	}
	return nil
}

func (p *toolPolicy) setObjectChoice(value object, anthropic bool) error {
	choice, _ := value["type"].(string)
	if !anthropic {
		function, _ := value["function"].(map[string]any)
		name, _ := function["name"].(string)
		if choice != "function" || name == "" {
			return fmt.Errorf("tool_choice requires type=function and function.name")
		}
		p.active, p.required, p.requiredName = true, true, name
		return nil
	}
	if disabled, ok := value["disable_parallel_tool_use"].(bool); ok {
		p.parallel = !disabled
	}
	switch choice {
	case "auto":
		p.active = len(p.tools) > 0
	case "none":
		p.active = false
	case "any":
		p.active, p.required = true, true
	case "tool":
		name, _ := value["name"].(string)
		if name == "" {
			return fmt.Errorf("tool_choice type=tool requires name")
		}
		p.active, p.required, p.requiredName = true, true, name
	default:
		return fmt.Errorf("unsupported tool_choice type %q", choice)
	}
	return nil
}

func (p toolPolicy) parse(text string) (parsedOutput, error) {
	if !p.active {
		return parsedOutput{text: text}, nil
	}
	output := parsedOutput{}
	cursor := 0
	var clean strings.Builder
	for _, indexes := range toolBlocks.FindAllStringSubmatchIndex(text, -1) {
		clean.WriteString(text[cursor:indexes[0]])
		cursor = indexes[1]
		call, err := p.parseCall(text[indexes[2]:indexes[3]])
		if err != nil {
			return output, err
		}
		output.calls = append(output.calls, call)
	}
	clean.WriteString(text[cursor:])
	output.text = strings.TrimSpace(clean.String())
	if strings.Contains(output.text, "```tool_call") || strings.Contains(output.text, "```function_call") {
		return output, fmt.Errorf("malformed tool call block")
	}
	if p.required && len(output.calls) == 0 {
		return output, fmt.Errorf("required tool call missing")
	}
	if !p.parallel && len(output.calls) > 1 {
		return output, fmt.Errorf("parallel tool calls were disabled")
	}
	return output, nil
}

func (p toolPolicy) parseCall(raw string) (parsedCall, error) {
	value, err := jsonObject(json.RawMessage(raw))
	if err != nil {
		return parsedCall{}, fmt.Errorf("malformed tool call: %w", err)
	}
	name, _ := value["name"].(string)
	if !p.declared(name) {
		return parsedCall{}, fmt.Errorf("upstream called undeclared tool %q", name)
	}
	if p.requiredName != "" && p.requiredName != name {
		return parsedCall{}, fmt.Errorf("required tool %q, received %q", p.requiredName, name)
	}
	arguments := object{}
	for _, key := range []string{"arguments", "args", "input"} {
		if value[key] == nil {
			continue
		}
		var ok bool
		arguments, ok = value[key].(map[string]any)
		if !ok {
			return parsedCall{}, fmt.Errorf("tool %q arguments must be an object", name)
		}
		break
	}
	return parsedCall{name: name, arguments: arguments}, nil
}

func (p toolPolicy) declared(name string) bool {
	for _, tool := range p.tools {
		if tool.Name == name {
			return true
		}
	}
	return false
}

func (p toolPolicy) instruction() string {
	if !p.active {
		return ""
	}
	requirement := ""
	if p.required {
		requirement = "\nYou MUST call at least one tool."
	}
	if p.requiredName != "" {
		requirement = "\nYou MUST call only the tool " + serialized(p.requiredName) + "."
	}
	if !p.parallel {
		requirement += "\nCall no more than one tool."
	}
	return "# Local Tool Protocol\n\nThe tools below are real and are executed by the client in the user's environment. " +
		"When a task needs a listed capability, call the appropriate tool instead of inventing a result.\n" +
		"Use exactly this format:\n```tool_call\n{\"name\":\"tool_name\",\"arguments\":{}}\n```\n" +
		"Output only one or more tool_call blocks when calling tools. Arguments must be one JSON object. " +
		"After a tool result arrives, continue from that result.\nAvailable tools:\n" + serialized(p.tools) + requirement
}

func callBlock(name string, arguments object) string {
	return "```tool_call\n" + serialized(object{"name": name, "arguments": arguments}) + "\n```"
}

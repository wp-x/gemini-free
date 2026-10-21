package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"gemini-free/internal/generation"
)

const (
	responseTextIndex     = 4
	responseModeIndex     = 58
	responseThinkingIndex = 59
)

type frame struct {
	text     string
	mode     int
	thinking int
}

func parseLine(line string) ([]frame, error) {
	if !strings.Contains(line, `"wrb.fr"`) {
		return nil, nil
	}
	var envelopes [][]json.RawMessage
	if err := json.Unmarshal([]byte(line), &envelopes); err != nil {
		return nil, fmt.Errorf("malformed upstream envelope: %w", err)
	}
	var frames []frame
	for _, envelope := range envelopes {
		found, err := parseEnvelope(envelope)
		if err != nil {
			return nil, err
		}
		frames = append(frames, found...)
	}
	return frames, nil
}

func parseEnvelope(envelope []json.RawMessage) ([]frame, error) {
	if len(envelope) < 3 || string(envelope[0]) != `"wrb.fr"` || string(envelope[2]) == "null" {
		return nil, nil
	}
	var body string
	if err := json.Unmarshal(envelope[2], &body); err != nil {
		return nil, fmt.Errorf("malformed upstream body: %w", err)
	}
	var inner []json.RawMessage
	if err := json.Unmarshal([]byte(body), &inner); err != nil {
		return nil, fmt.Errorf("malformed upstream body JSON: %w", err)
	}
	return extractFrames(inner)
}

func extractFrames(inner []json.RawMessage) ([]frame, error) {
	if len(inner) <= responseTextIndex {
		return nil, nil
	}
	var parts [][]json.RawMessage
	if string(inner[responseTextIndex]) != "null" {
		if err := json.Unmarshal(inner[responseTextIndex], &parts); err != nil {
			return nil, fmt.Errorf("malformed upstream candidates: %w", err)
		}
	}
	var frames []frame
	for _, part := range parts {
		if len(part) < 2 {
			continue
		}
		var texts []string
		if err := json.Unmarshal(part[1], &texts); err != nil {
			return nil, fmt.Errorf("malformed upstream text: %w", err)
		}
		if len(texts) != 0 && texts[0] != "" {
			frames = append(frames, frame{texts[0], intAt(inner, responseModeIndex), intAt(inner, responseThinkingIndex)})
		}
	}
	return frames, nil
}

func intAt(values []json.RawMessage, index int) int {
	if len(values) <= index || string(values[index]) == "null" {
		return 0
	}
	var value int
	if err := json.Unmarshal(values[index], &value); err != nil {
		return 0
	}
	return value
}

func validateRoute(model generation.Model, f frame) error {
	if model.Account && f.mode != model.Mode {
		return fmt.Errorf("model routing failed: requested mode %d, received %d", model.Mode, f.mode)
	}
	if model.Mode == generation.ArgonMode && f.thinking != model.Extra[generation.ArgonThinking] {
		return fmt.Errorf("Argon thinking mismatch: requested %d, received %d",
			model.Extra[generation.ArgonThinking], f.thinking)
	}
	return nil
}

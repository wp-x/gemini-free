package gemini

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"gemini-free/internal/generation"
)

func TestArgonLevelsMatchPayloadHeaderAndResponse(t *testing.T) {
	for _, level := range []string{"low", "medium", "high"} {
		t.Run(level, func(t *testing.T) {
			model, err := generation.Resolve(generation.Request{Model: "gemini-4-argon-" + level})
			if err != nil {
				t.Fatal(err)
			}
			encoded, err := payload("8+5", model, "matching-token")
			if err != nil {
				t.Fatal(err)
			}
			form, _ := url.ParseQuery(encoded)
			var outer []json.RawMessage
			if err := json.Unmarshal([]byte(form.Get("f.req")), &outer); err != nil {
				t.Fatal(err)
			}
			var inner string
			if err := json.Unmarshal(outer[1], &inner); err != nil {
				t.Fatal(err)
			}
			var fields, header []json.RawMessage
			if err := json.Unmarshal([]byte(inner), &fields); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(modelHeaders(model)[metadataHeader]), &header); err != nil {
				t.Fatal(err)
			}
			const headerModeIndex, headerThinkingIndex = 14, 15
			if intAt(fields, modeIndex) != generation.ArgonMode || intAt(header, headerModeIndex) != generation.ArgonMode {
				t.Fatal("Argon route missing from body or header")
			}
			want := model.Extra[generation.ArgonThinking]
			if intAt(fields, generation.ArgonThinking) != want || intAt(header, headerThinkingIndex) != want {
				t.Fatal("body and header requested different thinking levels")
			}
			response := &http.Response{Body: io.NopCloser(strings.NewReader(wireFrame("13", generation.ArgonMode, want)))}
			result, err := readGeneration(response, model, nil)
			if err != nil || result.ThinkingLevel != want || result.Text != "13" {
				t.Fatalf("matching response failed: %v", err)
			}
		})
	}
}

func TestArgonDowngradeNeverEmitsSuccessfulText(t *testing.T) {
	model, _ := generation.Resolve(generation.Request{Model: "gemini-4-argon-high"})
	response := &http.Response{Body: io.NopCloser(strings.NewReader(wireFrame("13", generation.ArgonMode, generation.ThinkingLow)))}
	emissions := 0
	result, err := readGeneration(response, model, func(generation.Event) error { emissions++; return nil })
	var mismatch *RoutingError
	if !errors.As(err, &mismatch) || mismatch.RequestedThinking != generation.ThinkingHigh || mismatch.ActualThinking != generation.ThinkingLow {
		t.Fatalf("lost actual downgrade evidence: %v", err)
	}
	if emissions != 0 || result.Text != "" {
		t.Fatal("downgraded response escaped validation")
	}
	if !strings.Contains(err.Error(), "更新 Cookie") || !strings.Contains(err.Error(), "请求 高（2），实际 低（1）") {
		t.Fatal("error lacks route evidence or recovery guidance")
	}
}

package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"gemini-free/internal/generation"
)

func (r *request) UnmarshalJSON(data []byte) error {
	type plain request
	var decoded plain
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	if fields == nil {
		return fmt.Errorf("request must be a JSON object")
	}
	decoded.unsupported = unsupportedParameters(fields)
	*r = request(decoded)
	return nil
}

func unsupportedParameters(fields map[string]json.RawMessage) []string {
	names := []string{"max_tokens", "max_completion_tokens", "temperature", "top_p", "top_k",
		"stop", "stop_sequences", "frequency_penalty", "presence_penalty", "logit_bias",
		"logprobs", "top_logprobs", "seed", "response_format", "n", "thinking",
		"output_config", "service_tier", "stream_options", "prediction", "modalities",
		"audio", "verbosity"}
	result := make([]string, 0)
	for _, name := range names {
		if _, exists := fields[name]; exists {
			result = append(result, name)
		}
	}
	return result
}

func routingHeaders(w http.ResponseWriter, result generation.Result) {
	w.Header().Set("X-Gemini-Actual-Mode", strconv.Itoa(result.ActualMode))
	w.Header().Set("X-Gemini-Thinking-Level", strconv.Itoa(result.ThinkingLevel))
}

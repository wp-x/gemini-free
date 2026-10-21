package api

import (
	"net/http"

	"gemini-free/internal/generation"
)

func (s *server) anthropic(w http.ResponseWriter, r *http.Request) {
	chat, ok := s.prepare(w, r, true)
	if !ok {
		return
	}
	if chat.stream && !chat.policy.active {
		s.streamAnthropicText(w, r, chat)
		return
	}
	result, err := s.options.Generator.Generate(r.Context(), chat.generation, nil)
	if err != nil {
		upstreamError(w, r, err)
		return
	}
	routingHeaders(w, result)
	output, err := chat.policy.parse(result.Text)
	if err != nil {
		upstreamError(w, r, err)
		return
	}
	response := anthropicResponse(chat, output)
	response["usage"] = object{"input_tokens": estimatedTokens(chat.generation.Prompt),
		"output_tokens": estimatedTokens(result.Text), "estimated": true}
	response["gemini"] = resultMetadata(result)
	if chat.stream {
		sendAnthropicOutput(startStream(w, r), response)
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *server) countTokens(w http.ResponseWriter, r *http.Request) {
	input, err := decodeRequest(r)
	if err != nil {
		requestError(w, r, err)
		return
	}
	chat, err := s.context(input, true)
	if err != nil {
		requestError(w, r, err)
		return
	}
	w.Header().Set("X-Gemini-Token-Usage", "estimated; utf8-bytes/4")
	writeJSON(w, http.StatusOK, object{"input_tokens": estimatedTokens(chat.generation.Prompt),
		"estimated": true, "estimation_method": "utf8_bytes_div_4"})
}

func anthropicResponse(chat chatContext, output parsedOutput) object {
	blocks := make([]object, 0, len(output.calls)+1)
	if output.text != "" {
		blocks = append(blocks, object{"type": "text", "text": output.text})
	}
	for _, call := range output.calls {
		blocks = append(blocks, object{"type": "tool_use", "id": newID("toolu_"),
			"name": call.name, "input": call.arguments})
	}
	if len(blocks) == 0 {
		blocks = append(blocks, object{"type": "text", "text": ""})
	}
	reason := "end_turn"
	if len(output.calls) > 0 {
		reason = "tool_use"
	}
	return object{"id": chat.id, "type": "message", "role": "assistant",
		"model": chat.generation.Model, "content": blocks,
		"stop_reason": reason, "stop_sequence": nil}
}

func (s *server) streamAnthropicText(w http.ResponseWriter, r *http.Request, chat chatContext) {
	stream := startStream(w, r)
	if err := startAnthropicMessage(stream, chat); err != nil {
		return
	}
	start := object{"type": "content_block_start", "index": 0,
		"content_block": object{"type": "text", "text": ""}}
	if err := stream.send("content_block_start", start); err != nil {
		return
	}
	result, err := s.options.Generator.Generate(r.Context(), chat.generation, func(event generation.Event) error {
		if event.Text == "" {
			return r.Context().Err()
		}
		return stream.send("content_block_delta", object{"type": "content_block_delta",
			"index": 0, "delta": object{"type": "text_delta", "text": event.Text}})
	})
	if err != nil {
		stream.failure(err, true)
		return
	}
	if err := stream.send("content_block_stop", object{"type": "content_block_stop", "index": 0}); err != nil {
		return
	}
	_ = finishAnthropicMessage(stream, object{"reason": "end_turn",
		"tokens": estimatedTokens(result.Text), "gemini": resultMetadata(result)})
}

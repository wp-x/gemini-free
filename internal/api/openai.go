package api

import (
	"net/http"
	"time"

	"gemini-free/internal/generation"
)

func (s *server) openAI(w http.ResponseWriter, r *http.Request) {
	chat, ok := s.prepare(w, r, false)
	if !ok {
		return
	}
	if chat.stream && !chat.policy.active {
		s.streamOpenAIText(w, r, chat)
		return
	}
	result, err := s.options.Generator.Generate(r.Context(), chat.generation, nil)
	if err != nil {
		upstreamError(w, r, err)
		return
	}
	output, err := chat.policy.parse(result.Text)
	if err != nil {
		upstreamError(w, r, err)
		return
	}
	routingHeaders(w, result)
	if chat.stream {
		sendOpenAITools(startStream(w, r), chat, output)
		return
	}
	message, reason := openAIMessageResult(output)
	inputTokens, outputTokens := estimatedTokens(chat.generation.Prompt), estimatedTokens(result.Text)
	writeJSON(w, http.StatusOK, object{"id": chat.id, "object": "chat.completion",
		"created": time.Now().Unix(), "model": chat.generation.Model,
		"choices": []object{{"index": 0, "message": message, "finish_reason": reason}},
		"usage": object{"prompt_tokens": inputTokens, "completion_tokens": outputTokens,
			"total_tokens": inputTokens + outputTokens, "estimated": true},
		"gemini": resultMetadata(result)})
}

func resultMetadata(result generation.Result) object {
	return object{"requested_model": result.RequestedModel, "actual_mode": result.ActualMode,
		"thinking_level": result.ThinkingLevel}
}

func openAIMessageResult(output parsedOutput) (object, string) {
	message := object{"role": "assistant", "content": nil}
	if output.text != "" {
		message["content"] = output.text
	}
	if len(output.calls) == 0 {
		return message, "stop"
	}
	calls := make([]object, 0, len(output.calls))
	for _, call := range output.calls {
		calls = append(calls, object{"id": newID("call_"), "type": "function",
			"function": object{"name": call.name, "arguments": serialized(call.arguments)}})
	}
	message["tool_calls"] = calls
	return message, "tool_calls"
}

func openAIChunk(chat chatContext, delta object, finish any) object {
	return object{"id": chat.id, "object": "chat.completion.chunk",
		"created": time.Now().Unix(), "model": chat.generation.Model,
		"choices": []object{{"index": 0, "delta": delta, "finish_reason": finish}}}
}

func (s *server) streamOpenAIText(w http.ResponseWriter, r *http.Request, chat chatContext) {
	stream := startStream(w, r)
	if err := stream.send("", openAIChunk(chat, object{"role": "assistant"}, nil)); err != nil {
		return
	}
	result, err := s.options.Generator.Generate(r.Context(), chat.generation, func(event generation.Event) error {
		if event.Text == "" {
			return r.Context().Err()
		}
		return stream.send("", openAIChunk(chat, object{"content": event.Text}, nil))
	})
	if err != nil {
		stream.failure(err, false)
		return
	}
	end := openAIChunk(chat, object{}, "stop")
	end["gemini"] = resultMetadata(result)
	if err := stream.send("", end); err != nil {
		return
	}
	_ = stream.done()
}

func sendOpenAITools(stream *eventStream, chat chatContext, output parsedOutput) {
	message, reason := openAIMessageResult(output)
	if err := stream.send("", openAIChunk(chat, object{"role": "assistant"}, nil)); err != nil {
		return
	}
	if output.text != "" {
		if err := stream.send("", openAIChunk(chat, object{"content": output.text}, nil)); err != nil {
			return
		}
	}
	calls, _ := message["tool_calls"].([]object)
	for index, call := range calls {
		indexed := object{"index": index, "id": call["id"], "type": call["type"], "function": call["function"]}
		if err := stream.send("", openAIChunk(chat, object{"tool_calls": []object{indexed}}, nil)); err != nil {
			return
		}
	}
	if err := stream.send("", openAIChunk(chat, object{}, reason)); err != nil {
		return
	}
	_ = stream.done()
}

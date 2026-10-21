package api

func startAnthropicMessage(stream *eventStream, chat chatContext) error {
	return stream.send("message_start", object{"type": "message_start", "message": object{
		"id": chat.id, "type": "message", "role": "assistant",
		"model": chat.generation.Model, "content": []object{},
		"stop_reason": nil, "stop_sequence": nil,
		"usage": object{"input_tokens": estimatedTokens(chat.generation.Prompt),
			"output_tokens": 0, "estimated": true}}})
}

func finishAnthropicMessage(stream *eventStream, summary object) error {
	err := stream.send("message_delta", object{"type": "message_delta",
		"delta":  object{"stop_reason": summary["reason"], "stop_sequence": nil},
		"usage":  object{"output_tokens": summary["tokens"], "estimated": true},
		"gemini": summary["gemini"]})
	if err != nil {
		return err
	}
	return stream.send("message_stop", object{"type": "message_stop"})
}

func sendAnthropicOutput(stream *eventStream, response object) {
	usage, _ := response["usage"].(object)
	startMessage := object{"id": response["id"], "type": "message", "role": "assistant",
		"model": response["model"], "content": []object{}, "stop_reason": nil,
		"stop_sequence": nil, "usage": object{"input_tokens": usage["input_tokens"],
			"output_tokens": 0, "estimated": true}}
	if err := stream.send("message_start", object{"type": "message_start", "message": startMessage}); err != nil {
		return
	}
	blocks, _ := response["content"].([]object)
	for index, block := range blocks {
		if err := sendAnthropicBlock(stream, index, block); err != nil {
			return
		}
	}
	_ = finishAnthropicMessage(stream, object{"reason": response["stop_reason"],
		"tokens": usage["output_tokens"], "gemini": response["gemini"]})
}

func sendAnthropicBlock(stream *eventStream, index int, block object) error {
	start := object{"type": "text", "text": ""}
	delta := object{"type": "text_delta", "text": block["text"]}
	if block["type"] == "tool_use" {
		start = object{"type": "tool_use", "id": block["id"], "name": block["name"], "input": object{}}
		delta = object{"type": "input_json_delta", "partial_json": serialized(block["input"])}
	}
	if err := stream.send("content_block_start", object{"type": "content_block_start",
		"index": index, "content_block": start}); err != nil {
		return err
	}
	if err := stream.send("content_block_delta", object{"type": "content_block_delta",
		"index": index, "delta": delta}); err != nil {
		return err
	}
	return stream.send("content_block_stop", object{"type": "content_block_stop", "index": index})
}

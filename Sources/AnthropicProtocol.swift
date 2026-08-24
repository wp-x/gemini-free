import Foundation

enum AnthropicProtocolError: Error, CustomStringConvertible {
    case invalidRequest(String)

    var description: String {
        switch self {
        case .invalidRequest(let message): return message
        }
    }
}

struct AnthropicMessageBuildInput {
    let model: String
    let prompt: String
    let rawOutput: String
    let output: ParsedToolOutput
}

private let approximateBytesPerToken = 4

func anthropicMessagesToPrompt(_ request: [String: Any]) throws -> String {
    guard let messages = request["messages"] as? [Any], !messages.isEmpty else {
        throw AnthropicProtocolError.invalidRequest("messages must be a non-empty array")
    }
    let tools = request["tools"] as? [Any]
    let toolChoice = request["tool_choice"] ?? ["type": "auto"]
    let policy = toolCallPolicy(tools, toolChoice: toolChoice)
    try validateAnthropicTools(tools, policy: policy)

    var parts: [String] = []
    if hasActiveTools(tools, toolChoice: toolChoice),
       let section = toolUseSection(tools, toolChoice: toolChoice) {
        parts.append(section)
    }
    let system = try anthropicSystemText(request["system"])
    if !system.isEmpty { parts.append("# System Instructions\n\n\(system)") }
    parts.append("# Conversation")
    parts.append(contentsOf: try anthropicConversation(messages))
    return parts.joined(separator: "\n\n")
}

private func validateAnthropicTools(_ tools: [Any]?, policy: ToolCallPolicy) throws {
    if policy.requiresCall, policy.allowedNames.isEmpty {
        throw AnthropicProtocolError.invalidRequest("tool_choice requires at least one declared tool")
    }
    if let name = policy.requiredName, !policy.allowedNames.contains(name) {
        throw AnthropicProtocolError.invalidRequest("tool_choice names an undeclared tool: \(name)")
    }
    guard let tools = tools else { return }
    for value in tools {
        guard let tool = value as? [String: Any],
              let name = tool["name"] as? String, !name.isEmpty,
              tool["input_schema"] is [String: Any] else {
            throw AnthropicProtocolError.invalidRequest(
                "each tool must contain a non-empty name and an input_schema object")
        }
    }
}

private func anthropicSystemText(_ value: Any?) throws -> String {
    if value == nil { return "" }
    if let text = value as? String { return text }
    guard let blocks = value as? [Any] else {
        throw AnthropicProtocolError.invalidRequest("system must be a string or text block array")
    }
    return try blocks.map { value in
        guard let block = value as? [String: Any], block["type"] as? String == "text",
              let text = block["text"] as? String else {
            throw AnthropicProtocolError.invalidRequest("system blocks must be text blocks")
        }
        return text
    }.joined(separator: "\n")
}

private func anthropicConversation(_ messages: [Any]) throws -> [String] {
    var toolNamesByID: [String: String] = [:]
    return try messages.map { value in
        guard let message = value as? [String: Any],
              let role = message["role"] as? String,
              role == "user" || role == "assistant" else {
            throw AnthropicProtocolError.invalidRequest("message role must be user or assistant")
        }
        let content = try anthropicMessageContent(message["content"], role: role, toolNamesByID: &toolNamesByID)
        return role == "assistant" ? "[Assistant]\n\(content)" : "[User]\n\(content)"
    }
}

private func anthropicMessageContent(
    _ value: Any?, role: String, toolNamesByID: inout [String: String]
) throws -> String {
    if let text = value as? String { return text }
    guard let blocks = value as? [Any] else {
        throw AnthropicProtocolError.invalidRequest("message content must be a string or content block array")
    }
    return try blocks.map { raw in
        guard let block = raw as? [String: Any], block["type"] is String else {
            throw AnthropicProtocolError.invalidRequest("invalid content block")
        }
        return try anthropicBlockText(block, role: role, toolNamesByID: &toolNamesByID)
    }.filter { !$0.isEmpty }.joined(separator: "\n")
}

private func anthropicBlockText(
    _ block: [String: Any], role: String, toolNamesByID: inout [String: String]
) throws -> String {
    guard let type = block["type"] as? String else {
        throw AnthropicProtocolError.invalidRequest("content block is missing type")
    }
    switch type {
    case "text": return block["text"] as? String ?? ""
    case "tool_use":
        guard role == "assistant" else {
            throw AnthropicProtocolError.invalidRequest("tool_use blocks must be in assistant messages")
        }
        return try anthropicToolUseText(block, toolNamesByID: &toolNamesByID)
    case "tool_result":
        guard role == "user" else {
            throw AnthropicProtocolError.invalidRequest("tool_result blocks must be in user messages")
        }
        return try anthropicToolResultText(block, toolNamesByID: toolNamesByID)
    case "thinking": return "[Assistant reasoning]\n\(block["thinking"] as? String ?? "")"
    case "redacted_thinking": return "[Assistant reasoning was redacted]"
    case "image": throw AnthropicProtocolError.invalidRequest("image content is not supported")
    default: throw AnthropicProtocolError.invalidRequest("unsupported content block type: \(type)")
    }
}

private func anthropicToolUseText(
    _ block: [String: Any], toolNamesByID: inout [String: String]
) throws -> String {
    guard let id = block["id"] as? String, !id.isEmpty,
          let name = block["name"] as? String, !name.isEmpty,
          let input = block["input"] as? [String: Any] else {
        throw AnthropicProtocolError.invalidRequest("tool_use requires id, name, and input object")
    }
    toolNamesByID[id] = name
    return toolCallBlock(name: name, arguments: input)
}

private func anthropicToolResultText(
    _ block: [String: Any], toolNamesByID: [String: String]
) throws -> String {
    guard let id = block["tool_use_id"] as? String, !id.isEmpty else {
        throw AnthropicProtocolError.invalidRequest("tool_result requires tool_use_id")
    }
    let name = toolNamesByID[id] ?? id
    let status = block["is_error"] as? Bool == true ? "error" : "success"
    let content = try anthropicToolResultContent(block["content"])
    return "[Tool result for \(name); id=\(id); status=\(status)]\n\(content)"
}

private func anthropicToolResultContent(_ value: Any?) throws -> String {
    if value == nil { return "" }
    if let text = value as? String { return text }
    guard let blocks = value as? [Any] else {
        throw AnthropicProtocolError.invalidRequest("tool_result content must be a string or text block array")
    }
    return try blocks.map { raw in
        guard let block = raw as? [String: Any], block["type"] as? String == "text" else {
            throw AnthropicProtocolError.invalidRequest("tool_result currently supports text blocks only")
        }
        return block["text"] as? String ?? ""
    }.joined(separator: "\n")
}

func makeAnthropicMessage(_ input: AnthropicMessageBuildInput) -> [String: Any] {
    let blocks = anthropicResponseBlocks(input.output)
    let stopReason = input.output.calls.isEmpty ? "end_turn" : "tool_use"
    return [
        "id": "msg_" + randomHex(24),
        "type": "message",
        "role": "assistant",
        "model": input.model,
        "content": blocks,
        "stop_reason": stopReason,
        "stop_sequence": NSNull(),
        "usage": [
            "input_tokens": approximateTokenCount(input.prompt),
            "output_tokens": approximateTokenCount(input.rawOutput),
        ],
    ]
}

private func anthropicResponseBlocks(_ output: ParsedToolOutput) -> [[String: Any]] {
    var blocks: [[String: Any]] = []
    if !output.text.isEmpty { blocks.append(["type": "text", "text": output.text]) }
    blocks.append(contentsOf: output.calls.map { call in
        [
            "type": "tool_use",
            "id": "toolu_" + randomHex(24),
            "name": call.name,
            "input": call.arguments,
        ]
    })
    if blocks.isEmpty { blocks.append(["type": "text", "text": ""]) }
    return blocks
}

func approximateTokenCount(_ text: String) -> Int {
    max(1, text.utf8.count / approximateBytesPerToken)
}

func anthropicSSE(_ message: [String: Any]) -> String {
    var events: [String] = [anthropicMessageStartEvent(message)]
    let blocks = message["content"] as? [[String: Any]] ?? []
    for (index, block) in blocks.enumerated() {
        events.append(contentsOf: anthropicBlockEvents(index: index, block: block))
    }
    let usage = message["usage"] as? [String: Any] ?? [:]
    let delta: [String: Any] = [
        "type": "message_delta",
        "delta": ["stop_reason": message["stop_reason"] ?? "end_turn", "stop_sequence": NSNull()],
        "usage": ["output_tokens": usage["output_tokens"] ?? 0],
    ]
    events.append(anthropicEvent("message_delta", payload: delta))
    events.append(anthropicEvent("message_stop", payload: ["type": "message_stop"]))
    return events.joined()
}

private func anthropicMessageStartEvent(_ message: [String: Any]) -> String {
    let startMessage: [String: Any] = [
        "id": message["id"] ?? "msg_" + randomHex(24),
        "type": "message",
        "role": "assistant",
        "model": message["model"] ?? "gemini",
        "content": [],
        "stop_reason": NSNull(),
        "stop_sequence": NSNull(),
        "usage": message["usage"] ?? ["input_tokens": 0, "output_tokens": 0],
    ]
    return anthropicEvent("message_start", payload: ["type": "message_start", "message": startMessage])
}

private func anthropicBlockEvents(index: Int, block: [String: Any]) -> [String] {
    let type = block["type"] as? String ?? "text"
    let startBlock: [String: Any]
    let delta: [String: Any]
    if type == "tool_use" {
        startBlock = [
            "type": "tool_use", "id": block["id"] ?? "toolu_" + randomHex(24),
            "name": block["name"] ?? "", "input": [:],
        ]
        delta = ["type": "input_json_delta", "partial_json": jsonString(block["input"] ?? [:])]
    } else {
        startBlock = ["type": "text", "text": ""]
        delta = ["type": "text_delta", "text": block["text"] ?? ""]
    }
    return [
        anthropicEvent("content_block_start", payload: [
            "type": "content_block_start", "index": index, "content_block": startBlock]),
        anthropicEvent("content_block_delta", payload: [
            "type": "content_block_delta", "index": index, "delta": delta]),
        anthropicEvent("content_block_stop", payload: ["type": "content_block_stop", "index": index]),
    ]
}

private func anthropicEvent(_ type: String, payload: [String: Any]) -> String {
    "event: \(type)\ndata: \(jsonString(payload))\n\n"
}

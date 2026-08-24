import Foundation

// OpenAI messages -> 单条 prompt（图片输入不支持，明确告知模型）
func messagesToPrompt(_ messages: [Any], tools: [Any]?, toolChoice: Any?) -> String {
    var parts: [String] = []
    if hasActiveTools(tools, toolChoice: toolChoice ?? "auto"),
       let section = toolUseSection(tools, toolChoice: toolChoice) {
        parts.append(section)
    }

    for value in messages {
        guard let message = value as? [String: Any] else { continue }
        let role = message["role"] as? String ?? "user"
        let content = openAIContent(message["content"])
        parts.append(openAIMessagePart(message, role: role, content: content))
    }
    return parts.filter { !$0.isEmpty }.joined(separator: "\n\n")
}

func toolUseSection(_ tools: [Any]?, toolChoice: Any?) -> String? {
    let definitions = toolDefinitions(tools)
    guard !definitions.isEmpty else { return nil }
    let serialized = jsonString(definitions, pretty: true)
    return """
    # Local Tool Protocol

    The tools below are real and are executed by the client in the user's environment. When a task requires files, shell commands, search, or another listed capability, call the appropriate tool instead of claiming that you cannot access it or inventing a result.

    Use exactly this format:
    ```tool_call
    {"name": "tool_name", "arguments": {}}
    ```
    Output only one or more tool_call blocks when calling tools. Arguments must be one JSON object. After a tool result arrives, continue from that result.

    Available tools:
    \(serialized)\(toolChoiceInstruction(toolChoice))
    """
}

func toolDefinitions(_ tools: [Any]?) -> [[String: Any]] {
    guard let tools = tools else { return [] }
    return tools.compactMap { value in
        guard let tool = value as? [String: Any] else { return nil }
        let function = openAIFunction(tool)
        guard let name = (function["name"] ?? tool["name"]) as? String, !name.isEmpty else { return nil }
        return [
            "name": name,
            "description": function["description"] ?? tool["description"] ?? "",
            "parameters": function["parameters"] ?? tool["input_schema"] ?? tool["parameters"] ?? [:],
        ]
    }
}

private func openAIFunction(_ tool: [String: Any]) -> [String: Any] {
    guard tool["type"] as? String == "function" else { return tool }
    return tool["function"] as? [String: Any] ?? tool
}

private func openAIContent(_ value: Any?) -> String {
    if let text = value as? String { return text }
    guard let blocks = value as? [Any] else { return "" }
    return blocks.compactMap { value -> String? in
        guard let block = value as? [String: Any] else { return nil }
        let type = block["type"] as? String
        if type == "text" || type == "input_text" { return block["text"] as? String ?? "" }
        if type == "image_url" || type == "image" { return "[Image input is not supported by Gemini Free.]" }
        return nil
    }.joined(separator: " ")
}

private func openAIMessagePart(_ message: [String: Any], role: String, content: String) -> String {
    switch role {
    case "system": return "[System instruction]\n\(content)"
    case "assistant": return openAIAssistantPart(message, content: content)
    case "tool":
        let name = message["name"] as? String ?? message["tool_call_id"] as? String ?? "unknown"
        return "[Tool result for \(name)]\n\(content)"
    default: return content.isEmpty ? "" : "[User]\n\(content)"
    }
}

private func openAIAssistantPart(_ message: [String: Any], content: String) -> String {
    guard let calls = message["tool_calls"] as? [Any], !calls.isEmpty else {
        return "[Assistant]\n\(content)"
    }
    let blocks = calls.compactMap { value -> String? in
        guard let call = value as? [String: Any],
              let function = call["function"] as? [String: Any],
              let name = function["name"] as? String else { return nil }
        let arguments = decodeArguments(function["arguments"]) ?? [:]
        return toolCallBlock(name: name, arguments: arguments)
    }
    return (["[Assistant]\n\(content)"] + blocks).joined(separator: "\n")
}

private func decodeArguments(_ value: Any?) -> [String: Any]? {
    if let object = value as? [String: Any] { return object }
    guard let text = value as? String, let data = text.data(using: .utf8) else { return nil }
    return (try? JSONSerialization.jsonObject(with: data)) as? [String: Any]
}

func toolCallBlock(name: String, arguments: [String: Any]) -> String {
    let payload: [String: Any] = ["name": name, "arguments": arguments]
    return "```tool_call\n\(jsonString(payload))\n```"
}

private func toolChoiceInstruction(_ value: Any?) -> String {
    if let choice = value as? String {
        if choice == "required" { return "\n\nIMPORTANT: You MUST call at least one tool." }
        return choice == "none" ? "\n\nIMPORTANT: Do not call tools." : ""
    }
    guard let choice = value as? [String: Any] else { return "" }
    let type = choice["type"] as? String
    if type == "any" { return "\n\nIMPORTANT: You MUST call at least one tool." }
    if type == "none" { return "\n\nIMPORTANT: Do not call tools." }
    let openAIName = (choice["function"] as? [String: Any])?["name"] as? String
    let name = openAIName ?? choice["name"] as? String
    guard (type == "tool" || openAIName != nil), let name = name, !name.isEmpty else { return "" }
    return "\n\nIMPORTANT: You MUST call only the tool \"\(name)\"."
}

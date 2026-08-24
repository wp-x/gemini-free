import Foundation

enum ToolCallParseError: Error, CustomStringConvertible {
    case malformedBlock
    case missingName
    case invalidArguments(String)
    case unknownTool(String)
    case requiredToolMissing
    case wrongRequiredTool(expected: String, actual: String)

    var description: String {
        switch self {
        case .malformedBlock: return "malformed tool_call block"
        case .missingName: return "tool_call is missing a non-empty name"
        case .invalidArguments(let name): return "tool_call arguments for \(name) must be a JSON object"
        case .unknownTool(let name): return "model called unknown tool \(name)"
        case .requiredToolMissing: return "model did not call a required tool"
        case .wrongRequiredTool(let expected, let actual):
            return "model called \(actual), but tool_choice requires \(expected)"
        }
    }
}

struct ParsedToolCall {
    let name: String
    let arguments: [String: Any]
}

struct ParsedToolOutput {
    let text: String
    let calls: [ParsedToolCall]
}

struct ToolCallPolicy {
    let active: Bool
    let allowedNames: Set<String>
    let requiresCall: Bool
    let requiredName: String?

    func validate(_ calls: [ParsedToolCall]) throws {
        if requiresCall, calls.isEmpty { throw ToolCallParseError.requiredToolMissing }
        guard let expected = requiredName else { return }
        if let wrong = calls.first(where: { $0.name != expected }) {
            throw ToolCallParseError.wrongRequiredTool(expected: expected, actual: wrong.name)
        }
    }
}

private let toolBlockPattern = "```(?:tool_call|function_call)\\s*(?:\\r?\\n)?(.*?)(?:\\r?\\n)?```"
private let toolBlockRegex = try! NSRegularExpression(
    pattern: toolBlockPattern,
    options: [.dotMatchesLineSeparators])

func hasActiveTools(_ tools: [Any]?, toolChoice: Any) -> Bool {
    guard !(tools?.isEmpty ?? true) else { return false }
    if toolChoice as? String == "none" { return false }
    if let choice = toolChoice as? [String: Any], choice["type"] as? String == "none" { return false }
    return true
}

func toolNames(_ tools: [Any]?) -> Set<String> {
    Set(toolDefinitions(tools).compactMap { $0["name"] as? String })
}

func toolCallPolicy(_ tools: [Any]?, toolChoice: Any) -> ToolCallPolicy {
    let names = toolNames(tools)
    let active = hasActiveTools(tools, toolChoice: toolChoice)
    let requiredName = selectedToolName(toolChoice)
    return ToolCallPolicy(
        active: active,
        allowedNames: names,
        requiresCall: requiresToolCall(toolChoice) || requiredName != nil,
        requiredName: requiredName)
}

private func requiresToolCall(_ choice: Any) -> Bool {
    if choice as? String == "required" { return true }
    return (choice as? [String: Any])?["type"] as? String == "any"
}

private func selectedToolName(_ choice: Any) -> String? {
    guard let object = choice as? [String: Any] else { return nil }
    let openAIName = (object["function"] as? [String: Any])?["name"] as? String
    let name = openAIName ?? object["name"] as? String
    let type = object["type"] as? String
    return (type == "tool" || openAIName != nil) ? name : nil
}

func parseStructuredToolCalls(
    _ text: String,
    allowedToolNames: Set<String>? = nil
) throws -> ParsedToolOutput {
    let nsText = text as NSString
    let fullRange = NSRange(location: 0, length: nsText.length)
    let matches = toolBlockRegex.matches(in: text, range: fullRange)
    let containsMarker = text.contains("```tool_call") || text.contains("```function_call")
    if matches.isEmpty, containsMarker { throw ToolCallParseError.malformedBlock }

    var calls: [ParsedToolCall] = []
    var cleanParts: [String] = []
    var cursor = 0
    for match in matches {
        cleanParts.append(nsText.substring(with: NSRange(
            location: cursor,
            length: match.range.location - cursor)))
        cursor = match.range.location + match.range.length
        calls.append(try parseToolBlock(
            nsText.substring(with: match.range(at: 1)),
            allowed: allowedToolNames))
    }
    cleanParts.append(nsText.substring(from: cursor))
    let clean = cleanParts.joined().trimmingCharacters(in: .whitespacesAndNewlines)
    return ParsedToolOutput(text: clean, calls: calls)
}

private func parseToolBlock(_ raw: String, allowed: Set<String>?) throws -> ParsedToolCall {
    let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let data = trimmed.data(using: .utf8),
          let object = try? JSONSerialization.jsonObject(with: data),
          let payload = object as? [String: Any] else {
        throw ToolCallParseError.malformedBlock
    }
    guard let name = payload["name"] as? String, !name.isEmpty else {
        throw ToolCallParseError.missingName
    }
    if let allowed = allowed, !allowed.contains(name) {
        throw ToolCallParseError.unknownTool(name)
    }
    let value = payload["arguments"] ?? payload["args"] ?? payload["input"] ?? [String: Any]()
    guard let arguments = value as? [String: Any] else {
        throw ToolCallParseError.invalidArguments(name)
    }
    return ParsedToolCall(name: name, arguments: arguments)
}

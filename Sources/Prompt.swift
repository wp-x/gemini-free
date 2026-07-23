import Foundation

// OpenAI messages -> 单条 prompt（图片输入不支持，忽略），移植自上游 tools.py
func messagesToPrompt(_ messages: [Any], tools: [Any]?, toolChoice: Any?) -> String {
    var parts: [String] = []

    if let tools = tools, !(toolChoice as? String == "none") {
        var toolDefs: [[String: Any]] = []
        for t in tools {
            guard let tool = t as? [String: Any] else { continue }
            let fn = (tool["type"] as? String == "function" ? tool["function"] as? [String: Any] : nil) ?? tool
            toolDefs.append([
                "name": fn["name"] ?? tool["name"] ?? "",
                "description": fn["description"] ?? tool["description"] ?? "",
                "parameters": fn["parameters"] ?? tool["parameters"] ?? [:],
            ])
        }
        if !toolDefs.isEmpty {
            let constraint = toolChoiceInstruction(toolChoice)
            let defsJSON = jsonString(toolDefs, pretty: true)
            parts.append(
                "# Tool Use\n\n"
                + "You can call the following tools. Call format:\n"
                + "```tool_call\n{\"name\": \"func_name\", \"arguments\": {...}}\n```\n"
                + "When calling tools, output ONLY the tool_call block(s).\n\n"
                + "Available tools:\n\(defsJSON)\(constraint)")
        }
    }

    for m in messages {
        guard let msg = m as? [String: Any] else { continue }
        let role = msg["role"] as? String ?? "user"
        var content = ""
        if let s = msg["content"] as? String {
            content = s
        } else if let arr = msg["content"] as? [Any] {
            var textParts: [String] = []
            for c in arr {
                guard let c = c as? [String: Any] else { continue }
                let type = c["type"] as? String
                if type == "text" || type == "input_text" {
                    textParts.append(c["text"] as? String ?? "")
                } else if type == "image_url" || type == "image" {
                    textParts.append("[Note: Image input not supported in this API. Please describe the image in text.]")
                }
            }
            content = textParts.joined(separator: " ")
        }

        switch role {
        case "system":
            parts.append("[System instruction]: \(content)")
        case "assistant":
            if let tcs = msg["tool_calls"] as? [Any], !tcs.isEmpty {
                var tcStrs: [String] = []
                for tc in tcs {
                    guard let tc = tc as? [String: Any], let fn = tc["function"] as? [String: Any] else { continue }
                    let name = fn["name"] as? String ?? ""
                    let args = fn["arguments"] as? String ?? "{}"
                    tcStrs.append("```tool_call\n{\"name\": \"\(name)\", \"arguments\": \(args)}\n```")
                }
                parts.append("[Assistant]: \(content)\n" + tcStrs.joined(separator: "\n"))
            } else {
                parts.append("[Assistant]: \(content)")
            }
        case "tool":
            let name = msg["name"] as? String ?? ""
            parts.append("[Tool result for \(name)]: \(content)")
        default:
            if !content.isEmpty { parts.append(content) }
        }
    }

    return parts.filter { !$0.isEmpty }.joined(separator: "\n\n")
}

private func toolChoiceInstruction(_ choice: Any?) -> String {
    if let s = choice as? String {
        if s == "none" { return "\n\nIMPORTANT: Do NOT call any tools. Respond with text only." }
        if s == "required" { return "\n\nIMPORTANT: You MUST call at least one tool. Do not respond with text only." }
    }
    if let d = choice as? [String: Any], let fn = d["function"] as? [String: Any],
       let name = fn["name"] as? String, !name.isEmpty {
        return "\n\nIMPORTANT: You MUST call the tool \"\(name)\". Do not call other tools."
    }
    return ""
}

// 从模型输出提取 tool_call 块，返回 (clean_text, tool_calls)
func parseToolCalls(_ text: String) -> (String, [[String: Any]]) {
    var toolCalls: [[String: Any]] = []
    let ns = text as NSString
    let regex = try! NSRegularExpression(pattern: "```tool_call\\s*\\n(.*?)\\n```", options: [.dotMatchesLineSeparators])
    let matches = regex.matches(in: text, range: NSRange(location: 0, length: ns.length))
    var clean = ""
    var last = 0
    for m in matches {
        clean += ns.substring(with: NSRange(location: last, length: m.range.location - last))
        last = m.range.location + m.range.length
        let inner = ns.substring(with: m.range(at: 1)).trimmingCharacters(in: .whitespacesAndNewlines)
        if let data = inner.data(using: .utf8),
           let d = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any],
           let name = d["name"] as? String {
            toolCalls.append([
                "id": "call_" + randomHex(8),
                "type": "function",
                "function": ["name": name, "arguments": jsonString(d["arguments"] ?? [:])],
            ])
        }
    }
    clean += ns.substring(from: last)
    return (clean.trimmingCharacters(in: .whitespacesAndNewlines), toolCalls)
}

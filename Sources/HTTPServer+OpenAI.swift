import Foundation
import Network

private enum OpenAIProtocolError: Error, CustomStringConvertible {
    case invalidRequest(String)

    var description: String {
        switch self {
        case .invalidRequest(let message): return message
        }
    }
}

private struct OpenAIChatContext {
    let model: ResolvedModel
    let generation: GenerationRequest
    let stream: Bool
    let toolPolicy: ToolCallPolicy
    let completionID: String
}

private struct OpenAIResult {
    let message: [String: Any]
    let completionTokens: Int
}

extension HTTPServer {
    func handleOpenAIChat(_ conn: NWConnection, body: Data) {
        guard let request = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] else {
            sendJSON(conn, ["error": ["message": "invalid JSON"]], status: 400)
            return
        }
        do {
            let context = try openAIContext(request)
            if context.stream && !context.toolPolicy.active {
                streamOpenAIText(conn, context: context)
                return
            }
            generateOpenAIResult(conn, context: context)
        } catch {
            sendJSON(conn, ["error": ["message": "\(error)"]], status: 400)
        }
    }

    private func openAIContext(_ request: [String: Any]) throws -> OpenAIChatContext {
        let modelName = request["model"] as? String ?? cfg.defaultModel
        let model = resolveModel(modelName, defaultModel: cfg.defaultModel)
        let tools = request["tools"] as? [Any]
        let toolChoice = request["tool_choice"] ?? "auto"
        let policy = toolCallPolicy(tools, toolChoice: toolChoice)
        try validateOpenAITools(tools, policy: policy)
        let prompt = messagesToPrompt(request["messages"] as? [Any] ?? [], tools: tools, toolChoice: toolChoice)
        guard !prompt.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw OpenAIProtocolError.invalidRequest("empty prompt")
        }
        let generation = GenerationRequest(prompt: prompt, mode: model.mode, think: model.think, extra: model.extra)
        return OpenAIChatContext(
            model: model,
            generation: generation,
            stream: request["stream"] as? Bool ?? false,
            toolPolicy: policy,
            completionID: "chatcmpl-" + randomHex(12))
    }

    private func validateOpenAITools(_ tools: [Any]?, policy: ToolCallPolicy) throws {
        if let tools = tools, toolDefinitions(tools).count != tools.count {
            throw OpenAIProtocolError.invalidRequest("each tool must contain a non-empty function name")
        }
        if policy.requiresCall, policy.allowedNames.isEmpty {
            throw OpenAIProtocolError.invalidRequest("tool_choice requires at least one declared tool")
        }
        if let name = policy.requiredName, !policy.allowedNames.contains(name) {
            throw OpenAIProtocolError.invalidRequest("tool_choice names an undeclared tool: \(name)")
        }
    }

    private func streamOpenAIText(_ conn: NWConnection, context: OpenAIChatContext) {
        let gone = ClientGone()
        conn.stateUpdateHandler = { state in
            if case .failed = state { gone.on = true }
            if case .cancelled = state { gone.on = true }
        }
        startSSE(conn)
        do {
            try generator.generateStream(context.generation, isCancelled: { gone.on }) { delta in
                let chunk = self.openAIChunk(context, delta: ["content": delta], finishReason: NSNull())
                self.sseSend(conn, "data: \(jsonString(chunk))\n\n", gone: gone)
            }
        } catch {
            let delta = ["content": "[upstream error: \(error)]"]
            sseSend(conn, "data: \(jsonString(openAIChunk(context, delta: delta, finishReason: NSNull())))\n\n")
        }
        let end = openAIChunk(context, delta: [:], finishReason: "stop")
        sseSend(conn, "data: \(jsonString(end))\n\n")
        sseFinish(conn, "data: [DONE]\n\n")
    }

    private func generateOpenAIResult(_ conn: NWConnection, context: OpenAIChatContext) {
        do {
            let rawOutput = try generator.generate(context.generation)
            let parsed = try openAIParsedOutput(rawOutput, context: context)
            let toolCalls = parsed.calls.map(openAIToolCall)
            var message: [String: Any] = [
                "role": "assistant",
                "content": parsed.text.isEmpty ? NSNull() : parsed.text,
            ]
            if !toolCalls.isEmpty { message["tool_calls"] = toolCalls }
            sendOpenAIResult(conn, context: context, result: OpenAIResult(
                message: message, completionTokens: approximateTokenCount(rawOutput)))
        } catch let error as ToolCallParseError {
            sendJSON(conn, ["error": ["message": "upstream tool protocol error: \(error)"]], status: 502)
        } catch {
            sendJSON(conn, ["error": ["message": "upstream error: \(error)"]], status: 502)
        }
    }

    private func openAIParsedOutput(
        _ rawOutput: String, context: OpenAIChatContext
    ) throws -> ParsedToolOutput {
        guard context.toolPolicy.active else { return ParsedToolOutput(text: rawOutput, calls: []) }
        let output = try parseStructuredToolCalls(
            rawOutput, allowedToolNames: context.toolPolicy.allowedNames)
        try context.toolPolicy.validate(output.calls)
        return output
    }

    private func openAIToolCall(_ call: ParsedToolCall) -> [String: Any] {
        [
            "id": "call_" + randomHex(16),
            "type": "function",
            "function": ["name": call.name, "arguments": jsonString(call.arguments)],
        ]
    }

    private func sendOpenAIResult(
        _ conn: NWConnection, context: OpenAIChatContext, result: OpenAIResult
    ) {
        let finishReason = result.message["tool_calls"] == nil ? "stop" : "tool_calls"
        if context.stream {
            startSSE(conn)
            let chunk = openAIChunk(context, delta: result.message, finishReason: finishReason)
            sseFinish(conn, "data: \(jsonString(chunk))\n\ndata: [DONE]\n\n")
            return
        }
        let inputTokens = approximateTokenCount(context.generation.prompt)
        sendJSON(conn, [
            "id": context.completionID,
            "object": "chat.completion",
            "created": nowUnix(),
            "model": context.model.name,
            "choices": [["index": 0, "message": result.message, "finish_reason": finishReason]],
            "usage": [
                "prompt_tokens": inputTokens,
                "completion_tokens": result.completionTokens,
                "total_tokens": inputTokens + result.completionTokens,
            ],
        ])
    }

    private func openAIChunk(
        _ context: OpenAIChatContext, delta: [String: Any], finishReason: Any
    ) -> [String: Any] {
        [
            "id": context.completionID,
            "object": "chat.completion.chunk",
            "created": nowUnix(),
            "model": context.model.name,
            "choices": [["index": 0, "delta": delta, "finish_reason": finishReason]],
        ]
    }
}

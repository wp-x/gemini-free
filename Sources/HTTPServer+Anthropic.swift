import Foundation
import Network

private struct AnthropicRequestContext {
    let model: ResolvedModel
    let generation: GenerationRequest
    let stream: Bool
    let toolPolicy: ToolCallPolicy
}

extension HTTPServer {
    func handleAnthropicMessages(_ conn: NWConnection, body: Data) {
        guard let request = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] else {
            sendAnthropicError(conn, status: 400, message: "invalid JSON")
            return
        }
        do {
            let context = try anthropicContext(request)
            try generateAnthropicMessage(conn, context: context)
        } catch let error as AnthropicProtocolError {
            sendAnthropicError(conn, status: 400, message: error.description)
        } catch let error as ToolCallParseError {
            sendAnthropicError(conn, status: 502, message: "upstream tool protocol error: \(error)")
        } catch {
            sendAnthropicError(conn, status: 502, message: "upstream error: \(error)")
        }
    }

    func handleAnthropicTokenCount(_ conn: NWConnection, body: Data) {
        guard let request = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] else {
            sendAnthropicError(conn, status: 400, message: "invalid JSON")
            return
        }
        do {
            let prompt = try anthropicMessagesToPrompt(request)
            sendJSON(conn, ["input_tokens": approximateTokenCount(prompt)])
        } catch {
            sendAnthropicError(conn, status: 400, message: "\(error)")
        }
    }

    private func anthropicContext(_ request: [String: Any]) throws -> AnthropicRequestContext {
        let modelName = request["model"] as? String ?? cfg.defaultModel
        let model = resolveModel(modelName, defaultModel: cfg.defaultModel)
        let prompt = try anthropicMessagesToPrompt(request)
        let tools = request["tools"] as? [Any]
        let toolChoice = request["tool_choice"] ?? ["type": "auto"]
        let policy = toolCallPolicy(tools, toolChoice: toolChoice)
        let generation = GenerationRequest(prompt: prompt, mode: model.mode, think: model.think, extra: model.extra)
        return AnthropicRequestContext(
            model: model,
            generation: generation,
            stream: request["stream"] as? Bool ?? false,
            toolPolicy: policy)
    }

    private func generateAnthropicMessage(
        _ conn: NWConnection, context: AnthropicRequestContext
    ) throws {
        let rawOutput = try generator.generate(context.generation)
        let output: ParsedToolOutput
        if context.toolPolicy.active {
            output = try parseStructuredToolCalls(
                rawOutput, allowedToolNames: context.toolPolicy.allowedNames)
            try context.toolPolicy.validate(output.calls)
        } else {
            output = ParsedToolOutput(text: rawOutput, calls: [])
        }
        let message = makeAnthropicMessage(AnthropicMessageBuildInput(
            model: context.model.name,
            prompt: context.generation.prompt,
            rawOutput: rawOutput,
            output: output))
        if context.stream {
            startSSE(conn)
            sseFinish(conn, anthropicSSE(message))
        } else {
            sendJSON(conn, message)
        }
    }

    private func sendAnthropicError(_ conn: NWConnection, status: Int, message: String) {
        let type = status >= 500 ? "api_error" : "invalid_request_error"
        sendJSON(conn, ["type": "error", "error": ["type": type, "message": message]], status: status)
    }
}

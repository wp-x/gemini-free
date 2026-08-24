import Foundation

private final class FakeGenerator: TextGenerating {
    func generate(_ request: GenerationRequest) throws -> String {
        "```tool_call\n{\"name\":\"Read\",\"arguments\":{\"file_path\":\"README.md\"}}\n```"
    }

    func generateStream(
        _ request: GenerationRequest,
        isCancelled: @escaping () -> Bool,
        onDelta: @escaping (String) -> Void
    ) throws {
        onDelta("fake stream")
    }
}

private struct HTTPResult {
    let status: Int
    let data: Data
    var text: String { String(decoding: data, as: UTF8.self) }
}

@main
struct HTTPServerIntegrationTests {
    private static let testPort = Int.random(in: 20_000...50_000)
    private static let readinessAttempts = 40
    private static let readinessDelay = 0.05
    private static let requestTimeoutSeconds = 5.0

    static func main() throws {
        let config = Store()
        config.host = "127.0.0.1"
        config.port = testPort
        let server = HTTPServer(generator: FakeGenerator(), config: config)
        try server.start()
        defer { server.stop() }
        try waitUntilReady(server)
        try testNonStreamingToolUse()
        try testStreamingToolUse()
        try testTokenCount()
        try testInvalidToolChoices()
        print("HTTPServerIntegrationTests passed")
    }

    private static func waitUntilReady(_ server: HTTPServer) throws {
        for _ in 0..<readinessAttempts {
            if server.running { return }
            Thread.sleep(forTimeInterval: readinessDelay)
        }
        throw NSError(domain: "HTTPServerIntegrationTests", code: 1,
                      userInfo: [NSLocalizedDescriptionKey: "server did not become ready"])
    }

    private static func testNonStreamingToolUse() throws {
        let result = try post(path: "/v1/messages", payload: requestPayload(stream: false))
        precondition(result.status == 200)
        let body = try jsonObject(result.data)
        precondition(body["stop_reason"] as? String == "tool_use")
        let blocks = body["content"] as? [[String: Any]]
        precondition(blocks?.first?["type"] as? String == "tool_use")
        precondition(blocks?.first?["name"] as? String == "Read")
    }

    private static func testStreamingToolUse() throws {
        let result = try post(path: "/v1/messages", payload: requestPayload(stream: true))
        precondition(result.status == 200)
        precondition(result.text.contains("event: message_start"))
        precondition(result.text.contains("\"type\":\"input_json_delta\""))
        precondition(result.text.contains("\"stop_reason\":\"tool_use\""))
        precondition(result.text.contains("event: message_stop"))
    }

    private static func testTokenCount() throws {
        let result = try post(path: "/v1/messages/count_tokens", payload: requestPayload(stream: false))
        precondition(result.status == 200)
        let body = try jsonObject(result.data)
        precondition((body["input_tokens"] as? Int ?? 0) > 0)
    }

    private static func testInvalidToolChoices() throws {
        let anthropic: [String: Any] = [
            "model": "gemini-3.6-flash",
            "messages": [["role": "user", "content": "test"]],
            "tool_choice": ["type": "any"],
        ]
        let anthropicResult = try post(path: "/v1/messages", payload: anthropic)
        precondition(anthropicResult.status == 400)

        let openAI: [String: Any] = [
            "model": "gemini-3.6-flash",
            "messages": [["role": "user", "content": "test"]],
            "tool_choice": "required",
        ]
        let openAIResult = try post(path: "/v1/chat/completions", payload: openAI)
        precondition(openAIResult.status == 400)
    }

    private static func requestPayload(stream: Bool) -> [String: Any] {
        [
            "model": "gemini-3.6-flash",
            "max_tokens": 1024,
            "stream": stream,
            "messages": [["role": "user", "content": "Read README.md"]],
            "tools": [[
                "name": "Read",
                "description": "Read a file",
                "input_schema": [
                    "type": "object",
                    "properties": ["file_path": ["type": "string"]],
                    "required": ["file_path"],
                ],
            ]],
        ]
    }

    private static func post(path: String, payload: [String: Any]) throws -> HTTPResult {
        let url = URL(string: "http://127.0.0.1:\(testPort)\(path)")!
        var request = URLRequest(url: url)
        request.httpMethod = "POST"
        request.timeoutInterval = requestTimeoutSeconds
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.httpBody = try JSONSerialization.data(withJSONObject: payload)
        let semaphore = DispatchSemaphore(value: 0)
        var captured: Result<HTTPResult, Error>!
        URLSession.shared.dataTask(with: request) { data, response, error in
            if let error = error {
                captured = .failure(error)
            } else {
                let status = (response as? HTTPURLResponse)?.statusCode ?? 0
                captured = .success(HTTPResult(status: status, data: data ?? Data()))
            }
            semaphore.signal()
        }.resume()
        guard semaphore.wait(timeout: .now() + requestTimeoutSeconds) == .success else {
            throw NSError(domain: "HTTPServerIntegrationTests", code: 3,
                          userInfo: [NSLocalizedDescriptionKey: "request timed out"])
        }
        return try captured.get()
    }

    private static func jsonObject(_ data: Data) throws -> [String: Any] {
        guard let object = try JSONSerialization.jsonObject(with: data) as? [String: Any] else {
            throw NSError(domain: "HTTPServerIntegrationTests", code: 2,
                          userInfo: [NSLocalizedDescriptionKey: "response is not a JSON object"])
        }
        return object
    }
}

import Foundation

@main
struct ModelProtocolTests {
    private static let defaultModel = "gemini-4-argon"

    static func main() throws {
        try testArgonModels()
        try testThinkingOverrides()
        try testLegacyModels()
        try testPayloadAndHeader()
        try testResponseRouting()
        try testMalformedResponses()
        print("ModelProtocolTests passed")
    }

    private static func testArgonModels() throws {
        let levels = [
            "gemini-4-argon": 2, "gemini-4-argon-low": 1,
            "gemini-4-argon-medium": 4, "gemini-4-argon-high": 2,
        ]
        for (name, level) in levels {
            let model = try resolveModel(name, defaultModel: defaultModel)
            precondition(model.name == name)
            precondition(model.mode == 8)
            precondition(model.extra?[80] == level)
            precondition(MODELS.contains { $0.id == name })
        }
    }

    private static func testThinkingOverrides() throws {
        for level in [1, 2, 4] {
            let model = try resolveModel("gemini-4-argon@think=\(level)", defaultModel: defaultModel)
            precondition(model.extra?[80] == level)
            precondition(model.think == 4)
        }
        for suffix in ["0", "3", "-1", "abc", ""] {
            try expectFailure {
                _ = try resolveModel("gemini-4-argon@think=\(suffix)", defaultModel: defaultModel)
            }
        }
        try expectFailure {
            _ = try resolveModel("unknown", defaultModel: "unknown-default")
        }
    }

    private static func testLegacyModels() throws {
        let model = try resolveModel("gemini-3.8-flash@think=0", defaultModel: defaultModel)
        precondition(model.mode == 1 && model.think == 0 && model.extra == nil)
        precondition(GeminiPayload.modelHeaders(for: request(model)).isEmpty)
        let enhanced = try resolveModel("gemini-3.1-pro-enhanced", defaultModel: defaultModel)
        precondition(enhanced.extra == [31: 2, 80: 3])
    }

    private static func testPayloadAndHeader() throws {
        let model = try resolveModel("gemini-4-argon-medium", defaultModel: defaultModel)
        let generation = request(model)
        let body = GeminiPayload.body(for: generation, xsrfToken: "token+&=")
        let components = URLComponents(string: "https://localhost/?" + body)!
        let encoded = components.queryItems!.first { $0.name == "f.req" }!.value!
        let outer = try JSONSerialization.jsonObject(with: Data(encoded.utf8)) as! [Any]
        let inner = try JSONSerialization.jsonObject(with: Data((outer[1] as! String).utf8)) as! [Any]
        precondition(inner[79] as? Int == 8 && inner[80] as? Int == 4)
        precondition(components.queryItems!.first { $0.name == "at" }!.value == "token+&=")
        let headers = GeminiPayload.modelHeaders(for: generation)
        let header = headers["X-Goog-Ext-525001261-Jspb"]!
        let metadata = try JSONSerialization.jsonObject(with: Data(header.utf8)) as! [Any]
        precondition(metadata[4] as? String == "95a4c0abcba24364")
        precondition(metadata[14] as? Int == 8 && metadata[15] as? Int == 4)
    }

    private static func testResponseRouting() throws {
        let model = try resolveModel(defaultModel, defaultModel: defaultModel)
        let frames = try GeminiResponseFrame.parse(frameLine(mode: 8, thinking: 2))
        precondition(frames.count == 1 && frames[0].texts == ["OK"])
        try frames[0].validateRoute(for: request(model))
        for frame in [
            GeminiResponseFrame(texts: ["wrong model"], mode: 6, thinkingLevel: 1),
            GeminiResponseFrame(texts: ["wrong level"], mode: 8, thinkingLevel: 1),
            GeminiResponseFrame(texts: ["missing routing"], mode: nil, thinkingLevel: nil),
        ] {
            try expectFailure { try frame.validateRoute(for: request(model)) }
        }
        let flash = try resolveModel("gemini-3.8-flash", defaultModel: defaultModel)
        try GeminiResponseFrame(texts: ["legacy"], mode: nil, thinkingLevel: nil)
            .validateRoute(for: request(flash))
    }

    private static func testMalformedResponses() throws {
        let prefix = try GeminiResponseFrame.parse(")]}'")
        precondition(prefix.isEmpty)
        try expectFailure { _ = try GeminiResponseFrame.parse("[[\"wrb.fr\", invalid JSON") }
        let line = jsonString([["wrb.fr", NSNull(), "invalid JSON"]])
        try expectFailure { _ = try GeminiResponseFrame.parse(line) }
        let metadataOnly = jsonString([["wrb.fr", NSNull(), jsonString(["status", 1, 2])]])
        let metadata = try GeminiResponseFrame.parse(metadataOnly)
        precondition(metadata.isEmpty)
    }

    private static func request(_ model: ResolvedModel) -> GenerationRequest {
        GenerationRequest(prompt: "test", mode: model.mode, think: model.think, extra: model.extra)
    }

    private static func frameLine(mode: Int, thinking: Int) -> String {
        let responseFieldCount = 60
        var body: [Any] = Array(repeating: NSNull(), count: responseFieldCount)
        body[4] = [["response", ["OK"]]]
        body[58] = mode
        body[59] = thinking
        return jsonString([["wrb.fr", NSNull(), jsonString(body)]])
    }

    private static func expectFailure(_ action: () throws -> Void) throws {
        do {
            try action()
        } catch {
            return
        }
        preconditionFailure("Expected an explicit failure")
    }
}

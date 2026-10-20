import Foundation

struct GeminiResponseFrame {
    private static let modeIndex = 58
    private static let thinkingIndex = 59
    private static let textIndex = 4

    let texts: [String]
    let mode: Int?
    let thinkingLevel: Int?

    static func texts(from line: String, for request: GenerationRequest) throws -> [String] {
        try parse(line).flatMap { frame in
            try frame.validateRoute(for: request)
            return frame.texts
        }
    }

    static func parse(_ line: String) throws -> [GeminiResponseFrame] {
        guard line.contains("\"wrb.fr\"") else { return [] }
        let value = try decodeJSON(line)
        guard let envelopes = value as? [[Any]] else {
            throw EngineError.response("Malformed Gemini response envelope")
        }
        return try envelopes.compactMap(parseEnvelope)
    }

    private static func parseEnvelope(_ envelope: [Any]) throws -> GeminiResponseFrame? {
        guard envelope.first as? String == "wrb.fr",
              envelope.count > 2, let body = envelope[2] as? String else { return nil }
        let value = try decodeJSON(body)
        guard let inner = value as? [Any] else {
            throw EngineError.response("Malformed Gemini response body")
        }
        guard inner.count > textIndex else { return nil }
        let parts = inner[textIndex] as? [[Any]] ?? []
        let texts = parts.flatMap { part -> [String] in
            guard part.count > 1, let strings = part[1] as? [String] else { return [] }
            return strings.filter { !$0.isEmpty }
        }
        return GeminiResponseFrame(
            texts: texts,
            mode: inner.count > modeIndex ? inner[modeIndex] as? Int : nil,
            thinkingLevel: inner.count > thinkingIndex ? inner[thinkingIndex] as? Int : nil)
    }

    func validateRoute(for request: GenerationRequest) throws {
        guard request.mode == Argon.mode, !texts.isEmpty else { return }
        guard mode == Argon.mode else {
            let actual = mode.map(String.init) ?? "missing"
            throw EngineError.response("Argon routing failed: upstream mode \(actual), expected \(Argon.mode). Check Google account access and cookie.")
        }
        let expected = request.extra?[Argon.thinkingField] ?? Argon.defaultThinkingLevel
        guard thinkingLevel == expected else {
            let actual = thinkingLevel.map(String.init) ?? "missing"
            throw EngineError.response("Argon thinking level mismatch: upstream \(actual), expected \(expected)")
        }
    }

    private static func decodeJSON(_ text: String) throws -> Any {
        do {
            return try JSONSerialization.jsonObject(with: Data(text.utf8))
        } catch {
            throw EngineError.response("Malformed Gemini response JSON: \(error.localizedDescription)")
        }
    }
}

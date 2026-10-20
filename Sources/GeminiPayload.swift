import Foundation

enum GeminiPayload {
    private static let fieldCount = 102
    private static let modeIndex = 79
    private static let metadataHeader = "X-Goog-Ext-525001261-Jspb"

    static func body(for request: GenerationRequest, xsrfToken: String?) -> String {
        var inner: [Any] = Array(repeating: NSNull(), count: fieldCount)
        inner[0] = [request.prompt, 0, NSNull(), NSNull(), NSNull(), NSNull(), 0]
        inner[1] = ["en"]
        inner[2] = ["", "", "", NSNull(), NSNull(), NSNull(), NSNull(), NSNull(), NSNull(), ""]
        inner[6] = [0]
        inner[7] = 1
        inner[10] = 1
        inner[11] = 0
        inner[17] = [[request.think]]
        inner[18] = 0
        inner[27] = 1
        inner[30] = [4]
        inner[41] = [2]
        inner[53] = 0
        inner[59] = UUID().uuidString.lowercased()
        inner[61] = [Any]()
        inner[68] = 1
        inner[modeIndex] = request.mode
        if let extra = request.extra {
            for (index, value) in extra { inner[index] = value }
        }
        var body = "f.req=" + formEncode(jsonString([NSNull(), jsonString(inner)]))
        if let token = xsrfToken, !token.isEmpty { body += "&at=" + formEncode(token) }
        return body
    }

    static func modelHeaders(for request: GenerationRequest) -> [String: String] {
        guard request.mode == Argon.mode else { return [:] }
        let level = request.extra?[Argon.thinkingField] ?? Argon.defaultThinkingLevel
        let metadata: [Any] = [
            1, NSNull(), NSNull(), NSNull(), Argon.modelID, NSNull(), NSNull(), 0, [4],
            NSNull(), NSNull(), 2, NSNull(), NSNull(), Argon.mode, level,
        ]
        return [metadataHeader: jsonString(metadata)]
    }
}

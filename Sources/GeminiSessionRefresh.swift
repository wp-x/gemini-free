import Foundation

struct GeminiSessionReply {
    let data: Data
    let response: HTTPURLResponse
}

struct GeminiSessionRefresh {
    typealias Send = (URLRequest, String?) throws -> GeminiSessionReply
    typealias Save = (GeminiSessionDocument, String) throws -> Void
    let send: Send
    let save: Save
    let now: () -> Date

    func renew(_ document: GeminiSessionDocument,
               settings: GeminiSessionSettings) throws -> GeminiSessionDocument {
        guard let path = settings.cookieFile, document.identity?.isEmpty == false else {
            throw GeminiSessionError.invalid("自动续期需要 Google 登录 Cookie（__Secure-1PSID），请重新导出")
        }
        let rotated = try rotate(document, settings: settings)
        let page = try send(pageRequest(rotated, settings: settings), settings.proxy)
        let current = rotated.merging(page.response, at: now())
        try save(current, path)
        try checkStatus(page.response)
        let metadata = try Self.metadata(from: page.data)
        let authenticated = current.authenticated(metadata: metadata, at: now(), authUser: settings.authUser)
        try save(authenticated, path)
        return authenticated
    }

    private func rotate(_ document: GeminiSessionDocument,
                        settings: GeminiSessionSettings) throws -> GeminiSessionDocument {
        guard document.needsRotation(at: now()) else { return document }
        var request = URLRequest(url: URL(string: "https://accounts.google.com/RotateCookies")!)
        request.httpMethod = "POST"
        request.timeoutInterval = GeminiNetworking.sessionTimeout
        request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        request.setValue("https://accounts.google.com", forHTTPHeaderField: "Origin")
        request.setValue(GeminiNetworking.userAgent, forHTTPHeaderField: "User-Agent")
        request.setValue(document.cookie, forHTTPHeaderField: "Cookie")
        request.httpBody = Data("[000,\"-0000000000000000000\"]".utf8)
        let reply = try send(request, settings.proxy)
        let merged = document.merging(reply.response, at: now())
        let hasRotation = GeminiSessionDocument.responseCookies(reply.response).contains {
            $0.name == "__Secure-1PSIDTS" && !$0.value.isEmpty && ($0.expiresDate ?? .distantFuture) > now()
        }
        let succeeded = reply.response.statusCode == 200 && hasRotation
        let rotated = succeeded ? merged.rotated(at: now()) : merged
        // Save server-side rotations even when status or the later page fetch fails.
        if let path = settings.cookieFile { try save(rotated, path) }
        try checkStatus(reply.response)
        guard hasRotation else {
            throw GeminiSessionError.invalid("Google 续期响应未返回有效的 __Secure-1PSIDTS Cookie")
        }
        return rotated
    }

    private func pageRequest(_ document: GeminiSessionDocument,
                             settings: GeminiSessionSettings) -> URLRequest {
        var components = URLComponents(string: "https://gemini.google.com")!
        let account = settings.authUser.map { "/u/\($0)" } ?? ""
        components.path = "\(account)/app"
        var request = URLRequest(url: components.url!)
        request.timeoutInterval = GeminiNetworking.sessionTimeout
        request.setValue(document.cookie, forHTTPHeaderField: "Cookie")
        request.setValue(GeminiNetworking.userAgent, forHTTPHeaderField: "User-Agent")
        return request
    }

    private func checkStatus(_ response: HTTPURLResponse) throws {
        guard response.statusCode == 200 else { throw GeminiSessionError.http(response.statusCode) }
    }

    static func metadata(from data: Data) throws -> [String: String] {
        let html = String(decoding: data, as: UTF8.self)
        return try ["SNlM0e", "cfb2h"].reduce(into: [:]) { result, key in
            let pattern = "\"\(key)\"\\s*:\\s*(\"(?:[^\"\\\\]|\\\\.)*\")"
            let regex = try NSRegularExpression(pattern: pattern)
            let range = NSRange(html.startIndex..., in: html)
            guard let match = regex.firstMatch(in: html, range: range),
                  let valueRange = Range(match.range(at: 1), in: html),
                  let value = try JSONSerialization.jsonObject(
                    with: Data(html[valueRange].utf8), options: [.fragmentsAllowed]) as? String,
                  !value.isEmpty else {
                throw GeminiSessionError.invalid("Gemini 页面缺少 \(key)，请确认 Google 登录仍有效")
            }
            result[key] = value
        }
    }
}

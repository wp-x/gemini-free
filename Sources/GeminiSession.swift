import Foundation

enum GeminiSessionError: Error, CustomStringConvertible {
    case invalid(String)
    case http(Int)

    var description: String {
        switch self {
        case .invalid(let message): return message
        case .http(let status) where status == 401 || status == 403:
            return "Google 拒绝会话续期（HTTP \(status)），请在浏览器确认登录状态并重新导出有效 Cookie"
        case .http(let status): return "Google 会话续期失败（HTTP \(status)）"
        }
    }
}

struct GeminiSessionSettings {
    let cookieFile: String?
    let autoRefresh: Bool
    let proxy: String?
    let authUser: String?
    let xsrfToken: String?
    let build: String

    init(store: Store) {
        cookieFile = store.cookieFile
        autoRefresh = store.sessionAutoRefresh
        proxy = store.proxy
        authUser = store.authUser
        xsrfToken = store.xsrfToken
        build = store.geminiBl
    }
}

struct GeminiSessionSnapshot {
    let cookieFile: String?
    let cookie: String
    let sapisid: String?
    let identity: String?
    let rotation: String?
    let xsrfToken: String?
    let build: String
}

struct GeminiSessionDocument {
    static let refreshInterval: TimeInterval = 180
    private let fields: [String: Any]

    init(fields: [String: Any] = [:]) {
        self.fields = fields
    }

    init(data: Data) throws {
        guard let content = String(data: data, encoding: .utf8) else {
            throw GeminiSessionError.invalid("Cookie 文件不是 UTF-8 文本")
        }
        let trimmed = content.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.hasPrefix("{") {
            guard let object = try JSONSerialization.jsonObject(with: data) as? [String: Any],
                  object["cookie"] is String else {
                throw GeminiSessionError.invalid("Cookie JSON 必须包含 cookie 字符串")
            }
            fields = object
        } else {
            fields = ["cookie": trimmed]
        }
    }

    var cookie: String { fields["cookie"] as? String ?? "" }

    var cookies: [String: String] {
        cookie.split(separator: ";").reduce(into: [:]) { result, pair in
            let parts = pair.split(separator: "=", maxSplits: 1)
            guard parts.count == 2 else { return }
            let name = parts[0].trimmingCharacters(in: .whitespaces)
            result[name] = String(parts[1]).trimmingCharacters(in: .whitespaces)
        }
    }

    var identity: String? { cookies["__Secure-1PSID"] }

    func needsRefresh(at date: Date, authUser: String? = nil) -> Bool {
        guard !cookie.isEmpty else { return false }
        guard fields["session_auth_user"] as? String == (authUser ?? "") else { return true }
        return isDue("session_refreshed_at", at: date) || isDue("session_rotated_at", at: date)
    }

    func needsRotation(at date: Date) -> Bool {
        isDue("session_rotated_at", at: date)
    }

    private func isDue(_ key: String, at date: Date) -> Bool {
        guard let timestamp = fields[key] as? TimeInterval else { return true }
        let elapsed = date.timeIntervalSince1970 - timestamp
        return elapsed < 0 || elapsed >= Self.refreshInterval
    }

    func rotated(at date: Date) -> GeminiSessionDocument {
        updating(["session_rotated_at": date.timeIntervalSince1970])
    }

    func snapshot(settings: GeminiSessionSettings) -> GeminiSessionSnapshot {
        let sapisid = cookies["SAPISID"] ?? fields["sapisid"] as? String
        return GeminiSessionSnapshot(
            cookieFile: settings.cookieFile, cookie: cookie,
            sapisid: sapisid?.isEmpty == false ? sapisid : nil,
            identity: identity, rotation: cookies["__Secure-1PSIDTS"],
            xsrfToken: fields["xsrf_token"] as? String ?? settings.xsrfToken,
            build: fields["gemini_bl"] as? String ?? settings.build)
    }

    func merging(_ response: HTTPURLResponse, at date: Date) -> GeminiSessionDocument {
        let received = Self.responseCookies(response)
        guard !received.isEmpty else { return self }
        let merged = received.reduce(into: cookies) { result, cookie in
            if let expiry = cookie.expiresDate, expiry <= date {
                result.removeValue(forKey: cookie.name)
            } else {
                result[cookie.name] = cookie.value
            }
        }
        let header = merged.keys.sorted().map { "\($0)=\(merged[$0]!)" }.joined(separator: "; ")
        let explicitSapisid = received.contains { $0.name == "SAPISID" } ? "" : fields["sapisid"] ?? ""
        return updating(["cookie": header, "sapisid": merged["SAPISID"] ?? explicitSapisid])
    }

    func authenticated(metadata: [String: String], at date: Date,
                       authUser: String? = nil) -> GeminiSessionDocument {
        updating(["xsrf_token": metadata["SNlM0e"]!, "gemini_bl": metadata["cfb2h"]!,
                  "session_auth_user": authUser ?? "",
                  "session_refreshed_at": date.timeIntervalSince1970])
    }

    func data() throws -> Data {
        try JSONSerialization.data(withJSONObject: fields, options: [.prettyPrinted, .sortedKeys])
    }

    static func responseCookies(_ response: HTTPURLResponse) -> [HTTPCookie] {
        guard let url = response.url else { return [] }
        let headers = response.allHeaderFields.reduce(into: [String: String]()) {
            $0[String(describing: $1.key)] = String(describing: $1.value)
        }
        return HTTPCookie.cookies(withResponseHeaderFields: headers, for: url).filter(accepts)
    }

    private func updating(_ values: [String: Any]) -> GeminiSessionDocument {
        GeminiSessionDocument(fields: fields.merging(values) { _, updated in updated })
    }

    private static func accepts(_ cookie: HTTPCookie) -> Bool {
        let domain = cookie.domain.lowercased().trimmingCharacters(in: CharacterSet(charactersIn: "."))
        return ["google.com", "gemini.google.com"].contains(domain) && cookie.path == "/"
    }
}

enum GeminiSessionFile {
    private static let privatePermissions = 0o600
    static func load(_ path: String?) throws -> GeminiSessionDocument {
        guard let path = path else { return GeminiSessionDocument() }
        do { return try GeminiSessionDocument(data: Data(contentsOf: URL(fileURLWithPath: path))) }
        catch { throw GeminiSessionError.invalid("无法读取 Cookie 文件：\(error)") }
    }

    static func save(_ document: GeminiSessionDocument, to path: String) throws {
        let manager = FileManager.default
        let destination = URL(fileURLWithPath: path)
        let temporary = destination.deletingLastPathComponent().appendingPathComponent(".session-\(UUID().uuidString)")
        defer { try? manager.removeItem(at: temporary) }
        do {
            guard manager.createFile(atPath: temporary.path, contents: nil,
                                     attributes: [.posixPermissions: privatePermissions]) else {
                throw GeminiSessionError.invalid("无法创建私有 Cookie 临时文件")
            }
            let handle = try FileHandle(forWritingTo: temporary)
            defer { try? handle.close() }
            try handle.write(contentsOf: document.data())
            try handle.synchronize()
            if manager.fileExists(atPath: path) {
                _ = try manager.replaceItemAt(destination, withItemAt: temporary, options: [.usingNewMetadataOnly])
            } else {
                try manager.moveItem(at: temporary, to: destination)
            }
        } catch {
            throw GeminiSessionError.invalid("无法保存 Cookie 文件：\(error)")
        }
    }
}

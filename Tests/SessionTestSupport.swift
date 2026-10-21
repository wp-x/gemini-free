import Foundation

enum SessionTest {
    static let date = Date(timeIntervalSince1970: 1_792_555_200)
    static let cookieFile = "/test/session.json"
    static let oldCookie = "__Secure-1PSID=test-identity; __Secure-1PSIDTS=old-ts; SAPISID=test-sapisid"
    static let page = Data(#"<script>window.WIZ_global_data={"SNlM0e":"new-xsrf","cfb2h":"new-build"};</script>"#.utf8)

    static func settings(enabled: Bool = true, path: String? = cookieFile) -> GeminiSessionSettings {
        let store = Store()
        store.cookieFile = path
        store.sessionAutoRefresh = enabled
        store.xsrfToken = "configured-xsrf"
        store.geminiBl = "configured-build"
        store.authUser = "0"
        store.proxy = "http://127.0.0.1:7890"
        return GeminiSessionSettings(store: store)
    }

    static func document(fresh: Bool = false) -> GeminiSessionDocument {
        let document = GeminiSessionDocument(fields: ["cookie": oldCookie, "custom": "preserved"])
        guard fresh else { return document }
        return document.rotated(at: date).authenticated(
            metadata: ["SNlM0e": "current-xsrf", "cfb2h": "current-build"], at: date, authUser: "0")
    }

    static func reply(url: URL, status: Int = 200, headers: [String: String] = [:]) -> GeminiSessionReply {
        GeminiSessionReply(data: Data(), response: HTTPURLResponse(
            url: url, statusCode: status, httpVersion: "HTTP/1.1", headerFields: headers)!)
    }

    static func pageReply(url: URL, data: Data) -> GeminiSessionReply {
        GeminiSessionReply(data: data, response: reply(url: url).response)
    }

    static func expectFailure(_ operation: () throws -> Void) {
        do { try operation(); preconditionFailure("Expected an explicit error") }
        catch {}
    }
}

final class SessionTestState: @unchecked Sendable {
    private let lock = NSLock()
    private var stored: GeminiSessionDocument
    private var instant = SessionTest.date
    private var renewalCount = 0

    init(document: GeminiSessionDocument = SessionTest.document()) { stored = document }

    func load() -> GeminiSessionDocument {
        lock.lock()
        defer { lock.unlock() }
        return stored
    }

    func save(_ document: GeminiSessionDocument) {
        lock.lock()
        defer { lock.unlock() }
        stored = document
    }

    func now() -> Date {
        lock.lock()
        defer { lock.unlock() }
        return instant
    }

    func advance() {
        lock.lock()
        defer { lock.unlock() }
        instant = instant.addingTimeInterval(GeminiSessionDocument.refreshInterval)
    }

    func renewed() -> Int {
        lock.lock()
        defer { lock.unlock() }
        renewalCount += 1
        return renewalCount
    }

    func count() -> Int {
        lock.lock()
        defer { lock.unlock() }
        return renewalCount
    }
}

import Foundation

@main
struct GeminiSessionTests {
    static func main() throws {
        try testCookieFormatsAndFile()
        try testCookieMerging()
        try testRenewalAndMetadata()
        try testInterruptedRenewal()
        testExplicitFailures()
        try testDisabledAndAnonymous()
        print("GeminiSessionTests passed")
    }

    private static func testCookieFormatsAndFile() throws {
        let plain = try GeminiSessionDocument(data: Data("SAPISID=value==;__Secure-1PSID=identity".utf8))
        precondition(plain.cookies["SAPISID"] == "value==")
        let root = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: root, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: root) }
        let path = root.appendingPathComponent("session.json").path
        try GeminiSessionFile.save(SessionTest.document(), to: path)
        let loaded = try GeminiSessionFile.load(path)
        precondition(loaded.identity == "test-identity")
        let fields = try JSONSerialization.jsonObject(with: loaded.data()) as! [String: Any]
        precondition(fields["custom"] as? String == "preserved")
        let attributes = try FileManager.default.attributesOfItem(atPath: path)
        precondition((attributes[.posixPermissions] as? NSNumber)?.intValue == 0o600)
        let supplied = loaded.snapshot(settings: SessionTest.settings())
        precondition(supplied.xsrfToken == "configured-xsrf")
        precondition(supplied.build == "configured-build")
        precondition(supplied.sapisid == "test-sapisid")
        SessionTest.expectFailure { _ = try GeminiSessionDocument(data: Data("{}".utf8)) }
        SessionTest.expectFailure { _ = try GeminiSessionFile.load(path + ".missing") }
    }

    private static func testCookieMerging() throws {
        let response = SessionTest.reply(url: URL(string: "https://accounts.google.com/RotateCookies")!,
            headers: ["Set-Cookie": "__Secure-1PSIDTS=new-ts==; Path=/; Domain=.google.com; Secure; HttpOnly; Partitioned"])
        let updated = SessionTest.document().merging(response.response, at: SessionTest.date)
        precondition(updated.cookies["__Secure-1PSIDTS"] == "new-ts==")
        precondition(updated.identity == "test-identity")
        let hostOnly = SessionTest.reply(url: response.response.url!,
            headers: ["Set-Cookie": "accounts-only=private; Path=/; Secure"])
        precondition(updated.merging(hostOnly.response, at: SessionTest.date).cookies["accounts-only"] == nil)
        let deleted = SessionTest.reply(url: response.response.url!,
            headers: ["Set-Cookie": "SAPISID=; Path=/; Domain=.google.com; Expires=Thu, 01 Jan 1970 00:00:00 GMT"])
        let removed = updated.merging(deleted.response, at: SessionTest.date)
        precondition(removed.cookies["SAPISID"] == nil)
        precondition(removed.snapshot(settings: SessionTest.settings()).sapisid == nil)
        let combined = SessionTest.reply(url: response.response.url!, headers: [
            "Set-Cookie": "__Secure-1PSIDTS=first; Domain=.google.com; Path=/; "
                + "Expires=Fri, 01 Jan 2028 00:00:00 GMT, "
                + "__Secure-1PSIDCC=second; Domain=.google.com; Path=/; Secure"])
        let multiple = updated.merging(combined.response, at: SessionTest.date)
        precondition(multiple.cookies["__Secure-1PSIDTS"] == "first")
        precondition(multiple.cookies["__Secure-1PSIDCC"] == "second")
    }

    private static func testRenewalAndMetadata() throws {
        let state = SessionTestState()
        var calls = 0
        let renewal = GeminiSessionRefresh(send: { request, proxy in
            calls += 1
            precondition(proxy == SessionTest.settings().proxy)
            if calls == 1 {
                precondition(request.httpMethod == "POST")
                precondition(String(data: request.httpBody!, encoding: .utf8) == "[000,\"-0000000000000000000\"]")
                return SessionTest.reply(url: request.url!, headers: [
                    "Set-Cookie": "__Secure-1PSIDTS=rotated; Path=/; Domain=.google.com; Secure"])
            }
            precondition(request.url!.path == "/u/0/app")
            precondition(request.value(forHTTPHeaderField: "Cookie")!.contains("__Secure-1PSIDTS=rotated"))
            precondition(state.load().cookies["__Secure-1PSIDTS"] == "rotated")
            return SessionTest.pageReply(url: request.url!, data: SessionTest.page)
        }, save: { document, _ in state.save(document) }, now: state.now)
        let renewed = try renewal.renew(state.load(), settings: SessionTest.settings())
        precondition(calls == 2)
        let session = renewed.snapshot(settings: SessionTest.settings())
        precondition(session.xsrfToken == "new-xsrf" && session.build == "new-build")
        precondition(!renewed.needsRefresh(at: SessionTest.date, authUser: "0"))
        precondition(renewed.needsRefresh(at: SessionTest.date, authUser: "1"))
        precondition(renewed.needsRefresh(
            at: SessionTest.date.addingTimeInterval(GeminiSessionDocument.refreshInterval), authUser: "0"))
        let escaped = Data(#"{"SNlM0e":"token\u003d","cfb2h":"build"}"#.utf8)
        let metadata = try GeminiSessionRefresh.metadata(from: escaped)
        precondition(metadata["SNlM0e"] == "token=")
    }

    private static func testInterruptedRenewal() throws {
        let state = SessionTestState()
        var rotations = 0
        var validPage = false
        let renewal = GeminiSessionRefresh(send: { request, _ in
            if request.httpMethod == "POST" {
                rotations += 1
                return SessionTest.reply(url: request.url!, headers: [
                    "Set-Cookie": "__Secure-1PSIDTS=persisted-before-error; Path=/; Domain=.google.com"])
            }
            return SessionTest.pageReply(url: request.url!, data: validPage ? SessionTest.page : Data())
        }, save: { document, _ in state.save(document) }, now: state.now)
        SessionTest.expectFailure { _ = try renewal.renew(state.load(), settings: SessionTest.settings()) }
        precondition(state.load().cookies["__Secure-1PSIDTS"] == "persisted-before-error")
        precondition(state.load().needsRefresh(at: SessionTest.date, authUser: "0"))
        validPage = true
        _ = try renewal.renew(state.load(), settings: SessionTest.settings())
        precondition(rotations == 1, "Metadata failure must not trigger another cookie rotation")
        precondition(!state.load().needsRefresh(at: SessionTest.date, authUser: "0"))
    }

    private static func testExplicitFailures() {
        let rejected = GeminiSessionRefresh(send: { request, _ in
            SessionTest.reply(url: request.url!, status: 401)
        }, save: { _, _ in }, now: { SessionTest.date })
        do {
            _ = try rejected.renew(SessionTest.document(), settings: SessionTest.settings())
            preconditionFailure("Revoked login must fail")
        } catch GeminiSessionError.http(let status) { precondition(status == 401) }
        catch { preconditionFailure("Unexpected error: \(error)") }
        var calls = 0
        let unwritable = GeminiSessionRefresh(send: { request, _ in
            calls += 1
            return SessionTest.reply(url: request.url!, headers: [
                "Set-Cookie": "__Secure-1PSIDTS=changed; Path=/; Domain=.google.com"])
        }, save: { _, _ in throw GeminiSessionError.invalid("test write denied") }, now: { SessionTest.date })
        SessionTest.expectFailure { _ = try unwritable.renew(SessionTest.document(), settings: SessionTest.settings()) }
        precondition(calls == 1, "Do not fetch a page after failing to persist rotated cookies")
        SessionTest.expectFailure { _ = try GeminiSessionRefresh.metadata(from: Data("login required".utf8)) }
    }

    private static func testDisabledAndAnonymous() throws {
        for options in [SessionTest.settings(enabled: false), SessionTest.settings(path: nil)] {
            let document = options.cookieFile == nil ? GeminiSessionDocument() : SessionTest.document()
            let dependencies = GeminiSessionDependencies(
                load: { _ in document }, save: { _, _ in preconditionFailure("Unexpected save") },
                renew: { _, _ in preconditionFailure("Unexpected renewal") },
                now: { SessionTest.date }, report: { _ in })
            let sessions = GeminiSessions(settings: { options }, dependencies: dependencies)
            try sessions.refreshIfNeeded()
            let snapshot = try sessions.acquire()
            sessions.release()
            precondition(snapshot.cookie == document.cookie)
        }
    }
}

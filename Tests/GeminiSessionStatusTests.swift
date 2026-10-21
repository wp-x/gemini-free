import Foundation

@main
struct GeminiSessionStatusTests {
    static func main() throws {
        try testManualRenewal()
        try testFailuresRemainVisible()
        try testMissingRotationFails()
        try testDisabledBackgroundDoesNotRead()
        print("GeminiSessionStatusTests passed")
    }

    private static func coordinator(
        state: SessionTestState,
        options: GeminiSessionSettings,
        renew: @escaping (GeminiSessionDocument, GeminiSessionSettings) throws -> GeminiSessionDocument
    ) -> GeminiSessions {
        let dependencies = GeminiSessionDependencies(
            load: { _ in state.load() }, save: { document, _ in state.save(document) },
            renew: renew, now: state.now, report: { _ in })
        return GeminiSessions(settings: { options }, dependencies: dependencies)
    }

    private static func testManualRenewal() throws {
        let state = SessionTestState(document: SessionTest.document(fresh: true))
        let sessions = coordinator(state: state, options: SessionTest.settings(enabled: false)) { document, _ in
            _ = state.renewed()
            return document
        }
        var statuses: [String] = []
        sessions.statusDidChange = { statuses.append(sessions.status) }
        try sessions.refreshIfNeeded()
        precondition(state.count() == 0)
        precondition(sessions.status.contains("已关闭"))
        try sessions.refreshNow()
        precondition(state.count() == 1, "Manual renewal must run even when automatic renewal is disabled")
        precondition(sessions.status.contains("续期成功"))
        precondition(statuses.contains("登录会话：正在续期"))
    }

    private static func testFailuresRemainVisible() throws {
        let state = SessionTestState()
        let sessions = coordinator(state: state, options: SessionTest.settings()) { _, _ in
            throw GeminiSessionError.http(401)
        }
        SessionTest.expectFailure { try sessions.refreshIfNeeded() }
        precondition(sessions.status.contains("续期失败"))
        precondition(sessions.status.contains("HTTP 401"))
        SessionTest.expectFailure { _ = try sessions.snapshot() }
        precondition(state.load().needsRotation(at: state.now()))
        precondition(!String(describing: GeminiSessionError.http(403)).contains("登录已失效"))
    }

    private static func testMissingRotationFails() throws {
        let state = SessionTestState()
        let renewal = GeminiSessionRefresh(send: { request, _ in
            precondition(request.httpMethod == "POST", "Do not fetch metadata after an invalid rotation")
            return SessionTest.reply(url: request.url!)
        }, save: { document, _ in state.save(document) }, now: state.now)
        SessionTest.expectFailure { _ = try renewal.renew(state.load(), settings: SessionTest.settings()) }
        precondition(state.load().needsRotation(at: state.now()), "HTTP 200 alone must not count as renewal")
    }

    private static func testDisabledBackgroundDoesNotRead() throws {
        let dependencies = GeminiSessionDependencies(
            load: { _ in preconditionFailure("Disabled background renewal must not read cookies") },
            save: { _, _ in preconditionFailure("Unexpected save") },
            renew: { _, _ in preconditionFailure("Unexpected renewal") },
            now: { SessionTest.date }, report: { _ in })
        let sessions = GeminiSessions(
            settings: { SessionTest.settings(enabled: false) }, dependencies: dependencies)
        try sessions.refreshIfNeeded()
        precondition(sessions.status.contains("已关闭"))
    }
}

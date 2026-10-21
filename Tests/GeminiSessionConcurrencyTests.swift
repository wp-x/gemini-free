import Foundation

@main
struct GeminiSessionConcurrencyTests {
    private static let waitSeconds: TimeInterval = 3
    private static let concurrentRequests = 12

    static func main() throws {
        try testConcurrentRenewal()
        try testRenewalDuringGeneration()
        try testAccountReplacement()
        print("GeminiSessionConcurrencyTests passed")
    }

    private static func coordinator(
        state: SessionTestState,
        renew: @escaping (GeminiSessionDocument, GeminiSessionSettings) throws -> GeminiSessionDocument
    ) -> GeminiSessions {
        let dependencies = GeminiSessionDependencies(
            load: { _ in state.load() }, save: { document, _ in state.save(document) },
            renew: renew, now: state.now, report: { _ in })
        return GeminiSessions(settings: { SessionTest.settings() }, dependencies: dependencies)
    }

    private static func testConcurrentRenewal() throws {
        let state = SessionTestState()
        let started = DispatchSemaphore(value: 0)
        let proceed = DispatchSemaphore(value: 0)
        let sessions = coordinator(state: state) { document, _ in
            precondition(state.renewed() == 1)
            started.signal()
            precondition(proceed.wait(timeout: .now() + waitSeconds) == .success)
            let renewed = document.rotated(at: state.now()).authenticated(
                metadata: ["SNlM0e": "fresh", "cfb2h": "build"], at: state.now(), authUser: "0")
            state.save(renewed)
            return renewed
        }
        let group = DispatchGroup()
        for _ in 0..<concurrentRequests {
            group.enter()
            DispatchQueue.global().async {
                defer { group.leave() }
                do {
                    let snapshot = try sessions.snapshot()
                    precondition(snapshot.xsrfToken == "fresh")
                } catch { preconditionFailure("Concurrent acquire failed: \(error)") }
            }
        }
        precondition(started.wait(timeout: .now() + waitSeconds) == .success)
        proceed.signal()
        precondition(group.wait(timeout: .now() + waitSeconds) == .success)
        precondition(state.count() == 1)
        let afterRestart = coordinator(state: state) { _, _ in preconditionFailure("Recent session renewed again") }
        _ = try afterRestart.snapshot()
    }

    private static func testRenewalDuringGeneration() throws {
        let state = SessionTestState(document: SessionTest.document(fresh: true))
        let sessions = coordinator(state: state) { document, _ in
            _ = state.renewed()
            let rotation = SessionTest.reply(url: URL(string: "https://accounts.google.com/RotateCookies")!,
                headers: ["Set-Cookie": "__Secure-1PSIDTS=rotated; Domain=.google.com; Path=/"])
            let renewed = document.merging(rotation.response, at: state.now()).rotated(at: state.now()).authenticated(
                metadata: ["SNlM0e": "after", "cfb2h": "build"], at: state.now(), authUser: "0")
            state.save(renewed)
            return renewed
        }
        let active = try sessions.snapshot()
        state.advance()
        try sessions.refreshIfNeeded()
        precondition(state.count() == 1, "Long generations must not postpone renewal")
        let response = SessionTest.reply(url: URL(string: "https://gemini.google.com/app")!, headers: [
            "Set-Cookie": "__Secure-1PSIDTS=stale-generation-cookie; Path=/; Domain=.google.com"])
        try sessions.receive(response.response, for: active)
        precondition(state.load().cookies["__Secure-1PSIDTS"] == "rotated")
        let next = try sessions.snapshot()
        precondition(next.xsrfToken == "after")
    }

    private static func testAccountReplacement() throws {
        let state = SessionTestState(document: SessionTest.document(fresh: true))
        let sessions = coordinator(state: state) { _, _ in preconditionFailure("Unexpected renewal") }
        let old = try sessions.snapshot()
        state.save(GeminiSessionDocument(fields: ["cookie": "__Secure-1PSID=different-account"]))
        let response = SessionTest.reply(url: URL(string: "https://gemini.google.com/app")!, headers: [
            "Set-Cookie": "__Secure-1PSIDTS=old-account-cookie; Path=/; Domain=.google.com"])
        SessionTest.expectFailure { try sessions.receive(response.response, for: old) }
        precondition(state.load().identity == "different-account")
        precondition(state.load().cookies["__Secure-1PSIDTS"] == nil)
    }
}

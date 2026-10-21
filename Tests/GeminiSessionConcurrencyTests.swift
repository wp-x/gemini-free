import Foundation

@main
struct GeminiSessionConcurrencyTests {
    private static let waitSeconds: TimeInterval = 3
    private static let concurrentRequests = 12

    static func main() throws {
        try testConcurrentRenewal()
        try testRenewalWaitsForGeneration()
        try testAccountReplacement()
        print("GeminiSessionConcurrencyTests passed")
    }

    private static func coordinator(
        state: SessionTestState,
        load: ((String?) throws -> GeminiSessionDocument)? = nil,
        renew: @escaping (GeminiSessionDocument, GeminiSessionSettings) throws -> GeminiSessionDocument
    ) -> GeminiSessions {
        let dependencies = GeminiSessionDependencies(
            load: load ?? { _ in state.load() }, save: { document, _ in state.save(document) },
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
            let renewed = document.authenticated(
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
                    let snapshot = try sessions.acquire()
                    defer { sessions.release() }
                    precondition(snapshot.xsrfToken == "fresh")
                } catch { preconditionFailure("Concurrent acquire failed: \(error)") }
            }
        }
        precondition(started.wait(timeout: .now() + waitSeconds) == .success)
        proceed.signal()
        precondition(group.wait(timeout: .now() + waitSeconds) == .success)
        precondition(state.count() == 1)
        let afterRestart = coordinator(state: state) { _, _ in preconditionFailure("Recent session renewed again") }
        _ = try afterRestart.acquire()
        afterRestart.release()
    }

    private static func testRenewalWaitsForGeneration() throws {
        let state = SessionTestState(document: SessionTest.document(fresh: true))
        let dueRead = DispatchSemaphore(value: 0)
        let completed = DispatchSemaphore(value: 0)
        let sessions = coordinator(state: state, load: { _ in
            if state.now() != SessionTest.date { dueRead.signal() }
            return state.load()
        }) { document, _ in
            precondition(document.cookies["__Secure-1PSIDTS"] == "generation-cookie")
            _ = state.renewed()
            let renewed = document.authenticated(
                metadata: ["SNlM0e": "after", "cfb2h": "build"], at: state.now(), authUser: "0")
            state.save(renewed)
            return renewed
        }
        let active = try sessions.acquire()
        state.advance()
        DispatchQueue.global().async {
            defer { completed.signal() }
            do { try sessions.refreshIfNeeded() }
            catch { preconditionFailure("Background refresh failed: \(error)") }
        }
        precondition(dueRead.wait(timeout: .now() + waitSeconds) == .success)
        precondition(state.count() == 0, "Do not rotate during an active generation")
        let response = SessionTest.reply(url: URL(string: "https://gemini.google.com/app")!, headers: [
            "Set-Cookie": "__Secure-1PSIDTS=generation-cookie; Path=/; Domain=.google.com"])
        try sessions.receive(response.response, for: active)
        sessions.release()
        precondition(completed.wait(timeout: .now() + waitSeconds) == .success)
        precondition(state.count() == 1)
        let next = try sessions.acquire()
        sessions.release()
        precondition(next.xsrfToken == "after")
    }

    private static func testAccountReplacement() throws {
        let state = SessionTestState(document: SessionTest.document(fresh: true))
        let sessions = coordinator(state: state) { _, _ in preconditionFailure("Unexpected renewal") }
        let old = try sessions.acquire()
        defer { sessions.release() }
        state.save(GeminiSessionDocument(fields: ["cookie": "__Secure-1PSID=different-account"]))
        let response = SessionTest.reply(url: URL(string: "https://gemini.google.com/app")!, headers: [
            "Set-Cookie": "__Secure-1PSIDTS=old-account-cookie; Path=/; Domain=.google.com"])
        SessionTest.expectFailure { try sessions.receive(response.response, for: old) }
        precondition(state.load().identity == "different-account")
        precondition(state.load().cookies["__Secure-1PSIDTS"] == nil)
    }
}

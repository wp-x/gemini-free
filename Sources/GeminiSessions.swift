import Foundation

protocol GeminiSessionProviding {
    func acquire() throws -> GeminiSessionSnapshot
    func release()
    func receive(_ response: HTTPURLResponse, for snapshot: GeminiSessionSnapshot) throws
}

struct GeminiSessionDependencies {
    let load: (String?) throws -> GeminiSessionDocument
    let save: (GeminiSessionDocument, String) throws -> Void
    let renew: (GeminiSessionDocument, GeminiSessionSettings) throws -> GeminiSessionDocument
    let now: () -> Date
    let report: (String) -> Void
}

final class GeminiSessions: GeminiSessionProviding {
    static let shared = configured()
    private let settings: () -> GeminiSessionSettings
    private let dependencies: GeminiSessionDependencies
    private let condition = NSCondition()
    private var activeRequests = 0
    private var refreshing = false
    private var timer: DispatchSourceTimer?

    init(settings: @escaping () -> GeminiSessionSettings, dependencies: GeminiSessionDependencies) {
        self.settings = settings
        self.dependencies = dependencies
    }

    func acquire() throws -> GeminiSessionSnapshot {
        condition.lock()
        defer { condition.unlock() }
        while refreshing { condition.wait() }
        let options = settings()
        let document = try refreshedDocument(settings: options)
        activeRequests += 1
        return document.snapshot(settings: options)
    }

    func release() {
        condition.lock()
        defer { condition.unlock() }
        activeRequests -= 1
        condition.broadcast()
    }

    func receive(_ response: HTTPURLResponse, for snapshot: GeminiSessionSnapshot) throws {
        guard let path = snapshot.cookieFile else { return }
        condition.lock()
        defer { condition.unlock() }
        let document = try dependencies.load(path)
        guard document.identity == snapshot.identity else {
            throw GeminiSessionError.invalid("生成期间登录账号发生变化，无法保存旧会话的 Cookie")
        }
        let updated = document.merging(response, at: dependencies.now())
        if updated.cookie != document.cookie { try dependencies.save(updated, path) }
    }

    func refreshIfNeeded() throws {
        condition.lock()
        defer { condition.unlock() }
        while refreshing { condition.wait() }
        _ = try refreshedDocument(settings: settings())
    }

    private func refreshedDocument(settings: GeminiSessionSettings) throws -> GeminiSessionDocument {
        let document = try dependencies.load(settings.cookieFile)
        guard settings.autoRefresh,
              document.needsRefresh(at: dependencies.now(), authUser: settings.authUser) else { return document }
        refreshing = true
        defer { refreshing = false; condition.broadcast() }
        while activeRequests > 0 { condition.wait() }
        let latest = try dependencies.load(settings.cookieFile)
        guard latest.needsRefresh(at: dependencies.now(), authUser: settings.authUser) else { return latest }
        let renewed = try dependencies.renew(latest, settings)
        dependencies.report("Google 会话续期成功，Cookie 和会话参数已保存")
        return renewed
    }

    func start() {
        stop()
        let source = DispatchSource.makeTimerSource(queue: .global(qos: .utility))
        source.schedule(deadline: .now(), repeating: GeminiSessionDocument.refreshInterval)
        source.setEventHandler { [weak self] in self?.refreshInBackground() }
        timer = source
        source.resume()
    }

    func stop() {
        timer?.cancel()
        timer = nil
    }

    private func refreshInBackground() {
        do { try refreshIfNeeded() }
        catch { dependencies.report("Google 自动续期失败：\(error)") }
    }

    private static func configured() -> GeminiSessions {
        let refresh = GeminiSessionRefresh(
            send: GeminiNetworking.send, save: GeminiSessionFile.save, now: Date.init)
        let dependencies = GeminiSessionDependencies(
            load: GeminiSessionFile.load, save: GeminiSessionFile.save,
            renew: refresh.renew, now: Date.init, report: { message in
                FileHandle.standardError.write(Data("[Session] \(message)\n".utf8))
            })
        return GeminiSessions(
            settings: { GeminiSessionSettings(store: Store.shared) }, dependencies: dependencies)
    }
}

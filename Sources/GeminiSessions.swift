import Foundation

protocol GeminiSessionProviding {
    func snapshot() throws -> GeminiSessionSnapshot
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
    private static let checkInterval: TimeInterval = 30
    private let settings: () -> GeminiSessionSettings
    private let dependencies: GeminiSessionDependencies
    private let lock = NSLock()
    private let statusLock = NSLock()
    private var statusText = "登录会话：等待检查"
    private var timer: DispatchSourceTimer?
    var statusDidChange: (() -> Void)?

    var status: String {
        statusLock.lock()
        defer { statusLock.unlock() }
        return statusText
    }

    init(settings: @escaping () -> GeminiSessionSettings, dependencies: GeminiSessionDependencies) {
        self.settings = settings
        self.dependencies = dependencies
    }

    func snapshot() throws -> GeminiSessionSnapshot {
        lock.lock()
        defer { lock.unlock() }
        let options = settings()
        let document = try refreshedDocument(settings: options)
        return document.snapshot(settings: options)
    }

    func receive(_ response: HTTPURLResponse, for snapshot: GeminiSessionSnapshot) throws {
        guard let path = snapshot.cookieFile else { return }
        lock.lock()
        defer { lock.unlock() }
        let document = try dependencies.load(path)
        guard document.identity == snapshot.identity else {
            throw GeminiSessionError.invalid("生成期间登录账号发生变化，无法保存旧会话的 Cookie")
        }
        guard document.cookies["__Secure-1PSIDTS"] == snapshot.rotation else {
            dependencies.report("会话已轮换，保留新 Cookie，不写入旧请求返回的 Cookie")
            return
        }
        let updated = document.merging(response, at: dependencies.now())
        if updated.cookie != document.cookie { try dependencies.save(updated, path) }
    }

    func refreshIfNeeded() throws {
        lock.lock()
        defer { lock.unlock() }
        let options = settings()
        guard options.autoRefresh else {
            updateStatus("登录会话：自动续期已关闭")
            return
        }
        _ = try refreshedDocument(settings: options)
    }

    func refreshNow() throws {
        lock.lock()
        defer { lock.unlock() }
        _ = try refreshedDocument(settings: settings(), force: true)
    }

    private func refreshedDocument(settings: GeminiSessionSettings,
                                   force: Bool = false) throws -> GeminiSessionDocument {
        do { return try checkedDocument(settings: settings, force: force) }
        catch {
            let detail: String
            if case GeminiSessionError.http(let status) = error { detail = "（HTTP \(status)）" }
            else { detail = "" }
            updateStatus("登录会话：续期失败\(detail)")
            throw error
        }
    }

    private func checkedDocument(settings: GeminiSessionSettings,
                                 force: Bool) throws -> GeminiSessionDocument {
        let document = try dependencies.load(settings.cookieFile)
        guard settings.autoRefresh || force else {
            updateStatus("登录会话：自动续期已关闭")
            return document
        }
        guard force || !document.cookie.isEmpty else {
            updateStatus("登录会话：匿名模式")
            return document
        }
        guard force || document.needsRefresh(at: dependencies.now(), authUser: settings.authUser) else {
            updateStatus("登录会话：自动续期已开启")
            return document
        }
        updateStatus("登录会话：正在续期")
        let renewed = try dependencies.renew(document, settings)
        dependencies.report("Google 会话续期成功，Cookie 和会话参数已保存")
        updateStatus("登录会话：续期成功")
        return renewed
    }

    private func updateStatus(_ text: String) {
        statusLock.lock()
        let changed = statusText != text
        statusText = text
        statusLock.unlock()
        if changed { statusDidChange?() }
    }

    func start() {
        stop()
        let source = DispatchSource.makeTimerSource(queue: .global(qos: .utility))
        source.schedule(deadline: .now(), repeating: Self.checkInterval)
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

import Foundation
import CryptoKit

enum EngineError: Error, CustomStringConvertible {
    case http(Int)
    case other(String)
    case response(String)
    var description: String {
        switch self {
        case .http(let c): return "HTTP \(c)"
        case .other(let s), .response(let s): return s
        }
    }
}

struct GenerationRequest {
    let prompt: String
    let mode: Int
    let think: Int
    let extra: [Int: Int]?
}

protocol TextGenerating {
    func generate(_ request: GenerationRequest) throws -> String
    func generateStream(
        _ request: GenerationRequest,
        isCancelled: @escaping () -> Bool,
        onDelta: @escaping (String) -> Void
    ) throws
}

// Gemini StreamGenerate 协议实现，忠实移植自上游 gemini.py
final class Engine: TextGenerating {
    static let shared = Engine(config: Store.shared, sessions: GeminiSessions.shared)
    private let cfg: Store
    private let sessions: GeminiSessionProviding

    init(config: Store, sessions: GeminiSessionProviding) {
        cfg = config
        self.sessions = sessions
    }

    func log(_ msg: String) {
        guard cfg.logRequests else { return }
        let ts = DateFormatter.localizedString(from: Date(), dateStyle: .none, timeStyle: .medium)
        FileHandle.standardError.write("[\(ts)] \(msg)\n".data(using: .utf8)!)
    }

    // MARK: Cookie

    private func sapisidHash(_ sapisid: String) -> String {
        let ts = nowUnix()
        let digest = Insecure.SHA1.hash(data: Data("\(ts) \(sapisid) https://gemini.google.com".utf8))
        let hex = digest.map { String(format: "%02x", $0) }.joined()
        return "SAPISIDHASH \(ts)_\(hex)"
    }

    private func accountPrefix() -> String {
        guard let u = cfg.authUser, !u.isEmpty else { return "" }
        return "/u/\(u)"
    }

    private func buildHeaders(_ request: GenerationRequest,
                              session: GeminiSessionSnapshot) -> [String: String] {
        let prefix = accountPrefix()
        var h: [String: String] = [
            "Content-Type": "application/x-www-form-urlencoded",
            "Origin": "https://gemini.google.com",
            "Referer": "https://gemini.google.com\(prefix)/app",
            "X-Same-Domain": "1",
            "User-Agent": GeminiNetworking.userAgent,
        ]
        if let u = cfg.authUser, !prefix.isEmpty { h["X-Goog-AuthUser"] = u }
        if !session.cookie.isEmpty { h["Cookie"] = session.cookie }
        if let sapisid = session.sapisid { h["Authorization"] = sapisidHash(sapisid) }
        h.merge(GeminiPayload.modelHeaders(for: request)) { _, value in value }
        return h
    }

    // MARK: Payload

    private func requestURL(build: String) -> String {
        let reqid = nowUnix() % 1000000
        return "https://gemini.google.com\(accountPrefix())/_/BardChatUi/data/"
            + "assistant.lamda.BardFrontendService/StreamGenerate"
            + "?bl=\(build)&hl=en&_reqid=\(reqid)&rt=c"
    }

    // MARK: Response parsing（移植自 gemini.py）

    func cleanText(_ text: String) -> String {
        stripArtifacts(text).trimmingCharacters(in: .whitespacesAndNewlines)
    }

    // 仅去除代码工件，不 trim（流式增量需保留首尾空白）
    private func stripArtifacts(_ text: String) -> String {
        var t = text
        let p1 = try! NSRegularExpression(
            pattern: "```(?:python|javascript|text)\\?code_(?:reference|stdout)&code_event_index=\\d+\\n.*?```\\n?",
            options: [.dotMatchesLineSeparators])
        t = p1.stringByReplacingMatches(in: t, range: NSRange(location: 0, length: (t as NSString).length), withTemplate: "")
        let p2 = try! NSRegularExpression(pattern: "http://googleusercontent\\.com/card_content/\\d+\\n?")
        t = p2.stringByReplacingMatches(in: t, range: NSRange(location: 0, length: (t as NSString).length), withTemplate: "")
        return t
    }

    // MARK: 请求（带重试），逐行回调

    private func perform(
        _ request: GenerationRequest,
        isCancelled: @escaping () -> Bool = { false },
        onLine: @escaping (String) throws -> Void
    ) throws {
        if isCancelled() { return }
        let session = try sessions.snapshot()
        if isCancelled() { return }
        let body = GeminiPayload.body(for: request, xsrfToken: session.xsrfToken)
        var req = URLRequest(url: URL(string: requestURL(build: session.build))!)
        req.httpMethod = "POST"
        req.httpBody = body.data(using: .utf8)
        req.timeoutInterval = cfg.requestTimeoutSec
        for (k, v) in buildHeaders(request, session: session) { req.setValue(v, forHTTPHeaderField: k) }
        try performWithRetries(req) {
            LineCollector(isCancelled: isCancelled, onLine: onLine,
                          onResponse: { try self.sessions.receive($0, for: session) })
        }
    }

    private func performWithRetries(_ req: URLRequest, collector: () -> LineCollector) throws {
        var lastErr: Error = EngineError.other("no attempt")
        for attempt in 0..<cfg.retryAttempts {
            do {
                try streamRequest(req, collector: collector())
                return
            } catch {
                // 4xx（cookie 失效等）重试无意义，直接抛出
                if case EngineError.http(let c) = error, c < 500 { throw error }
                if case EngineError.response = error { throw error }
                if error is GeminiSessionError { throw error }
                lastErr = error
                if attempt < cfg.retryAttempts - 1 {
                    log("Retry \(attempt + 1)/\(cfg.retryAttempts): \(error)")
                    Thread.sleep(forTimeInterval: cfg.retryDelaySec)
                }
            }
        }
        throw lastErr
    }

    private func streamRequest(_ req: URLRequest, collector: LineCollector) throws {
        let config = GeminiNetworking.configuration(proxy: cfg.proxy, timeout: cfg.requestTimeoutSec)
        let session = URLSession(configuration: config, delegate: collector, delegateQueue: nil)
        let task = session.dataTask(with: req)
        collector.task = task
        task.resume()
        collector.sem.wait()
        session.finishTasksAndInvalidate()
        if let e = collector.error {
            // 客户端断开触发的主动取消视为正常结束
            if collector.isCancelled(), (e as NSError).code == NSURLErrorCancelled { return }
            throw e
        }
        if let code = collector.statusCode, code >= 400 { throw EngineError.http(code) }
    }

    // MARK: 对外 API

    func generate(_ request: GenerationRequest) throws -> String {
        var longest = ""
        var longestN = 0
        try perform(request) { line in
            for text in try GeminiResponseFrame.texts(from: line, for: request) {
                let count = text.unicodeScalars.count
                if count > longestN { longest = text; longestN = count }
            }
        }
        let output = cleanText(longest)
        guard !output.isEmpty else { throw EngineError.response("Gemini returned no text") }
        return output
    }

    func generateStream(
        _ request: GenerationRequest,
        isCancelled: @escaping () -> Bool = { false },
        onDelta: @escaping (String) -> Void
    ) throws {
        var prev = ""
        try perform(request, isCancelled: isCancelled) { line in
            for text in try GeminiResponseFrame.texts(from: line, for: request) {
                prev = self.emitDelta(text, previous: prev, onDelta: onDelta)
            }
        }
        if prev.isEmpty && !isCancelled() { throw EngineError.response("Gemini returned no text") }
    }

    private func emitDelta(
        _ text: String, previous: String, onDelta: (String) -> Void
    ) -> String {
        let previousCount = previous.unicodeScalars.count
        let scalars = Array(text.unicodeScalars)
        guard scalars.count > previousCount else { return previous }
        let delta = stripArtifacts(String(String.UnicodeScalarView(scalars[previousCount...])))
        if !delta.isEmpty { onDelta(delta) }
        return text
    }
}

// URLSession 流式收集器：按 \n 切行回调
private final class LineCollector: NSObject, URLSessionDataDelegate {
    let isCancelled: () -> Bool
    let onLine: (String) throws -> Void
    let onResponse: (HTTPURLResponse) throws -> Void
    let sem = DispatchSemaphore(value: 0)
    var error: Error?
    var statusCode: Int?
    weak var task: URLSessionDataTask?
    private var buffer = Data()

    init(isCancelled: @escaping () -> Bool, onLine: @escaping (String) throws -> Void,
         onResponse: @escaping (HTTPURLResponse) throws -> Void) {
        self.isCancelled = isCancelled
        self.onLine = onLine
        self.onResponse = onResponse
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask,
                    didReceive response: URLResponse, completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
        statusCode = (response as? HTTPURLResponse)?.statusCode
        do {
            if let response = response as? HTTPURLResponse { try onResponse(response) }
            completionHandler(.allow)
        } catch {
            self.error = error
            completionHandler(.cancel)
        }
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
        if isCancelled() { task?.cancel(); return }
        guard error == nil else { return }
        buffer.append(data)
        while error == nil, let nl = buffer.firstIndex(of: 0x0A) {
            let lineData = Data(buffer[buffer.startIndex..<nl])
            buffer = Data(buffer[buffer.index(after: nl)...])
            emitLine(lineData)
        }
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError err: Error?) {
        if error == nil, !buffer.isEmpty { emitLine(buffer); buffer = Data() }
        if error == nil { error = err }
        sem.signal()
    }

    private func emitLine(_ data: Data) {
        do {
            try onLine(String(decoding: data, as: UTF8.self))
        } catch {
            self.error = error
            task?.cancel()
        }
    }
}

import Foundation
import CryptoKit

enum EngineError: Error, CustomStringConvertible {
    case http(Int)
    case other(String)
    var description: String {
        switch self {
        case .http(let c): return "HTTP \(c)"
        case .other(let s): return s
        }
    }
}

// Gemini StreamGenerate 协议实现，忠实移植自上游 gemini.py
final class Engine {
    static let shared = Engine()
    private let cfg = Store.shared

    func log(_ msg: String) {
        guard cfg.logRequests else { return }
        let ts = DateFormatter.localizedString(from: Date(), dateStyle: .none, timeStyle: .medium)
        FileHandle.standardError.write("[\(ts)] \(msg)\n".data(using: .utf8)!)
    }

    // MARK: Cookie

    private func loadCookie() -> (String, String?) {
        guard let file = cfg.cookieFile, FileManager.default.fileExists(atPath: file),
              let content = try? String(contentsOfFile: file, encoding: .utf8) else { return ("", nil) }
        let trimmed = content.trimmingCharacters(in: .whitespacesAndNewlines)
        if trimmed.hasPrefix("{") {
            if let data = trimmed.data(using: .utf8),
               let d = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] {
                let cookieStr = d["cookie"] as? String ?? ""
                let sapisid = d["sapisid"] as? String
                return (cookieStr, (sapisid?.isEmpty ?? true) ? nil : sapisid)
            }
            return ("", nil)
        }
        var sapisid: String? = nil
        for pair in trimmed.components(separatedBy: "; ") {
            let kv = pair.components(separatedBy: "=")
            if kv.count >= 2, kv[0] == "SAPISID" { sapisid = kv.dropFirst().joined(separator: "=") }
        }
        return (trimmed, (sapisid?.isEmpty ?? true) ? nil : sapisid)
    }

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

    private func buildHeaders() -> [String: String] {
        let prefix = accountPrefix()
        var h: [String: String] = [
            "Content-Type": "application/x-www-form-urlencoded",
            "Origin": "https://gemini.google.com",
            "Referer": "https://gemini.google.com\(prefix)/app",
            "X-Same-Domain": "1",
            "User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36",
        ]
        if let u = cfg.authUser, !prefix.isEmpty { h["X-Goog-AuthUser"] = u }
        let (cookieStr, sapisid) = loadCookie()
        if !cookieStr.isEmpty { h["Cookie"] = cookieStr }
        if let sapisid = sapisid { h["Authorization"] = sapisidHash(sapisid) }
        return h
    }

    // MARK: Payload

    private func buildPayload(_ prompt: String, mode: Int, think: Int, extra: [Int: Int]?) -> String {
        var inner: [Any] = Array(repeating: NSNull(), count: 102)
        inner[0] = [prompt, 0, NSNull(), NSNull(), NSNull(), NSNull(), 0]
        inner[1] = ["en"]
        inner[2] = ["", "", "", NSNull(), NSNull(), NSNull(), NSNull(), NSNull(), NSNull(), ""]
        inner[6] = [0]
        inner[7] = 1
        inner[10] = 1
        inner[11] = 0
        inner[17] = [[think]]
        inner[18] = 0
        inner[27] = 1
        inner[30] = [4]
        inner[41] = [2]
        inner[53] = 0
        inner[59] = UUID().uuidString.lowercased()
        inner[61] = [Any]()
        inner[68] = 1
        inner[79] = mode
        if let extra = extra { for (k, v) in extra { inner[k] = v } }

        let innerStr = jsonString(inner)
        let outerStr = jsonString([NSNull(), innerStr])
        var body = "f.req=" + formEncode(outerStr)
        if let x = cfg.xsrfToken, !x.isEmpty { body += "&at=" + formEncode(x) }
        return body
    }

    private func requestURL() -> String {
        let reqid = nowUnix() % 1000000
        return "https://gemini.google.com\(accountPrefix())/_/BardChatUi/data/"
            + "assistant.lamda.BardFrontendService/StreamGenerate"
            + "?bl=\(cfg.geminiBl)&hl=en&_reqid=\(reqid)&rt=c"
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

    private func parseTexts(_ line: String) -> [String] {
        guard line.contains("\"wrb.fr\""), line.unicodeScalars.count >= 200,
              let data = line.data(using: .utf8),
              let arr = (try? JSONSerialization.jsonObject(with: data)) as? [Any],
              let a0 = arr.first as? [Any], a0.count > 2,
              let innerStr = a0[2] as? String, innerStr.unicodeScalars.count >= 50,
              let innerData = innerStr.data(using: .utf8),
              let inner = (try? JSONSerialization.jsonObject(with: innerData)) as? [Any],
              inner.count > 4, let part4 = inner[4] as? [Any] else { return [] }
        var texts: [String] = []
        for part in part4 {
            if let pl = part as? [Any], pl.count > 1, let sub = pl[1] as? [Any] {
                for t in sub { if let s = t as? String, !s.isEmpty { texts.append(s) } }
            }
        }
        return texts
    }

    // MARK: 请求（带重试），逐行回调

    private func perform(_ prompt: String, mode: Int, think: Int, extra: [Int: Int]?,
                         isCancelled: @escaping () -> Bool = { false },
                         onLine: @escaping (String) -> Void) throws {
        let body = buildPayload(prompt, mode: mode, think: think, extra: extra)
        var req = URLRequest(url: URL(string: requestURL())!)
        req.httpMethod = "POST"
        req.httpBody = body.data(using: .utf8)
        req.timeoutInterval = cfg.requestTimeoutSec
        for (k, v) in buildHeaders() { req.setValue(v, forHTTPHeaderField: k) }

        var lastErr: Error = EngineError.other("no attempt")
        for attempt in 0..<cfg.retryAttempts {
            do {
                try streamRequest(req, isCancelled: isCancelled, onLine: onLine)
                return
            } catch {
                // 4xx（cookie 失效等）重试无意义，直接抛出
                if case EngineError.http(let c) = error, c < 500 { throw error }
                lastErr = error
                if attempt < cfg.retryAttempts - 1 {
                    log("Retry \(attempt + 1)/\(cfg.retryAttempts): \(error)")
                    Thread.sleep(forTimeInterval: cfg.retryDelaySec)
                }
            }
        }
        throw lastErr
    }

    private func streamRequest(_ req: URLRequest, isCancelled: @escaping () -> Bool,
                               onLine: @escaping (String) -> Void) throws {
        let collector = LineCollector(isCancelled: isCancelled, onLine: onLine)
        let config = URLSessionConfiguration.ephemeral
        config.timeoutIntervalForRequest = cfg.requestTimeoutSec
        if let proxy = cfg.proxy, let u = URL(string: proxy), let host = u.host {
            let port = u.port ?? (u.scheme == "https" ? 443 : 80)
            config.connectionProxyDictionary = [
                kCFNetworkProxiesHTTPEnable as String: 1,
                kCFNetworkProxiesHTTPProxy as String: host,
                kCFNetworkProxiesHTTPPort as String: port,
                "HTTPSEnable": 1, "HTTPSProxy": host, "HTTPSPort": port,
            ]
        }
        let session = URLSession(configuration: config, delegate: collector, delegateQueue: nil)
        let task = session.dataTask(with: req)
        collector.task = task
        task.resume()
        collector.sem.wait()
        session.finishTasksAndInvalidate()
        if let e = collector.error {
            // 客户端断开触发的主动取消视为正常结束
            if (e as NSError).code == NSURLErrorCancelled { return }
            throw e
        }
        if let code = collector.statusCode, code >= 400 { throw EngineError.http(code) }
    }

    // MARK: 对外 API

    func generate(_ prompt: String, mode: Int, think: Int, extra: [Int: Int]?) throws -> String {
        var longest = ""
        var longestN = 0
        try perform(prompt, mode: mode, think: think, extra: extra) { line in
            for t in self.parseTexts(line) {
                let n = t.unicodeScalars.count
                if n > longestN { longest = t; longestN = n }
            }
        }
        return cleanText(longest)
    }

    func generateStream(_ prompt: String, mode: Int, think: Int, extra: [Int: Int]?,
                        isCancelled: @escaping () -> Bool = { false },
                        onDelta: @escaping (String) -> Void) throws {
        var prev = ""
        try perform(prompt, mode: mode, think: think, extra: extra, isCancelled: isCancelled) { line in
            for t in self.parseTexts(line) {
                let prevScalars = Array(prev.unicodeScalars)
                let tScalars = Array(t.unicodeScalars)
                if tScalars.count > prevScalars.count {
                    let deltaScalars = tScalars[prevScalars.count...]
                    let delta = self.stripArtifacts(String(String.UnicodeScalarView(deltaScalars)))
                    if !delta.isEmpty { onDelta(delta) }
                    prev = t
                }
            }
        }
    }
}

// URLSession 流式收集器：按 \n 切行回调
private final class LineCollector: NSObject, URLSessionDataDelegate {
    let isCancelled: () -> Bool
    let onLine: (String) -> Void
    let sem = DispatchSemaphore(value: 0)
    var error: Error?
    var statusCode: Int?
    weak var task: URLSessionDataTask?
    private var buffer = Data()

    init(isCancelled: @escaping () -> Bool, onLine: @escaping (String) -> Void) {
        self.isCancelled = isCancelled
        self.onLine = onLine
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask,
                    didReceive response: URLResponse, completionHandler: @escaping (URLSession.ResponseDisposition) -> Void) {
        statusCode = (response as? HTTPURLResponse)?.statusCode
        completionHandler(.allow)
    }

    func urlSession(_ session: URLSession, dataTask: URLSessionDataTask, didReceive data: Data) {
        if isCancelled() { task?.cancel(); return }
        buffer.append(data)
        while let nl = buffer.firstIndex(of: 0x0A) {
            let lineData = Data(buffer[buffer.startIndex..<nl])
            buffer = Data(buffer[buffer.index(after: nl)...])
            onLine(String(decoding: lineData, as: UTF8.self))
        }
    }

    func urlSession(_ session: URLSession, task: URLSessionTask, didCompleteWithError err: Error?) {
        if !buffer.isEmpty { onLine(String(decoding: buffer, as: UTF8.self)); buffer = Data() }
        error = err
        sem.signal()
    }
}

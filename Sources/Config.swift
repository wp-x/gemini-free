import Foundation

// 配置：与上游 config.json 完全兼容，存于 ~/.config/gemini-web2api/config.json
final class Store {
    static let shared = Store()

    var port = 8081
    var host = "0.0.0.0"
    var retryAttempts = 3
    var retryDelaySec = 2.0
    var requestTimeoutSec = 180.0
    var geminiBl = "boq_assistant-bard-web-server_20260716.08_p0"
    var authUser: String? = nil
    var xsrfToken: String? = nil
    var defaultModel = "gemini-3.6-flash"
    var logRequests = true
    var cookieFile: String? = nil
    var proxy: String? = nil
    var apiKeys: [String] = []
    var launchAtLogin = false

    let path: URL = {
        let dir = FileManager.default.homeDirectoryForCurrentUser
            .appendingPathComponent(".config/gemini-web2api", isDirectory: true)
        try? FileManager.default.createDirectory(at: dir, withIntermediateDirectories: true)
        return dir.appendingPathComponent("config.json")
    }()

    func load() {
        guard let data = try? Data(contentsOf: path),
              let d = (try? JSONSerialization.jsonObject(with: data)) as? [String: Any] else { return }
        if let v = d["port"] as? Int { port = v }
        if let v = d["host"] as? String { host = v }
        if let v = d["retry_attempts"] as? Int { retryAttempts = v }
        if let v = d["retry_delay_sec"] as? NSNumber { retryDelaySec = v.doubleValue }
        if let v = d["request_timeout_sec"] as? NSNumber { requestTimeoutSec = v.doubleValue }
        if let v = d["gemini_bl"] as? String { geminiBl = v }
        authUser = str(d["auth_user"])
        xsrfToken = str(d["xsrf_token"])
        if let v = d["default_model"] as? String { defaultModel = v }
        if let v = d["log_requests"] as? Bool { logRequests = v }
        cookieFile = d["cookie_file"] as? String
        proxy = d["proxy"] as? String
        if let v = d["api_keys"] as? [String] { apiKeys = v }
        if let v = d["launch_at_login"] as? Bool { launchAtLogin = v }
    }

    func save() {
        let d: [String: Any] = [
            "port": port, "host": host,
            "retry_attempts": retryAttempts, "retry_delay_sec": retryDelaySec,
            "request_timeout_sec": requestTimeoutSec, "gemini_bl": geminiBl,
            "auth_user": authUser ?? NSNull(), "xsrf_token": xsrfToken ?? NSNull(),
            "default_model": defaultModel, "log_requests": logRequests,
            "cookie_file": cookieFile ?? NSNull(), "proxy": proxy ?? NSNull(),
            "api_keys": apiKeys, "launch_at_login": launchAtLogin,
        ]
        if let data = try? JSONSerialization.data(withJSONObject: d, options: [.prettyPrinted]) {
            try? data.write(to: path)
        }
    }

    // auth_user 可能是数字或字符串，统一转字符串
    private func str(_ v: Any?) -> String? {
        if let s = v as? String, !s.isEmpty { return s }
        if let n = v as? NSNumber { return n.stringValue }
        return nil
    }
}

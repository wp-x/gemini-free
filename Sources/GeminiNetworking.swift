import Foundation

enum GeminiNetworking {
    static let sessionTimeout: TimeInterval = 30

    static func configuration(proxy: String?, timeout: TimeInterval) -> URLSessionConfiguration {
        let configuration = URLSessionConfiguration.ephemeral
        configuration.timeoutIntervalForRequest = timeout
        configuration.httpCookieStorage = nil
        configuration.httpShouldSetCookies = false
        if let proxy = proxy, let url = URL(string: proxy), let host = url.host {
            let defaultPort = url.scheme == "https" ? 443 : 80
            let port = url.port ?? defaultPort
            configuration.connectionProxyDictionary = [
                kCFNetworkProxiesHTTPEnable as String: 1,
                kCFNetworkProxiesHTTPProxy as String: host,
                kCFNetworkProxiesHTTPPort as String: port,
                "HTTPSEnable": 1, "HTTPSProxy": host, "HTTPSPort": port,
            ]
        }
        return configuration
    }

    static func send(_ request: URLRequest, proxy: String?) throws -> GeminiSessionReply {
        let configuration = configuration(proxy: proxy, timeout: sessionTimeout)
        configuration.timeoutIntervalForResource = sessionTimeout
        let collector = SessionReplyCollector()
        let session = URLSession(configuration: configuration)
        defer { session.finishTasksAndInvalidate() }
        session.dataTask(with: request) { data, response, error in
            collector.complete(data: data, response: response, error: error)
        }.resume()
        collector.finished.wait()
        return try collector.reply()
    }
}

private final class SessionReplyCollector: @unchecked Sendable {
    let finished = DispatchSemaphore(value: 0)
    private var result: Result<GeminiSessionReply, Error>?

    func complete(data: Data?, response: URLResponse?, error: Error?) {
        defer { finished.signal() }
        if let error = error { result = .failure(error); return }
        guard let response = response as? HTTPURLResponse, let data = data else {
            result = .failure(GeminiSessionError.invalid("Google 续期响应缺少 HTTP 数据"))
            return
        }
        result = .success(GeminiSessionReply(data: data, response: response))
    }

    func reply() throws -> GeminiSessionReply {
        guard let result = result else {
            throw GeminiSessionError.invalid("Google 续期请求未完成")
        }
        return try result.get()
    }
}

import Foundation
import Network

// 极简 HTTP/1.1 服务：仅实现自用所需的 OpenAI 兼容端点
final class HTTPServer {
    static let shared = HTTPServer()
    private var listener: NWListener?
    private let queue = DispatchQueue(label: "gemini.http", attributes: .concurrent)
    private(set) var running = false
    private let cfg = Store.shared

    func start() throws {
        stop()
        let params = NWParameters.tcp
        params.allowLocalEndpointReuse = true
        let port = NWEndpoint.Port(rawValue: UInt16(cfg.port))!
        let l: NWListener
        if cfg.host == "0.0.0.0" || cfg.host.isEmpty {
            l = try NWListener(using: params, on: port)  // 所有网卡（局域网可访问）
        } else {
            params.requiredLocalEndpoint = NWEndpoint.hostPort(host: NWEndpoint.Host(cfg.host), port: port)
            l = try NWListener(using: params)
        }
        l.newConnectionHandler = { [weak self] conn in self?.accept(conn) }
        l.stateUpdateHandler = { [weak self] state in
            if case .failed = state { self?.running = false }
        }
        l.start(queue: queue)
        listener = l
        running = true
    }

    func stop() {
        listener?.cancel()
        listener = nil
        running = false
    }

    // MARK: 连接读取

    private func accept(_ conn: NWConnection) {
        conn.start(queue: queue)
        let state = ConnState()
        readMore(conn, state)
    }

    private final class ConnState {
        var buffer = Data()
        var headersDone = false
        var headerEnd = 0
        var contentLength = 0
        var headerText = ""
    }

    private func readMore(_ conn: NWConnection, _ st: ConnState) {
        conn.receive(minimumIncompleteLength: 1, maximumLength: 1 << 16) { [weak self] data, _, isComplete, err in
            guard let self = self else { return }
            if let d = data, !d.isEmpty { st.buffer.append(d) }

            if !st.headersDone, let r = st.buffer.range(of: Data("\r\n\r\n".utf8)) {
                st.headersDone = true
                st.headerEnd = r.upperBound
                st.headerText = String(decoding: st.buffer[st.buffer.startIndex..<r.lowerBound], as: UTF8.self)
                st.contentLength = self.contentLength(st.headerText)
            }

            if st.headersDone && st.buffer.count >= st.headerEnd + st.contentLength {
                let body = Data(st.buffer[st.headerEnd..<(st.headerEnd + st.contentLength)])
                self.route(conn, header: st.headerText, body: body)
                return
            }

            if err != nil || isComplete { conn.cancel(); return }
            self.readMore(conn, st)
        }
    }

    private func contentLength(_ header: String) -> Int {
        for line in header.components(separatedBy: "\r\n") {
            let parts = line.split(separator: ":", maxSplits: 1)
            if parts.count == 2, parts[0].lowercased() == "content-length" {
                return Int(parts[1].trimmingCharacters(in: .whitespaces)) ?? 0
            }
        }
        return 0
    }

    // MARK: 路由

    private func route(_ conn: NWConnection, header: String, body: Data) {
        let lines = header.components(separatedBy: "\r\n")
        let reqLine = lines.first ?? ""
        let comps = reqLine.split(separator: " ")
        let method = comps.count > 0 ? String(comps[0]) : ""
        let path = comps.count > 1 ? String(comps[1]) : "/"
        var headers: [String: String] = [:]
        for line in lines.dropFirst() {
            let kv = line.split(separator: ":", maxSplits: 1)
            if kv.count == 2 { headers[kv[0].lowercased()] = kv[1].trimmingCharacters(in: .whitespaces) }
        }

        if method == "OPTIONS" {
            sendRaw(conn, "HTTP/1.1 204 No Content\r\nAccess-Control-Allow-Origin: *\r\nAccess-Control-Allow-Methods: GET, POST, OPTIONS\r\nAccess-Control-Allow-Headers: *\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", close: true)
            return
        }

        if path.hasPrefix("/v1") && !authorized(headers, path: path) {
            sendJSON(conn, ["error": ["message": "invalid api key"]], status: 401)
            return
        }

        switch (method, pathOnly(path)) {
        case ("GET", "/v1/models"):
            let data = MODELS.map { ["id": $0.id, "object": "model", "created": 1700000000, "owned_by": "google", "description": $0.desc] as [String: Any] }
            sendJSON(conn, ["object": "list", "data": data])
        case ("GET", "/"):
            sendJSON(conn, ["status": "ok", "models": MODELS.map { $0.id }])
        case ("POST", "/v1/chat/completions"):
            handleChat(conn, body: body)
        default:
            sendJSON(conn, ["error": "not found"], status: 404)
        }
    }

    private func pathOnly(_ path: String) -> String {
        path.split(separator: "?", maxSplits: 1).first.map(String.init) ?? path
    }

    private func authorized(_ headers: [String: String], path: String) -> Bool {
        let keys = cfg.apiKeys
        if keys.isEmpty { return true }
        if let auth = headers["authorization"], auth.hasPrefix("Bearer "), keys.contains(String(auth.dropFirst(7))) { return true }
        for h in ["x-api-key", "x-goog-api-key"] {
            if let v = headers[h], keys.contains(v) { return true }
        }
        if let q = path.split(separator: "?", maxSplits: 1).dropFirst().first {
            for pair in q.split(separator: "&") where pair.hasPrefix("key=") {
                if keys.contains(String(pair.dropFirst(4))) { return true }
            }
        }
        return false
    }

    // MARK: /v1/chat/completions

    private func handleChat(_ conn: NWConnection, body: Data) {
        guard let req = (try? JSONSerialization.jsonObject(with: body)) as? [String: Any] else {
            sendJSON(conn, ["error": ["message": "invalid JSON"]], status: 400); return
        }
        let modelName = req["model"] as? String ?? cfg.defaultModel
        let rm = resolveModel(modelName, defaultModel: cfg.defaultModel)
        let tools = req["tools"] as? [Any]
        let toolChoice = req["tool_choice"] ?? "auto"
        let prompt = messagesToPrompt(req["messages"] as? [Any] ?? [], tools: tools, toolChoice: toolChoice)
        if prompt.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            sendJSON(conn, ["error": ["message": "empty prompt"]], status: 400); return
        }
        let stream = req["stream"] as? Bool ?? false
        let cid = "chatcmpl-" + randomHex(12)
        let toolsActive = tools != nil && !(toolChoice as? String == "none")

        // 纯流式（无工具）：边生成边推送，客户端断开则取消上游请求
        if stream && !toolsActive {
            let gone = ClientGone()
            conn.stateUpdateHandler = { state in
                switch state {
                case .failed, .cancelled: gone.on = true
                default: break
                }
            }
            startSSE(conn)
            do {
                try Engine.shared.generateStream(prompt, mode: rm.mode, think: rm.think, extra: rm.extra,
                                                 isCancelled: { gone.on }) { delta in
                    let chunk: [String: Any] = ["id": cid, "object": "chat.completion.chunk", "created": nowUnix(),
                        "model": rm.name, "choices": [["index": 0, "delta": ["content": delta], "finish_reason": NSNull()]]]
                    self.sseSend(conn, "data: \(jsonString(chunk))\n\n", gone: gone)
                }
            } catch {
                let chunk: [String: Any] = ["id": cid, "object": "chat.completion.chunk", "created": nowUnix(),
                    "model": rm.name, "choices": [["index": 0, "delta": ["content": "[upstream error: \(error)]"], "finish_reason": NSNull()]]]
                sseSend(conn, "data: \(jsonString(chunk))\n\n")
            }
            let end: [String: Any] = ["id": cid, "object": "chat.completion.chunk", "created": nowUnix(),
                "model": rm.name, "choices": [["index": 0, "delta": [:], "finish_reason": "stop"]]]
            sseSend(conn, "data: \(jsonString(end))\n\n")
            sseFinish(conn, "data: [DONE]\n\n")
            return
        }

        // 非流式 / 带工具
        var text: String
        do {
            text = try Engine.shared.generate(prompt, mode: rm.mode, think: rm.think, extra: rm.extra)
        } catch {
            sendJSON(conn, ["error": ["message": "upstream error: \(error)"]], status: 502); return
        }

        var toolCalls: [[String: Any]] = []
        if toolsActive, !text.isEmpty { (text, toolCalls) = parseToolCalls(text) }
        var msg: [String: Any] = ["role": "assistant", "content": text.isEmpty ? NSNull() : text]
        if !toolCalls.isEmpty { msg["tool_calls"] = toolCalls }
        let finish = toolCalls.isEmpty ? "stop" : "tool_calls"

        if stream {
            startSSE(conn)
            let chunk: [String: Any] = ["id": cid, "object": "chat.completion.chunk", "created": nowUnix(),
                "model": rm.name, "choices": [["index": 0, "delta": msg, "finish_reason": finish]]]
            sseSend(conn, "data: \(jsonString(chunk))\n\n")
            sseFinish(conn, "data: [DONE]\n\n")
        } else {
            let pt = prompt.count / 4, ct = text.count / 4
            sendJSON(conn, ["id": cid, "object": "chat.completion", "created": nowUnix(), "model": rm.name,
                "choices": [["index": 0, "message": msg, "finish_reason": finish]],
                "usage": ["prompt_tokens": pt, "completion_tokens": ct, "total_tokens": pt + ct]])
        }
    }

    // MARK: 写响应

    private func sendJSON(_ conn: NWConnection, _ obj: Any, status: Int = 200) {
        let body = Data(jsonString(obj).utf8)
        var head = "HTTP/1.1 \(status) \(reason(status))\r\n"
        head += "Content-Type: application/json\r\n"
        head += "Access-Control-Allow-Origin: *\r\n"
        head += "Content-Length: \(body.count)\r\n"
        head += "Connection: close\r\n\r\n"
        var out = Data(head.utf8); out.append(body)
        conn.send(content: out, completion: .contentProcessed { _ in conn.cancel() })
    }

    private func sendRaw(_ conn: NWConnection, _ s: String, close: Bool) {
        conn.send(content: Data(s.utf8), completion: .contentProcessed { _ in if close { conn.cancel() } })
    }

    private func startSSE(_ conn: NWConnection) {
        let head = "HTTP/1.1 200 OK\r\nContent-Type: text/event-stream\r\nCache-Control: no-cache\r\nAccess-Control-Allow-Origin: *\r\nConnection: close\r\n\r\n"
        conn.send(content: Data(head.utf8), completion: .contentProcessed { _ in })
    }

    // ponytail: on 标记无锁读写，Bool 竞争无害，仅用于尽早停止上游
    final class ClientGone { var on = false }

    private func sseSend(_ conn: NWConnection, _ s: String, gone: ClientGone? = nil) {
        conn.send(content: Data(s.utf8), completion: .contentProcessed { err in
            if err != nil { gone?.on = true }
        })
    }

    private func sseFinish(_ conn: NWConnection, _ s: String) {
        conn.send(content: Data(s.utf8), completion: .contentProcessed { _ in conn.cancel() })
    }

    private func reason(_ status: Int) -> String {
        switch status {
        case 200: return "OK"
        case 204: return "No Content"
        case 400: return "Bad Request"
        case 401: return "Unauthorized"
        case 404: return "Not Found"
        case 502: return "Bad Gateway"
        default: return "OK"
        }
    }
}

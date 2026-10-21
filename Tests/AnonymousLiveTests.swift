import Foundation

private enum ProbeError: Error, CustomStringConvertible {
    case invalidArguments
    case unexpectedAnswer(String)
    case http(Int)

    var description: String {
        switch self {
        case .invalidArguments:
            return "Usage: anonymous-live <answer|stream|raw> <model>"
        case .unexpectedAnswer(let answer):
            return "Unexpected anonymous answer: \(answer)"
        case .http(let status):
            return "Anonymous upstream returned HTTP \(status)"
        }
    }
}

private struct AnonymousProbeOptions {
    enum Operation: String { case answer, stream, raw }
    static let requestTimeout: TimeInterval = 40
    static let oneAttempt = 1
    static let expectedAnswer = "ANONYMOUS_OK_7391"
    static let defaultPrompt = "Reply with exactly ANONYMOUS_OK_7391 and nothing else."

    let operation: Operation
    let model: String
    let proxy: String?
    let build: String?
    let prompt: String
    let expectedAnswer: String?
    let responsePath: String?

    init(arguments: [String], environment: [String: String]) throws {
        let expectedArguments = 2
        guard arguments.count == expectedArguments,
              let operation = Operation(rawValue: arguments[0]) else {
            throw ProbeError.invalidArguments
        }
        self.operation = operation
        model = arguments[1]
        proxy = environment["GEMINI_PROBE_PROXY"]
        build = environment["GEMINI_PROBE_BUILD"]
        prompt = environment["GEMINI_PROBE_PROMPT"] ?? Self.defaultPrompt
        expectedAnswer = environment["GEMINI_PROBE_EXPECTED"]
            ?? (prompt == Self.defaultPrompt ? Self.expectedAnswer : nil)
        responsePath = environment["GEMINI_PROBE_RESPONSE"]
    }

    func makeStore() -> Store {
        let store = Store()
        store.retryAttempts = Self.oneAttempt
        store.requestTimeoutSec = Self.requestTimeout
        store.sessionAutoRefresh = false
        store.proxy = proxy
        if let build { store.geminiBl = build }
        return store
    }
}

private final class AnonymousProbeSession: GeminiSessionProviding {
    private let current: GeminiSessionSnapshot

    init(snapshot: GeminiSessionSnapshot) {
        current = snapshot
        precondition(snapshot.cookieFile == nil && snapshot.cookie.isEmpty)
        precondition(snapshot.sapisid == nil && snapshot.identity == nil)
        precondition(snapshot.rotation == nil && snapshot.xsrfToken == nil)
    }

    func snapshot() throws -> GeminiSessionSnapshot { current }

    func receive(_ response: HTTPURLResponse, for snapshot: GeminiSessionSnapshot) throws {
        print("upstream_http=\(response.statusCode)")
    }
}

private struct AnonymousProbe {
    let options: AnonymousProbeOptions
    let config: Store
    let request: GenerationRequest
    let session: AnonymousProbeSession

    init(options: AnonymousProbeOptions) throws {
        self.options = options
        config = options.makeStore()
        let model = try resolveModel(options.model, defaultModel: options.model)
        request = GenerationRequest(
            prompt: options.prompt, mode: model.mode, think: model.think, extra: model.extra)
        let snapshot = GeminiSessionDocument().snapshot(settings: GeminiSessionSettings(store: config))
        session = AnonymousProbeSession(snapshot: snapshot)
    }

    func run() throws {
        print("operation=\(options.operation.rawValue) requested_model=\(options.model)")
        print("account_cookie=false authorization=false xsrf_token=false config_loaded=false")
        print("proxy=\(options.proxy ?? "system/default") build=\(config.geminiBl)")
        let start = Date()
        switch options.operation {
        case .answer: try answer()
        case .stream: try stream()
        case .raw: try raw()
        }
        print("elapsed_seconds=\(Date().timeIntervalSince(start))")
    }

    private func answer() throws {
        let engine = Engine(config: config, sessions: session)
        let output = try engine.generate(request)
        print("answer=\(output)")
        try validate(output)
    }

    private func stream() throws {
        let engine = Engine(config: config, sessions: session)
        var output = ""
        var chunks = 0
        let start = Date()
        try engine.generateStream(request) { delta in
            chunks += 1
            output += delta
            print("chunk=\(chunks) seconds=\(Date().timeIntervalSince(start)) text=\(delta.debugDescription)")
        }
        print("answer=\(output)")
        try validate(output)
    }

    private func raw() throws {
        let upstream = try upstreamRequest()
        precondition(upstream.value(forHTTPHeaderField: "Cookie") == nil)
        precondition(upstream.value(forHTTPHeaderField: "Authorization") == nil)
        let reply = try GeminiNetworking.send(upstream, proxy: config.proxy)
        print("upstream_http=\(reply.response.statusCode) response_bytes=\(reply.data.count)")
        if let path = options.responsePath {
            try reply.data.write(to: URL(fileURLWithPath: path), options: .atomic)
            print("raw_response=\(path)")
        }
        guard reply.response.statusCode == 200 else { throw ProbeError.http(reply.response.statusCode) }
        let frames = try parseFrames(reply.data)
        print("response_frames=\(frames.count)")
        print("upstream_modes=\(frames.compactMap(\.mode))")
        print("upstream_thinking_levels=\(frames.compactMap(\.thinkingLevel))")
        let output = frames.flatMap(\.texts).max(by: { $0.count < $1.count }) ?? ""
        print("answer=\(output)")
        try validate(output)
    }

    private func upstreamRequest() throws -> URLRequest {
        var url = URLComponents(
            string: "https://gemini.google.com/_/BardChatUi/data/assistant.lamda.BardFrontendService/StreamGenerate")!
        let requestIDModulus = 1_000_000
        url.queryItems = [
            URLQueryItem(name: "bl", value: config.geminiBl),
            URLQueryItem(name: "hl", value: "en"),
            URLQueryItem(name: "_reqid", value: String(nowUnix() % requestIDModulus)),
            URLQueryItem(name: "rt", value: "c"),
        ]
        var upstream = URLRequest(url: url.url!)
        upstream.httpMethod = "POST"
        upstream.timeoutInterval = AnonymousProbeOptions.requestTimeout
        upstream.httpBody = Data(GeminiPayload.body(for: request, xsrfToken: nil).utf8)
        upstream.allHTTPHeaderFields = [
            "Content-Type": "application/x-www-form-urlencoded",
            "Origin": "https://gemini.google.com",
            "Referer": "https://gemini.google.com/app",
            "X-Same-Domain": "1",
            "User-Agent": GeminiNetworking.userAgent,
        ].merging(GeminiPayload.modelHeaders(for: request)) { _, value in value }
        return upstream
    }

    private func parseFrames(_ data: Data) throws -> [GeminiResponseFrame] {
        guard let text = String(data: data, encoding: .utf8) else {
            throw EngineError.response("Anonymous response is not UTF-8")
        }
        return try text.components(separatedBy: .newlines).flatMap(GeminiResponseFrame.parse)
    }

    private func validate(_ output: String) throws {
        guard !output.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else {
            throw ProbeError.unexpectedAnswer("empty response")
        }
        guard let expected = options.expectedAnswer else {
            print("result=TEXT_RECEIVED validation=nonempty_only")
            return
        }
        guard output.trimmingCharacters(in: .whitespacesAndNewlines) == expected else {
            throw ProbeError.unexpectedAnswer(output)
        }
        print("result=PASS")
    }
}

@main
struct AnonymousLiveTests {
    static func main() {
        setbuf(stdout, nil)
        do {
            let options = try AnonymousProbeOptions(
                arguments: Array(CommandLine.arguments.dropFirst()),
                environment: ProcessInfo.processInfo.environment)
            try AnonymousProbe(options: options).run()
        } catch {
            FileHandle.standardError.write(Data("result=FAIL error=\(error)\n".utf8))
            exit(EXIT_FAILURE)
        }
    }
}

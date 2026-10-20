import Foundation

@main
struct LiveModelTests {
    static func main() {
        DispatchQueue.global().async {
            do {
                try run()
                exit(EXIT_SUCCESS)
            } catch {
                FileHandle.standardError.write(Data("Live model test failed: \(error)\n".utf8))
                exit(EXIT_FAILURE)
            }
        }
        dispatchMain()
    }

    private static func run() throws {
        let config = Store.shared
        config.load()
        config.retryAttempts = 1
        let modelName = CommandLine.arguments.dropFirst().first ?? "gemini-4-argon-low"
        let model = try resolveModel(modelName, defaultModel: config.defaultModel)
        let request = GenerationRequest(
            prompt: "Reply with exactly ARGON_API_OK and nothing else.",
            mode: model.mode, think: model.think, extra: model.extra
        )
        let start = Date()
        let output: String
        if CommandLine.arguments.contains("--stream") {
            var text = ""
            try Engine.shared.generateStream(request, onDelta: { delta in
                text += delta
                print(delta, terminator: "")
                fflush(stdout)
            })
            output = text
            print()
        } else {
            output = try Engine.shared.generate(request)
        }
        print("model=\(model.name), seconds=\(Date().timeIntervalSince(start))")
        print(output)
        precondition(output.trimmingCharacters(in: .whitespacesAndNewlines) == "ARGON_API_OK")
    }
}

import Foundation

@main
struct StreamRoutingTests {
    static func main() {
        precondition(!hasActiveTools(nil, toolChoice: "auto"))
        precondition(!hasActiveTools([], toolChoice: "auto"))
        precondition(!hasActiveTools([toolDefinition], toolChoice: "none"))
        precondition(hasActiveTools([toolDefinition], toolChoice: "auto"))
        print("StreamRoutingTests passed")
    }

    private static let toolDefinition: [String: Any] = [
        "type": "function",
        "function": ["name": "lookup"],
    ]
}

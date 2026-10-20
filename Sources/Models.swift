import Foundation

enum Argon {
    static let mode = 8
    static let modelID = "95a4c0abcba24364"
    static let thinkingField = 80
    static let defaultThinkingLevel = ThinkingLevel.high.rawValue

    enum ThinkingLevel: Int {
        case low = 1
        case high = 2
        case medium = 4
    }
}

enum ModelResolutionError: Error, CustomStringConvertible {
    case invalidThinkingLevel(String)
    case unknownDefault(String)

    var description: String {
        switch self {
        case .invalidThinkingLevel(let value):
            return "Invalid Argon thinking level '\(value)': use 1 (low), 4 (medium), or 2 (high)"
        case .unknownDefault(let name):
            return "Unknown default model: \(name)"
        }
    }
}

// 模型定义，映射自 Gemini 前端 JS 的 MODE_CATEGORY 枚举（与上游一致）
struct ModelDef {
    let id: String
    let mode: Int
    let think: Int
    let desc: String
    let extra: [Int: Int]?
}

let MODELS: [ModelDef] = [
    ModelDef(id: "gemini-4-argon", mode: Argon.mode, think: 4, desc: "Gemini 4 Argon, high thinking (requires eligible Google account)", extra: [Argon.thinkingField: Argon.defaultThinkingLevel]),
    ModelDef(id: "gemini-4-argon-low", mode: Argon.mode, think: 4, desc: "Gemini 4 Argon, low thinking", extra: [Argon.thinkingField: Argon.ThinkingLevel.low.rawValue]),
    ModelDef(id: "gemini-4-argon-medium", mode: Argon.mode, think: 4, desc: "Gemini 4 Argon, medium thinking", extra: [Argon.thinkingField: Argon.ThinkingLevel.medium.rawValue]),
    ModelDef(id: "gemini-4-argon-high", mode: Argon.mode, think: 4, desc: "Gemini 4 Argon, high thinking", extra: [Argon.thinkingField: Argon.ThinkingLevel.high.rawValue]),
    ModelDef(id: "gemini-3.8-flash", mode: 1, think: 4, desc: "Latest all-around model (Gemini 3.8 Flash)", extra: nil),
    ModelDef(id: "gemini-3.7-flash", mode: 1, think: 4, desc: "Previous all-around model (Gemini 3.7 Flash)", extra: nil),
    ModelDef(id: "gemini-3.6-flash", mode: 1, think: 4, desc: "Latest all-around model (Gemini 3.6 Flash)", extra: nil),
    ModelDef(id: "gemini-3.5-flash", mode: 1, think: 4, desc: "Alias for gemini-3.6-flash (backend upgraded)", extra: nil),
    ModelDef(id: "gemini-3.5-flash-thinking", mode: 2, think: 0, desc: "Deep thinking mode, longest output (~20k chars)", extra: nil),
    ModelDef(id: "gemini-3.1-pro", mode: 3, think: 4, desc: "Pro model (requires cookie for real routing)", extra: nil),
    ModelDef(id: "gemini-3.1-pro-enhanced", mode: 3, think: 4, desc: "Pro with enhanced output (experimental)", extra: [31: 2, 80: 3]),
    ModelDef(id: "gemini-auto", mode: 4, think: 4, desc: "Auto model selection", extra: nil),
    ModelDef(id: "gemini-3.5-flash-thinking-lite", mode: 5, think: 0, desc: "Dynamic thinking with adaptive depth", extra: nil),
    ModelDef(id: "gemini-flash-lite", mode: 6, think: 4, desc: "Lightweight fast model", extra: nil),
]

private let modelIndex: [String: ModelDef] = Dictionary(uniqueKeysWithValues: MODELS.map { ($0.id, $0) })

struct ResolvedModel {
    let name: String
    let mode: Int
    let think: Int
    let extra: [Int: Int]?
}

// 解析模型名，支持 @think=N 后缀；未知模型回退到默认（与上游一致）
func resolveModel(_ modelName: String, defaultModel: String) throws -> ResolvedModel {
    var name = modelName
    var thinkSuffix: String?
    if let r = name.range(of: "@think=", options: .backwards) {
        thinkSuffix = String(name[r.upperBound...])
        name = String(name[..<r.lowerBound])
    }
    guard let cfg = modelIndex[name] ?? modelIndex[defaultModel] else {
        throw ModelResolutionError.unknownDefault(defaultModel)
    }
    let resolvedName = modelIndex[name] != nil ? name : defaultModel
    if cfg.mode == Argon.mode {
        return try resolveArgon(name: resolvedName, definition: cfg, suffix: thinkSuffix)
    }
    return ResolvedModel(
        name: resolvedName, mode: cfg.mode,
        think: thinkSuffix.flatMap(Int.init) ?? cfg.think, extra: cfg.extra)
}

private func resolveArgon(
    name: String, definition: ModelDef, suffix: String?
) throws -> ResolvedModel {
    var extra = definition.extra ?? [:]
    if let suffix = suffix {
        guard let value = Int(suffix), Argon.ThinkingLevel(rawValue: value) != nil else {
            throw ModelResolutionError.invalidThinkingLevel(suffix)
        }
        extra[Argon.thinkingField] = value
    }
    return ResolvedModel(name: name, mode: definition.mode, think: definition.think, extra: extra)
}

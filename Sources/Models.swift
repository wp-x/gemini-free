import Foundation

// 模型定义，映射自 Gemini 前端 JS 的 MODE_CATEGORY 枚举（与上游一致）
struct ModelDef {
    let id: String
    let mode: Int
    let think: Int
    let desc: String
    let extra: [Int: Int]?
}

let MODELS: [ModelDef] = [
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
func resolveModel(_ modelName: String, defaultModel: String) -> ResolvedModel {
    var name = modelName
    var thinkOverride: Int? = nil
    if let r = name.range(of: "@think=", options: .backwards) {
        let suffix = String(name[r.upperBound...])
        name = String(name[..<r.lowerBound])
        thinkOverride = Int(suffix)
    }
    let cfg = modelIndex[name] ?? modelIndex[defaultModel]!
    let resolvedName = modelIndex[name] != nil ? name : defaultModel
    return ResolvedModel(name: resolvedName, mode: cfg.mode, think: thinkOverride ?? cfg.think, extra: cfg.extra)
}

import Foundation

// JSON 序列化为字符串（非 ASCII 直出，等价 ensure_ascii=False）
func jsonString(_ obj: Any, pretty: Bool = false) -> String {
    let opts: JSONSerialization.WritingOptions = pretty ? [.prettyPrinted] : []
    guard JSONSerialization.isValidJSONObject(obj) || obj is String,
          let data = try? JSONSerialization.data(withJSONObject: obj, options: opts),
          let s = String(data: data, encoding: .utf8) else {
        if let s = obj as? String { return s }
        return "{}"
    }
    return s
}

func randomHex(_ n: Int) -> String {
    let chars = "0123456789abcdef"
    var s = ""
    for _ in 0..<n { s.append(chars.randomElement()!) }
    return s
}

// application/x-www-form-urlencoded（quote_plus）：空格->+，仅保留 A-Za-z0-9_.-~
func formEncode(_ s: String) -> String {
    var out = ""
    for byte in s.utf8 {
        if (byte >= 48 && byte <= 57) || (byte >= 65 && byte <= 90) || (byte >= 97 && byte <= 122)
            || byte == 0x5F || byte == 0x2E || byte == 0x2D || byte == 0x7E {
            out.append(Character(UnicodeScalar(byte)))
        } else if byte == 0x20 {
            out.append("+")
        } else {
            out.append(String(format: "%%%02X", byte))
        }
    }
    return out
}

func nowUnix() -> Int { Int(Date().timeIntervalSince1970) }

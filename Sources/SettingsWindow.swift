import AppKit

// 极简设置窗口：端口 / API Key / Cookie / 代理 / 默认模型 / 认证参数
enum SettingsWindow {
    private final class Controller: NSObject {
        let cfg = Store.shared
        let onSave: () -> Void
        var cookieField: NSTextField!
        var portField: NSTextField!
        var keysField: NSTextField!
        var proxyField: NSTextField!
        var authUserField: NSTextField!
        var xsrfField: NSTextField!
        var modelPopup: NSPopUpButton!
        var lanCheckbox: NSButton!
        weak var window: NSWindow?

        init(onSave: @escaping () -> Void) { self.onSave = onSave }

        @objc func browseCookie() {
            let panel = NSOpenPanel()
            panel.canChooseFiles = true
            panel.canChooseDirectories = false
            if panel.runModal() == .OK, let url = panel.url { cookieField.stringValue = url.path }
        }

        @objc func save() {
            cfg.port = Int(portField.stringValue) ?? cfg.port
            cfg.apiKeys = keysField.stringValue.split(separator: ",").map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
            cfg.cookieFile = cookieField.stringValue.isEmpty ? nil : cookieField.stringValue
            cfg.proxy = proxyField.stringValue.isEmpty ? nil : proxyField.stringValue
            cfg.authUser = authUserField.stringValue.isEmpty ? nil : authUserField.stringValue
            cfg.xsrfToken = xsrfField.stringValue.isEmpty ? nil : xsrfField.stringValue
            cfg.defaultModel = modelPopup.titleOfSelectedItem ?? cfg.defaultModel
            cfg.host = lanCheckbox.state == .on ? "0.0.0.0" : "127.0.0.1"
            onSave()
            window?.close()
        }
    }

    static func make(onSave: @escaping () -> Void) -> NSWindow {
        let cfg = Store.shared
        let ctl = Controller(onSave: onSave)

        let win = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 480, height: 380),
                           styleMask: [.titled, .closable], backing: .buffered, defer: false)
        win.title = "Gemini Free 设置"
        win.center()
        ctl.window = win
        objc_setAssociatedObject(win, "ctl", ctl, .OBJC_ASSOCIATION_RETAIN)

        let grid = NSGridView()
        grid.translatesAutoresizingMaskIntoConstraints = false
        grid.rowSpacing = 10
        grid.columnSpacing = 10

        func field(_ value: String, placeholder: String = "") -> NSTextField {
            let f = NSTextField(string: value)
            f.placeholderString = placeholder
            f.widthAnchor.constraint(equalToConstant: 300).isActive = true
            return f
        }
        func label(_ s: String) -> NSTextField {
            let l = NSTextField(labelWithString: s)
            l.alignment = .right
            return l
        }

        ctl.portField = field(String(cfg.port))
        ctl.keysField = field(cfg.apiKeys.joined(separator: ", "), placeholder: "留空则免密；多个用逗号分隔")
        ctl.proxyField = field(cfg.proxy ?? "", placeholder: "http://127.0.0.1:7890")
        ctl.authUserField = field(cfg.authUser ?? "", placeholder: "登录账号序号，如 1")
        ctl.xsrfField = field(cfg.xsrfToken ?? "", placeholder: "SNlM0e（登录态需要）")

        let cookieRow = NSStackView()
        cookieRow.orientation = .horizontal
        ctl.cookieField = field(cfg.cookieFile ?? "", placeholder: "cookie.txt 路径（Pro 需要）")
        ctl.cookieField.widthAnchor.constraint(equalToConstant: 220).isActive = true
        let browse = NSButton(title: "选择…", target: ctl, action: #selector(Controller.browseCookie))
        cookieRow.addArrangedSubview(ctl.cookieField)
        cookieRow.addArrangedSubview(browse)

        ctl.modelPopup = NSPopUpButton()
        ctl.modelPopup.addItems(withTitles: MODELS.map { $0.id })
        ctl.modelPopup.selectItem(withTitle: cfg.defaultModel)

        ctl.lanCheckbox = NSButton(checkboxWithTitle: "允许局域网访问（建议同时设置 API Key 鉴权）", target: nil, action: nil)
        ctl.lanCheckbox.state = (cfg.host == "0.0.0.0" || cfg.host.isEmpty) ? .on : .off

        grid.addRow(with: [label("端口"), ctl.portField])
        grid.addRow(with: [label("局域网"), ctl.lanCheckbox])
        grid.addRow(with: [label("API Key 鉴权"), ctl.keysField])
        grid.addRow(with: [label("默认模型"), ctl.modelPopup])
        grid.addRow(with: [label("Cookie 文件"), cookieRow])
        grid.addRow(with: [label("代理"), ctl.proxyField])
        grid.addRow(with: [label("auth_user"), ctl.authUserField])
        grid.addRow(with: [label("xsrf_token"), ctl.xsrfField])

        let saveBtn = NSButton(title: "保存并重启服务", target: ctl, action: #selector(Controller.save))
        saveBtn.keyEquivalent = "\r"

        let root = NSStackView(views: [grid, saveBtn])
        root.orientation = .vertical
        root.spacing = 16
        root.edgeInsets = NSEdgeInsets(top: 20, left: 20, bottom: 20, right: 20)
        root.translatesAutoresizingMaskIntoConstraints = false

        let content = NSView()
        content.addSubview(root)
        NSLayoutConstraint.activate([
            root.topAnchor.constraint(equalTo: content.topAnchor),
            root.leadingAnchor.constraint(equalTo: content.leadingAnchor),
            root.trailingAnchor.constraint(equalTo: content.trailingAnchor),
            root.bottomAnchor.constraint(equalTo: content.bottomAnchor),
        ])
        win.contentView = content
        return win
    }
}

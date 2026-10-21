import AppKit

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
        var renewalCheckbox: NSButton!
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
            cfg.apiKeys = keysField.stringValue.split(separator: ",")
                .map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }
            cfg.cookieFile = cookieField.stringValue.isEmpty ? nil : cookieField.stringValue
            cfg.proxy = proxyField.stringValue.isEmpty ? nil : proxyField.stringValue
            cfg.authUser = authUserField.stringValue.isEmpty ? nil : authUserField.stringValue
            cfg.xsrfToken = xsrfField.stringValue.isEmpty ? nil : xsrfField.stringValue
            cfg.defaultModel = modelPopup.titleOfSelectedItem ?? cfg.defaultModel
            cfg.host = lanCheckbox.state == .on ? "0.0.0.0" : "127.0.0.1"
            cfg.sessionAutoRefresh = renewalCheckbox.state == .on
            onSave()
            window?.close()
        }

        func prepareFields() {
            portField = SettingsWindow.field(String(cfg.port))
            keysField = SettingsWindow.field(cfg.apiKeys.joined(separator: ", "),
                placeholder: "留空则免密；多个用逗号分隔")
            proxyField = SettingsWindow.field(cfg.proxy ?? "", placeholder: "http://127.0.0.1:7890")
            authUserField = SettingsWindow.field(cfg.authUser ?? "", placeholder: "登录账号序号，如 0")
            xsrfField = SettingsWindow.field(cfg.xsrfToken ?? "", placeholder: "关闭自动续期时手工填写")
            modelPopup = NSPopUpButton()
            modelPopup.addItems(withTitles: MODELS.map { $0.id })
            modelPopup.selectItem(withTitle: cfg.defaultModel)
            lanCheckbox = NSButton(checkboxWithTitle: "允许局域网访问", target: nil, action: nil)
            lanCheckbox.state = (cfg.host == "0.0.0.0" || cfg.host.isEmpty) ? .on : .off
            renewalCheckbox = NSButton(checkboxWithTitle: "自动续期登录 Cookie 和会话参数", target: nil, action: nil)
            renewalCheckbox.state = cfg.sessionAutoRefresh ? .on : .off
        }

        func cookieRow() -> NSView {
            cookieField = SettingsWindow.field(cfg.cookieFile ?? "",
                placeholder: "Google 登录 Cookie 文件", width: 220)
            let browse = NSButton(title: "选择…", target: self, action: #selector(browseCookie))
            return NSStackView(views: [cookieField, browse])
        }

        func makeGrid() -> NSGridView {
            let grid = NSGridView()
            grid.rowSpacing = 10
            grid.columnSpacing = 10
            let rows: [(String, NSView)] = [
                ("端口", portField), ("局域网", lanCheckbox), ("API Key 鉴权", keysField),
                ("默认模型", modelPopup), ("Cookie 文件", cookieRow()),
                ("登录续期", renewalCheckbox), ("代理", proxyField),
                ("auth_user", authUserField), ("xsrf_token", xsrfField),
            ]
            for (title, view) in rows {
                let label = NSTextField(labelWithString: title)
                label.alignment = .right
                grid.addRow(with: [label, view])
            }
            return grid
        }
    }

    static func make(onSave: @escaping () -> Void) -> NSWindow {
        let ctl = Controller(onSave: onSave)
        ctl.prepareFields()
        let win = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 480, height: 400),
                           styleMask: [.titled, .closable], backing: .buffered, defer: false)
        win.title = "Gemini Free 设置"
        win.center()
        ctl.window = win
        objc_setAssociatedObject(win, "ctl", ctl, .OBJC_ASSOCIATION_RETAIN)

        let saveBtn = NSButton(title: "保存并重启服务", target: ctl, action: #selector(Controller.save))
        saveBtn.keyEquivalent = "\r"

        let root = NSStackView(views: [ctl.makeGrid(), saveBtn])
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

    private static func field(_ value: String, placeholder: String = "", width: CGFloat = 300) -> NSTextField {
        let field = NSTextField(string: value)
        field.placeholderString = placeholder
        field.widthAnchor.constraint(equalToConstant: width).isActive = true
        return field
    }
}

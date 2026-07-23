import AppKit
import ServiceManagement

final class AppDelegate: NSObject, NSApplicationDelegate {
    private var statusItem: NSStatusItem!
    private let cfg = Store.shared
    private var settingsWindow: NSWindow?

    func applicationDidFinishLaunching(_ notification: Notification) {
        cfg.load()
        statusItem = NSStatusBar.system.statusItem(withLength: NSStatusItem.variableLength)
        statusItem.button?.title = "◐"
        buildMenu()
        startServer()
    }

    func applicationWillTerminate(_ notification: Notification) {
        HTTPServer.shared.stop()
    }

    // MARK: 菜单

    private func buildMenu() {
        let menu = NSMenu()
        let running = HTTPServer.shared.running
        let status = NSMenuItem(title: running ? "● 运行中  http://localhost:\(cfg.port)" : "○ 已停止", action: nil, keyEquivalent: "")
        status.isEnabled = false
        menu.addItem(status)
        menu.addItem(.separator())

        menu.addItem(withTitle: "复制 Base URL", action: #selector(copyBaseURL), keyEquivalent: "c").target = self
        menu.addItem(withTitle: running ? "停止服务" : "启动服务", action: #selector(toggleServer), keyEquivalent: "").target = self
        menu.addItem(withTitle: "设置…", action: #selector(openSettings), keyEquivalent: ",").target = self

        let launch = NSMenuItem(title: "开机自启", action: #selector(toggleLaunchAtLogin), keyEquivalent: "")
        launch.target = self
        launch.state = cfg.launchAtLogin ? .on : .off
        menu.addItem(launch)

        menu.addItem(.separator())
        menu.addItem(withTitle: "退出", action: #selector(NSApplication.terminate(_:)), keyEquivalent: "q")
        statusItem.menu = menu
    }

    private func refresh() {
        statusItem.button?.title = HTTPServer.shared.running ? "◉" : "○"
        buildMenu()
    }

    // MARK: 动作

    private func startServer() {
        do { try HTTPServer.shared.start() }
        catch { alert("启动失败", "端口 \(cfg.port) 可能被占用：\(error)") }
        refresh()
    }

    @objc private func toggleServer() {
        if HTTPServer.shared.running { HTTPServer.shared.stop(); refresh() }
        else { startServer() }
    }

    @objc private func copyBaseURL() {
        let s = "http://localhost:\(cfg.port)/v1"
        NSPasteboard.general.clearContents()
        NSPasteboard.general.setString(s, forType: .string)
    }

    @objc private func toggleLaunchAtLogin() {
        cfg.launchAtLogin.toggle()
        do {
            if cfg.launchAtLogin { try SMAppService.mainApp.register() }
            else { try SMAppService.mainApp.unregister() }
        } catch { alert("开机自启设置失败", "\(error)") }
        cfg.save()
        refresh()
    }

    // MARK: 设置窗口

    @objc private func openSettings() {
        if settingsWindow == nil { settingsWindow = SettingsWindow.make(onSave: { [weak self] in self?.applySettings() }) }
        NSApp.activate(ignoringOtherApps: true)
        settingsWindow?.makeKeyAndOrderFront(nil)
    }

    private func applySettings() {
        cfg.save()
        if HTTPServer.shared.running { HTTPServer.shared.stop(); startServer() } else { refresh() }
    }

    private func alert(_ title: String, _ msg: String) {
        let a = NSAlert()
        a.messageText = title
        a.informativeText = msg
        a.runModal()
    }
}

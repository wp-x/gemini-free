import AppKit

let app = NSApplication.shared
app.setActivationPolicy(.accessory) // 菜单栏应用，无 Dock 图标
let delegate = AppDelegate()
app.delegate = delegate
app.run()

package desktop

import (
	"gemini-free/internal/app"
	"gemini-free/internal/config"

	"github.com/egoist/mygo"
)

func applicationMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{Role: mygo.RoleAppMenu},
		{Label: "文件", Submenu: []*mygo.MenuItem{
			{Label: "关闭窗口", Role: mygo.RoleClose},
		}},
		{Role: mygo.RoleEditMenu},
	})
}

func (d *Desktop) menu() *mygo.Menu {
	cfg := d.controller.Config()
	state := d.controller.State()
	label := "启动服务"
	action := d.controller.Start
	if state.Service.Running {
		label, action = "停止服务", d.controller.Stop
	}
	return mygo.NewMenu([]*mygo.MenuItem{
		{Label: "打开 Gemini Free", Click: func(*mygo.MenuItem, *mygo.Window) { d.show() }},
		{Label: label, Click: func(*mygo.MenuItem, *mygo.Window) {
			go func() {
				if err := action(); err != nil {
					d.controller.AddLog("error", err.Error())
					d.show()
				}
			}()
		}},
		mygo.Separator(),
		d.modeItem("匿名模式", config.Anonymous, cfg.Mode),
		d.modeItem("登录会话模式", config.AccountMode, cfg.Mode),
		mygo.Separator(),
		{Label: "退出", Role: mygo.RoleQuit},
	})
}

func (d *Desktop) modeItem(label string, mode string, selected string) *mygo.MenuItem {
	return &mygo.MenuItem{Label: label, Type: mygo.MenuItemRadio, Checked: mode == selected,
		Click: func(*mygo.MenuItem, *mygo.Window) {
			go func() {
				cfg := d.controller.Config()
				cfg.Mode = mode
				if err := d.controller.Configure(app.SettingsUpdate{Config: cfg}); err != nil {
					d.controller.AddLog("error", err.Error())
					d.show()
				}
			}()
		}}
}

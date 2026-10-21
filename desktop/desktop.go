package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/egoist/mygo"

	"gemini-free/internal/app"
	"gemini-free/internal/config"
	"gemini-free/internal/gemini"
	"gemini-free/internal/network"
	"gemini-free/internal/vault"
)

const (
	windowWidth       = 1080
	windowHeight      = 800
	windowMinWidth    = 700
	windowMinHeight   = 540
	titleBarHeight    = 32
	trafficLightInset = 12
	desktopVersion    = "0.1.0"
)

type Options struct {
	ConfigPath  string
	Proxy       string
	Headless    bool
	NoStart     bool
	ControlFile string
}

type Desktop struct {
	options       Options
	window        *mygo.Window
	tray          *mygo.Tray
	controller    *app.Controller
	controlServer *http.Server
	cancelRefresh context.CancelFunc
	quitting      bool
	url           string
}

func Run(options Options) error {
	d := &Desktop{options: options}
	mygo.App.SetName("Gemini Free")
	mygo.App.SetVersion(desktopVersion)
	if !mygo.App.RequestSingleInstanceLock() {
		return fmt.Errorf("Gemini Free 已运行，已唤醒现有窗口")
	}
	mygo.App.OnSecondInstance(func(_ []string, _ string) { d.show() })
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { d.quitting = true })
	mygo.App.OnActivate(func(_ bool) { d.show() })
	mygo.App.OnQuit(d.shutdown)
	mygo.App.WhenReady(func() {
		if err := d.initialize(); err != nil {
			fmt.Println("Gemini Free:", err)
			mygo.App.Quit()
		}
	})
	return mygo.App.Run()
}

func (d *Desktop) initialize() error {
	path := d.options.ConfigPath
	if path == "" {
		var err error
		path, err = config.Path()
		if err != nil {
			return err
		}
	}
	cfg, err := config.Load(path)
	if err != nil {
		return err
	}
	if d.options.Proxy != "" {
		cfg.Proxy = d.options.Proxy
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	keys := mygo.Secrets.Service("dev.gemini-free.desktop")
	secrets := vault.New(vault.Options{Directory: filepath.Join(filepath.Dir(path), "credentials"),
		Keys: keys, NotFound: func(err error) bool { return errors.Is(err, mygo.ErrSecretNotFound) }})
	controller, err := app.New(app.Options{Config: cfg, Path: path, Credentials: secrets,
		Client: gemini.ClientFactory(), Addresses: network.Discover})
	if err != nil {
		return err
	}
	d.controller = controller
	ctx, cancel := context.WithCancel(context.Background())
	d.cancelRefresh = cancel
	go controller.RefreshLoop(ctx)
	return d.start()
}

func (d *Desktop) start() error {
	files, err := Files()
	if err != nil {
		return err
	}
	control := app.NewControl(d.controller, files)
	listener, err := control.Listen()
	if err != nil {
		return err
	}
	d.url = "http://" + control.Host + "/#" + control.Token
	d.controlServer = &http.Server{Handler: control, ReadHeaderTimeout: 10 * time.Second}
	go func() {
		if err := d.controlServer.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			d.controller.AddLog("error", "管理服务失败："+err.Error())
		}
	}()
	if !d.options.NoStart {
		if err := d.controller.Start(); err != nil {
			d.controller.AddLog("error", "API 启动失败："+err.Error())
		}
	}
	if d.options.ControlFile != "" {
		if err := d.writeControlFile(); err != nil {
			return err
		}
	}
	if d.options.Headless {
		return nil
	}
	return d.createUI()
}

func (d *Desktop) createUI() error {
	icon, err := trayIcon()
	if err != nil {
		return err
	}
	d.window = mygo.NewWindow(mygo.WindowOptions{
		Title: "Gemini Free", URL: d.url, Width: windowWidth, Height: windowHeight,
		MinWidth: windowMinWidth, MinHeight: windowMinHeight,
		TitleBarStyle: mygo.TitleBarHidden, TitleBarHeight: titleBarHeight,
		TrafficLightPosition: &mygo.Point{X: trafficLightInset, Y: trafficLightInset},
	})
	d.window.OnClose(func(event *mygo.CloseEvent) {
		if !d.quitting {
			event.PreventDefault()
			d.window.Hide()
		}
	})
	tray, err := mygo.NewTray(mygo.TrayOptions{Icon: icon, IconIsTemplate: true, ToolTip: "Gemini Free"})
	if err != nil {
		return fmt.Errorf("create tray: %w", err)
	}
	d.tray = tray
	tray.OnClick(d.show)
	tray.OnRightClick(func() {
		tray.SetMenu(d.menu())
		tray.PopUpMenu()
		tray.SetMenu(nil)
	})
	mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{{Role: mygo.RoleAppMenu}, {Role: mygo.RoleEditMenu}}))
	return nil
}

func (d *Desktop) show() {
	mygo.RunOnMain(func() {
		if d.window == nil {
			return
		}
		d.window.Show()
		d.window.Restore()
		d.window.Focus()
	})
}

func (d *Desktop) shutdown() {
	if d.cancelRefresh != nil {
		d.cancelRefresh()
	}
	if d.controlServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := d.controlServer.Shutdown(ctx); err != nil {
			fmt.Println("management shutdown:", err)
		}
		cancel()
	}
	if d.controller != nil {
		if err := d.controller.Close(); err != nil {
			fmt.Println("API shutdown:", err)
		}
	}
}

func (d *Desktop) writeControlFile() error {
	data, err := json.Marshal(map[string]string{"url": d.url})
	if err != nil {
		return err
	}
	return config.WritePrivate(d.options.ControlFile, data)
}

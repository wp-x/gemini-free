package desktop

import (
	"embed"
	"io/fs"
)

//go:embed all:web
var assets embed.FS

func Files() (fs.FS, error) { return fs.Sub(assets, "web") }

func trayIcon() ([]byte, error) { return assets.ReadFile("web/assets/tray.png") }

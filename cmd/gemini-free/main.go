package main

import (
	"flag"
	"fmt"
	"os"

	"gemini-free/desktop"
)

func main() {
	var options desktop.Options
	flag.StringVar(&options.ConfigPath, "config", "", "Configuration file")
	flag.StringVar(&options.Proxy, "proxy", "", "Explicit upstream proxy URL")
	flag.BoolVar(&options.Headless, "headless", false, "Run management/API services without a window")
	flag.BoolVar(&options.NoStart, "no-start", false, "Do not start the compatible API on launch")
	flag.StringVar(&options.ControlFile, "control-file", "", "Write private temporary management URL for local UI testing")
	flag.Parse()
	if err := desktop.Run(options); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

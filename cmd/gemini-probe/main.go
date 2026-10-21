package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"gemini-free/internal/config"
	"gemini-free/internal/gemini"
	"gemini-free/internal/generation"
)

const probeTimeout = 40

func main() {
	cfg := config.Default()
	model := flag.String("model", generation.DefaultModel, "Local model alias")
	prompt := flag.String("prompt", "17+25等于多少？只输出数字。", "Question")
	proxy := flag.String("proxy", "", "Explicit upstream proxy URL")
	stream := flag.Bool("stream", false, "Print real streaming deltas as JSON")
	expected := flag.String("expected", "", "Exact expected answer")
	flag.Parse()
	cfg.Proxy, cfg.TimeoutSeconds = *proxy, probeTimeout
	if err := cfg.Validate(); err != nil {
		fail(err)
	}
	engine := gemini.New(gemini.EngineOptions{Config: func() config.Config { return cfg }, Client: gemini.Client})
	emit := func(event generation.Event) error {
		if !*stream {
			return nil
		}
		return json.NewEncoder(os.Stdout).Encode(map[string]any{"delta": event})
	}
	result, err := engine.Generate(context.Background(), generation.Request{Model: *model, Prompt: *prompt}, emit)
	if err != nil {
		fail(err)
	}
	if err := json.NewEncoder(os.Stdout).Encode(result); err != nil {
		fail(err)
	}
	if *expected != "" && result.Text != *expected {
		fail(fmt.Errorf("answer mismatch: expected %q, got %q", *expected, result.Text))
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

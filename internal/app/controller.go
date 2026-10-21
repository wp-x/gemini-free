package app

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"gemini-free/internal/api"
	"gemini-free/internal/config"
	"gemini-free/internal/gemini"
	"gemini-free/internal/generation"
	"gemini-free/internal/network"
)

const (
	logCapacity     = 300
	shutdownTimeout = 5 * time.Second
)

type Credentials interface {
	gemini.Repository
	LoadBlob(string) ([]byte, error)
	SaveBlob(string, []byte) error
}

type Options struct {
	Config      config.Config
	Path        string
	Credentials Credentials
	Client      func(config.Config) (gemini.HTTPDoer, error)
	Addresses   func() ([]network.Address, error)
}

type Controller struct {
	mu          sync.RWMutex
	change      sync.Mutex
	cfg         config.Config
	path        string
	credentials Credentials
	sessions    *gemini.Sessions
	engine      generation.Generator
	client      func(config.Config) (gemini.HTTPDoer, error)
	keys        []string
	logs        []Log
	server      *http.Server
	service     Service
	addresses   func() ([]network.Address, error)
}

func New(options Options) (*Controller, error) {
	if options.Addresses == nil {
		return nil, fmt.Errorf("network address provider is required")
	}
	c := &Controller{cfg: options.Config, path: options.Path, credentials: options.Credentials,
		client: options.Client, addresses: options.Addresses, logs: []Log{}}
	keys, err := c.loadAPIKeys(options.Config.APIKeyRef)
	if err != nil {
		return nil, err
	}
	if err == nil {
		if err := json.Unmarshal(keys, &c.keys); err != nil {
			return nil, fmt.Errorf("read API keys: %w", err)
		}
	}
	c.sessions = gemini.NewSessions(gemini.SessionOptions{Repository: options.Credentials, Now: time.Now})
	c.engine = gemini.New(gemini.EngineOptions{Config: c.Config, Sessions: c.sessions, Client: options.Client})
	c.AddLog("info", "应用已就绪")
	return c, nil
}

func (c *Controller) loadAPIKeys(reference string) ([]byte, error) {
	if reference == "" {
		return []byte("[]"), nil
	}
	return c.credentials.LoadBlob(reference)
}

func (c *Controller) Config() config.Config {
	c.mu.RLock()
	defer c.mu.RUnlock()
	cfg := c.cfg
	cfg.Accounts = append([]config.Account(nil), c.cfg.Accounts...)
	return cfg
}

func (c *Controller) AddLog(level string, message string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.logs = append(c.logs, Log{Time: time.Now().UTC().Format(time.RFC3339), Level: level, Message: message})
	if len(c.logs) > logCapacity {
		c.logs = append([]Log(nil), c.logs[len(c.logs)-logCapacity:]...)
	}
}

func (c *Controller) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.RLock()
	keys := append([]string(nil), c.keys...)
	c.mu.RUnlock()
	cfg := c.Config()
	handler := api.New(api.Options{Generator: c, Models: generation.Models(),
		APIKeys: keys, DefaultModel: cfg.DefaultModel})
	handler.ServeHTTP(w, r)
}

func (c *Controller) Generate(ctx context.Context, request generation.Request, emit func(generation.Event) error) (generation.Result, error) {
	started := time.Now()
	result, err := c.engine.Generate(ctx, request, emit)
	if err != nil {
		c.AddLog("error", fmt.Sprintf("%s：%s", request.Model, err))
		return result, err
	}
	c.AddLog("info", fmt.Sprintf("%s → 路由 %d，思考 %d，耗时 %.2fs",
		request.Model, result.ActualMode, result.ThinkingLevel, time.Since(started).Seconds()))
	return result, nil
}

func (c *Controller) Close() error {
	return c.Stop()
}

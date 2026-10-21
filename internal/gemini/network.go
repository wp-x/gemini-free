package gemini

import (
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"gemini-free/internal/config"
)

func ClientFactory() func(config.Config) (HTTPDoer, error) {
	var mu sync.Mutex
	var previous *http.Client
	var proxy string
	var timeout int
	return func(cfg config.Config) (HTTPDoer, error) {
		mu.Lock()
		defer mu.Unlock()
		if previous != nil && proxy == cfg.Proxy && timeout == cfg.TimeoutSeconds {
			return previous, nil
		}
		next, err := Client(cfg)
		if err != nil {
			return nil, err
		}
		if previous != nil {
			previous.CloseIdleConnections()
		}
		previous = next.(*http.Client)
		proxy, timeout = cfg.Proxy, cfg.TimeoutSeconds
		return previous, nil
	}
}

func Client(cfg config.Config) (HTTPDoer, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	if cfg.Proxy != "" {
		proxy, err := url.Parse(cfg.Proxy)
		if err != nil {
			return nil, fmt.Errorf("proxy: %w", err)
		}
		transport.Proxy = http.ProxyURL(proxy)
	}
	return &http.Client{Transport: transport, Timeout: time.Duration(cfg.TimeoutSeconds) * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}}, nil
}

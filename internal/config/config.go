package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gemini-free/internal/generation"
)

const (
	Anonymous      = "anonymous"
	AccountMode    = "account"
	DefaultPort    = 8081
	DefaultTimeout = 180
	DefaultBuild   = "boq_gemini-web-uiserver_20261019.01_p0"
)

type Account struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	AuthUser string `json:"auth_user"`
}

type Config struct {
	Mode            string    `json:"mode"`
	SelectedAccount string    `json:"selected_account"`
	Host            string    `json:"host"`
	Port            int       `json:"port"`
	Proxy           string    `json:"proxy"`
	TimeoutSeconds  int       `json:"timeout_seconds"`
	DefaultModel    string    `json:"default_model"`
	Build           string    `json:"build"`
	Accounts        []Account `json:"accounts,omitempty"`
	APIKeyRef       string    `json:"api_key_ref,omitempty"`
}

func Default() Config {
	return Config{Mode: Anonymous, Host: "127.0.0.1", Port: DefaultPort,
		TimeoutSeconds: DefaultTimeout, DefaultModel: generation.DefaultModel,
		Build: DefaultBuild, Accounts: []Account{}}
}

func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Gemini Free", "config.json"), nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Default(), nil
	}
	if err != nil {
		return Config{}, err
	}
	cfg := Default()
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	return cfg, cfg.Validate()
}

func Save(path string, cfg Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return WritePrivate(path, data)
}

func (c Config) Address() string {
	return net.JoinHostPort(c.Host, strconv.Itoa(c.Port))
}

func (c Config) Validate() error {
	if c.Mode != Anonymous && c.Mode != AccountMode {
		return fmt.Errorf("mode must be anonymous or account")
	}
	if c.Port < 1 || c.Port > 65535 || c.TimeoutSeconds <= 0 {
		return fmt.Errorf("invalid port or timeout")
	}
	if net.ParseIP(c.Host) == nil && c.Host != "localhost" {
		return fmt.Errorf("host must be an IP address or localhost")
	}
	if _, err := generation.Resolve(generation.Request{Model: c.DefaultModel}); err != nil {
		return err
	}
	if strings.TrimSpace(c.Build) == "" {
		return fmt.Errorf("build must not be empty")
	}
	return validateProxy(c.Proxy)
}

func validateProxy(proxy string) error {
	if proxy == "" {
		return nil
	}
	u, err := url.Parse(proxy)
	if err != nil || u.Hostname() == "" {
		return fmt.Errorf("invalid proxy URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" && u.Scheme != "socks5" {
		return fmt.Errorf("proxy must use http, https or socks5")
	}
	return nil
}

package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestConfigRejectsInvalidFields(t *testing.T) {
	tests := map[string]func(*Config){
		"mode":        func(c *Config) { c.Mode = "auto" },
		"port low":    func(c *Config) { c.Port = 0 },
		"port high":   func(c *Config) { c.Port = 65536 },
		"timeout":     func(c *Config) { c.TimeoutSeconds = 0 },
		"host":        func(c *Config) { c.Host = "example.com" },
		"model":       func(c *Config) { c.DefaultModel = "unknown" },
		"build":       func(c *Config) { c.Build = " \n" },
		"proxy host":  func(c *Config) { c.Proxy = "http://" },
		"proxy type":  func(c *Config) { c.Proxy = "ftp://localhost:1234" },
		"proxy parse": func(c *Config) { c.Proxy = "http://[::1" },
	}
	for name, invalidate := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := Default()
			invalidate(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("invalid configuration accepted")
			}
		})
	}
}

func TestConfigDefaultsRoundTripAndPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.json")
	defaults, err := Load(path)
	if err != nil || defaults.Mode != Anonymous || defaults.Port != DefaultPort {
		t.Fatalf("missing file defaults: %+v err=%v", defaults, err)
	}
	defaults.Proxy = "socks5://127.0.0.1:7890"
	defaults.Accounts = []Account{{ID: "local", Name: "Example", AuthUser: "2"}}
	if err := Save(path, defaults); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.Proxy != defaults.Proxy || len(loaded.Accounts) != 1 {
		t.Fatalf("config roundtrip failed: %+v err=%v", loaded, err)
	}
	if runtime.GOOS == "windows" {
		return
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != privateFile {
		t.Fatalf("config permissions=%o want=%o", info.Mode().Perm(), privateFile)
	}
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil || parent.Mode().Perm() != privateDir {
		t.Fatalf("config directory is not private: %v %v", parent, err)
	}
}

func TestInvalidSavePreservesExistingConfiguration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	cfg := Default()
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Mode = "invalid"
	if err := Save(path, cfg); err == nil {
		t.Fatal("invalid save succeeded")
	}
	after, err := os.ReadFile(path)
	if err != nil || string(before) != string(after) {
		t.Fatal("invalid save modified existing configuration")
	}
}

func TestMalformedConfigurationDoesNotResetToDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"mode":`), privateFile); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("malformed configuration was silently reset")
	}
}

func TestAddressPreservesIPv6Brackets(t *testing.T) {
	cfg := Default()
	cfg.Host = "::1"
	if err := cfg.Validate(); err != nil {
		t.Fatal(err)
	}
	if cfg.Address() != "[::1]:8081" {
		t.Fatalf("invalid IPv6 listener address: %s", cfg.Address())
	}
}

func TestRetiredRefreshSettingIsDroppedOnSave(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := WritePrivate(path, []byte(`{"auto_refresh":true,"proxy":"http://127.0.0.1:7890"}`)); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(path)
	if err != nil || cfg.Proxy != "http://127.0.0.1:7890" {
		t.Fatalf("existing configuration did not load: %v", err)
	}
	if err := Save(path, cfg); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var stored map[string]any
	if err := json.Unmarshal(data, &stored); err != nil {
		t.Fatal(err)
	}
	if _, present := stored["auto_refresh"]; present {
		t.Fatal("retired refresh setting was written back")
	}
}

package app

import (
	"encoding/json"
	"fmt"
	"strings"

	"gemini-free/internal/config"
)

type SettingsUpdate struct {
	config.Config
	APIKeys *[]string `json:"api_keys"`
}

func (c *Controller) Configure(update SettingsUpdate) error {
	c.change.Lock()
	defer c.change.Unlock()
	cfg := update.Config
	current := c.Config()
	cfg.Accounts, cfg.APIKeyRef = current.Accounts, current.APIKeyRef
	c.mu.RLock()
	running := c.server != nil
	c.mu.RUnlock()
	if running && (cfg.Host != current.Host || cfg.Port != current.Port) {
		return fmt.Errorf("请先停止 API 服务，再修改监听地址")
	}
	if err := cfg.Validate(); err != nil {
		return err
	}
	if err := selectedAccountValid(cfg); err != nil {
		return err
	}
	keys, reference, err := c.updateKeys(update.APIKeys)
	if err != nil {
		return err
	}
	cfg.APIKeyRef = reference
	if err := config.Save(c.path, cfg); err != nil {
		return err
	}
	c.mu.Lock()
	c.cfg, c.keys = cfg, keys
	c.mu.Unlock()
	c.AddLog("info", "配置已保存，当前模式："+cfg.Mode)
	return nil
}

func selectedAccountValid(cfg config.Config) error {
	if cfg.Mode != config.AccountMode {
		return nil
	}
	for _, account := range cfg.Accounts {
		if account.ID == cfg.SelectedAccount {
			return nil
		}
	}
	return fmt.Errorf("账号模式需要选择登录会话")
}

func (c *Controller) updateKeys(input *[]string) ([]string, string, error) {
	c.mu.RLock()
	keys := append([]string(nil), c.keys...)
	c.mu.RUnlock()
	if input == nil {
		return keys, c.Config().APIKeyRef, nil
	}
	keys = append([]string(nil), (*input)...)
	for _, key := range keys {
		if strings.TrimSpace(key) == "" || strings.ContainsAny(key, "\r\n") {
			return nil, "", fmt.Errorf("API key must not be empty or contain newlines")
		}
	}
	data, err := json.Marshal(keys)
	if err != nil {
		return nil, "", err
	}
	reference := "api-keys-" + randomAccountID()
	if err := c.credentials.SaveBlob(reference, data); err != nil {
		return nil, "", err
	}
	return keys, reference, nil
}

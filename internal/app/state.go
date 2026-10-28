package app

import (
	"time"

	"gemini-free/internal/config"
	"gemini-free/internal/generation"
	"gemini-free/internal/network"
)

type Log struct {
	Time    string `json:"time"`
	Level   string `json:"level"`
	Message string `json:"message"`
}

type Service struct {
	Running bool   `json:"running"`
	Address string `json:"address"`
	Error   string `json:"error"`
}

type AccountSummary struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	AuthUser    string `json:"auth_user"`
	RefreshedAt string `json:"refreshed_at"`
	Status      string `json:"status"`
	Error       string `json:"error"`
}

type State struct {
	Config            config.Config      `json:"config"`
	Service           Service            `json:"service"`
	Accounts          []AccountSummary   `json:"accounts"`
	Models            []generation.Model `json:"models"`
	Logs              []Log              `json:"logs"`
	APIKeysConfigured int                `json:"api_keys_configured"`
	Network           network.State      `json:"network"`
}

func (c *Controller) State() State {
	cfg := c.Config()
	c.mu.RLock()
	state := State{Config: cfg, Service: c.service, Models: generation.Models(),
		Logs: append([]Log(nil), c.logs...), APIKeysConfigured: len(c.keys)}
	c.mu.RUnlock()
	state.Network = c.networkState(cfg)
	state.Accounts = make([]AccountSummary, 0, len(cfg.Accounts))
	for _, account := range cfg.Accounts {
		state.Accounts = append(state.Accounts, c.accountSummary(account))
	}
	return state
}

func (c *Controller) networkState(cfg config.Config) network.State {
	if !network.Enabled(cfg.Host) {
		return network.Describe(cfg.Host, cfg.Port, nil)
	}
	addresses, err := c.addresses()
	state := network.Describe(cfg.Host, cfg.Port, addresses)
	if err != nil {
		state.Error = "无法读取局域网地址：" + err.Error()
	}
	return state
}

func (c *Controller) accountSummary(account config.Account) AccountSummary {
	summary := AccountSummary{ID: account.ID, Name: account.Name, AuthUser: account.AuthUser, Status: "待验证"}
	doc, err := c.credentials.Load(account.ID)
	if err != nil {
		summary.Status, summary.Error = "读取失败", err.Error()
		return summary
	}
	if !doc.RefreshedAt.IsZero() {
		summary.RefreshedAt = doc.RefreshedAt.Format(time.RFC3339)
		summary.Status = "参数已同步，模型权限待问答验证"
	}
	c.mu.RLock()
	summary.Error = c.parameterErrors[account.ID]
	c.mu.RUnlock()
	if summary.Error != "" {
		summary.Status = "参数同步失败"
	}
	return summary
}

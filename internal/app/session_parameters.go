package app

import (
	"context"
	"fmt"
	"gemini-free/internal/gemini"
)

func (c *Controller) SyncAccountParameters(ctx context.Context, id string) error {
	err := c.syncAccountParameters(ctx, id)
	c.mu.Lock()
	if err != nil {
		c.parameterErrors[id] = err.Error()
	} else {
		delete(c.parameterErrors, id)
	}
	c.mu.Unlock()
	if err != nil {
		c.AddLog("error", "会话参数同步失败："+err.Error())
		return err
	}
	c.AddLog("info", "会话参数已同步")
	return nil
}

func (c *Controller) syncAccountParameters(ctx context.Context, id string) error {
	cfg := c.Config()
	client, err := c.client(cfg)
	if err != nil {
		return err
	}
	for _, account := range cfg.Accounts {
		if account.ID == id {
			_, err := c.sessions.SyncParameters(ctx, gemini.SessionRequest{ID: id, AuthUser: account.AuthUser, Client: client})
			return err
		}
	}
	return fmt.Errorf("unknown account")
}

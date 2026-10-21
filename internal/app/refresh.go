package app

import (
	"context"
	"time"

	"gemini-free/internal/config"
)

const backgroundRefreshInterval = 3 * time.Minute

func (c *Controller) RefreshLoop(ctx context.Context) {
	ticker := time.NewTicker(backgroundRefreshInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			cfg := c.Config()
			if cfg.Mode == config.AccountMode && cfg.AutoRefresh {
				if err := c.RefreshAccount(ctx, cfg.SelectedAccount); err != nil {
					c.AddLog("error", "后台续期失败："+err.Error())
				}
			}
		}
	}
}

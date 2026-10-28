package app

import (
	"fmt"
	"slices"

	"gemini-free/internal/config"
)

func (c *Controller) ReplaceAccountSession(id string, input AccountInput) error {
	c.change.Lock()
	defer c.change.Unlock()
	cfg := c.Config()
	index := slices.IndexFunc(cfg.Accounts, func(account config.Account) bool { return account.ID == id })
	if index < 0 {
		return fmt.Errorf("unknown account")
	}
	doc, err := accountDocument(input)
	if err != nil {
		return err
	}
	account := cfg.Accounts[index]
	if doc.AuthUser != "" && accountNumber(doc.AuthUser) != accountNumber(account.AuthUser) {
		return fmt.Errorf("会话账号序号不一致，请导入当前账号的 Cookie，或新增登录会话")
	}
	doc.AuthUser = account.AuthUser
	if err := c.sessions.Replace(id, doc); err != nil {
		return err
	}
	c.mu.Lock()
	delete(c.parameterErrors, id)
	c.mu.Unlock()
	c.AddLog("info", "登录会话 Cookie 已更新，请通过问答验证模型权限")
	return nil
}

func accountNumber(value string) string {
	if value == "" {
		return "0"
	}
	return value
}

package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strings"

	"gemini-free/internal/config"
	"gemini-free/internal/gemini"
)

type AccountInput struct {
	Name     string `json:"name"`
	Cookie   string `json:"cookie"`
	AuthUser string `json:"auth_user"`
	XSRF     string `json:"xsrf_token"`
	Build    string `json:"build"`
}

func (c *Controller) AddAccount(input AccountInput) error {
	c.change.Lock()
	defer c.change.Unlock()
	doc, err := accountDocument(input)
	if err != nil {
		return err
	}
	id := randomAccountID()
	if err := c.credentials.Save(id, doc); err != nil {
		return err
	}
	cfg := c.Config()
	name := strings.TrimSpace(input.Name)
	if name == "" {
		name = fmt.Sprintf("登录会话 %d", len(cfg.Accounts)+1)
	}
	cfg.Accounts = append(cfg.Accounts, config.Account{ID: id, Name: name, AuthUser: doc.AuthUser})
	if err := config.Save(c.path, cfg); err != nil {
		return err
	}
	c.mu.Lock()
	c.cfg = cfg
	c.mu.Unlock()
	c.AddLog("info", "登录会话已加密导入")
	return nil
}

func accountDocument(input AccountInput) (gemini.Document, error) {
	doc, err := gemini.ImportDocument(input.Cookie)
	if err != nil {
		return gemini.Document{}, err
	}
	if input.AuthUser != "" {
		doc.AuthUser = input.AuthUser
	}
	if strings.IndexFunc(doc.AuthUser, func(r rune) bool { return r < '0' || r > '9' }) >= 0 {
		return gemini.Document{}, fmt.Errorf("账号编号必须为空或数字")
	}
	if input.XSRF != "" {
		doc.XSRF = input.XSRF
	}
	if input.Build != "" {
		doc.Build = input.Build
	}
	return doc, nil
}

func (c *Controller) DeleteAccount(id string) error {
	c.change.Lock()
	defer c.change.Unlock()
	cfg := c.Config()
	index := slices.IndexFunc(cfg.Accounts, func(account config.Account) bool { return account.ID == id })
	if index < 0 {
		return fmt.Errorf("unknown account")
	}
	if cfg.Mode == config.AccountMode && cfg.SelectedAccount == id {
		return fmt.Errorf("请先切换模式或选择其他会话，再删除当前会话")
	}
	next := cfg
	next.Accounts = slices.Delete(slices.Clone(cfg.Accounts), index, index+1)
	if next.SelectedAccount == id {
		next.SelectedAccount = ""
	}
	if err := config.Save(c.path, next); err != nil {
		return err
	}
	if err := c.sessions.Remove(id); err != nil {
		return errors.Join(err, config.Save(c.path, cfg))
	}
	c.mu.Lock()
	c.cfg = next
	delete(c.parameterErrors, id)
	c.mu.Unlock()
	c.AddLog("info", "登录会话已删除")
	return nil
}

func randomAccountID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(id[:])
}

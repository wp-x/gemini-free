package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gemini-free/internal/gemini"
)

func (c *Controller) ImportLegacy() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	path := filepath.Join(home, ".config", "gemini-web2api", "config.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("读取旧版配置：%w", err)
	}
	input, err := legacyAccount(data, home)
	if err != nil {
		return err
	}
	return c.AddAccount(input)
}

func legacyAccount(data []byte, home string) (AccountInput, error) {
	var legacy struct {
		CookieFile string          `json:"cookie_file"`
		AuthUser   json.RawMessage `json:"auth_user"`
		XSRF       string          `json:"xsrf_token"`
		Build      string          `json:"gemini_bl"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return AccountInput{}, fmt.Errorf("解析旧版配置：%w", err)
	}
	if legacy.CookieFile == "" {
		return AccountInput{}, fmt.Errorf("旧版配置没有 Cookie 文件")
	}
	path := legacy.CookieFile
	if strings.HasPrefix(path, "~/") {
		path = filepath.Join(home, path[2:])
	}
	cookie, err := os.ReadFile(path)
	if err != nil {
		return AccountInput{}, fmt.Errorf("读取旧版 Cookie 文件：%w", err)
	}
	authUser, err := legacyAuthUser(legacy.AuthUser)
	if err != nil {
		return AccountInput{}, err
	}
	doc, err := gemini.ImportDocument(string(cookie))
	if err != nil {
		return AccountInput{}, err
	}
	if doc.XSRF != "" {
		legacy.XSRF = doc.XSRF
	}
	if doc.Build != "" {
		legacy.Build = doc.Build
	}
	return AccountInput{Name: "旧版 Gemini 会话", Cookie: string(cookie),
		AuthUser: authUser, XSRF: legacy.XSRF, Build: legacy.Build}, nil
}

func legacyAuthUser(data json.RawMessage) (string, error) {
	if len(data) == 0 || string(data) == "null" {
		return "", nil
	}
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		return text, nil
	}
	var number int
	if err := json.Unmarshal(data, &number); err != nil {
		return "", fmt.Errorf("旧版账号编号格式错误")
	}
	return strconv.Itoa(number), nil
}

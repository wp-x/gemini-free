package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
)

const origin = "https://gemini.google.com"
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

func (s *Sessions) syncParameters(ctx context.Context, request SessionRequest, doc Document) (Document, error) {
	if err := validateAuthUser(request.AuthUser); err != nil {
		return Document{}, err
	}
	page, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+accountPrefix(request.AuthUser)+"/app", nil)
	if err != nil {
		return Document{}, err
	}
	page.Header.Set("Cookie", doc.Cookie)
	page.Header.Set("User-Agent", userAgent)
	reply, err := request.Client.Do(page)
	if err != nil {
		return Document{}, fmt.Errorf("读取 Gemini 会话参数：%w", err)
	}
	defer reply.Body.Close()
	if err := checkSessionStatus(reply, sessionPageStage); err != nil {
		return Document{}, err
	}
	data, err := io.ReadAll(reply.Body)
	if err != nil {
		return Document{}, err
	}
	metadata, err := pageMetadata(string(data))
	if err != nil {
		return Document{}, err
	}
	next := doc
	next.XSRF, next.Build = metadata["SNlM0e"], metadata["cfb2h"]
	next.AuthUser, next.RefreshedAt = request.AuthUser, s.now()
	return next, s.repository.Save(request.ID, next)
}

func pageMetadata(html string) (map[string]string, error) {
	result := make(map[string]string)
	for _, key := range []string{"SNlM0e", "cfb2h"} {
		pattern := regexp.MustCompile(`"` + key + `"\s*:\s*("(?:[^"\\]|\\.)*")`)
		match := pattern.FindStringSubmatch(html)
		if len(match) != 2 {
			return nil, fmt.Errorf("Gemini 页面缺少 %s，请确认账号登录有效", key)
		}
		var value string
		if err := json.Unmarshal([]byte(match[1]), &value); err != nil || value == "" {
			return nil, fmt.Errorf("invalid session metadata %s", key)
		}
		result[key] = value
	}
	return result, nil
}

func validateAuthUser(value string) error {
	for _, digit := range value {
		if digit < '0' || digit > '9' {
			return fmt.Errorf("账号编号必须为空或数字")
		}
	}
	return nil
}

func accountPrefix(authUser string) string {
	if authUser == "" {
		return ""
	}
	return "/u/" + authUser
}

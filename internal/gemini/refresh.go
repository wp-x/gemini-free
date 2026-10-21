package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const origin = "https://gemini.google.com"
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.36"

func (s *Sessions) renew(ctx context.Context, request SessionRequest, doc Document) (Document, error) {
	if err := validateAuthUser(request.AuthUser); err != nil {
		return Document{}, err
	}
	if doc.RotatedAt.IsZero() || s.now().Sub(doc.RotatedAt) >= refreshInterval {
		rotated, err := s.rotate(ctx, request, doc)
		if err != nil {
			return Document{}, err
		}
		doc = rotated
	}
	page, err := http.NewRequestWithContext(ctx, http.MethodGet, origin+accountPrefix(request.AuthUser)+"/app", nil)
	if err != nil {
		return Document{}, err
	}
	page.Header.Set("Cookie", doc.Cookie)
	page.Header.Set("User-Agent", userAgent)
	reply, err := request.Client.Do(page)
	if err != nil {
		return Document{}, fmt.Errorf("refresh session page: %w", err)
	}
	defer reply.Body.Close()
	doc, err = s.saveResponse(request.ID, doc, reply)
	if err != nil {
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
	doc.XSRF, doc.Build = metadata["SNlM0e"], metadata["cfb2h"]
	doc.AuthUser, doc.RefreshedAt = request.AuthUser, s.now()
	return doc, s.repository.Save(request.ID, doc)
}

func (s *Sessions) rotate(ctx context.Context, request SessionRequest, doc Document) (Document, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://accounts.google.com/RotateCookies",
		strings.NewReader(`[000,"-0000000000000000000"]`))
	if err != nil {
		return Document{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", "https://accounts.google.com")
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Cookie", doc.Cookie)
	reply, err := request.Client.Do(req)
	if err != nil {
		return Document{}, fmt.Errorf("rotate cookies: %w", err)
	}
	defer reply.Body.Close()
	next, err := s.saveResponse(request.ID, doc, reply)
	if err != nil {
		return Document{}, err
	}
	if !hasRotation(reply) {
		return Document{}, fmt.Errorf("续期响应未返回有效 __Secure-1PSIDTS")
	}
	next.RotatedAt = s.now()
	return next, s.repository.Save(request.ID, next)
}

func hasRotation(response *http.Response) bool {
	for _, cookie := range response.Cookies() {
		if cookie.Name == "__Secure-1PSIDTS" && cookie.Value != "" && cookie.MaxAge >= 0 {
			return true
		}
	}
	return false
}

func (s *Sessions) saveResponse(id string, doc Document, response *http.Response) (Document, error) {
	next, err := mergeCookies(doc, response.Cookies(), s.now())
	if err != nil {
		return Document{}, err
	}
	if err := s.repository.Save(id, next); err != nil {
		return Document{}, err
	}
	if response.StatusCode != http.StatusOK {
		return Document{}, fmt.Errorf("session HTTP %d", response.StatusCode)
	}
	return next, nil
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

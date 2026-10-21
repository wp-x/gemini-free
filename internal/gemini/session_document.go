package gemini

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const refreshInterval = 3 * time.Minute

type Document struct {
	Cookie      string    `json:"cookie"`
	XSRF        string    `json:"xsrf_token"`
	Build       string    `json:"gemini_bl"`
	AuthUser    string    `json:"session_auth_user"`
	RefreshedAt time.Time `json:"refreshed_at"`
	RotatedAt   time.Time `json:"rotated_at"`
}

type Repository interface {
	Load(string) (Document, error)
	Save(string, Document) error
	Delete(string) error
}

func ImportDocument(text string) (Document, error) {
	text = strings.TrimSpace(text)
	doc := Document{Cookie: text}
	if strings.HasPrefix(text, "{") {
		if err := json.Unmarshal([]byte(text), &doc); err != nil {
			return Document{}, fmt.Errorf("invalid session JSON: %w", err)
		}
	}
	if strings.HasPrefix(text, "[") {
		cookie, err := importCookieArray(text)
		if err != nil {
			return Document{}, err
		}
		doc = Document{Cookie: cookie}
	}
	if _, err := cookieMap(doc.Cookie); err != nil {
		return Document{}, err
	}
	values, _ := cookieMap(doc.Cookie)
	if values["__Secure-1PSID"] == "" {
		return Document{}, fmt.Errorf("登录会话需要 __Secure-1PSID Cookie")
	}
	return doc, nil
}

func cookieMap(header string) (map[string]string, error) {
	if strings.TrimSpace(header) == "" || strings.ContainsAny(header, "\r\n") {
		return nil, fmt.Errorf("invalid Cookie header")
	}
	cookies, err := http.ParseCookie(header)
	if err != nil {
		return nil, fmt.Errorf("invalid Cookie header")
	}
	values := make(map[string]string, len(cookies))
	for _, cookie := range cookies {
		if _, exists := values[cookie.Name]; exists {
			return nil, fmt.Errorf("duplicate Cookie name; import one account at a time")
		}
		values[cookie.Name] = cookie.Value
	}
	return values, nil
}

func mergeCookies(doc Document, received []*http.Cookie, now time.Time) (Document, error) {
	if len(received) == 0 {
		return doc, nil
	}
	values, err := cookieMap(doc.Cookie)
	if err != nil {
		return Document{}, err
	}
	for _, cookie := range received {
		if !acceptCookie(cookie) {
			continue
		}
		if cookie.MaxAge < 0 || (!cookie.Expires.IsZero() && cookie.Expires.Before(now)) {
			delete(values, cookie.Name)
			continue
		}
		values[cookie.Name] = cookie.Value
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	pairs := make([]string, 0, len(names))
	for _, name := range names {
		pairs = append(pairs, name+"="+values[name])
	}
	doc.Cookie = strings.Join(pairs, "; ")
	return doc, nil
}

func acceptCookie(cookie *http.Cookie) bool {
	domain := strings.TrimPrefix(strings.ToLower(cookie.Domain), ".")
	validDomain := domain == "" || domain == "google.com" || domain == "gemini.google.com"
	return validDomain && (cookie.Path == "" || cookie.Path == "/")
}

func importCookieArray(text string) (string, error) {
	var cookies []struct {
		Name   string `json:"name"`
		Value  string `json:"value"`
		Domain string `json:"domain"`
		Path   string `json:"path"`
	}
	if err := json.Unmarshal([]byte(text), &cookies); err != nil {
		return "", fmt.Errorf("invalid cookie export: %w", err)
	}
	var pairs []string
	for _, cookie := range cookies {
		if acceptCookie(&http.Cookie{Domain: cookie.Domain, Path: cookie.Path}) {
			pairs = append(pairs, cookie.Name+"="+cookie.Value)
		}
	}
	return strings.Join(pairs, "; "), nil
}

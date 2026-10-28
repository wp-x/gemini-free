package gemini

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestSessionHTTPErrorIdentifiesFailedStage(t *testing.T) {
	cases := []struct {
		name  string
		stage string
		code  int
	}{
		{"page forbidden", sessionPageStage, http.StatusForbidden},
		{"page limited", sessionPageStage, http.StatusTooManyRequests},
		{"page unauthorized", sessionPageStage, http.StatusUnauthorized},
		{"page redirect", sessionPageStage, http.StatusFound},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			assertParameterHTTPError(t, test.stage, test.code)
		})
	}
}

func assertParameterHTTPError(t *testing.T, stage string, code int) {
	t.Helper()
	repo := &memoryRepository{doc: Document{Cookie: "__Secure-1PSID=private-identity"}}
	sessions := NewSessions(SessionOptions{Repository: repo, Now: time.Now})
	var calls int
	client := doFunc(func(request *http.Request) (*http.Response, error) {
		calls++
		if request.URL.Host != "gemini.google.com" || request.Method != http.MethodGet {
			t.Error("parameter sync requested a non-Gemini endpoint")
		}
		cookies := "__Secure-1PSIDTS=server-replacement; Domain=.google.com; Path=/"
		return &http.Response{StatusCode: code, Header: http.Header{"Set-Cookie": {cookies}},
			Body: io.NopCloser(strings.NewReader("upstream-error-body"))}, nil
	})
	_, err := sessions.SyncParameters(context.Background(), SessionRequest{ID: "one", Client: client})
	var upstream *SessionHTTPError
	if !errors.As(err, &upstream) || upstream.Stage != stage || upstream.StatusCode != code {
		t.Fatalf("wrong session error: %v", err)
	}
	if !strings.Contains(err.Error(), stage) || strings.Contains(err.Error(), "private-identity") {
		t.Fatal("error omitted the request stage or exposed credentials")
	}
	if calls != 1 {
		t.Fatalf("made %d requests, expected 1", calls)
	}
	saved, _ := repo.Load("one")
	if saved.Cookie != "__Secure-1PSID=private-identity" || !saved.RefreshedAt.IsZero() {
		t.Fatal("failed parameter sync modified the stored session")
	}
}

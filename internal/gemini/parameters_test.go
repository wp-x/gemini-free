package gemini

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestParameterSyncChangesAccountPathAndPreservesCookie(t *testing.T) {
	original := Document{Cookie: "__Secure-1PSID=identity", AuthUser: "0", XSRF: "old", Build: "old"}
	repo := &memoryRepository{doc: original}
	sessions := NewSessions(SessionOptions{Repository: repo, Now: time.Now})
	client := doFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Path != "/u/2/app" || r.Header.Get("Cookie") != original.Cookie {
			t.Error("wrong account page or cookie")
		}
		body := `{"SNlM0e":"new-token","cfb2h":"new-build"}`
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	doc, err := sessions.Snapshot(context.Background(), SessionRequest{ID: "one", AuthUser: "2", Client: client})
	if err != nil || doc.AuthUser != "2" || doc.XSRF != "new-token" || doc.Cookie != original.Cookie {
		t.Fatalf("account parameters were not synchronized: %v", err)
	}
}

func TestParameterSyncFailuresDoNotModifySession(t *testing.T) {
	original := Document{Cookie: "__Secure-1PSID=identity", XSRF: "saved-token", Build: "saved-build"}
	for _, body := range []string{`{}`, `{"SNlM0e":"","cfb2h":"build"}`, `{"SNlM0e":"token"}`} {
		repo := &memoryRepository{doc: original}
		sessions := NewSessions(SessionOptions{Repository: repo, Now: time.Now})
		client := doFunc(func(*http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		})
		_, err := sessions.SyncParameters(context.Background(), SessionRequest{ID: "one", Client: client})
		saved, _ := repo.Load("one")
		if err == nil || saved != original {
			t.Fatal("invalid page parameters were accepted or changed the session")
		}
	}
}

func TestParameterSyncExposesNetworkFailure(t *testing.T) {
	repo := &memoryRepository{doc: Document{Cookie: "__Secure-1PSID=identity"}}
	sessions := NewSessions(SessionOptions{Repository: repo, Now: time.Now})
	client := doFunc(func(*http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection disconnected")
	})
	_, err := sessions.SyncParameters(context.Background(), SessionRequest{ID: "one", Client: client})
	if err == nil || !strings.Contains(err.Error(), "connection disconnected") {
		t.Fatalf("network error hidden: %v", err)
	}
}

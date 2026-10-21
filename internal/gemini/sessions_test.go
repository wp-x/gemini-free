package gemini

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type memoryRepository struct {
	mu  sync.Mutex
	doc Document
}

func (m *memoryRepository) Load(string) (Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.doc, nil
}

func (m *memoryRepository) Save(_ string, doc Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.doc = doc
	return nil
}

func (m *memoryRepository) Delete(string) error { return nil }

func TestRefreshSerializesAndPersistsRotation(t *testing.T) {
	now := time.Now()
	repo := &memoryRepository{doc: Document{Cookie: "__Secure-1PSID=identity; SAPISID=signed"}}
	sessions := NewSessions(SessionOptions{Repository: repo, Now: func() time.Time { return now }})
	var calls atomic.Int32
	client := doFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		headers := http.Header{}
		body := `{"SNlM0e":"new-token","cfb2h":"new-build"}`
		if r.URL.Host == "accounts.google.com" {
			headers.Set("Set-Cookie", "__Secure-1PSIDTS=rotated; Domain=.google.com; Path=/; Secure")
			body = "[]"
		}
		return &http.Response{StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	var wait sync.WaitGroup
	for range 8 {
		wait.Go(func() {
			doc, err := sessions.Snapshot(context.Background(), SessionRequest{ID: "one", AutoRefresh: true, Client: client})
			if err != nil || doc.XSRF != "new-token" || doc.Build != "new-build" {
				t.Errorf("doc=%+v err=%v", doc, err)
			}
		})
	}
	wait.Wait()
	if calls.Load() != 2 {
		t.Fatalf("concurrent refresh used %d requests, expected 2", calls.Load())
	}
}

func TestRotationSavedEvenIfPageFails(t *testing.T) {
	repo := &memoryRepository{doc: Document{Cookie: "__Secure-1PSID=identity"}}
	sessions := NewSessions(SessionOptions{Repository: repo, Now: time.Now})
	client := doFunc(func(r *http.Request) (*http.Response, error) {
		if r.URL.Host != "accounts.google.com" {
			return nil, fmt.Errorf("page disconnected")
		}
		headers := http.Header{"Set-Cookie": {"__Secure-1PSIDTS=rotated; Domain=.google.com; Path=/"}}
		return &http.Response{StatusCode: 200, Header: headers, Body: io.NopCloser(strings.NewReader("[]"))}, nil
	})
	if _, err := sessions.Refresh(context.Background(), SessionRequest{ID: "one", Client: client}); err == nil {
		t.Fatal("refresh error hidden")
	}
	saved, _ := repo.Load("one")
	if !strings.Contains(saved.Cookie, "__Secure-1PSIDTS=rotated") || saved.RotatedAt.IsZero() {
		t.Fatal("server-side rotation lost")
	}
}

func TestStaleResponseDoesNotOverwriteNewSession(t *testing.T) {
	original := Document{Cookie: "__Secure-1PSID=old"}
	current := Document{Cookie: "__Secure-1PSID=new"}
	repo := &memoryRepository{doc: current}
	sessions := NewSessions(SessionOptions{Repository: repo, Now: time.Now})
	err := sessions.Receive("one", original, []*http.Cookie{{Name: "__Secure-1PSIDTS", Value: "stale"}})
	saved, _ := repo.Load("one")
	if err != nil || saved.Cookie != current.Cookie {
		t.Fatal("stale response replaced current credentials")
	}
}

func TestCookieImportsValidateAndFilter(t *testing.T) {
	for _, input := range []string{"", "bad\r\ncookie", "SAPISID=only", "__Secure-1PSID=one; __Secure-1PSID=two"} {
		if _, err := ImportDocument(input); err == nil {
			t.Errorf("accepted invalid cookie")
		}
	}
	doc, err := ImportDocument(`[{"name":"__Secure-1PSID","value":"identity","domain":".google.com","path":"/"},{"name":"unrelated","value":"secret","domain":"example.com","path":"/"}]`)
	if err != nil || strings.Contains(doc.Cookie, "unrelated") {
		t.Fatalf("cookie filter failed: %v", err)
	}
}

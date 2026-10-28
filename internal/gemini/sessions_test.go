package gemini

import (
	"context"
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

func TestMissingParametersFetchedOnceWithoutChangingCookies(t *testing.T) {
	now := time.Now()
	repo := &memoryRepository{doc: Document{Cookie: "__Secure-1PSID=identity; SAPISID=signed"}}
	original := repo.doc.Cookie
	sessions := NewSessions(SessionOptions{Repository: repo, Now: func() time.Time { return now }})
	var calls atomic.Int32
	client := doFunc(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		if r.Method != http.MethodGet || r.URL.String() != origin+"/app" || r.Header.Get("Cookie") != original {
			t.Error("parameter sync did not use the imported cookie on the Gemini page")
		}
		headers := http.Header{"Set-Cookie": {"__Secure-1PSIDTS=replacement; Domain=.google.com; Path=/"}}
		body := `{"SNlM0e":"new-token","cfb2h":"new-build"}`
		return &http.Response{StatusCode: http.StatusOK, Header: headers, Body: io.NopCloser(strings.NewReader(body))}, nil
	})
	var wait sync.WaitGroup
	const workers = 8
	for range workers {
		wait.Go(func() {
			doc, err := sessions.Snapshot(context.Background(), SessionRequest{ID: "one", Client: client})
			if err != nil || doc.XSRF != "new-token" || doc.Build != "new-build" || doc.Cookie != original {
				t.Errorf("parameter sync changed the cookie or failed: %v", err)
			}
		})
	}
	wait.Wait()
	if calls.Load() != 1 {
		t.Fatalf("concurrent parameter sync used %d requests, expected 1", calls.Load())
	}
	saved, _ := repo.Load("one")
	if saved.Cookie != original || !saved.RefreshedAt.Equal(now) {
		t.Fatal("parameter sync did not preserve credentials and save its timestamp")
	}
}

func TestExistingParametersNeverExpireLocally(t *testing.T) {
	for _, synced := range []time.Time{{}, time.Unix(0, 0)} {
		doc := Document{Cookie: "__Secure-1PSID=identity", XSRF: "saved-token", Build: "saved-build", RefreshedAt: synced}
		repo := &memoryRepository{doc: doc}
		sessions := NewSessions(SessionOptions{Repository: repo, Now: time.Now})
		client := doFunc(func(*http.Request) (*http.Response, error) {
			t.Fatal("existing parameters triggered an automatic request")
			return nil, nil
		})
		got, err := sessions.Snapshot(context.Background(), SessionRequest{ID: "one", Client: client})
		if err != nil || got != doc {
			t.Fatalf("stored parameters were not reused: %v", err)
		}
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

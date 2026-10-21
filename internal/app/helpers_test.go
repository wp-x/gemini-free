package app

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gemini-free/internal/config"
	"gemini-free/internal/gemini"
	"gemini-free/internal/network"
)

type memoryCredentials struct {
	mu    sync.Mutex
	docs  map[string]gemini.Document
	blobs map[string][]byte
}

func newCredentials() *memoryCredentials {
	return &memoryCredentials{docs: make(map[string]gemini.Document), blobs: make(map[string][]byte)}
}

func (m *memoryCredentials) Load(id string) (gemini.Document, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	doc, ok := m.docs[id]
	if !ok {
		return gemini.Document{}, os.ErrNotExist
	}
	return doc, nil
}

func (m *memoryCredentials) Save(id string, doc gemini.Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.docs[id] = doc
	return nil
}

func (m *memoryCredentials) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.docs, id)
	return nil
}

func (m *memoryCredentials) LoadBlob(id string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.blobs[id]
	if !ok {
		return nil, os.ErrNotExist
	}
	return append([]byte(nil), data...), nil
}

func (m *memoryCredentials) SaveBlob(id string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.blobs[id] = append([]byte(nil), data...)
	return nil
}

type doerFunc func(*http.Request) (*http.Response, error)

func (f doerFunc) Do(request *http.Request) (*http.Response, error) {
	return f(request)
}

func testController(t *testing.T) (*Controller, *memoryCredentials) {
	t.Helper()
	repo := newCredentials()
	c, err := New(Options{Config: config.Default(), Path: filepath.Join(t.TempDir(), "config.json"),
		Credentials: repo, Addresses: func() ([]network.Address, error) { return nil, nil },
		Client: func(config.Config) (gemini.HTTPDoer, error) {
			return doerFunc(func(*http.Request) (*http.Response, error) {
				return nil, fmt.Errorf("unexpected upstream request")
			}), nil
		}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Error(err)
		}
	})
	return c, repo
}

func testControl(t *testing.T, c *Controller) *Control {
	t.Helper()
	control := NewControl(c, os.DirFS(t.TempDir()))
	control.Host = "127.0.0.1:23456"
	return control
}

func controlRequest(control *Control, method, path string) *http.Request {
	request := httptest.NewRequest(method, "http://"+control.Host+path, nil)
	request.Header.Set("X-Desktop-Token", control.Token)
	return request
}

func controlJSON(control *Control, path string, value any) *httptest.ResponseRecorder {
	data, _ := json.Marshal(value)
	request := controlRequest(control, http.MethodPost, path)
	request.Body = io.NopCloser(strings.NewReader(string(data)))
	response := httptest.NewRecorder()
	control.ServeHTTP(response, request)
	return response
}

func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	return port
}

func expectStatus(t *testing.T, response *httptest.ResponseRecorder, status int) {
	t.Helper()
	if response.Code != status {
		t.Fatalf("status=%d want=%d body=%s", response.Code, status, response.Body.String())
	}
}

package gemini

import (
	"context"
	"net/http"
	"sync"
	"time"
)

type HTTPDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type SessionOptions struct {
	Repository Repository
	Now        func() time.Time
}

type Sessions struct {
	repository Repository
	now        func() time.Time
	locks      sync.Map
}

type SessionRequest struct {
	ID       string
	AuthUser string
	Client   HTTPDoer
}

func NewSessions(options SessionOptions) *Sessions {
	return &Sessions{repository: options.Repository, now: options.Now}
}

func (s *Sessions) lock(id string) func() {
	lock, _ := s.locks.LoadOrStore(id, &sync.Mutex{})
	mu := lock.(*sync.Mutex)
	mu.Lock()
	return mu.Unlock
}

func (s *Sessions) Snapshot(ctx context.Context, request SessionRequest) (Document, error) {
	if err := validateAuthUser(request.AuthUser); err != nil {
		return Document{}, err
	}
	unlock := s.lock(request.ID)
	defer unlock()
	doc, err := s.repository.Load(request.ID)
	if err != nil {
		return Document{}, err
	}
	if doc.AuthUser != request.AuthUser || doc.XSRF == "" || doc.Build == "" {
		return s.syncParameters(ctx, request, doc)
	}
	return doc, nil
}

func (s *Sessions) SyncParameters(ctx context.Context, request SessionRequest) (Document, error) {
	unlock := s.lock(request.ID)
	defer unlock()
	doc, err := s.repository.Load(request.ID)
	if err != nil {
		return Document{}, err
	}
	return s.syncParameters(ctx, request, doc)
}

func (s *Sessions) Remove(id string) error {
	unlock := s.lock(id)
	defer unlock()
	return s.repository.Delete(id)
}

func (s *Sessions) Replace(id string, doc Document) error {
	unlock := s.lock(id)
	defer unlock()
	return s.repository.Save(id, doc)
}

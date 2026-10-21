package gemini

import (
	"context"
	"fmt"
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
	ID          string
	AuthUser    string
	AutoRefresh bool
	Client      HTTPDoer
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
	unlock := s.lock(request.ID)
	defer unlock()
	doc, err := s.repository.Load(request.ID)
	if err != nil {
		return Document{}, err
	}
	due := doc.RefreshedAt.IsZero() || s.now().Sub(doc.RefreshedAt) >= refreshInterval
	if request.AutoRefresh && (due || doc.AuthUser != request.AuthUser) {
		return s.renew(ctx, request, doc)
	}
	if doc.AuthUser != request.AuthUser || doc.XSRF == "" || doc.Build == "" {
		return Document{}, fmt.Errorf("会话参数缺失或账号编号不匹配，请刷新登录会话")
	}
	return doc, nil
}

func (s *Sessions) Refresh(ctx context.Context, request SessionRequest) (Document, error) {
	unlock := s.lock(request.ID)
	defer unlock()
	doc, err := s.repository.Load(request.ID)
	if err != nil {
		return Document{}, err
	}
	return s.renew(ctx, request, doc)
}

func (s *Sessions) Receive(id string, original Document, cookies []*http.Cookie) error {
	unlock := s.lock(id)
	defer unlock()
	current, err := s.repository.Load(id)
	if err != nil {
		return err
	}
	if current.Cookie != original.Cookie {
		return nil
	}
	next, err := mergeCookies(current, cookies, s.now())
	if err != nil {
		return err
	}
	return s.repository.Save(id, next)
}

func (s *Sessions) Remove(id string) error {
	unlock := s.lock(id)
	defer unlock()
	return s.repository.Delete(id)
}

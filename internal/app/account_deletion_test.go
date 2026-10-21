package app

import (
	"errors"
	"path/filepath"
	"testing"
	"time"

	"gemini-free/internal/config"
	"gemini-free/internal/gemini"
)

func TestFailedAccountConfigSavePreservesCredentials(t *testing.T) {
	c, repo := testController(t)
	if err := c.AddAccount(AccountInput{Cookie: "__Secure-1PSID=test-cookie"}); err != nil {
		t.Fatal(err)
	}
	id := c.Config().Accounts[0].ID
	validPath := c.path
	c.path = filepath.Join(validPath, "config.json")
	if err := c.DeleteAccount(id); err == nil {
		t.Fatal("configuration save unexpectedly succeeded")
	}
	if _, err := repo.Load(id); err != nil {
		t.Fatalf("failed deletion destroyed credentials: %v", err)
	}
	assertAccountRetained(t, c, validPath)
}

func TestFailedCredentialDeletionRestoresAccountConfig(t *testing.T) {
	c, repo := testController(t)
	if err := c.AddAccount(AccountInput{Cookie: "__Secure-1PSID=test-cookie"}); err != nil {
		t.Fatal(err)
	}
	failure := errors.New("credential deletion denied")
	c.sessions = gemini.NewSessions(gemini.SessionOptions{
		Repository: deleteFailure{Credentials: repo, err: failure}, Now: time.Now})
	id := c.Config().Accounts[0].ID
	if err := c.DeleteAccount(id); !errors.Is(err, failure) {
		t.Fatalf("expected credential deletion failure, got %v", err)
	}
	assertAccountRetained(t, c, c.path)
}

func TestSuccessfulAccountDeletionPersistsRemoval(t *testing.T) {
	c, repo := testController(t)
	if err := c.AddAccount(AccountInput{Cookie: "__Secure-1PSID=test-cookie"}); err != nil {
		t.Fatal(err)
	}
	id := c.Config().Accounts[0].ID
	if err := c.DeleteAccount(id); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(c.path)
	if err != nil || len(cfg.Accounts) != 0 || len(c.Config().Accounts) != 0 {
		t.Fatalf("successful deletion did not persist removal: %v", err)
	}
	if _, err := repo.Load(id); err == nil {
		t.Fatal("successful deletion retained credentials")
	}
}

type deleteFailure struct {
	Credentials
	err error
}

func (d deleteFailure) Delete(string) error { return d.err }

func assertAccountRetained(t *testing.T, c *Controller, path string) {
	t.Helper()
	saved, err := config.Load(path)
	if err != nil || len(saved.Accounts) != 1 || len(c.Config().Accounts) != 1 {
		t.Fatalf("failed deletion lost account configuration: %v", err)
	}
}

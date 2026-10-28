package app

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"gemini-free/internal/config"
	"gemini-free/internal/gemini"
)

func TestParameterSyncReportsOneErrorAndAccountStatus(t *testing.T) {
	c, repo := testController(t)
	c.cfg.Mode, c.cfg.SelectedAccount = config.AccountMode, "one"
	c.cfg.Accounts = []config.Account{{ID: "one", Name: "test"}}
	repo.docs["one"] = gemini.Document{Cookie: "__Secure-1PSID=private-identity",
		XSRF: "old-token", Build: "old-build", RefreshedAt: time.Now()}
	status, calls := http.StatusUnauthorized, 0
	c.client = func(config.Config) (gemini.HTTPDoer, error) {
		return doerFunc(func(request *http.Request) (*http.Response, error) {
			calls++
			header := http.Header{}
			body := `{"SNlM0e":"fresh-token","cfb2h":"fresh-build"}`
			if request.URL.Host != "gemini.google.com" || request.Method != http.MethodGet {
				t.Error("parameter sync called a non-Gemini endpoint")
			}
			return &http.Response{StatusCode: status, Header: header, Body: io.NopCloser(strings.NewReader(body))}, nil
		}), nil
	}
	before := len(c.State().Logs)
	if err := c.SyncAccountParameters(context.Background(), "one"); err == nil {
		t.Fatal("parameter sync failure was hidden")
	}
	failed := c.State()
	if calls != 1 || len(failed.Logs)-before != 1 {
		t.Fatalf("one failed sync produced %d requests and %d logs", calls, len(failed.Logs)-before)
	}
	account := failed.Accounts[0]
	if account.Status != "参数同步失败" || !strings.Contains(account.Error, "Gemini 会话参数读取") {
		t.Fatalf("account hid the sync failure: %+v", account)
	}
	status = http.StatusOK
	if err := c.SyncAccountParameters(context.Background(), "one"); err != nil {
		t.Fatal(err)
	}
	recovered := c.State()
	if recovered.Accounts[0].Error != "" || recovered.Accounts[0].Status == "参数同步失败" {
		t.Fatal("successful sync retained the previous error")
	}
	if len(recovered.Logs)-len(failed.Logs) != 1 {
		t.Fatal("successful sync emitted duplicate logs")
	}
}

func TestParameterSyncControlRouteAndRemovedRefreshRoute(t *testing.T) {
	c, repo := testController(t)
	c.cfg.Accounts = []config.Account{{ID: "one", Name: "test"}}
	repo.docs["one"] = gemini.Document{Cookie: "__Secure-1PSID=identity"}
	control := testControl(t, c)
	calls := 0
	c.client = func(config.Config) (gemini.HTTPDoer, error) {
		return doerFunc(func(*http.Request) (*http.Response, error) {
			calls++
			body := `{"SNlM0e":"token","cfb2h":"build"}`
			return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body))}, nil
		}), nil
	}
	expectStatus(t, controlJSON(control, "/control/accounts/one/refresh", nil), http.StatusNotFound)
	expectStatus(t, controlJSON(control, "/control/accounts/one/sync", nil), http.StatusOK)
	if calls != 1 {
		t.Fatalf("expected one explicit parameter sync request, got %d", calls)
	}
}

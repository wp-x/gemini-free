package app

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"gemini-free/internal/config"
)

func TestReplaceSessionPreservesSelectionAndClearsStaleParameters(t *testing.T) {
	c, repo := testController(t)
	if err := c.AddAccount(AccountInput{Name: "selected", Cookie: "__Secure-1PSID=old", XSRF: "old-token", Build: "old-build"}); err != nil {
		t.Fatal(err)
	}
	account := c.Config().Accounts[0]
	c.cfg.Mode, c.cfg.SelectedAccount = config.AccountMode, account.ID
	c.parameterErrors[account.ID] = "previous sync failed"
	before := c.Config()
	response := controlJSON(testControl(t, c), "/control/accounts/"+account.ID+"/session",
		AccountInput{Cookie: "__Secure-1PSID=new-private-value"})
	expectStatus(t, response, http.StatusOK)
	if !reflect.DeepEqual(before, c.Config()) {
		t.Fatal("replacement changed account identity, selection or settings")
	}
	doc, err := repo.Load(account.ID)
	if err != nil || doc.Cookie != "__Secure-1PSID=new-private-value" || doc.XSRF != "" || doc.Build != "" {
		t.Fatalf("replacement did not discard old session parameters: %v", err)
	}
	if c.State().Accounts[0].Error != "" || strings.Contains(response.Body.String(), "new-private-value") {
		t.Fatal("replacement retained an old error or exposed credentials")
	}
}

func TestInvalidReplacementLeavesExistingSessionUntouched(t *testing.T) {
	c, repo := testController(t)
	if err := c.AddAccount(AccountInput{Cookie: "__Secure-1PSID=old", AuthUser: "2"}); err != nil {
		t.Fatal(err)
	}
	account := c.Config().Accounts[0]
	before, _ := repo.Load(account.ID)
	inputs := []AccountInput{{Cookie: "invalid"},
		{Cookie: `{"cookie":"__Secure-1PSID=new","session_auth_user":"3"}`}}
	for _, input := range inputs {
		if err := c.ReplaceAccountSession(account.ID, input); err == nil {
			t.Fatal("invalid session accepted")
		}
		after, _ := repo.Load(account.ID)
		if after != before {
			t.Fatal("invalid replacement changed credentials")
		}
	}
	if err := c.ReplaceAccountSession("missing", AccountInput{Cookie: "__Secure-1PSID=new"}); err == nil {
		t.Fatal("replacement created an unknown account")
	}
}

func TestReplacementKeepsMatchingSessionBundle(t *testing.T) {
	c, repo := testController(t)
	if err := c.AddAccount(AccountInput{Cookie: "__Secure-1PSID=old"}); err != nil {
		t.Fatal(err)
	}
	account := c.Config().Accounts[0]
	input := AccountInput{Cookie: `{"cookie":"__Secure-1PSID=new","session_auth_user":"0","xsrf_token":"matching-token","gemini_bl":"matching-build"}`}
	if err := c.ReplaceAccountSession(account.ID, input); err != nil {
		t.Fatal(err)
	}
	doc, _ := repo.Load(account.ID)
	if doc.XSRF != "matching-token" || doc.Build != "matching-build" || doc.AuthUser != account.AuthUser {
		t.Fatal("replacement mixed session parameters")
	}
	state, _ := json.Marshal(c.State())
	if strings.Contains(string(state), "matching-token") {
		t.Fatal("state exposed session token")
	}
}

package app

import (
	"testing"

	"gemini-free/internal/config"
)

func TestAccountImportNeedsOnlyCookie(t *testing.T) {
	c, repo := testController(t)
	if err := c.AddAccount(AccountInput{Cookie: "__Secure-1PSID=test-cookie"}); err != nil {
		t.Fatal(err)
	}
	state := c.State()
	if len(state.Accounts) != 1 || state.Accounts[0].Name == "" {
		t.Fatal("account was not automatically named")
	}
	doc, err := repo.Load(state.Accounts[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if doc.XSRF != "" || doc.Build != "" || state.Config.Mode != config.Anonymous {
		t.Fatal("cookie import fabricated parameters or switched mode")
	}
}

func TestAccountImportPreservesExportedAccountNumber(t *testing.T) {
	c, repo := testController(t)
	const session = `{"cookie":"__Secure-1PSID=test-cookie","session_auth_user":"3"}`
	if err := c.AddAccount(AccountInput{Cookie: session}); err != nil {
		t.Fatal(err)
	}
	account := c.State().Accounts[0]
	doc, err := repo.Load(account.ID)
	if err != nil || doc.AuthUser != "3" || account.AuthUser != "3" {
		t.Fatal("import discarded the exported account number")
	}
	if err := c.AddAccount(AccountInput{Cookie: session, AuthUser: "2"}); err != nil {
		t.Fatal(err)
	}
	if c.State().Accounts[1].AuthUser != "2" {
		t.Fatal("explicit account number did not override the imported number")
	}
}

func TestAccountImportValidatesExportedAccountNumber(t *testing.T) {
	c, _ := testController(t)
	const session = `{"cookie":"__Secure-1PSID=test-cookie","session_auth_user":"invalid"}`
	if err := c.AddAccount(AccountInput{Cookie: session}); err == nil {
		t.Fatal("invalid exported account number was accepted")
	}
	if len(c.State().Accounts) != 0 {
		t.Fatal("invalid import mutated account state")
	}
}

package vault

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"gemini-free/internal/gemini"
)

var errMissing = errors.New("missing key")

type testKeys struct{ value []byte }

func (k *testKeys) Get(string) ([]byte, error) {
	if k.value == nil {
		return nil, errMissing
	}
	return append([]byte(nil), k.value...), nil
}

func (k *testKeys) Set(_ string, value []byte) error {
	k.value = append([]byte(nil), value...)
	return nil
}

func TestEncryptedSessionRoundTripAndTamper(t *testing.T) {
	dir := t.TempDir()
	vault := New(Options{Directory: dir, Keys: &testKeys{}, NotFound: func(err error) bool { return errors.Is(err, errMissing) }})
	doc := gemini.Document{Cookie: "__Secure-1PSID=sensitive-value", XSRF: "sensitive-token"}
	if err := vault.Save("account", doc); err != nil {
		t.Fatal(err)
	}
	ciphertext, _ := os.ReadFile(filepath.Join(dir, "account.bin"))
	if bytes.Contains(ciphertext, []byte("sensitive")) {
		t.Fatal("credentials stored as plaintext")
	}
	decoded, err := vault.Load("account")
	if err != nil || decoded.Cookie != doc.Cookie || decoded.XSRF != doc.XSRF {
		t.Fatalf("round trip failed: %v", err)
	}
	ciphertext[len(ciphertext)-1] ^= 1
	os.WriteFile(filepath.Join(dir, "account.bin"), ciphertext, 0600)
	if _, err := vault.Load("account"); err == nil {
		t.Fatal("tampering accepted")
	}
	if err := vault.Save("../escape", doc); err == nil {
		t.Fatal("path traversal accepted")
	}
}

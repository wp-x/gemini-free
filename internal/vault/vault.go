package vault

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"gemini-free/internal/config"
	"gemini-free/internal/gemini"
)

const (
	keySize      = 32
	keyName      = "gemini-free-vault-v1"
	vaultVersion = byte(1)
)

type KeyStore interface {
	Get(string) ([]byte, error)
	Set(string, []byte) error
}

type Options struct {
	Directory string
	Keys      KeyStore
	NotFound  func(error) bool
}

type Vault struct {
	options Options
	mu      sync.Mutex
	master  []byte
}

func New(options Options) *Vault { return &Vault{options: options} }

func (v *Vault) key() ([]byte, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	if len(v.master) != 0 {
		return append([]byte(nil), v.master...), nil
	}
	key, err := v.options.Keys.Get(keyName)
	if err == nil {
		if len(key) != keySize {
			return nil, fmt.Errorf("invalid vault key size")
		}
		v.master = append([]byte(nil), key...)
		return key, nil
	}
	if !v.options.NotFound(err) {
		return nil, fmt.Errorf("read credential store: %w", err)
	}
	key = make([]byte, keySize)
	if _, err := rand.Read(key); err != nil {
		return nil, err
	}
	if err := v.options.Keys.Set(keyName, key); err != nil {
		return nil, fmt.Errorf("save credential store: %w", err)
	}
	v.master = append([]byte(nil), key...)
	return key, nil
}

func (v *Vault) cipher() (cipher.AEAD, error) {
	key, err := v.key()
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func (v *Vault) path(id string) (string, error) {
	if !regexp.MustCompile(`^[a-zA-Z0-9_-]+$`).MatchString(id) {
		return "", fmt.Errorf("invalid vault identifier")
	}
	return filepath.Join(v.options.Directory, id+".bin"), nil
}

func (v *Vault) SaveBlob(id string, data []byte) error {
	path, err := v.path(id)
	if err != nil {
		return err
	}
	box, err := v.cipher()
	if err != nil {
		return err
	}
	nonce := make([]byte, box.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	header := append([]byte{vaultVersion}, nonce...)
	encrypted := box.Seal(header, nonce, data, []byte(id))
	return config.WritePrivate(path, encrypted)
}

func (v *Vault) LoadBlob(id string) ([]byte, error) {
	path, err := v.path(id)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read encrypted session: %w", err)
	}
	box, err := v.cipher()
	if err != nil {
		return nil, err
	}
	headerSize := 1 + box.NonceSize()
	if len(data) < headerSize || data[0] != vaultVersion {
		return nil, fmt.Errorf("invalid encrypted session format")
	}
	return box.Open(nil, data[1:headerSize], data[headerSize:], []byte(id))
}

func (v *Vault) Load(id string) (gemini.Document, error) {
	data, err := v.LoadBlob(id)
	if err != nil {
		return gemini.Document{}, err
	}
	var doc gemini.Document
	if err := json.Unmarshal(data, &doc); err != nil {
		return gemini.Document{}, err
	}
	return doc, nil
}

func (v *Vault) Save(id string, doc gemini.Document) error {
	data, err := json.Marshal(doc)
	if err != nil {
		return err
	}
	return v.SaveBlob(id, data)
}

func (v *Vault) Delete(id string) error {
	path, err := v.path(id)
	if err != nil {
		return err
	}
	return os.Remove(path)
}

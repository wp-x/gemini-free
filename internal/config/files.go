package config

import (
	"errors"
	"os"
	"path/filepath"
)

const (
	privateDir  = 0700
	privateFile = 0600
)

func WritePrivate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), privateDir); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".gemini-*")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	if err := file.Chmod(privateFile); err != nil {
		return errors.Join(err, file.Close())
	}
	if _, err := file.Write(data); err != nil {
		return errors.Join(err, file.Close())
	}
	if err := errors.Join(file.Sync(), file.Close()); err != nil {
		return err
	}
	return os.Rename(name, path)
}

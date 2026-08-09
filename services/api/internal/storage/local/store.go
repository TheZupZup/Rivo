package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

var ErrInvalidKey = errors.New("invalid video storage key")

type Store struct {
	root string
}

func New(root string) (*Store, error) {
	absoluteRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(absoluteRoot, 0o755); err != nil {
		return nil, err
	}

	return &Store{root: absoluteRoot}, nil
}

func (store *Store) Save(_ context.Context, key string, source io.Reader) error {
	path, err := store.pathForKey(key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	temporaryFile, err := os.CreateTemp(filepath.Dir(path), ".upload-*")
	if err != nil {
		return err
	}

	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	if _, err := io.Copy(temporaryFile, source); err != nil {
		temporaryFile.Close()
		return err
	}

	if err := temporaryFile.Close(); err != nil {
		return err
	}

	return os.Rename(temporaryPath, path)
}

func (store *Store) Open(_ context.Context, key string) (io.ReadCloser, error) {
	path, err := store.pathForKey(key)
	if err != nil {
		return nil, err
	}

	return os.Open(path)
}

func (store *Store) Delete(_ context.Context, key string) error {
	path, err := store.pathForKey(key)
	if err != nil {
		return err
	}

	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}

	return err
}

func (store *Store) pathForKey(key string) (string, error) {
	cleanKey := filepath.Clean(key)
	if cleanKey == "." || filepath.IsAbs(cleanKey) || cleanKey == ".." || strings.HasPrefix(cleanKey, ".."+string(filepath.Separator)) {
		return "", ErrInvalidKey
	}

	path := filepath.Join(store.root, cleanKey)
	relativePath, err := filepath.Rel(store.root, path)
	if err != nil {
		return "", err
	}
	if relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(filepath.Separator)) {
		return "", ErrInvalidKey
	}

	return path, nil
}

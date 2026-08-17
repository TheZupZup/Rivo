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

func (store *Store) Save(_ context.Context, key string, source io.Reader) (err error) {
	path, err := store.pathForKey(key)
	if err != nil {
		return err
	}

	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}

	// An upload that fails part way through — a truncated stream, a client that
	// exceeded the size limit — must not leave the directory it created behind.
	// Without this, repeatedly failing uploads accumulate empty directories and
	// eventually exhaust inodes.
	defer func() {
		if err != nil {
			store.pruneEmptyParents(directory)
		}
	}()

	temporaryFile, err := os.CreateTemp(directory, ".upload-*")
	if err != nil {
		return err
	}

	temporaryPath := temporaryFile.Name()
	defer os.Remove(temporaryPath)

	if _, err = io.Copy(temporaryFile, source); err != nil {
		temporaryFile.Close()
		return err
	}

	if err = temporaryFile.Close(); err != nil {
		return err
	}

	err = os.Rename(temporaryPath, path)
	return err
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
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	store.pruneEmptyParents(filepath.Dir(path))

	return nil
}

// pruneEmptyParents removes the per-video directories a deleted object leaves
// behind, walking up until it reaches the storage root or a directory that still
// holds something. os.Remove refuses to delete a non-empty directory, so a failure
// here simply means there is nothing more to prune.
func (store *Store) pruneEmptyParents(directory string) {
	for directory != store.root && strings.HasPrefix(directory, store.root+string(filepath.Separator)) {
		if err := os.Remove(directory); err != nil {
			return
		}

		directory = filepath.Dir(directory)
	}
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

package metadata

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"oci-remote-cache/internal/utils"
	"os"
	"path/filepath"
)

type MetadataStore interface {
	Get(ctx context.Context, path string) ([]byte, error)
	Put(ctx context.Context, path string, data []byte) error
	Delete(ctx context.Context, path string) error
}

type FSMetadataStore struct {
	root string
}

func NewFSMetadataStore(root string) *FSMetadataStore {
	return &FSMetadataStore{root: root}
}

func (s *FSMetadataStore) fullPath(p string) string {
	// Metadata is stored under <root>/meta/<path>
	return filepath.Join(s.root, "meta", p)
}

func (s *FSMetadataStore) Get(ctx context.Context, path string) ([]byte, error) {
	fp := s.fullPath(path)
	return os.ReadFile(fp)
}

func (s *FSMetadataStore) Put(ctx context.Context, path string, data []byte) error {
	fp := s.fullPath(path)
	dir := filepath.Dir(fp)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Atomic write using your helper
	return utils.AtomicWriteFile(fp, io.NopCloser(bytes.NewReader(data)), 0o644)
}

func (s *FSMetadataStore) Delete(ctx context.Context, path string) error {
	fp := s.fullPath(path)

	// Delete is not atomic by nature, but safe enough for metadata
	if err := os.Remove(fp); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("delete metadata %s: %w", path, err)
	}

	return nil
}

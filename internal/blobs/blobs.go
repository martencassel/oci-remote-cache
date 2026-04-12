package blobs

import (
	"context"
	"fmt"
	"io"
	"oci-remote-cache/internal/utils"
	"os"
	"path/filepath"

	digest "github.com/opencontainers/go-digest"
)

type BlobStore interface {
	Has(ctx context.Context, dgst digest.Digest) (bool, error)
	Get(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error)
	Put(ctx context.Context, dgst digest.Digest, r io.Reader) error
}

type FSBlobStore struct {
	root string
}

func NewFSBlobStore(root string) *FSBlobStore {
	return &FSBlobStore{root: root}
}

func (s *FSBlobStore) blobPath(dgst digest.Digest) string {
	algo := dgst.Algorithm().String()
	hex := dgst.Encoded()
	return filepath.Join(s.root, "blobs", algo, hex[:2], hex)
}

func (s *FSBlobStore) Has(ctx context.Context, dgst digest.Digest) (bool, error) {
	_, err := os.Stat(s.blobPath(dgst))
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (s *FSBlobStore) Get(ctx context.Context, dgst digest.Digest) (io.ReadCloser, error) {
	return os.Open(s.blobPath(dgst))
}

func (s *FSBlobStore) Put(ctx context.Context, dgst digest.Digest, r io.Reader) error {
	finalPath := s.blobPath(dgst)
	dir := filepath.Dir(finalPath)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	// Compute digest while streaming
	h := dgst.Algorithm().Hash()
	tee := io.TeeReader(r, h)

	// Write to temp file atomically (streaming)
	if err := utils.AtomicWriteFile(finalPath, tee, 0o644); err != nil {
		return err
	}

	// Verify digest AFTER writing
	computed := digest.NewDigest(dgst.Algorithm(), h)
	if computed != dgst {
		// Remove incorrect blob
		_ = os.Remove(finalPath)
		return fmt.Errorf("digest mismatch: expected %s, got %s", dgst, computed)
	}

	return nil
}

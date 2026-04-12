package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"oci-remote-cache/internal/blobs"
	"oci-remote-cache/internal/metadata"

	digest "github.com/opencontainers/go-digest"
)

type CAS struct {
	Root     string
	Blobs    blobs.BlobStore
	Metadata metadata.MetadataStore
}

func NewCAS(root string, b blobs.BlobStore, m metadata.MetadataStore) *CAS {
	return &CAS{Root: root, Blobs: b, Metadata: m}
}

// ----------------------
// Blob read path
// ----------------------

func (c *CAS) OpenBlob(repoKey, repo string, dgst digest.Digest) (io.ReadCloser, error) {
	ok, err := c.Blobs.Has(context.Background(), dgst)
	if err != nil || !ok {
		return nil, fmt.Errorf("blob not found")
	}
	return c.Blobs.Get(context.Background(), dgst)
}

// ----------------------
// Blob write path
// ----------------------

type BlobWriter struct {
	repoKey string
	repo    string
	dgst    digest.Digest
	pipeW   *io.PipeWriter
	done    chan error
}

func (c *CAS) CreateBlob(repoKey, repo string, dgst digest.Digest) (*BlobWriter, error) {
	err := os.MkdirAll(c.Root, 0o755)
	if err != nil {
		return nil, err
	}

	pr, pw := io.Pipe()
	done := make(chan error, 1)

	// Stream into BlobStore.Put in a goroutine
	go func() {
		err := c.Blobs.Put(context.Background(), dgst, pr)
		_ = pr.CloseWithError(err)
		done <- err
	}()

	return &BlobWriter{
		repoKey: repoKey,
		repo:    repo,
		dgst:    dgst,
		pipeW:   pw,
		done:    done,
	}, nil
}

func (bw *BlobWriter) Write(p []byte) (int, error) {
	return bw.pipeW.Write(p)
}

// Abort cancels the in-flight write. Safe to call after Commit (no-op then).
func (bw *BlobWriter) Abort() {
	bw.pipeW.CloseWithError(io.ErrUnexpectedEOF)
	// Drain done so the background goroutine can exit.
	select {
	case <-bw.done:
	default:
	}
}

func (bw *BlobWriter) Commit(c *CAS) error {
	// Close writer → flush into BlobStore
	if err := bw.pipeW.Close(); err != nil {
		return err
	}

	// Wait for Put to finish
	if err := <-bw.done; err != nil {
		return err
	}

	// Write metadata
	meta := map[string]any{
		"repoKey":  bw.repoKey,
		"repo":     bw.repo,
		"digest":   bw.dgst.String(),
		"cachedAt": time.Now().UTC().Format(time.RFC3339),
	}

	data, _ := json.MarshalIndent(meta, "", "  ")

	metaPath := filepath.Join(
		bw.repoKey,
		bw.repo,
		"blobs",
		bw.dgst.Encoded()+".json",
	)

	return c.Metadata.Put(context.Background(), metaPath, data)
}

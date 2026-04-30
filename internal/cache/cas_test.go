package cache

import (
	"bytes"
	"io"
	"oci-remote-cache/internal/blobs"
	"path/filepath"
	"testing"

	metadata "oci-remote-cache/internal/metadata"

	digest "github.com/opencontainers/go-digest"
)

func TestCAS(t *testing.T) {
	// Setup temporary directory for testing
	tempDir := t.TempDir()

	ms := metadata.NewFSMetadataStore(filepath.Join(tempDir, "metadata"))

	// Create a new CAS instance with a filesystem blob store and in-memory metadata store
	cas := NewCAS(tempDir, blobs.NewFSBlobStore(filepath.Join(tempDir, "blobs")), ms)

	// Define test data
	repoKey := "test-repo-key"
	repo := "test-repo"
	testData := []byte("This is some test data for the CAS.")
	dgst := digest.FromBytes(testData)

	// Test CreateBlob and OpenBlob
	t.Run("Create and Open Blob", func(t *testing.T) {
		// Create a new blob
		writer, err := cas.CreateBlob(repoKey, repo, dgst)
		if err != nil {
			t.Fatalf("Failed to create blob: %v", err)
		}

		// Write test data to the blob
		_, err = writer.Write(testData)
		if err != nil {
			t.Fatalf("Failed to write to blob: %v", err)
		}

		if err := writer.Commit(cas); err != nil {
			t.Fatalf("Failed to finalize blob: %v", err)
		}

		// Open the blob for reading
		reader, err := cas.OpenBlob(repoKey, repo, dgst)
		if err != nil {
			t.Fatalf("Failed to open blob: %v", err)
		}
		defer reader.Close()

		// Read the data back and verify it matches the original test data
		readData, err := io.ReadAll(reader)
		if err != nil {
			t.Fatalf("Failed to read from blob: %v", err)
		}
		if !bytes.Equal(readData, testData) {
			t.Fatalf("Data mismatch: expected %q, got %q", testData, readData)
		}
	})
}

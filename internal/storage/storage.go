package storage

import (
	"oci-remote-cache/internal/blobs"
	"oci-remote-cache/internal/metadata"
)

type Storage interface {
	Blobs() blobs.BlobStore
	Meta() metadata.MetadataStore
}

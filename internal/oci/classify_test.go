package oci

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseV2_Coverage(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		want    ParseResult
		wantErr bool
	}{
		{
			name:   "Ping",
			method: "GET",
			path:   "/v2/",
			want:   ParseResult{IsPing: true, RawPath: "/v2/"},
		},
		{
			name:   "Simple manifest",
			method: "GET",
			path:   "/v2/repo/manifests/latest",
			want: ParseResult{
				RepoKey:   "repo",
				Verb:      VerbManifests,
				Reference: "latest",
				RawPath:   "/v2/repo/manifests/latest",
			},
		},
		{
			name:   "Blob digest",
			method: "GET",
			path:   "/v2/repo/blobs/sha256:abc123",
			want: ParseResult{
				RepoKey: "repo",
				Verb:    VerbBlobs,
				Digest:  "sha256:abc123",
				RawPath: "/v2/repo/blobs/sha256:abc123",
			},
		},
		{
			name:   "Tags list",
			method: "GET",
			path:   "/v2/repo/tags/list",
			want: ParseResult{
				RepoKey: "repo",
				Verb:    VerbTags,
				SubVerb: "list",
				RawPath: "/v2/repo/tags/list",
			},
		},
		{
			name:   "Uploads UUID",
			method: "GET",
			path:   "/v2/repo/uploads/uuid123",
			want: ParseResult{
				RepoKey:    "repo",
				Verb:       VerbBlobs,
				SubVerb:    "uploads",
				UploadUUID: "uuid123",
				RawPath:    "/v2/repo/uploads/uuid123",
			},
		},
		{
			name:   "Referrers digest",
			method: "GET",
			path:   "/v2/repo/referrers/sha256:abc123",
			want: ParseResult{
				RepoKey:   "repo",
				Verb:      VerbReferrers,
				Reference: "sha256:abc123",
				RawPath:   "/v2/repo/referrers/sha256:abc123",
			},
		},
		{
			name:   "Special _catalog",
			method: "GET",
			path:   "/v2/repo/_catalog",
			want: ParseResult{
				RepoKey: "repo",
				Verb:    VerbUnknown,
				SubVerb: "_catalog",
				RawPath: "/v2/repo/_catalog",
			},
		},
		{
			name:   "Multi-segment repoKey",
			method: "GET",
			path:   "/v2/docker.io/alpine/manifests/latest",
			want: ParseResult{
				RepoKey:    "docker.io",
				Repository: "alpine",
				Verb:       VerbManifests,
				Reference:  "latest",
				RawPath:    "/v2/docker.io/alpine/manifests/latest",
			},
		},
		{
			name:   "Special segment before verb",
			method: "GET",
			path:   "/v2/acme/foo/_layers/manifests/latest",
			want: ParseResult{
				RepoKey:    "acme",
				Repository: "foo",
				Verb:       VerbManifests,
				Reference:  "latest",
				SubVerb:    "_layers",
				RawPath:    "/v2/acme/foo/_layers/manifests/latest",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse(tt.method, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			}
			assert.NoError(t, err)
			assert.Equal(t, tt.want.Verb, got.Verb)
			assert.Equal(t, tt.want.SubVerb, got.SubVerb)
			assert.Equal(t, tt.want.RepoKey, got.RepoKey)
			assert.Equal(t, tt.want.Repository, got.Repository)
			assert.Equal(t, tt.want.Digest, got.Digest)
			assert.Equal(t, tt.want.Reference, got.Reference)
			assert.Equal(t, tt.want.UploadUUID, got.UploadUUID)
			assert.Equal(t, tt.want.Query, got.Query)
			assert.Equal(t, tt.want.RawPath, got.RawPath)
		})
	}
}

func TestParseV2_Negative(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		path    string
		wantErr bool
	}{
		{
			name:    "Missing /v2 prefix",
			method:  "GET",
			path:    "/foo/bar",
			wantErr: true,
		},
		{
			name:    "Only /v2 without trailing slash",
			method:  "GET",
			path:    "/v2",
			wantErr: true,
		},
		{
			name:    "Empty after /v2/",
			method:  "GET",
			path:    "/v2/",
			wantErr: false, // ping is valid
		},
		{
			name:    "Missing repoKey",
			method:  "GET",
			path:    "/v2//manifests/latest",
			wantErr: true,
		},
		{
			name:    "Missing verb",
			method:  "GET",
			path:    "/v2/repo",
			wantErr: false, // valid but VerbUnknown
		},
		{
			name:    "Unknown verb",
			method:  "GET",
			path:    "/v2/repo/unknownverb/latest",
			wantErr: false, // VerbUnknown but not an error
		},
		{
			name:    "Special segment with no verb",
			method:  "GET",
			path:    "/v2/repo/_layers",
			wantErr: false, // SubVerb set, VerbUnknown
		},
		{
			name:    "Verb with no identifier",
			method:  "GET",
			path:    "/v2/repo/manifests",
			wantErr: false, // VerbManifests, Reference=""
		},
		{
			name:    "Double slash inside repo",
			method:  "GET",
			path:    "/v2/repo//manifests/latest",
			wantErr: true,
		},
		{
			name:    "Double slash before verb",
			method:  "GET",
			path:    "/v2/repo/foo//manifests/latest",
			wantErr: true,
		},
		{
			name:    "Special segment as repoKey",
			method:  "GET",
			path:    "/v2/_catalog/manifests/latest",
			wantErr: false, // RepoKey="_catalog", Repository="", VerbManifests
		},
		{
			name:    "Special segment only",
			method:  "GET",
			path:    "/v2/_catalog",
			wantErr: false, // RepoKey="_catalog", SubVerb="_catalog"
		},
		{
			name:    "Trailing slash",
			method:  "GET",
			path:    "/v2/repo/manifests/latest/",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Parse(tt.method, tt.path)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

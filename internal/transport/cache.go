package transport

import (
	"fmt"
	"net/http"
	"oci-remote-cache/internal/cache"
	"oci-remote-cache/internal/oci"
	"strings"

	"github.com/opencontainers/go-digest"
	log "github.com/sirupsen/logrus"
)

type CacheRoundTripper struct {
	next http.RoundTripper
	cas  *cache.CAS
}

func NewCachingRoundTripper(cas *cache.CAS) RTFactory {
	return func(next http.RoundTripper) http.RoundTripper {
		return &CacheRoundTripper{
			next: next,
			cas:  cas,
		}
	}
}

func (rt *CacheRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	meta := oci.ClassifyRequest(r.Method, r.URL.Path)
	log.Infof("CacheRoundTripper: %s %s classified as %v", r.Method, r.URL.Path, meta.Kind)
	if meta.Kind != oci.KindBlobGet {
		// Only cache GET blob requests
		return rt.next.RoundTrip(r)
	}
	// 1. Try CAS
	if rc, err := rt.tryCAS(r, meta); err == nil {
		log.Infof("Cache hit for %s %s", r.Method, r.URL.Path)
		return rc, nil
	}
	log.Infof("Cache miss for %s %s", r.Method, r.URL.Path)
	// 2. Forward to next
	return rt.next.RoundTrip(r)
}

func (rt *CacheRoundTripper) tryCAS(r *http.Request, meta oci.RequestMeta) (*http.Response, error) {
	// 1. Extract repoKey and digest
	repoKey, repo, dgst, ok := ExtractBlobMeta(r.URL.Path)
	if !ok {
		return nil, fmt.Errorf("failed to extract blob meta from path: %s", r.URL.Path)
	}
	// 2. Open blob from CAS
	d := digest.Digest(dgst)
	f, err := rt.cas.OpenBlob(repoKey, repo, d)
	if err != nil {
		return nil, err
	}
	// 3. Create http.Response with f as Body
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Body:       f,
	}
	return resp, nil
}

func ExtractBlobMeta(path string) (repoKey, repo, digest string, ok bool) {
	// Expecting /v2/<repoKey>/blobs/<digest>
	parts := strings.Split(path, "/")
	if len(parts) < 5 {
		return "", "", "", false
	}
	if parts[0] != "v2" || parts[len(parts)-3] != "blobs" {
		return "", "", "", false
	}
	repoKey = joinPath(parts[1 : len(parts)-3])
	digest = parts[len(parts)-1]
	return repoKey, "", digest, true
}

func joinPath(parts []string) string {
	return fmt.Sprintf("%s", parts)
}

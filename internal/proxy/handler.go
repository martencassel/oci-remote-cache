package proxy

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"oci-remote-cache/internal/cache"
	"oci-remote-cache/internal/config"
	"oci-remote-cache/internal/oci"
	"path"
	"strings"
	"time"

	"github.com/opencontainers/go-digest"
	log "github.com/sirupsen/logrus"
)

type Handler struct {
	Client             *http.Client // with your RoundTripper chain
	RepoConfigProvider config.RepoConfigProvider
	CAS                *cache.CAS
}

func NewHandler(client *http.Client, repoConfigProvider config.RepoConfigProvider, cas *cache.CAS) *Handler {
	return &Handler{Client: client, RepoConfigProvider: repoConfigProvider, CAS: cas}
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Handle /v2/ ping
	if r.URL.Path == "/v2/" {
		h.handlePing(w, r, oci.RequestMeta{Kind: oci.KindPing})
		return
	}
	// 1. Extract repo key and remainder
	repoKey, remainder, ok := oci.ExtractRepoKey(r.URL.Path)
	if !ok {
		log.Warnf("Failed to extract repo key from path: %s", r.URL.Path)
		http.NotFound(w, r)
		return
	}

	if !ok {
		http.NotFound(w, r)
		return
	}

	cfg, err := h.RepoConfigProvider.GetRepoConfig(repoKey)
	if err != nil {
		http.Error(w, fmt.Sprintf("repository config not found for key: %s", repoKey), http.StatusBadRequest)
		return
	}
	log.Infof("Received request for repoKey: %s, remainder: %s", repoKey, remainder)

	// 2. Classify request BEFORE rewriting
	meta := oci.ClassifyRequest(r.Method, r.URL.Path)

	// 3. Canonical Docker Hub rewrite
	remainder = rewriteDockerHubRemainder(repoKey, meta.Repo, remainder)

	// 4. Build upstream URL
	upstream, err := buildUpstreamURL(cfg.RemoteURL, remainder)
	if err != nil {
		http.Error(w, "invalid upstream URL", 500)
		return
	}

	log.Infof("Proxying request to %s with meta: %+v", upstream.String(), meta)
	log.Infof("Original request: %s %s", r.Method, r.URL.String())
	log.Infof("Upstream request: %s %s", r.Method, upstream.String())

	// Check that upstream path has /v2/ prefix, as a sanity check that our rewrites are correct
	if !strings.HasPrefix(upstream.Path, "/v2/") {
		upstream.Path = path.Join("/v2/", upstream.Path)
	}
	// 5. Build outbound request (trust-boundary safe)
	req, err := http.NewRequest(r.Method, upstream.String(), r.Body)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	// Preserve ContentLength so http.Client can follow 307 redirects.
	// When TraceHandler reads+replaces the body with io.NopCloser, Go cannot
	// infer the length (defaults to -1). For 307, Go's redirect logic skips
	// following when GetBody==nil && ContentLength != 0 (-1 != 0 is true), so
	// the 307 is returned raw and we try to cache an empty body.
	req.ContentLength = r.ContentLength
	if (r.Method == http.MethodGet || r.Method == http.MethodHead) && req.ContentLength < 0 {
		req.ContentLength = 0
	}
	req = req.WithContext(r.Context())

	// Copy headers
	for k, v := range r.Header {
		req.Header[k] = v
	}

	// 6. Dispatch to handler
	switch meta.Kind {
	case oci.KindPing:
		h.handlePing(w, req, meta)
	case oci.KindBlobGet:
		h.handleBlobGet(w, req, meta)
	case oci.KindBlobHead:
		h.handleBlobHead(w, req, meta)
	case oci.KindBlobMount:
		h.handleBlobMount(w, req, meta)
	case oci.KindManifestGet:
		h.handleManifestGet(w, req, meta)
	case oci.KindManifestHead:
		h.handleManifestHead(w, req, meta)
	case oci.KindManifestPut:
		h.handleManifestPut(w, req, meta)
	case oci.KindCatalogList:
		h.handleCatalogList(w, req, meta)
	case oci.KindTagsList:
		h.handleTagList(w, req, meta)
	default:
		http.NotFound(w, req)
	}
}

func (h *Handler) handlePing(w http.ResponseWriter, r *http.Request, meta oci.RequestMeta) {
	// Just return 200 OK for /v2/ ping requests
	w.WriteHeader(http.StatusOK)
	// Header
	// distribution-spec-version: 2.0
	w.Header().Set("Docker-Distribution-API-Version", "registry/2.0")
}

func copyHeader(dst, src http.Header) {
	for k, v := range src {
		for _, vv := range v {
			dst.Add(k, vv)
		}
	}
}

func (h *Handler) handleBlobGet(w http.ResponseWriter, r *http.Request, meta oci.RequestMeta) {
	log.Debugf("Handling blob get for repo: %s, digest: %s", meta.Repo, meta.Digest)

	// Extract repoKey and digest for CAS operations
	repoKey := meta.RepoKey
	dgst := meta.Digest
	repo := meta.Repo

	// 1. Try cache
	d := digest.Digest(dgst)
	f, err := h.CAS.OpenBlob(repoKey, repo, d)
	if err == nil {
		defer f.Close()

		// FIX: assert to ReadSeeker
		if rs, ok := f.(io.ReadSeeker); ok {
			http.ServeContent(w, r, "", time.Time{}, rs)
			return
		}

		// fallback: no seeking support
		if _, err := io.Copy(w, f); err != nil {
			log.Errorf("Failed to copy blob to response: %v", err)
		}
		return
	}

	// Cache miss → fetch upstream
	upstreamResp, err := h.Client.Do(r)
	if err != nil {
		http.Error(w, "upstream error", http.StatusBadGateway)
		return
	}
	defer upstreamResp.Body.Close()

	// Write headers to client
	copyHeader(w.Header(), upstreamResp.Header)
	w.WriteHeader(upstreamResp.StatusCode)

	// If upstream returned error, do not cache
	// Only cache a successful 200. Any redirect (3xx) or error (4xx/5xx) is
	// passed through; attempting to cache a redirect body would store 0 bytes
	// and fail digest verification with the sha256 of an empty string.
	if upstreamResp.StatusCode != http.StatusOK {
		io.Copy(w, upstreamResp.Body)
		return
	}

	bw, err := h.CAS.CreateBlob(repoKey, repo, digest.Digest(dgst))
	if err != nil {
		io.Copy(w, upstreamResp.Body)
		return
	}
	defer bw.Abort() // no-op if Commit is reached; unblocks goroutine on any early return

	tee := io.TeeReader(upstreamResp.Body, bw)

	if _, err := io.Copy(w, tee); err != nil {
		return
	}

	err = bw.Commit(h.CAS)
	if err != nil {
		log.Errorf("Failed to commit blob to CAS: %v", err)
		return
	}

	log.Infof("Cached blob %s in CAS", dgst)
}

func (h *Handler) handleBlobHead(w http.ResponseWriter, r *http.Request, meta oci.RequestMeta) {
	log.Debugf("Handling blob head for repo: %s, digest: %s", meta.Repo, meta.Digest)

	upstreamURL := *r.URL
	upReq, err := http.NewRequest(r.Method, upstreamURL.String(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for k, v := range r.Header {
		upReq.Header[k] = v
	}

	resp, err := h.Client.Do(upReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)
}

func (h *Handler) handleBlobMount(w http.ResponseWriter, r *http.Request, meta oci.RequestMeta) {
	// Not implemented
	http.NotFound(w, r)
}

func (h *Handler) handleManifestHead(w http.ResponseWriter, r *http.Request, meta oci.RequestMeta) {
	// Not implemented
	http.NotFound(w, r)
}

func (h *Handler) handleManifestPut(w http.ResponseWriter, r *http.Request, meta oci.RequestMeta) {
	// Not implemented
	http.NotFound(w, r)
}

func (h *Handler) handleCatalogList(w http.ResponseWriter, r *http.Request, meta oci.RequestMeta) {
	// Not implemented
	http.NotFound(w, r)
}

func (h *Handler) handleManifestGet(w http.ResponseWriter, r *http.Request, meta oci.RequestMeta) {
	log.Debugf("Handling manifest get for repo: %s, reference: %s", meta.Repo, meta.Reference)

	upstreamURL := *r.URL
	upReq, err := http.NewRequest(r.Method, upstreamURL.String(), nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers
	for k, v := range r.Header {
		upReq.Header[k] = v
	}

	resp, err := h.Client.Do(upReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Read manifest once
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "failed to read manifest", http.StatusInternalServerError)
		return
	}

	log.Infof("Fetched manifest of size %d bytes", len(bodyBytes))

	// Log mediaType without modifying anything
	var manifest struct {
		SchemaVersion int    `json:"schemaVersion"`
		MediaType     string `json:"mediaType"`
	}
	if err := json.Unmarshal(bodyBytes, &manifest); err == nil {
		log.Infof("Manifest media type: %s", manifest.MediaType)
	}

	// Copy upstream headers exactly
	for k, v := range resp.Header {
		w.Header()[k] = v
	}

	w.WriteHeader(resp.StatusCode)
	w.Write(bodyBytes)
}

func (h *Handler) handleTagList(w http.ResponseWriter, r *http.Request, meta oci.RequestMeta) {
	fmt.Println("Handling tag list for repo:", meta.Repo)
	log.Debugf("Handling tag list for repo: %s", meta.Repo)
	upstreamURL := *r.URL

	upReq, err := http.NewRequest(r.Method, upstreamURL.String(), r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Copy headers (minus hop-by-hop)
	for k, v := range r.Header {
		upReq.Header[k] = v
	}

	resp, err := h.Client.Do(upReq)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	w.WriteHeader(resp.StatusCode)

	// Stream body to client
	io.Copy(w, resp.Body)
}

type ReverseProxy struct {
	Client *http.Client // with your RoundTripper chain
}

func proxyRequest(r *http.Request) (*http.Response, error) {
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (h *Handler) reverseProxy(w http.ResponseWriter, r *http.Request) {
	// Clone request for upstream
	req := r.Clone(r.Context())

	resp, err := h.Client.Do(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	// Copy headers
	for k, v := range resp.Header {
		for _, vv := range v {
			w.Header().Add(k, vv)
		}
	}
	w.WriteHeader(resp.StatusCode)

	// Stream body to client
	io.Copy(w, resp.Body)
}

func rewriteDockerHubRemainder(repoKey, repo, remainder string) string {
	// For docker.io, if the repo is singular (no slash), rewrite to library/
	if repoKey == "docker.io" && len(strings.Split(repo, "/")) == 1 {
		return path.Join("library", remainder)
	}
	return remainder
}

func buildUpstreamURL(remoteURL, remainder string) (url.URL, error) {
	base, err := url.Parse(remoteURL)
	if err != nil {
		return url.URL{}, err
	}
	base.Path = path.Join(base.Path, remainder)
	return *base, nil
}

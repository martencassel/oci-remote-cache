package oci

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/url"

	"strings"
)

// ErrNotOCI
var ErrNotOCI = http.ErrNotSupported

type Kind int

const (
	KindUnknown Kind = iota
	KindBlobGet
	KindBlobHead
	KindBlobMount
	KindManifestGet
	KindManifestHead
	KindManifestPut
	KindCatalogList
	KindPing
	KindTagsList
)

type RequestMeta struct {
	Kind        Kind
	RepoKey     string
	Verb        VerbType
	Repository  string
	SubVerb     string
	Repo        string
	Digest      string
	Reference   string
	IsDigestRef bool
	UploadUUID  string
	Query       url.Values
	RawPath     string
}

type ResponseMeta struct {
	Kind          Kind
	ContentType   string
	ContentLength int64
}

func kindFromVerb(method string, meta ParseResult) Kind {
	switch meta.Verb {
	case VerbBlobs:
		return classifyBlob(method)
	case VerbManifests:
		return classifyManifest(method)
	case VerbTags:
		if method == http.MethodGet && meta.SubVerb == "list" {
			return KindTagsList
		}
	}
	return KindUnknown
}

func ClassifyRequest(method, path string) RequestMeta {
	meta, err := Parse(method, path)
	if err != nil {
		return RequestMeta{Kind: KindUnknown}
	}

	return RequestMeta{
		Kind:       kindFromVerb(method, meta),
		RepoKey:    meta.RepoKey,
		Repository: meta.Repository,
		Verb:       meta.Verb,
		SubVerb:    meta.SubVerb,
		Digest:     meta.Digest,
		Reference:  meta.Reference,
		UploadUUID: meta.UploadUUID,
		Query:      meta.Query,
		RawPath:    meta.RawPath,
	}
}

func isBlobRequest(req *http.Request) bool {
	return req.Method == http.MethodGet && contains(req.URL.Path, "/blobs/")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[len(s)-len(substr):] == substr
}

// ErrUnknown
var ErrUnknown = fmt.Errorf("unknown request")

func Parse(method, path string) (ParseResult, error) {
	meta := ParseResult{RawPath: path}

	// 1. Ping
	if path == "/v2/" {
		return ParseResult{
			IsPing:  true,
			RawPath: path,
		}, nil
	}

	// 2. Split query
	var query string
	if i := strings.Index(path, "?"); i != -1 {
		query = path[i+1:]
		path = path[:i]
	}
	if query != "" {
		meta.Query, _ = url.ParseQuery(query)
	}

	// 3. Must start with /v2/
	if !strings.HasPrefix(path, "/v2/") {
		return ParseResult{}, ErrUnknown
	}

	rest := strings.TrimPrefix(path, "/v2/")
	segments := strings.Split(rest, "/")

	// Reject empty segments except a single trailing slash
	for idx, seg := range segments {
		if seg == "" {
			// allow exactly one trailing empty segment
			if idx == len(segments)-1 {
				continue
			}
			return ParseResult{}, ErrUnknown
		}
	}

	if len(segments) == 0 {
		return ParseResult{}, ErrUnknown
	}

	// Reserved verbs
	verbs := map[string]VerbType{
		"blobs":     VerbBlobs,
		"manifests": VerbManifests,
		"tags":      VerbTags,
		"referrers": VerbReferrers,
		"uploads":   VerbBlobs,
	}

	// 4. repoKey is ALWAYS the first segment
	meta.RepoKey = segments[0]
	i := 1

	// 5. repository continues until verb or special
	repoParts := []string{}
	for ; i < len(segments); i++ {
		seg := segments[i]
		if _, ok := verbs[seg]; ok {
			break
		}
		if strings.HasPrefix(seg, "_") {
			break
		}
		repoParts = append(repoParts, seg)
	}
	meta.Repository = strings.Join(repoParts, "/")

	// 6. special segment
	if i < len(segments) && strings.HasPrefix(segments[i], "_") {
		meta.SubVerb = segments[i]
		i++
	}

	// 7. verb
	if i < len(segments) {
		seg := segments[i]
		if vt, ok := verbs[seg]; ok {
			meta.Verb = vt

			tail := segments[i+1:]
			switch seg {
			case "blobs":
				if len(tail) > 0 {
					meta.Digest = tail[0]
				}
			case "uploads":
				if len(tail) > 0 {
					meta.UploadUUID = tail[0]
				}
				meta.SubVerb = "uploads"
			case "manifests":
				if len(tail) > 0 {
					meta.Reference = tail[0]
				}
			case "tags":
				if len(tail) > 0 && tail[0] == "list" {
					meta.SubVerb = "list"
				}
			case "referrers":
				if len(tail) > 0 {
					meta.Reference = tail[0]
				}
			}
		}
	}

	return meta, nil
}

func classifyManifest(method string) Kind {
	switch method {
	case http.MethodGet:
		return KindManifestGet
	case http.MethodHead:
		return KindManifestHead
	case http.MethodPut:
		return KindManifestPut
	}
	return KindUnknown
}

func summarizeBinary(body []byte) string {
	h := sha256.Sum256(body)
	return fmt.Sprintf("<binary %d bytes, sha256=%s>", len(body), hex.EncodeToString(h[:]))
}

func ParseAndClassify(method string, url *url.URL) (RequestMeta, error) {
	parsed, err := Parse(method, url.Path)
	if err != nil {
		return RequestMeta{}, err
	}

	// Convert ParseResult → RequestMeta
	meta := RequestMeta{
		RepoKey:     parsed.RepoKey,
		Repository:  parsed.Repository,
		Verb:        parsed.Verb,
		SubVerb:     parsed.SubVerb,
		Digest:      parsed.Digest,
		Reference:   parsed.Reference,
		UploadUUID:  parsed.UploadUUID,
		Query:       parsed.Query,
		RawPath:     parsed.RawPath,
		IsDigestRef: strings.HasPrefix(parsed.Reference, "sha256:"),
	}

	// Compute Kind
	meta.Kind = kindFromVerb(method, parsed)

	return meta, nil
}

func classifyBlob(method string) Kind {
	switch method {
	case http.MethodGet:
		return KindBlobGet
	case http.MethodHead:
		return KindBlobHead
	case http.MethodPost:
		return KindBlobMount
	}
	return KindUnknown
}

func ExtractRepoKey(path string) (repoKey string, remainder string, ok bool) {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) < 3 || parts[0] != "v2" {
		return "", "", false
	}

	repoKey = parts[1]
	remainder = "/" + strings.Join(parts[2:], "/")
	return repoKey, remainder, true
}

// VerbType
type VerbType int

const (
	VerbUnknown VerbType = iota
	VerbBlobs
	VerbManifests
	VerbTags
	VerbReferrers
)

type ParseResult struct {
	IsPing bool

	RepoKey    string
	Repository string

	// Operation classification
	Verb    VerbType
	SubVerb string // e.g. "uploads" for blobs/uploads or "list" for tags/list

	// Identifiers
	Digest     string
	Reference  string
	UploadUUID string

	// Query parameters
	Query url.Values

	// Raw
	RawPath string
}

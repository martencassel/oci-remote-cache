

# Upstream registry
- May require auth
- May redirect
- May throttle
- May return partial blobs
- May lie about content length

# Local cache (trusted but integrity critical)
- Must be content-addressed
- Must be atomic
- Must not serve corruped blobs
- Must be concurrency-safe

# Downstream clients
- Must receive streaming responses
- Must not block cache writes
- Must not cause memory blowups

Client → Proxy → Cache → Upstream


# Skeleton
- an HTTP server
- a single handle for /v2/.../blobs/<digest>
- a single RoundTripper chain
- a CAS directory on disk
- stream from upstream to client

# RoundTripper capabilities as a chain
- AuthRoundTripper
Adds Authorization headers, refreshes tokens, retries on 401.

- CachingRoundTripper
Intercepts blob GETs, serves from cache, writes through to cache.

- RetryRoundTripper
Handles transient upstream failures.

- Logging/MetricsRoundTripper
Emits structured events.

- Transport
The actual HTTP transport.

# Implement a CAS cache with strict invariants

OCI blobs are content‑addressed by digest.

- atomic writes
Write to a temp file, fsync, rename.

- no partial blobs
If the stream fails, delete the temp file.

- no double writes
If the file exists, never rewrite it.

- concurrency safety
Use file locks or atomic rename semantics.

# Streaming correctness and back-preassure
This is where your earlier io.Pipe + TeeReader pattern is essential.
Rules:
- Never buffer entire blobs
- Never block client on disk I/O
- Never block upstream on slow clients
- Always propagate upstream errors
- Always close pipes correctly

# Production
- handle auth flows
- handle redirects (registries love 307)
- handle range requests (optional but useful)
- handle manifest caching
- handle concurrency storms (multiple clients requesting same blob)
- add cache warming (optional)

/cmd/proxy
    main.go

/internal/
    http/
        auth_roundtripper.go        A clean RoundTripper that uses stream.Duplicate.
        cache_roundtripper.go
        retry_roundtripper.go
        metrics_roundtripper.go
    cache/
        cas.go              Atomic writes, digest verification, concurrency safety.
        writer.go
        reader.go
    upstream/
        client.go
    proxy/
        handler.go
        router.go

/pkg/
    stream/
        duplicator.go

package stream

import (
    "io"
    "os"
)

// Duplicate returns a new io.ReadCloser that streams data from `upstream`
// while simultaneously writing the same bytes to `cacheFile`.
//
// The returned ReadCloser must be consumed by the caller (e.g. http client).
// The cache write happens asynchronously and respects back-pressure.
func Duplicate(upstream io.ReadCloser, cacheFile *os.File) io.ReadCloser {
    pr, pw := io.Pipe()

    tee := io.TeeReader(upstream, pw)

    // Async: write to cache
    go func() {
        _, err := io.Copy(cacheFile, pr)
        pr.CloseWithError(err)
        cacheFile.Close()
    }()

    // Wrap tee + upstream into a single ReadCloser
    return &readCloser{
        Reader: tee,
        Closer: upstream,
    }
}

// readCloser is a small helper to combine a Reader and a Closer.
type readCloser struct {
    io.Reader
    io.Closer
}

# CachingRoundTripper
resp.Body = stream.Duplicate(resp.Body, f)

# Handlers
body := stream.Duplicate(upstream, f)
io.Copy(w, body)

# Background prefetchers
cached := stream.Duplicate(remoteBlob, f)
io.Copy(io.Discard, cached)


# Handler
- understands OCI semantics
- decides blob vs manifest vs catalog
- decides cache hit vs miss
- decides routing

# RoundTripper chain:

handles auth

handles redirects

handles retries

handles metrics

handles cache write‑through

handles stream duplication

handles upstream quirks

# CAS
handles atomicity

handles integrity

handles concurrency

handles content addressing



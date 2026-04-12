# OCI Remote Cache

Small OCI registry proxy with content-addressed blob caching on local disk.

This project sits between clients (podman, regctl, etc.) and upstream registries.
It forwards normal registry API traffic and caches blob downloads by digest.

## Why This Exists

Registry blobs are immutable and digest-addressed, which makes them ideal cache material.
The goal here is simple:

- First pull: stream from upstream to client and cache at the same time.
- Next pull of the same digest: serve locally.
- Never store partial or corrupt blobs.

## Current Scope

Implemented now:

- Proxying OCI API paths (notably blob GETs).
- Upstream auth challenge handling (Bearer token flow).
- CAS-style blob store on disk.
- Digest verification before accepting a cached blob.
- Atomic file writes.

Still evolving:

- Manifest caching strategy.
- Better cache-hit observability and metrics.
- Concurrency storm handling for the same digest.

## Quick Start

Requirements:

- Go 1.25+
- podman and/or regctl for manual testing

Run the proxy:

```bash
make run
```

By default it listens on `:8080` and stores cache under `/tmp/cache/oci`.

Configure local client tools:

```bash
regctl registry set --tls disabled localhost:8080 -v trace
```

Example pull through the proxy:

```bash
podman image pull localhost:8080/docker.io/alpine:latest --tls-verify=false
```

## How It Works

High-level flow:

Client -> Proxy Handler -> Upstream Registry
                    \\-> CAS (local disk cache)

Blob GET flow:

1. Parse request and classify it as blob GET.
2. Check local CAS for digest.
3. On hit: stream cached file to client.
4. On miss: fetch from upstream, stream to client, tee into CAS writer.
5. Commit cache entry only if digest matches expected value.

## Cache Layout

Blob files are stored by digest path:

`/tmp/cache/oci/blobs/<algorithm>/<prefix>/<hex>`

Example:

`/tmp/cache/oci/blobs/sha256/11/11c182...`

Metadata is written separately by the metadata store.

## Safety Invariants

The cache should keep these guarantees:

- Content-addressed: key is the OCI digest.
- Atomic writes: temp file + fsync + rename.
- No partial writes: failed streams are discarded.
- Digest validated before commit.

## Project Layout

Key paths:

- `main.go`: wiring for config, transport chain, stores, and router.
- `internal/proxy/handler.go`: request classification and blob cache logic.
- `internal/cache/cas.go`: blob writer lifecycle and metadata commit.
- `internal/blobs/blobs.go`: filesystem blob store and digest verification.
- `internal/transport/auth.go`: token challenge flow and retry.
- `internal/transport/logger.go`: verbose request/response tracing.
- `pkg/stream/duplicator.go`: stream duplication helpers.

##

If you record one feature, show this:

"Cold miss -> warm hit for the same blob digest"

Why this is the best demo:

- It proves functional correctness quickly.
- It shows performance value without synthetic benchmarks.
- It highlights the core engineering promise: digest-safe local reuse.

### Demo Script (5 minutes)

1. Clean cache directory.
2. Start proxy with visible logs.
3. Pull a medium image once (cold path).
4. Show cached blob files created under `/tmp/cache/oci/blobs/sha256/...`.
5. Pull the same image again (warm path).
6. Show fewer upstream fetch logs / faster completion.

Example command sequence:

```bash
rm -rf /tmp/cache/oci
mkdir -p /tmp/cache/oci

make run
```

In another terminal:

```bash
regctl registry set --tls disabled localhost:8080 -v trace
podman image rm localhost:8080/docker.io/alpine:latest || true
podman image pull localhost:8080/docker.io/alpine:latest --tls-verify=false

find /tmp/cache/oci/blobs -type f | head

podman image rm localhost:8080/docker.io/alpine:latest || true
podman image pull localhost:8080/docker.io/alpine:latest --tls-verify=false
```

Optional final line in the recording:

"First pull paid the network cost, second pull reused verified local blobs."

## Notes

- This is a learning-oriented project with production-minded constraints.
- The logging middleware is intentionally verbose for debugging request lifecycles.



# Research: Auto-Init & Docker Support

**Date**: 2026-03-21

## R1: Auto-Init Strategy

**Decision**: Add `EnsureInit()` to `MemoryStore` — idempotent
init that creates only missing dirs/files, never overwrites.

**Rationale**: `Init()` errors on "already initialized" (by design
for explicit use). `EnsureInit()` is safe to call every time —
it checks each file/dir individually and creates only what's
missing. This also handles partial corruption (e.g., someone
deleted `principles.md`).

**Implementation**:
```go
func (s *MemoryStore) EnsureInit() (bool, error) {
    if _, err := os.Stat(s.Root); err == nil {
        // Root exists — ensure all files are present
        s.ensureMissingFiles()
        return false, nil
    }
    // Root doesn't exist — full init
    if err := s.doInit(); err != nil {
        return false, err
    }
    return true, nil
}
```

Returns `(created bool, err)` so the caller can print the stderr
notice only when actually created.

---

## R2: Docker Base Image

**Decision**: Use `scratch` as final image (not Alpine).

**Rationale**: `mem` is a statically-linked Go binary with zero
runtime dependencies. `scratch` produces the smallest possible
image (~3 MB for just the binary). Alpine would add ~5 MB for no
benefit since `mem` doesn't need a shell, libc, or any OS tools.

Build with `CGO_ENABLED=0` to ensure static linking.

**Dockerfile**:
```dockerfile
FROM golang:1.26-alpine AS builder
WORKDIR /build
COPY go.mod ./
COPY cmd/ cmd/
COPY internal/ internal/
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o mem ./cmd/mem/

FROM scratch
COPY --from=builder /build/mem /mem
WORKDIR /project
ENTRYPOINT ["/mem"]
```

**Image size**: ~3 MB (binary ~2.9 MB stripped).

---

## R3: GitHub Actions Workflow

**Decision**: Standard `docker/build-push-action` workflow
triggered on release tags.

**Pattern**:
```yaml
on:
  release:
    types: [published]
```

Builds for `linux/amd64` and `linux/arm64`, pushes to
`ghcr.io/snow-ghost/mem` with tags: `latest` + version.

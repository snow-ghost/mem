# Tasks: Auto-Init & Docker Support

**Input**: Design documents from `/specs/003-auto-init-docker/`
**Prerequisites**: plan.md, spec.md, research.md

**Tests**: BDD mandatory. Tests FIRST, then implement.

**Organization**: Grouped by user story. Small feature — ~75 lines total.

## Format: `[ID] [P?] [Story] Description`

---

## Phase 1: Foundational

**Purpose**: EnsureInit method — required by all user stories

- [x] T001 BDD test + implement `EnsureInit() (bool, error)` on MemoryStore in `internal/store/store.go` and `internal/store/store_test.go`. Idempotent: creates `.memory/` with all files if root doesn't exist, creates only missing files if root exists, never overwrites. Returns `(true, nil)` when created, `(false, nil)` when already existed. Scenarios: Given no .memory/ dir, When EnsureInit called, Then all files/dirs created and returns true; Given .memory/ already exists with all files, When EnsureInit called, Then nothing changed and returns false; Given .memory/ exists but principles.md is missing, When EnsureInit called, Then only principles.md created, episodes.jsonl untouched; Given read-only parent dir, When EnsureInit called, Then returns error with OS message

---

## Phase 2: User Story 1 — Auto-Init on First Use (Priority: P1)

**Goal**: Any command auto-creates `.memory/` if missing

**Independent Test**: Run `mem status` in a fresh dir without `mem init` — works on first try.

- [x] T002 [US1] Wire `EnsureInit()` into all commands except `init` in `cmd/mem/main.go` — add a helper `ensureStore(s *store.MemoryStore) int` that calls `s.EnsureInit()`, prints `mem: initialized memory store at <path>` to stderr if created, returns 0 on success or error exit code on failure. Call it at the start of `runExtract`, `runConsolidate`, `runInject`, `runStatus`. `runInit` is unchanged.
- [x] T003 [US1] BDD test auto-init via CLI — build binary, run in temp dir without .memory/. Scenarios: Given fresh dir, When `mem status` run, Then .memory/ created + status output on stdout + notice on stderr; Given fresh dir, When `mem inject` run, Then .memory/ created + empty context on stdout + notice on stderr only; Given existing .memory/ with episodes, When `mem status` run, Then no notice on stderr and episodes count correct

---

## Phase 3: User Story 2 — Dockerfile (Priority: P2)

**Goal**: `docker build . && docker run mem status` works

- [x] T004 [P] [US2] Create `Dockerfile` in repository root — multi-stage build: stage 1 `golang:1.26-alpine` builds with `CGO_ENABLED=0 go build -ldflags="-s -w"`, stage 2 `scratch` copies binary only. Entrypoint `/mem`, workdir `/project`.
- [x] T005 [P] [US2] Create `.dockerignore` in repository root — exclude `.git/`, `.memory/`, `specs/`, `.specify/`, `*.md` (except Dockerfile-relevant), `.opencode/`
- [x] T006 [US2] Test Docker build + run — `docker build -t mem .` succeeds, `docker run --rm mem status` shows output (auto-init inside container), verify image size < 10 MB with `docker images mem --format "{{.Size}}"`

---

## Phase 4: User Story 3 — GitHub Actions Workflow (Priority: P3)

**Goal**: Docker image auto-published on release

- [x] T007 [US3] Create `.github/workflows/docker.yml` — triggered on release published, builds multi-platform image (linux/amd64, linux/arm64), pushes to `ghcr.io/snow-ghost/mem` with tags `latest` + version. Uses `docker/setup-buildx-action`, `docker/login-action` (GHCR), `docker/build-push-action`.

---

## Phase 5: Polish

- [x] T008 [P] Update README.md — add Docker usage section: `docker run --rm -v $(pwd):/project ghcr.io/snow-ghost/mem status`, note about auto-init, note about mounting backend binaries
- [x] T009 [P] Run `go test -race -shuffle=on ./...` and `go vet ./...` — verify all tests pass, no regressions

---

## Dependencies & Execution Order

- **Phase 1**: No deps — start immediately
- **US1 (Phase 2)**: Depends on Phase 1 (needs EnsureInit)
- **US2 (Phase 3)**: Depends on Phase 1 (auto-init makes Docker UX better)
- **US3 (Phase 4)**: Depends on Phase 3 (needs Dockerfile)
- **Polish (Phase 5)**: After all user stories

## Parallel Opportunities

- Phase 3: T004, T005 can run in parallel
- Phase 5: T008, T009 can run in parallel

---

## Implementation Strategy

### MVP: Phase 1 + Phase 2 (T001-T003)

Auto-init works. Users never need `mem init` again.

### Full: + Phase 3-4 (T004-T007)

Docker image built and published.

---

## Notes

- 9 tasks total, ~75 lines of code changes
- BDD mandatory for T001, T003
- T004-T007 are config/infra files (Dockerfile, YAML), not Go code
- `scratch` image = no shell in container, but `mem` doesn't need one

# Feature Specification: Auto-Init & Docker Support

**Feature Branch**: `003-auto-init-docker`
**Created**: 2026-03-21
**Status**: Draft
**Input**: User description: "автоинициализация при запуске любой команды + запуск в докере"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Auto-Initialization on First Use (Priority: P1)

As a developer who just installed `mem`, I want any command
(`extract`, `consolidate`, `inject`, `status`) to automatically
initialize the memory store if it doesn't exist yet, so that I
don't have to remember to run `mem init` separately.

Today, if I run `mem extract` in a project without `.memory/`,
it fails with an error. Instead, it should silently create the
`.memory/` directory and proceed normally.

**Why this priority**: This removes the most common friction point
for new users. Every other feature depends on having an initialized
store, so auto-init unblocks everything.

**Independent Test**: In a fresh git project with no `.memory/`
directory, run `mem status`. Verify that `.memory/` is created
automatically and the command succeeds.

**Acceptance Scenarios**:

1. **Given** a project directory without `.memory/`, **When**
   `mem extract` is run, **Then** the memory store is
   automatically initialized and extraction proceeds normally.
2. **Given** a project directory without `.memory/`, **When**
   `mem status` is run, **Then** the memory store is initialized
   and status is displayed (all zeros).
3. **Given** a project directory without `.memory/`, **When**
   `mem inject` is run, **Then** the memory store is initialized
   and empty context is output (no error).
4. **Given** a project that already has `.memory/`, **When** any
   command is run, **Then** no re-initialization occurs, existing
   data is preserved.
5. **Given** `mem init` is run explicitly, **When** the store
   already exists, **Then** the current behavior is preserved
   (error: already initialized). Explicit `init` remains
   idempotent-safe.
6. **Given** auto-init runs, **When** the user checks output,
   **Then** a one-line notice is printed to stderr:
   `mem: initialized memory store at <path>` so the user knows
   what happened, but it does not interfere with stdout output
   (important for `mem inject` piping).

---

### User Story 2 - Run mem in Docker (Priority: P2)

As a developer or CI/CD engineer, I want to run `mem` inside a
Docker container so that I can use it in automated pipelines, air-
gapped environments, or without installing Go on the host.

I pull the official Docker image, mount my project directory, and
run `mem` commands. The container has `mem` pre-installed and
ready to use.

**Why this priority**: Docker support enables CI/CD integration
and removes the Go install dependency. But it's lower priority
than auto-init because most users run `mem` locally.

**Independent Test**: Run `docker run --rm -v $(pwd):/project
ghcr.io/snow-ghost/mem status` in a project with `.memory/`.
Verify that status is displayed correctly.

**Acceptance Scenarios**:

1. **Given** a Dockerfile exists in the repository, **When**
   `docker build -t mem .` is run, **Then** a working image is
   produced with the `mem` binary as the entrypoint.
2. **Given** the Docker image is built, **When** `docker run
   --rm -v $(pwd):/project mem status` is run from a project
   directory, **Then** `mem` sees the project's `.memory/` and
   displays correct status.
3. **Given** the Docker image, **When** `docker run --rm -v
   $(pwd):/project mem extract --backend custom
   --backend-binary echo` is run, **Then** extraction runs inside
   the container (backend binary must be available in the
   container or mounted).
4. **Given** the Docker image, **When** it's used in a CI/CD
   pipeline (e.g., GitHub Actions), **Then** the image starts
   quickly (<2 seconds) and commands execute without additional
   setup.

---

### User Story 3 - Docker Image Published to Registry (Priority: P3)

As a developer, I want the `mem` Docker image to be published to
a container registry (GitHub Container Registry) so I can pull it
directly without building locally.

**Why this priority**: Convenience layer on top of US2. Building
locally works first; registry publishing is optimization.

**Independent Test**: Run `docker pull ghcr.io/snow-ghost/mem`
and verify the image contains a working `mem` binary.

**Acceptance Scenarios**:

1. **Given** a GitHub Actions workflow exists, **When** a release
   is tagged, **Then** the Docker image is automatically built
   and pushed to `ghcr.io/snow-ghost/mem`.
2. **Given** the published image, **When** pulled and run,
   **Then** it behaves identically to a locally-built image.

---

### Edge Cases

- What happens when auto-init fails due to permissions (read-only
  filesystem)? The system MUST show the original error from the
  directory creation, not a confusing "store not initialized"
  message.
- What happens when `.memory/` exists but is corrupted (e.g.,
  episodes.jsonl is missing)? Auto-init MUST NOT overwrite
  existing files. It should only create missing files/directories.
- What happens when `mem` runs inside Docker without a mounted
  volume? The `.memory/` is created inside the container and lost
  when the container exits. This is expected behavior — no special
  handling needed, but documentation should warn about it.
- What happens when the Docker container has no backend CLI
  installed? `mem inject` and `mem status` work (no backend
  needed). `mem extract` and `mem consolidate` fail with the
  standard "no supported backend found" error.
- What happens when `mem init` is run inside Docker? It works
  the same as locally. If a volume is mounted, the store is
  created on the host filesystem.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: All commands (`extract`, `consolidate`, `inject`,
  `status`) MUST automatically initialize the memory store if
  `.memory/` does not exist in the resolved path. The `init`
  command itself remains unchanged (explicit, errors if already
  initialized).
- **FR-002**: Auto-initialization MUST print a one-line notice to
  stderr: `mem: initialized memory store at <path>`. It MUST NOT
  print to stdout (to preserve piping for `mem inject`).
- **FR-003**: Auto-initialization MUST NOT overwrite any existing
  files. If `.memory/` exists but some files are missing (e.g.,
  `principles.md` was deleted), only the missing files MUST be
  created.
- **FR-004**: Auto-initialization MUST create prompt template
  files in `.memory/prompts/` with the same defaults as `mem
  init`.
- **FR-005**: A `Dockerfile` MUST be provided in the repository
  root that builds a minimal image with the `mem` binary.
- **FR-006**: The Docker image MUST use a multi-stage build to
  keep the final image small (binary only, no Go toolchain).
- **FR-007**: The Docker image entrypoint MUST be `mem`, so that
  `docker run mem <command>` works directly.
- **FR-008**: The Docker image MUST set the working directory to
  `/project` and expect the user's project to be mounted there.
- **FR-009**: The Dockerfile MUST be buildable with a single
  `docker build .` command without external dependencies or build
  arguments.
- **FR-010**: A GitHub Actions workflow MUST build and publish
  the Docker image to `ghcr.io/snow-ghost/mem` on each release
  tag.

## Assumptions

- Auto-init creates the same directory structure as `mem init`
  (episodes.jsonl, principles.md, skills/, consolidation-log.md,
  prompts/).
- The Docker image is based on a minimal Linux distro (Alpine or
  scratch) for small size.
- No backend CLIs (claude, opencode, codex) are pre-installed in
  the Docker image — users mount them or use custom backend config.
  `mem inject` and `mem status` work without a backend.
- The GitHub Actions workflow uses the standard
  `docker/build-push-action` pattern.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A new user can run `mem status` in any git project
  without running `mem init` first and see correct output on the
  first try.
- **SC-002**: Auto-init adds less than 10ms to command startup
  (measured as time to check if `.memory/` exists and create it if
  missing).
- **SC-003**: The Docker image is smaller than 10 MB.
- **SC-004**: `docker run --rm -v $(pwd):/project ghcr.io/snow-ghost/mem status`
  works in under 2 seconds from a cold start.
- **SC-005**: The GitHub Actions workflow builds and publishes the
  image in under 5 minutes.

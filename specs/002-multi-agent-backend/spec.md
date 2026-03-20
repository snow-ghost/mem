# Feature Specification: Multi-Agent Backend Support

**Feature Branch**: `002-multi-agent-backend`
**Created**: 2026-03-20
**Status**: Draft
**Input**: User description: "требуется сделать поддержку opencode и codex и чтобы работало без всяких оберток"

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Use mem with OpenCode (Priority: P1)

As a developer using OpenCode as my AI coding agent, I want `mem`
to work natively with OpenCode without requiring Claude Code or
any wrapper scripts so that I get the same memory capabilities
regardless of which agent I use.

I run `mem extract` and the system automatically detects that
OpenCode is available (or I configure it), invokes OpenCode with
the extraction prompt in quiet mode (suppressing the TUI spinner),
and captures episodes exactly the same way it does with Claude
Code today.

**Why this priority**: OpenCode is a popular open-source AI coding
agent. Supporting it first validates the multi-backend architecture
and proves `mem` is agent-agnostic.

**Independent Test**: Install OpenCode, configure `mem` to use it,
run `mem extract` after a coding session, verify episodes are
captured identically to how they would be with Claude Code.

**Acceptance Scenarios**:

1. **Given** OpenCode is installed and configured as the backend,
   **When** `mem extract` is run, **Then** OpenCode is invoked
   with `-p "<prompt>" -q` and episodes are captured normally.
2. **Given** the user sets `MEM_BACKEND=opencode`, **When** `mem
   extract` is run, **Then** OpenCode is used instead of Claude.
3. **Given** OpenCode is configured but not installed on the
   system, **When** `mem extract` is run, **Then** an error
   message states: `backend "opencode" not found — binary
   "opencode" is not installed or not in PATH`.
4. **Given** OpenCode is the backend, **When** `mem consolidate`
   is run, **Then** it uses OpenCode for LLM analysis. **When**
   `mem inject` is run, **Then** it operates locally with no
   backend invocation.

---

### User Story 2 - Use mem with Codex (Priority: P2)

As a developer using OpenAI Codex CLI as my AI coding agent, I
want `mem` to work natively with Codex so that I can use memory
capabilities with OpenAI's ecosystem.

I configure `mem` to use Codex, and all LLM commands (extract,
consolidate) invoke Codex via its non-interactive `exec`
subcommand with the appropriate prompt format.

**Why this priority**: Codex CLI is another major agent platform.
Supporting it alongside OpenCode proves true agent-agnosticism.

**Independent Test**: Configure `mem` with Codex backend, run
`mem extract`, verify episodes are captured correctly.

**Acceptance Scenarios**:

1. **Given** Codex is configured as the backend, **When** `mem
   extract` is run, **Then** Codex is invoked via
   `codex exec "<prompt>" -m <model>`.
2. **Given** Codex streams progress to stderr and final answer to
   stdout, **When** the system reads the response, **Then** only
   stdout is parsed for episodes (stderr is ignored during normal
   operation but surfaced on error).
3. **Given** Codex returns a response with non-JSON preamble text
   followed by a JSON array, **When** extraction parses the
   response, **Then** episodes are extracted correctly (JSON
   extraction from mixed text already works).

---

### User Story 3 - Auto-Detection of Available Backend (Priority: P3)

As a developer who has not explicitly configured a backend, I want
`mem` to automatically detect which AI coding agent is available
on my system so that it works out of the box without any manual
configuration.

When I run `mem extract` without setting `MEM_BACKEND` or
`--backend`, the system probes for installed binaries via PATH
lookup in the fixed priority order: `claude` > `opencode` >
`codex`. The first binary found is used.

**Why this priority**: Zero-configuration experience is important
for adoption, but explicit configuration (US1, US2) must work
first.

**Independent Test**: With only OpenCode installed (no Claude),
run `mem extract` without any configuration. Verify OpenCode is
automatically detected and used.

**Acceptance Scenarios**:

1. **Given** only `opencode` is installed on the system, **When**
   `mem extract` is run without backend configuration, **Then**
   OpenCode is automatically detected and used.
2. **Given** both `claude` and `opencode` are installed, **When**
   `mem extract` is run without backend configuration, **Then**
   Claude is selected (higher priority). The status output MUST
   show which backend was auto-detected.
3. **Given** no supported backend is installed, **When** `mem
   extract` is run, **Then** an error message lists all supported
   backends (`claude`, `opencode`, `codex`) and shows how to
   configure a custom backend.
4. **Given** the user had OpenCode auto-detected yesterday, and
   today installs Claude, **When** `mem extract` is run, **Then**
   Claude is now selected (auto-detection runs fresh each time,
   no caching between runs).

---

### User Story 4 - Custom Backend Support (Priority: P4)

As a developer using a less common AI coding agent or a custom
setup, I want to configure `mem` to use any CLI tool that accepts
a prompt and returns text so that I'm not locked into a specific
set of supported agents.

I set `MEM_BACKEND=custom` plus `MEM_BACKEND_BINARY` (binary path)
and `MEM_BACKEND_ARGS` (argument template with `{prompt}` and
optionally `{model}` placeholders), and `mem` invokes it
accordingly.

**Why this priority**: Extensibility ensures `mem` remains useful
as new AI agents emerge, but it's lower priority than the two
named integrations.

**Independent Test**: Configure a custom backend pointing to a
simple echo script, run `mem extract`, verify the script is
invoked with the correct prompt.

**Acceptance Scenarios**:

1. **Given** `MEM_BACKEND=custom`, `MEM_BACKEND_BINARY=my-agent`,
   `MEM_BACKEND_ARGS="-p {prompt} --model {model}"`, **When**
   `mem extract` is run, **Then** `my-agent` is invoked with
   `{prompt}` replaced by the actual prompt and `{model}` replaced
   by the model name.
2. **Given** `MEM_BACKEND=custom` but `MEM_BACKEND_BINARY` is not
   set, **When** `mem extract` is run, **Then** an error states:
   `MEM_BACKEND_BINARY is required when MEM_BACKEND=custom`.
3. **Given** `MEM_BACKEND=custom` and `MEM_BACKEND_BINARY` is set
   but `MEM_BACKEND_ARGS` is not set, **When** `mem extract` is
   run, **Then** the system uses a default template `{prompt}` (the
   prompt is passed as the sole positional argument).
4. **Given** `MEM_BACKEND_ARGS` contains `{prompt}` but not
   `{model}`, **When** `mem extract --model sonnet` is run,
   **Then** the model flag is silently ignored (no error) since
   the template has no `{model}` placeholder.
5. **Given** a custom backend binary does not exist at the
   specified path, **When** `mem extract` is run, **Then** an
   error states the binary was not found.

---

### Edge Cases

- What happens when the configured backend returns an empty
  response? The system MUST handle it gracefully (no episodes
  extracted, no error).
- What happens when the backend returns non-JSON mixed with JSON?
  The system already extracts JSON from mixed text — this MUST
  continue to work with all backends.
- What happens when the user switches backends between sessions?
  Episodes MUST remain compatible regardless of which backend
  created them. The backend name MUST NOT be stored in episodes.
- What happens when a backend requires authentication that has
  expired? The error from the backend (stderr + exit code) MUST
  be surfaced to the user in the error message without being
  swallowed.
- What happens when `MEM_BACKEND` is set to an unrecognized value
  (e.g., "gemini")? The system MUST return an error listing valid
  values: `claude`, `opencode`, `codex`, `custom`.
- What happens when a backend binary exists but is not executable
  (permission denied)? The system MUST surface the OS-level error
  ("permission denied") in the error message.
- What happens when the backend takes longer than 5 minutes? The
  system does NOT enforce its own timeout — it relies on the
  backend's internal timeout. The user can interrupt with Ctrl+C.
- What happens when extraction is interrupted mid-way (backend
  crashes)? No partial episodes are written — episodes are only
  appended after the full response is parsed and validated.
- What happens when `MEM_BACKEND_ARGS` contains a prompt with
  special characters (quotes, newlines)? The prompt is passed as
  a single argument element (not shell-expanded). The template is
  split by whitespace, but `{prompt}` is replaced as a whole
  element, not interpolated into the split result.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST support multiple LLM backends for
  extraction and consolidation. Only these two commands invoke an
  LLM backend. All other commands (`init`, `inject`, `status`)
  operate locally with no backend invocation.
- **FR-002**: Users MUST be able to select a backend via
  environment variable `MEM_BACKEND`. Accepted values: `claude`,
  `opencode`, `codex`, `custom`. Any unrecognized value MUST
  produce an error listing valid values.
- **FR-003**: System MUST auto-detect available backends when no
  explicit selection is made. Detection uses PATH lookup
  (`exec.LookPath`) in fixed priority order:
  `claude` > `opencode` > `codex`. The first binary found in PATH
  is used. Auto-detection runs fresh on every command invocation
  (no caching between runs).
- **FR-004**: Each built-in backend MUST be defined by its exact
  CLI invocation pattern:
  - **Claude Code**: `claude -p "<prompt>" --model <model>`
  - **OpenCode**: `opencode -p "<prompt>" -q` (quiet mode
    suppresses TUI spinner for scripted use; model selection uses
    the provider's configured default — the `--model` flag is
    not passed to OpenCode)
  - **Codex**: `codex exec "<prompt>" -m <model>` (non-interactive
    `exec` subcommand; progress streams to stderr, final answer
    to stdout)
- **FR-005**: System MUST support a custom backend configured via:
  - `MEM_BACKEND=custom` (selects custom mode)
  - `MEM_BACKEND_BINARY` (required — binary path or name)
  - `MEM_BACKEND_ARGS` (optional — argument template with
    `{prompt}` and `{model}` placeholders; default: `{prompt}`)
  Template processing: split `MEM_BACKEND_ARGS` by whitespace,
  replace `{prompt}` and `{model}` tokens with actual values as
  whole argument elements (not shell-interpolated). If `{model}`
  is absent from the template, the model flag is silently ignored.
- **FR-006**: The `--model` flag MUST continue to work for
  backends that support it. The `--model` flag value takes
  precedence over any backend default. For backends that do not
  support model selection (OpenCode), the flag is silently
  ignored. Per-command defaults remain: `haiku` for extract,
  `sonnet` for consolidate.
- **FR-007**: System MUST provide clear, differentiated error
  messages for each failure mode:
  - Binary not found: `backend "<name>" not found — binary
    "<binary>" is not installed or not in PATH`
  - Binary not executable: surface OS error (e.g., "permission
    denied")
  - Backend returned non-zero exit: include the backend's stderr
    output in the error message
  - No backend found (auto-detect): list all supported backends
    and custom backend configuration instructions
  - Invalid `MEM_BACKEND` value: list valid values
  - Custom backend missing `MEM_BACKEND_BINARY`: specific error
    stating the requirement
- **FR-008**: The injection command (`mem inject`) MUST NOT be
  affected by backend selection — it is a local file operation
  with no LLM call. Similarly, `mem init` and `mem status` MUST
  NOT require or invoke a backend.
- **FR-009**: Episodes and all memory files MUST remain
  backend-agnostic — no backend-specific data stored in memory
  files. The `agent_id` field in episodes identifies the agent
  instance (hostname/PID), NOT the backend provider.
- **FR-010**: System MUST allow users to override backend per
  command via `--backend` flag. The `--backend` flag MUST accept
  the same values as `MEM_BACKEND` (`claude`, `opencode`, `codex`,
  `custom`). The flag always takes precedence over the
  environment variable.
- **FR-011**: System MUST be backward compatible. When
  `MEM_BACKEND` is not set and `claude` is found via auto-detect,
  the behavior MUST be identical to the current implementation
  (same binary, same arguments, same defaults). Existing
  Claude-only users MUST NOT experience any behavioral change.
- **FR-012**: When auto-detection selects a backend, `mem status`
  MUST display which backend was detected (e.g.,
  `Backend: claude (auto-detected)`).
- **FR-013**: System MUST read backend responses from stdout only.
  Stderr from the backend is captured but only surfaced in error
  messages when the backend exits with a non-zero code. During
  normal operation, stderr is discarded.

### Key Entities

- **Backend**: A supported LLM CLI tool. Attributes: name (unique
  identifier), binary (executable name or path), argument builder
  (function producing CLI args from prompt and model), model
  support flag (whether --model is passed to this backend).
- **Backend Registry**: The collection of all known backends
  (3 built-in + optional user-configured custom backend).
  Resolution order: `--backend` flag > `MEM_BACKEND` env >
  auto-detect (PATH lookup in priority: claude > opencode > codex).

## Assumptions

- OpenCode CLI invocation pattern: `opencode -p "<prompt>" -q`
  confirmed via official documentation
  (https://opencode.ai/docs/cli/). OpenCode does not accept a
  `--model` flag — it uses the provider configured in its own
  settings.
- Codex CLI invocation pattern: `codex exec "<prompt>" -m <model>`
  confirmed via official documentation
  (https://developers.openai.com/codex/noninteractive). Codex
  streams progress to stderr and prints the final answer to stdout.
- All backends return text to stdout that contains (or consists
  of) JSON. The existing JSON extraction logic (find first `[` or
  `{` in output) handles wrapper text from all tested backends.
- Backend selection does not affect the quality of extraction or
  consolidation — the prompts are identical, only the LLM provider
  changes.
- The system does NOT enforce its own execution timeout on backends.
  Each backend manages its own timeout internally. Users can
  interrupt with Ctrl+C (SIGINT propagation is handled by the OS).
- Prompt text is passed as a single CLI argument. Backends that
  require file-based prompt input (if any exist) are not supported
  by built-in backends but could be handled via custom backend
  with a wrapper script.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: A user with only OpenCode installed (no Claude Code)
  can run `mem init` + `mem extract` + `mem status` and see
  episodes captured in `.memory/episodes.jsonl` and status showing
  `Backend: opencode (auto-detected)`.
- **SC-002**: Given the same git diff input, episodes produced by
  different backends contain the same JSON fields (`ts`, `session`,
  `type`, `summary`, `tags`) and no backend-specific metadata.
  Episode content may differ (different LLMs produce different
  text), but the schema and format are identical.
- **SC-003**: Adding a new built-in backend requires adding one
  struct definition with name, binary, and argument builder — zero
  changes to `internal/runner/`, `internal/episode/`, or any other
  package outside `internal/agent/`.
- **SC-004**: Backend resolution (flag + env + auto-detect) adds
  less than 50ms to command startup, measured as the time between
  process start and first backend invocation minus the baseline
  (current Claude-only path).
- **SC-005**: Auto-detection via `exec.LookPath` completes within
  100ms for 3 binary lookups on a standard Linux/macOS system with
  a PATH containing <20 directories.

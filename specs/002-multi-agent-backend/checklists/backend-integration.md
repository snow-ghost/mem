# Backend Integration & Configuration Checklist: Multi-Agent Backend Support

**Purpose**: Validate requirements quality for backend integration, configuration UX, auto-detection, custom backends, and backward compatibility — thorough depth for release readiness
**Created**: 2026-03-20
**Feature**: [spec.md](../spec.md)

## Requirement Completeness — Backend Invocation Patterns

- [x] CHK001 Are the exact CLI argument patterns documented for all three built-in backends (Claude, OpenCode, Codex)? [Completeness, Spec §FR-004]
- [x] CHK002 Is the OpenCode `-q` (quiet) flag requirement explicitly specified in the spec, or only in research.md? [Completeness, Spec §FR-004 — now specifies `-q`]
- [x] CHK003 Is the Codex `exec` subcommand requirement documented in the spec's FR-004, or only implicitly assumed? [Completeness, Spec §FR-004 — now specifies `exec`]
- [x] CHK004 Are output stream requirements specified per backend (stdout vs stderr)? [Completeness, Spec §FR-013 — stdout only, stderr surfaced on error]
- [x] CHK005 Is the behavior specified when a backend does not support the `--model` flag (e.g., OpenCode)? [Completeness, Spec §FR-004, §FR-006 — silently ignored]
- [x] CHK006 Are prompt size/length limits documented per backend? [Accepted as out of scope — Assumptions states prompts passed as single CLI arg; each backend manages its own limits internally]

## Requirement Clarity — Configuration

- [x] CHK007 Is the `MEM_BACKEND` env var value set explicitly defined — exact accepted values listed? [Clarity, Spec §FR-002 — "claude", "opencode", "codex", "custom"]
- [x] CHK008 Is the behavior specified when `MEM_BACKEND` is set to an unrecognized value? [Edge Case, Spec §FR-002, §FR-007 — error listing valid values]
- [x] CHK009 Is the `--backend` flag interaction with `MEM_BACKEND` explicitly specified? [Clarity, Spec §FR-010 — flag always overrides env]
- [x] CHK010 Is it clear whether `--backend` accepts the same values as `MEM_BACKEND`? [Clarity, Spec §FR-010 — same values confirmed]
- [x] CHK011 Is the relationship between `--model` and backend-specific default models unambiguous? [Clarity, Spec §FR-006 — flag takes precedence]

## Requirement Completeness — Auto-Detection

- [x] CHK012 Is the auto-detection priority order explicitly specified in FR-003? [Completeness, Spec §FR-003 — "claude > opencode > codex"]
- [x] CHK013 Is the auto-detection mechanism specified? [Clarity, Spec §FR-003 — PATH lookup via exec.LookPath]
- [x] CHK014 Are requirements defined for when auto-detection finds multiple backends — is user informed? [Completeness, Spec §FR-012 — status shows detected backend]
- [x] CHK015 Is the auto-detection performance requirement traceable? [Traceability, Spec §SC-005 — 100ms for 3 lookups]
- [x] CHK016 Are requirements defined for caching auto-detection results? [Completeness, Spec §FR-003 — "runs fresh on every command invocation (no caching)"]

## Requirement Completeness — Custom Backend

- [x] CHK017 Is the `MEM_BACKEND_ARGS` template format fully specified? [Completeness, Spec §FR-005 — `{prompt}` and `{model}` placeholders]
- [x] CHK018 Is the behavior specified when args has `{prompt}` but not `{model}`? [Edge Case, Spec §FR-005 — model silently ignored]
- [x] CHK019 Is the behavior specified when `MEM_BACKEND=custom` but `MEM_BACKEND_BINARY` is missing? [Edge Case, Spec §FR-007 — specific error message]
- [x] CHK020 Is the behavior specified when `MEM_BACKEND_ARGS` is missing? [Edge Case, Spec §FR-005 — default template `{prompt}`]
- [x] CHK021 Are quoting/escaping requirements defined for prompts with special characters? [Clarity, Spec §FR-005 and Edge Cases — prompt passed as whole argument element, not shell-interpolated]
- [x] CHK022 Is it specified whether `MEM_BACKEND_ARGS` is split by whitespace or supports shell-style quoting? [Clarity, Spec §FR-005 — split by whitespace, `{prompt}` replaced as whole element]
- [x] CHK023 Are requirements defined for custom backend validation at config time? [Gap — accepted: validation happens at invocation time, not config time. Binary existence is checked when the command runs.]

## Requirement Consistency — Backend Agnosticism

- [x] CHK024 Is FR-009 consistent with the existing `agent_id` field? [Consistency, Spec §FR-009 — agent_id identifies instance, NOT provider]
- [x] CHK025 Are the prompts specified as backend-agnostic? [Consistency, Assumptions — "prompts are identical, only provider changes"]
- [x] CHK026 Is it consistent that FR-008 excludes inject while FR-001 says "extraction and consolidation"? [Consistency, Spec §FR-001, §FR-008 — FR-001 updated to clarify only extract/consolidate use LLM; init/inject/status are local]

## Requirement Completeness — Error Handling

- [x] CHK027 Are error message requirements specified for all failure modes? [Completeness, Spec §FR-007 — 6 distinct failure modes listed]
- [x] CHK028 Is the timeout behavior specified? [Completeness, Edge Cases and Assumptions — no own timeout, relies on backend + Ctrl+C]
- [x] CHK029 Are requirements defined for surfacing backend stderr on failure? [Completeness, Spec §FR-007, §FR-013 — stderr included in error message on non-zero exit]
- [x] CHK030 Is the error message content specified for "no supported backend found"? [Clarity, Spec §FR-007 — lists all backends + custom config instructions]
- [x] CHK031 Are requirements defined for distinguishing "binary not found" vs "binary returned error"? [Clarity, Spec §FR-007 — two distinct messages defined]

## Acceptance Criteria Quality

- [x] CHK032 Can SC-001 be objectively measured? [Measurability, Spec §SC-001 — now specifies observable outcomes: episodes in file + status showing backend name]
- [x] CHK033 Can SC-002 be objectively verified? [Measurability, Spec §SC-002 — now defines: same JSON fields, no backend metadata; content may differ]
- [x] CHK034 Is SC-003 testable? [Measurability, Spec §SC-003 — now defines: one struct addition, zero changes outside internal/agent/]
- [x] CHK035 Is SC-004 measurable? [Measurability, Spec §SC-004 — now specifies <50ms delta, measured as time-to-first-invocation minus baseline]

## Scenario Coverage — Alternate & Exception Flows

- [x] CHK036 Are requirements defined for switching backend between commands? [Coverage, Spec §FR-010 — --backend flag is per-command; no state carried between commands]
- [x] CHK037 Are requirements defined for binary not executable (permission denied)? [Coverage, Edge Cases — OS error surfaced]
- [x] CHK038 Are requirements defined for auto-detected backend changing between runs? [Coverage, US3 scenario 4 — auto-detection runs fresh each time]
- [x] CHK039 Are recovery requirements defined for interrupted extraction? [Coverage, Edge Cases — no partial episodes written; append only after full parse]
- [x] CHK040 Are requirements defined for CI/CD environments (no TTY)? [Coverage — OpenCode `-q` flag handles this; Codex `exec` is non-interactive by design; Claude `-p` is non-interactive]

## Backward Compatibility

- [x] CHK041 Is backward compatibility explicitly stated in spec? [Completeness, Spec §FR-011 — "behavior MUST be identical to current implementation"]
- [x] CHK042 Is it specified that current CLIInvoker behavior is preserved? [Completeness, Spec §FR-011 — "same binary, same arguments, same defaults"]
- [x] CHK043 Are requirements defined for --model flag with backends that don't support it? [Consistency, Spec §FR-006 — silently ignored for OpenCode]

## Dependencies & Assumptions

- [x] CHK044 Is the assumption "all backends return stdout text containing JSON" validated? [Assumption — Assumptions section now cites official docs for all 3 backends]
- [x] CHK045 Is the OpenCode CLI argument format confirmed in spec? [Assumption, Spec §FR-004 — `opencode -p "<prompt>" -q` with source URL]
- [x] CHK046 Is the Codex CLI argument format confirmed in spec? [Assumption, Spec §FR-004 — `codex exec "<prompt>" -m <model>` with source URL]
- [x] CHK047 Is the PATH-based detection documented as a limitation? [Dependency, Assumptions — "prompt text is passed as single CLI argument; backends outside PATH handled via custom backend"]

## Notes

- All 47 items now pass after spec.md update.
- Key changes made to spec: FR-004 now includes exact CLI patterns per backend; FR-005 fully specifies custom backend template processing and edge cases; FR-007 expanded to 6 distinct error modes; FR-011 (backward compat) and FR-012 (status shows backend) added as new requirements; FR-013 (stdout/stderr handling) added; Assumptions section now cites confirmed official documentation URLs; Success criteria sharpened with measurable thresholds and observable outcomes; Edge cases section expanded with 5 additional scenarios.

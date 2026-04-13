# Research: Agent Memory Companion

**Date**: 2026-03-20
**Feature**: 001-agent-memory-companion

## R1: Go CLI Structure (Minimal Dependencies)

**Decision**: Use stdlib `os.Args` dispatch + `flag.FlagSet` per
subcommand. No external CLI framework.

**Rationale**: The CLI has 4 subcommands (`extract`, `consolidate`,
`inject`, `status`) with minimal flags each. Go's `flag` package
handles this well. Cobra/urfave-cli would add ~5k LOC of dependency
for negligible benefit at this scale.

**Alternatives considered**:
- Cobra: Feature-rich but heavy dependency. Overkill for 4 commands.
- urfave/cli: Lighter than Cobra but still an external dep.
- `os.Args` only: Too manual for flag parsing within subcommands.

**Pattern**:
```go
func main() {
    if len(os.Args) < 2 { usage(); os.Exit(1) }
    switch os.Args[1] {
    case "extract":  runExtract(os.Args[2:])
    case "consolidate": runConsolidate(os.Args[2:])
    case "inject":   runInject(os.Args[2:])
    case "status":   runStatus(os.Args[2:])
    default:         usage(); os.Exit(1)
    }
}
```

---

## R2: JSONL Episode Storage

**Decision**: Append-only JSONL using `encoding/json` +
`bufio.Scanner`. One JSON object per line.

**Rationale**: JSONL is simple, git-diffable, and requires only
stdlib. Each line is independently parseable, so corrupt lines
can be skipped without losing the entire file.

**Alternatives considered**:
- SQLite: Requires CGo or pure-Go driver (external dep). Not
  git-diffable. Violates FR-013.
- CSV: Loses type information. Tags field (array) awkward to
  represent.
- Single JSON array: Entire file must be parsed/rewritten on
  append. Concurrent append is harder.

**Schema** (per line):
```json
{
  "ts": "2026-03-20T14:32:00Z",
  "session": "abc123",
  "type": "decision",
  "summary": "Chose JSONL over SQLite — simpler git diffs",
  "tags": ["architecture", "storage"],
  "agent_id": ""
}
```

**Read pattern**: `bufio.Scanner` with `MaxScanTokenSize` set to
64KB (episodes are short, but safety margin for malformed lines).
Skip lines that fail `json.Unmarshal` (edge case: corruption).

**Write pattern**: `json.Marshal` + append newline + `os.OpenFile`
with `O_APPEND|O_WRONLY|O_CREATE`. File lock held during write.

---

## R3: File Locking for Concurrent Access

**Decision**: Use `syscall.Flock` (advisory file locking on Unix).
Create a `.memory/.lock` sentinel file.

**Rationale**: `syscall.Flock` is available in Go's stdlib
`syscall` package on Linux and macOS (the only target platforms
for Claude Code). No external dependency needed. Advisory locking
is sufficient because all writers are cooperative (this tool only).

**Alternatives considered**:
- `github.com/gofrs/flock`: Cross-platform flock wrapper. Adds
  external dep for Windows support we don't need.
- Lockfile (PID-based): Fragile if process crashes without cleanup.
  Flock is automatically released on process exit.
- No locking: Violates FR-018. Concurrent appends would interleave
  bytes.

**Pattern**:
```go
func withLock(lockPath string, fn func() error) error {
    f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
    if err != nil { return err }
    defer f.Close()
    if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
        return fmt.Errorf("acquire lock: %w", err)
    }
    defer syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
    return fn()
}
```

---

## R4: Claude CLI Invocation

**Decision**: Invoke `claude` CLI via `os/exec.Command` with prompt
passed via `--print` / `-p` flag and stdin. Model selected via
`--model` flag.

**Rationale**: The companion spec prescribes subagent invocation
via `claude -p "prompt" --model haiku`. Go's `os/exec` is the
natural way to do this. The prompt is read from a template file in
`.memory/prompts/` and the current memory context is appended.

**Alternatives considered**:
- Anthropic HTTP API directly: Requires API key management, HTTP
  client code, and `net/http` complexity. Claude CLI handles auth
  and retries.
- Hardcoded prompts in Go binary: Less flexible. Prompt iteration
  requires recompilation.

**Pattern**:
```go
func invokeAgent(model, prompt string) (string, error) {
    cmd := exec.Command("claude", "-p", prompt, "--model", model)
    out, err := cmd.CombinedOutput()
    if err != nil {
        return "", fmt.Errorf("claude %s: %w\n%s", model, err, out)
    }
    return string(out), nil
}
```

**Models by mode**:
- Extraction: `haiku` (cheap, sufficient for event identification)
- Consolidation: `sonnet` (requires analysis and synthesis)
- Injection: No LLM call (pure file assembly)

---

## R5: Markdown Parsing for Principles and Skills

**Decision**: Line-based parsing with simple heuristics. No
Markdown AST library.

**Rationale**: Principles and skills follow rigid, known formats
(headings start with `## ` or `# `, rules start with `- `). A
full Markdown parser (goldmark, blackfriday) is unnecessary and
would add an external dependency. Line-by-line processing with
`strings.HasPrefix` is sufficient and more predictable.

**Alternatives considered**:
- goldmark: Full CommonMark parser. Overkill for the simple
  heading + bullet structure we use.
- regexp: More fragile than prefix matching for this use case.

**Principles format**:
```
# Project Principles

## Architecture
- Use JSONL for append-only logs — git-diffable, no driver dep
- Memory files <= 150 lines — longer and agent ignores

## Testing
- Always add file lock for concurrent writes — prevents races
```

**Parsing**: Split by lines. Track current `##` heading. Collect
`- ` lines under each heading into a `map[string][]string`.

---

## R6: Injection Strategy

**Decision**: Assemble a plain-text context block from memory files
and write to stdout (or a temp file). No LLM call needed.

**Rationale**: Injection is a pure data assembly task — read
principles, last N episodes, and matching skills, then format them
for the agent's context window. The companion spec confirms this:
"Does not require a separate subagent. Implemented through
CLAUDE.md referencing memory files."

**Implementation options** (both supported):
1. **CLAUDE.md integration**: Append memory references to CLAUDE.md
   so the agent reads them at session start.
2. **Pre-session hook**: Run `mem inject` which outputs formatted
   context to stdout. The hook pipes this into the session.

---

## R7: Consolidation Trigger Logic

**Decision**: Track session count in a `.memory/.session-count`
file (plain integer). Check thresholds on each `mem extract` run.

**Rationale**: The companion spec defines triggers as "every 10
sessions OR when episodes.jsonl > 100 records OR manual". Session
count is a simple counter file incremented by extraction. Episode
count is derived by line count of episodes.jsonl.

**Thresholds** (configurable via environment variables):
- `MEM_SESSION_THRESHOLD=10`
- `MEM_EPISODE_THRESHOLD=100`
- `MEM_PRINCIPLES_MAX=100`
- `MEM_EPISODES_MAX=200`
- `MEM_EPISODES_KEEP=50`

---

## R8: BDD Testing Strategy (Mandatory)

**Decision**: Use Behavior-Driven Development with Go stdlib
`testing` package. BDD scenarios expressed as table-driven tests
with Given/When/Then naming. No external BDD framework.

**Rationale**: The spec already defines acceptance scenarios in
Given/When/Then format. Translating these directly into Go
table-driven tests is natural and requires zero external
dependencies. BDD ensures tests are written FIRST (Red-Green-
Refactor), satisfying the constitution's testing mandate (Principle
II). Go's table-driven test pattern maps cleanly to BDD scenarios.

**Alternatives considered**:
- goconvey: BDD framework with web UI. External dependency, heavy
  for a CLI tool.
- ginkgo/gomega: Popular BDD framework. Adds ~10k LOC dependency.
  Overkill when stdlib table-driven tests achieve the same result.
- testify: Assertion library. External dep. stdlib `t.Errorf` and
  `t.Fatalf` are sufficient.

**Pattern**:
```go
func TestEpisodeAppend_GivenValidEpisode_WhenAppended_ThenFileContainsRecord(t *testing.T) {
    // Given: a temp dir with empty episodes.jsonl
    dir := t.TempDir()
    path := filepath.Join(dir, "episodes.jsonl")
    os.WriteFile(path, nil, 0644)

    // When: a valid episode is appended
    ep := Episode{Ts: "2026-03-20T10:00:00Z", Type: "decision",
        Summary: "chose JSONL", Tags: []string{"arch"}}
    err := Append(path, ep)

    // Then: no error and file contains the episode
    if err != nil { t.Fatalf("unexpected error: %v", err) }
    episodes, _ := ReadAll(path)
    if len(episodes) != 1 { t.Fatalf("got %d episodes, want 1", len(episodes)) }
    if episodes[0].Summary != "chose JSONL" { t.Errorf("wrong summary") }
}
```

**Agent testability**: The `agent` package defines an `Invoker`
interface so runners can inject a stub during tests:
```go
type Invoker interface {
    Invoke(model, prompt string) (string, error)
}

// StubInvoker returns canned responses for testing
type StubInvoker struct{ Response string; Err error }
func (s *StubInvoker) Invoke(_, _ string) (string, error) {
    return s.Response, s.Err
}
```

---

## R2 Addendum: Dedup Algorithm Specification

**Decision**: Exact match on `(type + normalized_summary)`.
Normalization = `strings.ToLower(strings.TrimSpace(summary))`.

**Rationale**: Exact match is deterministic, requires no external
deps (no NLP/fuzzy matching), and has zero false positives. The
LLM extraction already produces concise one-sentence summaries, so
near-duplicates from the same session would have identical
phrasing. If semantic dedup is needed later, it can be added as
an enhancement using LLM-based comparison in consolidation (not
extraction).

**Alternatives considered**:
- Fuzzy string matching (Levenshtein): External dep or complex
  stdlib implementation. High false-positive risk.
- LLM-based semantic comparison: Too expensive for per-episode
  dedup during extraction. Appropriate for consolidation only.
- Hash-based (SHA of summary): Equivalent to exact match but
  less debuggable.

---

## R9: Skill Trigger Matching for Injection

**Decision**: Keyword-based matching. Compare skill trigger
keywords against recent episode tags and summary words.

**Rationale**: Injection is a read-only, non-LLM operation. Simple
keyword overlap between skill triggers and recent episode tags
provides sufficient relevance filtering without requiring an LLM
call. Skills whose trigger keywords appear in the last N episodes'
tags are surfaced; others are omitted.

**Pattern**:
```go
func MatchSkills(skills []Skill, recentTags []string) []Skill {
    tagSet := make(map[string]bool)
    for _, t := range recentTags {
        tagSet[strings.ToLower(t)] = true
    }
    var matched []Skill
    for _, s := range skills {
        for _, trigger := range s.Triggers {
            words := strings.Fields(strings.ToLower(trigger))
            for _, w := range words {
                if tagSet[w] {
                    matched = append(matched, s)
                    goto next
                }
            }
        }
    next:
    }
    return matched
}
```

If no skills match, injection falls back to listing all skills
(so the agent still has access to the full skill library).

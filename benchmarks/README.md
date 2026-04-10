# Benchmarks

Evaluation harness for `mem` against standard AI memory benchmarks.

Each benchmark is its own `main` package under `benchmarks/<name>/`.

```bash
# Build any benchmark
go build -o /tmp/longmemeval-bench ./benchmarks/longmemeval/
go build -o /tmp/locomo-bench ./benchmarks/locomo/
```

## LongMemEval (ICLR 2025)

[LongMemEval](https://github.com/xiaowu0162/LongMemEval) is the standard benchmark
for long-term memory evaluation — 500 curated questions across 6 types
(single-session-user, single-session-assistant, single-session-preference,
multi-session, knowledge-update, temporal-reasoning).

### Download the dataset

```bash
mkdir -p /tmp/longmemeval-data
cd /tmp/longmemeval-data
wget https://huggingface.co/datasets/xiaowu0162/longmemeval-cleaned/resolve/main/longmemeval_oracle.json
```

### Build and run

```bash
go build -o /tmp/longmemeval-bench ./benchmarks/longmemeval/
/tmp/longmemeval-bench /tmp/longmemeval-data/longmemeval_oracle.json
```

### What it measures

- **Recall@1 / @5 / @10** — does the correct evidence appear in the top-N results?
- **Per-type breakdown** — accuracy on each of the 6 question types
- **Timing** — total runtime, index build time, average query latency

### Current results (mem BM25, single-session drawers)

| Metric | Value |
|---|---|
| Recall@1 | 45.6% |
| **Recall@5** | **69.4%** |
| Recall@10 | 78.4% |
| Index build | 27.4s (948 sessions → 861 drawers) |
| Search total | 3.5s (500 queries) |
| Avg query latency | 7.1ms |
| **Full run** | **~31 seconds** |

#### Per-type Recall@5

| Type | R@5 |
|---|---|
| single-session-assistant | **98.2%** |
| single-session-user | 81.4% |
| knowledge-update | 78.2% |
| multi-session | 64.7% |
| single-session-preference | 56.7% |
| temporal-reasoning | 53.4% |

#### Comparison with other memory systems

| System | R@5 | Notes |
|---|---|---|
| MemPalace (ChromaDB embeddings) | 96.6% | Semantic vectors, Python |
| Mem0 | ~85% | LLM extraction |
| Zep | ~85% | LLM extraction |
| **mem (BM25, ours)** | **69.4%** | Pure Go, BM25 only, no LLM, no embeddings, no network |
| BM25 flat baseline (published) | ~70% | LongMemEval paper |

### Notes

- `mem` uses **pure BM25** retrieval with our own inverted index (SQLite-backed).
  No semantic embeddings, no external API calls, no LLM in the hot path.
- The score matches the **published BM25 baseline (~70%)** from the LongMemEval paper,
  confirming the BM25 implementation is correct.
- To reach 85%+ (Mem0/Zep level) or 96%+ (MemPalace level) you'd need semantic
  embeddings — a separate feature that would add model dependencies.

### Chunking strategy (important)

Whole-session drawers outperform chunk-based drawers on LongMemEval because:

1. **IDF dilution** — splitting 948 sessions into 20k chunks inflates the doc count,
   shrinking IDF signal and hurting ranking.
2. **Context continuity** — questions often require evidence scattered across a
   whole session; a single drawer keeps all of it together.

We tested three approaches:

| Approach | Drawers | R@5 |
|---|---|---|
| **Whole session** (current) | 861 | **69.4%** |
| Overlap chunking (800 chars, 200 overlap) | 20,041 | 42.6% |
| Exchange-pair chunks | 9,520 | 38.4% |

## LoCoMo (Snap Research)

[LoCoMo](https://github.com/snap-research/locomo) — 10 long-form conversations,
1986 QAs across 5 categories (single-hop, temporal, multi-hop, open-domain,
adversarial). Each question lists one or more evidence dia_ids
(e.g., `D1:3` = session 1, message 3).

### Download

```bash
mkdir -p /tmp/locomo-data
curl -sL https://github.com/snap-research/locomo/raw/main/data/locomo10.json \
  -o /tmp/locomo-data/locomo10.json
```

### Run

```bash
go build -o /tmp/locomo-bench ./benchmarks/locomo/
/tmp/locomo-bench /tmp/locomo-data/locomo10.json
# Optional: LOCOMO_CHUNK_SIZE=50 to tune grouping (default 50, ≈ whole session)
```

### Evaluation method

- Each conversation gets its own palace (fair — agents don't share memory across users)
- Messages chunked into groups of N (default 50 ≈ whole session); each drawer's
  `source_file` is a CSV of its constituent `dia_id`s
- A retrieval is a hit if any evidence `dia_id` appears in the drawer CSV

### Current results (mem BM25, whole-session drawers)

| Metric | Value |
|---|---|
| Recall@1 | 60.0% |
| **Recall@5** | **88.2%** |
| Recall@10 | 93.7% |
| Non-adversarial R@5 | 86.8% |
| Index build | 6.2s (5882 messages → 272 drawers) |
| Search total | 3.3s (1986 queries) |
| Avg query latency | 1.7ms |
| **Full run** | **~11 seconds** |

#### Per-category Recall@5

| Category | QAs | R@1 | R@5 |
|---|---:|---:|---:|
| adversarial | 446 | 66.6% | **93.0%** |
| open-domain | 841 | 64.9% | **92.7%** |
| temporal | 321 | 58.3% | 85.0% |
| single-hop | 282 | 46.8% | 80.5% |
| multi-hop | 96 | 31.2% | 59.4% |

### Chunk-size sweep (R@5)

| Chunk size | Drawers | R@5 |
|---:|---:|---:|
| 1 (per message) | 5880 | 46.0% |
| 3 | 2057 | 70.5% |
| 5 | 1283 | 77.1% |
| 10 | 699 | 82.4% |
| 20 | 399 | 87.2% |
| **50 (≈ whole session)** | **272** | **88.2%** |

Same IDF-dilution pattern as LongMemEval: larger drawers → stronger BM25 signals.

## ConvoMem (Salesforce, arXiv 2511.10523)

[ConvoMem](https://huggingface.co/datasets/Salesforce/ConvoMem) —
*"Why Your First 150 Conversations Don't Need RAG"*. 75,336 QA pairs
across 6 evidence categories (user, assistant_facts, changing, abstention,
preference, implicit_connection), with haystacks of 1–300 conversations.

### Download (small scales, user_evidence/1_evidence)

```bash
mkdir -p /tmp/convomem-data
BASE="https://huggingface.co/datasets/Salesforce/ConvoMem/resolve/main/core_benchmark/pre_mixed_testcases/user_evidence/1_evidence"
for i in 010 020 030 040 044; do
  curl -sL "$BASE/batched_$i.json" -o /tmp/convomem-data/batch_$i.json
done
```

Batch number → haystack size mapping is non-linear. The batches above cover
context sizes 1, 2, 3, 5, 6. Larger sizes (20–300 convs) live in
`batched_045.json`–`batched_049.json` (50MB–850MB each — not recommended for
a quick run).

### Run

```bash
go build -o /tmp/convomem-bench ./benchmarks/convomem/
/tmp/convomem-bench /tmp/convomem-data
```

### Evaluation method

- Each test case gets its own palace (`~/.mempalace/palace.db`)
- Entire conversations are indexed as whole-conv drawers (source_file = conv ID)
- A retrieval is a hit if the top-k contains any conversation marked
  `containsEvidence=true`

### Current results (mem BM25, whole-conversation drawers)

| Context size | Convs | R@1 | R@5 | R@10 | N |
|---:|---:|---:|---:|---:|---:|
| 1 | 1 | 100.0% | 100.0% | 100.0% | 413 |
| 2 | 2 | 99.9% | 99.9% | 99.9% | 826 |
| 3 | 3 | 100.0% | 100.0% | 100.0% | 1239 |
| 5 | 5 | 100.0% | 100.0% | 100.0% | 2065 |
| 6 | 6 | 100.0% | 100.0% | 100.0% | 2478 |
| **Total** | — | **100.0%** | **100.0%** | **100.0%** | **7021** |

- **Total runtime:** ~15 minutes (≈ 5m36s indexing + 9.5s searching)
- **Avg query latency:** 1.4 ms

### Why 100%

The ConvoMem paper's thesis is precisely that **BM25 alone suffices at small
haystacks** — which is exactly what we measure here. Evidence questions use
specific, rare vocabulary ("trivia category", "1980s one-hit wonders") that
BM25 separates cleanly from filler conversations on unrelated topics. The
harder regime is 50–300 conversations with `changing_evidence` (latest-value
tracking), which BM25 cannot solve without temporal signals. Running that
level would require downloading the ≥100MB batches.

## Future benchmarks

- [ ] **ConvoMem changing_evidence** — latest-value tracking (BM25 alone should fail)
- [ ] **ConvoMem at scale (50–300 convs)** — requires large batch downloads
- [ ] **MemoryBench** (Supermemory) — unified runner for cross-provider comparison

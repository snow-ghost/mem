# Benchmarks

Evaluation harness for `mem` against standard AI memory benchmarks.

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
go build -o /tmp/longmemeval-bench ./benchmarks/
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

## Future benchmarks

- [ ] **LoCoMo** (Snap Research) — 10 long-term conversations, multi-hop reasoning
- [ ] **ConvoMem** (Salesforce) — 75k QA pairs, 6 evidence categories
- [ ] **MemoryBench** (Supermemory) — unified runner for cross-provider comparison

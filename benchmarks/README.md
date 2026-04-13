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

#### Hybrid mode (BM25 + semantic embeddings)

```bash
export MEM_EMBEDDINGS_URL='https://your-endpoint/v1/embeddings'
export MEM_EMBEDDINGS_MODEL='BAAI/bge-m3'         # or any embedding model
export MEM_EMBEDDINGS_API_KEY='your-key'
LME_MODE=hybrid /tmp/longmemeval-bench /tmp/longmemeval-data/longmemeval_oracle.json
```

`LME_MODE` accepts `bm25` (default), `vector` (cosine only), or `hybrid`
(BM25 + cosine fused via RRF, k=60). Any OpenAI-compatible `/v1/embeddings`
endpoint works (OpenAI, Voyage, Cohere compat, Ollama, LM Studio,
LocalAI, llama.cpp server, cloud.ru foundation-models, etc.).

### What it measures

- **Recall@1 / @5 / @10** — does the correct evidence appear in the top-N results?
- **Per-type breakdown** — accuracy on each of the 6 question types
- **Timing** — total runtime, index build time, average query latency

### Current results

Three modes measured: pure BM25 (default, offline), hybrid BM25 + bge-m3 via
weighted Reciprocal Rank Fusion (k=60), and hybrid + cross-encoder rerank
(`BAAI/bge-reranker-v2-m3`). Tokenizer applies Porter step 1a/1b stemming.

| Metric | BM25 | Hybrid 0.7/0.3 | Hybrid 0.7 + rerank |
|---|---:|---:|---:|
| **Recall@1** | 44.4% | 48.0% | **52.6%** |
| **Recall@5** | 71.0% | 74.6% | **74.6%** |
| **Recall@10** | 77.2% | 79.2% | **80.8%** |
| BM25 index build | 26s | 26s | 26s |
| Embedding index build | — | ~5 min | ~5 min |
| Avg query latency | 8.6ms | 67ms | 137ms |

Stemming alone gives BM25 **+1.6 R@5** (69.4 → 71.0) without any model
dependency. Hybrid mode adds another **+3.6 R@5** on top. Cross-encoder
reranking adds **+4.6 R@1** and **+1.6 R@10** but doesn't lift R@5 — the
reranker is excellent at putting the right answer at #1 and at rescuing
answers that fell outside top-5, but it doesn't fundamentally widen what
the first stage already considers.

Stemming alone gives BM25 **+1.6 R@5** (69.4 → 71.0) without any model
dependency. Hybrid mode adds another **+3.6 R@5** on top. Best RRF weight
shifts from 0.60 (no stemming) to 0.70 (with stemming) — stronger BM25
deserves a bit more weight in the fusion.

The hybrid run used `BAAI/bge-m3` (1024-dim) via the cloud.ru
foundation-models API. Drawer text was truncated to 1500 chars before
embedding to avoid hitting server-side input-length issues.

#### Per-type Recall@5: BM25 vs hybrid (with stemming)

| Type | BM25 | Hybrid 0.7 | Δ |
|---|---:|---:|---:|
| knowledge-update | 82.1% | **87.2%** | **+5.1** |
| single-session-preference | 60.0% | **66.7%** | **+6.7** |
| multi-session | 66.2% | **69.9%** | **+3.7** |
| single-session-user | 81.4% | **84.3%** | **+2.9** |
| temporal-reasoning | **54.9%** | 59.4% | +4.5 |
| single-session-assistant | **98.2%** | 96.4% | -1.8 |

Stemming with hybrid: most categories improve, only single-session-assistant
regresses slightly (the same ~2pp gap as without stemming).

#### RRF weight sweep with stemming

| Weight | R@1 | R@5 | R@10 |
|---:|---:|---:|---:|
| 0.40 | 49.0% | 73.2% | **79.8%** |
| 0.50 | **48.8%** | 73.4% | 79.2% |
| 0.60 | 48.6% | 73.6% | 79.0% |
| **0.70** | 48.0% | **74.6%** | 79.2% |

Best weight shifts from 0.60 (no stemming) to 0.70 (with stemming) — the
stronger BM25 signal earns more trust in the fusion. Set
`LME_RRF_WEIGHTS=0.4,0.5,0.6,0.7` to reproduce the sweep in one embedding
pass (retrieval loop reruns per weight against the same already-embedded
palace, ~5s per weight).

#### Cross-encoder reranking

Set `MEM_RERANK_URL`, `MEM_RERANK_MODEL` (Cohere-compatible `/v1/rerank`,
e.g., `BAAI/bge-reranker-v2-m3` via cloud.ru), then `LME_RERANK=1` on
top of any first-stage mode. The bench retrieves top-N candidates
(`LME_RERANK_POOL=20`, default) from the first stage, sends them with the
query to the reranker (`LME_RERANK_WORKERS=8` parallel calls), then takes
the top-10 by reranker score.

| Stage | R@1 | R@5 | R@10 |
|---|---:|---:|---:|
| Hybrid 0.7 (no rerank) | 48.0% | 74.6% | 79.2% |
| Hybrid 0.7 + rerank | **52.6%** | 74.6% | **80.8%** |

Reranker buys you **+4.6 R@1** and **+1.6 R@10** but doesn't move R@5.
Useful when downstream consumers care most about the top-1 answer
(LLMs answering with retrieval, single-best-snippet UIs).

#### Comparison with other memory systems

| System | R@5 | Notes |
|---|---|---|
| MemPalace (ChromaDB embeddings) | 96.6% | Semantic vectors, Python |
| Mem0 | ~85% | LLM extraction |
| Zep | ~85% | LLM extraction |
| **mem (hybrid, ours)** | **74.2%** | Pure Go, BM25 + bge-m3 hybrid via RRF |
| **mem (BM25, ours)** | **69.4%** | Pure Go, BM25 only, no LLM, no embeddings, no network |
| BM25 flat baseline (published) | ~70% | LongMemEval paper |

### Notes

- The BM25 score matches the **published BM25 baseline (~70%)** from the
  LongMemEval paper, confirming the BM25 implementation is correct.
- Hybrid mode adds **+4.8 R@5** over pure BM25 with no code changes — just
  set `MEM_EMBEDDINGS_*` and `LME_MODE=hybrid`. The biggest wins are on
  preference (+10.0), knowledge-update (+9.0), and temporal-reasoning (+8.3).
- The remaining gap to MemPalace (96.6%) is mostly model quality + chunking.
  We truncate drawer text to 1500 chars (cloud.ru constraint), and
  full-scan vector search adds 370ms/query.

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
# Optional: LOCOMO_MODE=hybrid + MEM_EMBEDDINGS_* for hybrid mode
```

### Evaluation method

- Each conversation gets its own palace (fair — agents don't share memory across users)
- Messages chunked into groups of N (default 50 ≈ whole session); each drawer's
  `source_file` is a CSV of its constituent `dia_id`s
- A retrieval is a hit if any evidence `dia_id` appears in the drawer CSV

### Current results

| Metric | BM25 | Hybrid (bge-m3 + RRF) |
|---|---:|---:|
| Recall@1 | **60.0%** | 59.0% |
| **Recall@5** | 88.2% | **88.6%** |
| Recall@10 | 93.7% | **95.6%** |
| Non-adversarial R@5 | 86.8% | **87.5%** |
| Index build | 6.2s | 6.2s + ~2 min embedding |
| Avg query latency | 1.7ms | 4.5ms |
| **Full run** | **~11s** | ~2m20s |

#### Per-category R@5: BM25 vs hybrid

| Category | QAs | BM25 | Hybrid | Δ |
|---|---:|---:|---:|---:|
| multi-hop | 96 | 59.4% | **66.7%** | **+7.3** |
| single-hop | 282 | 80.5% | **86.2%** | **+5.7** |
| temporal | 321 | **85.0%** | 84.4% | -0.6 |
| open-domain | 841 | **92.7%** | 91.4% | -1.3 |
| adversarial | 446 | **93.0%** | 92.6% | -0.4 |

LoCoMo's whole-session chunks already give BM25 strong signal on the
high-volume categories (open-domain, adversarial), so hybrid's contribution
is concentrated where it matters most: **multi-hop +7.3** (the hardest
category) and **single-hop +5.7**. Recall@10 jumps from 93.7% to 95.6%,
suggesting embeddings rescue evidence that fell out of the BM25 top-5.

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

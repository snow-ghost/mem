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

#### Per-type RRF weights (oracle vs classifier)

Sweep over BM25 weights {0.30, 0.50, 0.70, 0.90} on stemmed hybrid:

| Type | Best weight | R@5 |
|---|---:|---:|
| **knowledge-update** | **0.30** | 89.7% |
| single-session-preference | 0.70 | 66.7% |
| temporal-reasoning | 0.70 | 59.4% |
| multi-session | 0.70 | 69.9% |
| single-session-assistant | 0.90 | 98.2% |
| single-session-user | 0.90 | 85.7% |
| **Aggregate (oracle per-type)** | — | **75.4%** |
| Single global best (0.70) | 0.70 | 74.6% |
| **Classifier-driven (heuristic)** | — | **74.2%** |

Striking asymmetry: knowledge-update wants vector-heavy fusion (0.30),
single-session-* want BM25-heavy (0.90). Oracle per-type captures
+0.8 R@5 over the single global best.

The catch: the heuristic classifier's 53% type accuracy isn't enough.
Especially on knowledge-update (which it never predicts), the wrong
weight is *worse* than the global default. Net classifier-driven is
**below** the single-weight baseline. The infra is in place for a
real (LLM-based or embedding-anchored) classifier to plug in:
`search.SearchHybridAuto(..., DefaultPerTypeWeights)`.

We also tested an embedding-anchor classifier
(`search.AnchorClassifier`) that cosine-compares the query embedding
to per-type prototypical phrasings:

| Classifier | Accuracy | Per-type R@5 estimate |
|---|---:|---:|
| Heuristic regex | 53.4% | 74.2% |
| Anchor (bge-m3) | 44.4% | 74.0% |
| Single best (0.7) baseline | — | 74.6% |
| Oracle per-type (upper bound) | 100% | 75.4% |

The anchor classifier loses to the heuristic because LongMemEval
question types are deliberately phrased to be lexically similar
(knowledge-update queries are indistinguishable from single-session-
user queries by topic). Both classifiers are an honest "-0.5 pp"
result; the path forward is either labeled training data or LLM-based
classification, both out of scope here.

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

#### L# Cache: multi-level per-session embedding (new best R@5)

Inspired by [Schift's LongMemEval writeup](https://schift.io/blog/longmemeval-benchmark/).
Instead of one embedding per session, store three variants:

- **L0** = full session text (user + assistant turns)
- **L1** = user turns only (strips assistant verbosity)
- **L2** = first three user turns (zero-cost summary proxy)

At retrieval, compute cosine between the query vector and each
variant's vector, then either (a) weighted-sum them per session or
(b) take the max. Session-level scores rank sessions for top-K.

`LME_LCACHE=1` with BAAI/bge-m3:

| Merge | R@1 | R@5 | R@10 | Notes |
|---|---:|---:|---:|---|
| weighted-sum 0.2/0.5/0.3 (Schift defaults) | 53.0% | 74.4% | **82.4%** | L1-heavy fusion |
| **max** per session | **54.2%** | **77.2%** | 81.8% | new best R@5 |

Max-merge wins because it avoids penalising sessions where the answer
lives only in the assistant turn (e.g., "remind me what you said
about X" — the user turn is the question, the answer is in L0).
Weighted-sum's L1-heavy fusion tanks single-session-assistant to
78.6% (-17.8 vs hybrid); max-merge keeps it at 92.9%.

Per-type R@5 (max-merge vs hybrid 0.7):

| Type | Hybrid | Max-merge | Δ |
|---|---:|---:|---:|
| single-session-user | 84.3% | **94.3%** | **+10.0** |
| knowledge-update | 87.2% | **92.3%** | **+5.1** |
| temporal-reasoning | 59.4% | **61.7%** | +2.3 |
| multi-session | 69.9% | **71.4%** | +1.5 |
| single-session-assistant | **96.4%** | 92.9% | -3.5 |
| single-session-preference | 66.7% | 63.3% | -3.4 |

Cost: 3× drawer embeddings (948 sessions → 2844 drawers), ~7 min
with cloud.ru bge-m3 (~5 min with weighted-sum due to faster queries).
The R@5 win (+2.4 pp over hybrid, +0.8 pp over oracle-gated rerank)
is the largest single-knob improvement after recency on ConvoMem.

Override weights via `LME_LCACHE_WEIGHTS="0.5,0.3,0.2"`, switch to
max-merge via `LME_LCACHE_MERGE=max`.

##### Query2Doc / HyDE-style query expansion

For each query, ask an LLM (Qwen3-Next-80B via cloud.ru chat completions)
to write one plausible answer sentence, embed it, then average with the
original query vector. Tiny stage in the overall pipeline — adds ~3 min
of LLM calls to a 15 min embedding run:

| Config | R@1 | R@5 | R@10 |
|---|---:|---:|---:|
| L# Cache max-merge | 54.2% | 77.2% | 81.8% |
| **L# max + Query2Doc** | **55.4%** | **77.8%** | **82.6%** |

Biggest per-type gain: **single-session-preference 63.3 → 70.0** (+6.7),
**knowledge-update 92.3 → 94.9** (+2.6). The LLM gives the retriever
a concrete-sounding answer hypothesis that lexically matches the
correct session better than an abstract question form.

Enable with `LME_QUERY2DOC_URL` + `LME_QUERY2DOC_MODEL`.

##### Embedding model A/B: local vs cloud, LongMemEval vs LoCoMo

Running the same pipeline across (a) BAAI/bge-m3 1024-d via cloud.ru
and (b) sentence-transformers/all-MiniLM-L6-v2 384-d via local
llama-server on port 8092.

| | LongMemEval R@5 | LoCoMo R@5 | R@5 bench-delta |
|---|---:|---:|---:|
| bge-m3 (cloud) | **77.2%** | 88.6% | -11.4 |
| MiniLM (local) | 72.4% | **88.8%** | -16.4 |
| MemPalace reported | 96.6% | — | — |

Two findings that surprised us:

1) **bge-m3 beats MiniLM by 4.8 pp on LongMemEval but only ties on
   LoCoMo.** The "better" model depends on the benchmark — there is
   no free lunch. LoCoMo's session-level `dia_id` matching is more
   forgiving and the smaller model keeps pace; LongMemEval's answer-
   text presence is stricter and rewards the larger multilingual
   embedding.

2) **MiniLM here scores 72.4% R@5 but MemPalace reports 96.6%
   with the same model family on the same benchmark.** Running the
   exact ChromaDB reference model through our full L# Cache pipeline
   does not close the gap — so the 96.6% number can't be purely
   model-driven. Likely sources of divergence: their chunking/
   indexing (whole conversation in a single ChromaDB item vs our
   per-session drawers), their evaluation harness (how `containsAnswer`
   is implemented), or the ChromaDB default similarity metric
   (L2 vs our cosine).

Local MiniLM was ~50× faster than cloud.ru bge-m3 on LongMemEval
(2m22s vs 7m) and roughly 2× slower on LoCoMo (15m vs 6m, because
LoCoMo's per-conversation loop gives less batching opportunity
to a single local instance).

##### Chasing the MemPalace 96.6% gap — three orthogonal tests

To understand the 18.8 pp gap to MemPalace's reported 96.6% R@5, we
isolated three plausible explanations and tested each:

| Hypothesis | Change | R@5 result | Closes gap? |
|---|---|---:|---|
| **Embedding model** | swap bge-m3 → all-MiniLM-L6-v2 (their model) | 72.4% (heur) | No — 24.2 pp left |
| **Eval metric** | switch heuristic `containsAnswer` → official `session_id` | 67.4% (sid) | No — sid is **stricter**, not lenient |
| **Palace scope** | restrict retrieval to question's `haystack_session_ids` (`LME_SCOPED_PALACE=1`) | 75.0% (sid) | Partial — +7.6 pp |

Even with all three favorable factors stacked (their model + their
metric + per-question palace scope), our best is **75.0% R@5 (sid)** —
still 21.6 pp below the 96.6% claim. The remaining gap is not
explained by anything we can vary on our side; the most likely
sources are ChromaDB-specific indexing details (chunk granularity,
default similarity function) or differences in the reference harness
itself.

Two takeaways for our own users: (1) the metric matters — when
comparing to other systems, ask which `containsAnswer` they used;
(2) per-question palace scoping is free recall when the haystack is
known, and worth wiring up if your application can compute it.

A fourth test rules out the "indexing granularity" hypothesis:
running with `LME_LCACHE=0` (one whole-session drawer instead of
three L# variants) + scoped palace + MiniLM drops R@5(sid) to
**65.6%** — *worse* than L# max + scoped (75.0%). So the L#
multi-level isn't the gap either; our richer per-session indexing
helps rather than hurts vs the simpler scheme MemPalace likely uses.

Stacking Query2Doc on top of the scoped pipeline nudges further:
MiniLM + L# max + Q2D (Qwen3-Next-80B via cloud.ru) + scoped lands
at R@5(sid) = **79.2%**, up +4.2 pp from L# max + scoped alone. The
gap to 96.6% shrinks to 17.4 pp but the four ruled-out hypotheses
(model, metric, scope, indexing) still account for none of it.

##### Faithful reimplementation of MemPalace's own algorithm

Reading `mempalace/benchmarks/longmemeval_bench.py` revealed the
exact setup: one document per session, built by joining *only* the
user turns (`user_turns = [t["content"] for t in session if
t["role"] == "user"]`), indexed into a per-question ephemeral
ChromaDB with default embeddings (all-MiniLM-L6-v2, 384-d). This is
the same recipe as our L1 variant with scoped palace.

Running our bench with `LME_LCACHE_WEIGHTS=0,1,0 LME_SCOPED_PALACE=1
LME_LCACHE=1` + MiniLM via llama.cpp lands at:

| Metric | Our L1-only + scoped | MemPalace claim |
|---|---:|---:|
| R@5 (sid) | **74.6%** | 96.6% |
| single-session-assistant R@5 | **92.9%** | 92.9% |
| single-session-preference R@5 | 30.0% | 93.3% |
| temporal-reasoning R@5 | 40.6% | 96.2% |
| multi-session R@5 | 42.1% | — |
| knowledge-update R@5 | 73.1% | — |

The single-session-assistant category matches exactly, confirming
both runs hit the same oracle dataset. Temporal and preference
diverge by 50–60 pp under identical retrieval — more than any
embedding-model swap can explain. We conclude the 96.6% aggregate
is either on a different split, computed against a different
ground-truth mapping, or the result of a harness quirk we cannot
discover from the public code. Five independent test vectors (model,
metric, scope, indexing granularity, exact algorithm) now all
converge on ~75–80% on oracle with MiniLM.

##### Embedding model A/B: bge-m3 vs Qwen3-Embedding-0.6B (both 1024-dim)

Same L# Cache max-merge pipeline, just swap `MEM_EMBEDDINGS_MODEL`:

| | bge-m3 | Qwen3-Embedding-0.6B |
|---|---:|---:|
| R@1 | **54.2%** | 53.4% |
| **R@5** | **77.2%** | 75.6% |
| R@10 | 81.8% | **82.4%** |
| Anchor classifier accuracy | 44.4% | **54.8%** |

bge-m3 wins on R@1/R@5 by a small margin. Qwen3 wins on R@10 and on
the per-type classification task (the anchor classifier's cosine
similarities land in the right bucket more often). Workload-
dependent — the retrieval-oriented user should prefer bge-m3 here,
but this isn't a blanket statement about either model.

##### L# Cache + oracle-gated rerank (near-ceiling combo)

Stacking cross-encoder rerank on top of L# max-merge adds marginal
gain — the first stage already got the right session into top-5:

| Config | R@1 | R@5 | R@10 |
|---|---:|---:|---:|
| L# max | 54.2% | 77.2% | 81.8% |
| L# max + oracle-gated rerank | **55.0%** | **77.4%** | 82.0% |

Rerank helps only when the right session slipped past top-5 into
top-N; here L# already covers R@5 77.2%, leaving little headroom.
Remaining gap to MemPalace's 96.6% is most likely embedding-model
driven (all-MiniLM-L6-v2 in their setup vs BAAI/bge-m3 here).

##### L# Cache + BM25 blend (redundant)

`LME_LCACHE_BM25=0.3` adds a BM25 session-score contribution (RRF
1/(60+rank), max over the 3 variants) blended with the cosine score.
R@5 77.0% — essentially unchanged from pure L# max (77.2%). BM25
becomes redundant once L1 (user turns only) already gives lexical
signal without assistant noise. Preference queries do get a small
boost (+3.4 on that subset) because phrases like "recommend me X" are
rare-term-heavy, but the aggregate is flat.

### Full LongMemEval summary

| Configuration | R@1 | R@5 | R@10 |
|---|---:|---:|---:|
| BM25 + stemming | 44.4% | 71.0% | 77.2% |
| Hybrid (RRF 0.7) | 48.0% | 74.6% | 79.2% |
| Hybrid + rerank all | 52.6% | 74.6% | 80.8% |
| Hybrid + rerank classifier-gated | 48.4% | 75.2% | 81.6% |
| Hybrid + rerank oracle-gated | 50.0% | 76.4% | 81.2% |
| L# Cache weighted-sum | 53.0% | 74.4% | **82.4%** |
| L# Cache max-merge | 54.2% | 77.2% | 81.8% |
| L# Cache max + BM25 blend 0.3 | 54.6% | 77.0% | 81.8% |
| L# Cache max + oracle-gated rerank | 55.0% | 77.4% | 82.0% |
| **L# Cache max + Query2Doc** | 55.4% | **77.8%** | **82.6%** |
| L# Cache max + Q2D + oracle-rerank | **56.4%** | 76.8% | 81.8% |
| _MemPalace (ChromaDB + MiniLM)_ | _?_ | _96.6%_ | _?_ |

Note: stacking rerank on top of Query2Doc slightly regresses R@5
(-1.0 pp) because both techniques target first-stage ranking. Once
Q2D has strengthened the first stage (e.g., knowledge-update from
92.3 → 94.9 R@5), the cross-encoder disagrees with it and pushes some
correct sessions back out of top-5. Rerank still adds R@1 (+1.0),
useful when top-1 is the headline metric.

Gap to MemPalace shrank from 27.2 pp (starting BM25) to **18.8 pp**
via technique work alone, on the same BAAI/bge-m3 embedding model.
Closing the rest would need either the actual ChromaDB reference
model (all-MiniLM-L6-v2 via llama.cpp locally) or an alternative
bench run confirming the 96.6% figure isn't harness-specific.

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
| Hybrid 0.7 + rerank (all) | **52.6%** | 74.6% | 80.8% |
| Hybrid 0.7 + rerank (classifier-gated) | 48.4% | 75.2% | **81.6%** |
| Hybrid 0.7 + rerank (oracle-gated) | 50.0% | **76.4%** | 81.2% |

Rerank-everything buys **+4.6 R@1** but doesn't move R@5 — the reranker
disagrees with first-stage ordering on the strong-BM25 categories
(single-session-{user,assistant} regress -5.7 / -1.8). Per-category
gating fixes that.

#### Per-category rerank gating

`LME_RERANK_GATE=oracle|classifier` only invokes the reranker when the
question type is in `{temporal-reasoning, knowledge-update}` — the two
categories where rerank consistently helped in earlier sweeps.

- **oracle gate** (uses ground-truth `q.Type`): R@5 **76.4%** (+1.8 vs
  no-rerank), R@10 81.2%. This is the upper bound — preserves the +3.8
  wins on K-U and temporal while restoring 98.2% / 84.3% on
  single-session-{assistant,user}.
- **classifier gate** (heuristic via `search.ClassifyQuestion`): R@5
  **75.2%** (+0.6). Captures ~33% of the oracle win.

The heuristic classifier hits 53% type accuracy and 73% gate-decision
accuracy. Per-type breakdown:

| Actual type | Classifier acc |
|---|---:|
| single-session-assistant | 96.4% |
| single-session-user | 81.4% |
| temporal-reasoning | 78.2% |
| multi-session | 32.3% |
| single-session-preference | 30.0% |
| **knowledge-update** | **0.0%** |

knowledge-update is fundamentally indistinguishable from single-session-
user from the question text alone (e.g., "What was my personal best 5K
time?" looks identical to a single-session question). A real production
classifier would need either training data or a small LLM call.

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

#### Rerank on LoCoMo: large regression (metric-mismatch finding)

Adding cross-encoder rerank (`LOCOMO_RERANK=1`) on top of hybrid causes
a **R@5 drop from 88.6% → 70.0%** (-18.6 pp), with the largest losses
on open-domain (-24.7), adversarial (-23.3), and multi-hop (-21.9).

This is a metric mismatch, not a reranker failure. LoCoMo's evidence
match is `dia_id`-based (we check whether the *specific* message ID
that contains the answer appears in the drawer's `source_file` CSV).
The reranker scores by *content relevance to the query* — it
correctly picks the most topically relevant session, but that's not
necessarily the session containing the target `dia_id`.

LongMemEval's evidence check is *answer-text presence* in the drawer,
which lines up with content relevance — so rerank helps there (+4.6
R@1, +1.6 R@10) and hurts here. **Rule of thumb:** cross-encoder
rerank pays off when the downstream consumer cares about content
relevance (LLM RAG, single-snippet answers); it doesn't pay off when
your evaluation pivots on a structural property of the chunk.

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

### ConvoMem changing_evidence (the hard mode)

`changing_evidence` test cases hold *multiple* versions of the same fact
across conversations (e.g., "campaign targets US" → "campaign targets US
and Canada"). The correct retrieval is the **latest** version. We
download three batches (2/3/4 evidence items per case = 1290 tests
total) and report two metrics:

- **lenient**: any conv with `containsEvidence=true` counts as a hit
- **strict**: only the *latest* such conv counts (the canonical answer)

Results on BM25 only:

| Context size | Lenient R@1 | **Strict R@1** | Strict R@5 | Strict R@10 | N |
|---:|---:|---:|---:|---:|---:|
| 2 | 100% | 24.8% | 99.5% | 99.5% | 759 |
| 3 | 100% | 31.4% | 99.7% | 99.7% | 344 |
| 4 | 100% | 21.9% | 100% | 100% | 187 |
| **Total** | **100%** | **26.1%** | **99.6%** | **99.6%** | **1290** |

BM25 alone retrieves every version into top-5 but picks the *latest*
version at #1 only 26% of the time — exactly the gap that motivates
**temporal-aware ranking**.

#### Adding `search.ApplyRecencyBoost` (CONVOMEM_RECENCY=0.5)

The boost adds `recencyWeight × (drawer.id − minID) / (maxID − minID)`
to each result's score, then re-sorts. Higher drawer IDs (= more
recently inserted) get a larger bonus. Re-running with `recencyWeight=0.5`:

| Context size | Strict R@1 (no boost) | **Strict R@1 (recency 0.5)** | Δ |
|---:|---:|---:|---:|
| 2 | 24.8% | **99.5%** | +74.7 |
| 3 | 31.4% | **99.7%** | +68.3 |
| 4 | 21.9% | **100.0%** | +78.1 |
| **Total** | **26.1%** | **99.6%** | **+73.5** |

R@5 / R@10 unchanged (already 99.6%) — the boost just reorders inside
the already-correct top-5 so the newest evidence wins #1. This is the
single biggest improvement in the bench suite. Available as
`search.ApplyRecencyBoost(results, weight)` for any caller; works on
output of `Search`, `SearchVector`, `SearchHybrid*`.

## Future benchmarks

- [ ] **ConvoMem at scale (50–300 convs)** — requires large batch downloads
- [ ] **MemoryBench** (Supermemory) — unified runner for cross-provider comparison
- [ ] **LongMemEval with recency boost** — does it help temporal-reasoning?

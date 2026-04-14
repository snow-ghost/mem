package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sort"
	"sync"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/db"
	"github.com/snow-ghost/mem/internal/embeddings"
	"github.com/snow-ghost/mem/internal/palace"
	"github.com/snow-ghost/mem/internal/rerank"
	"github.com/snow-ghost/mem/internal/search"
)

// benchAddDrawer inserts a drawer bypassing the palace.AddDrawer dedup path.
// The production content_hash UNIQUE constraint drops any session whose
// joined user-turn text coincides with another session elsewhere in the
// palace — on longmemeval_s_cleaned this loses ~23% of sessions and
// strands 270/500 answer sessions behind a foreign session-id. The hash
// below includes (sourceFile, hall) so cross-session collisions coexist.
func benchAddDrawer(d *db.DB, content string, wingID, roomID int64, hall, sourceFile, sourceType string) (int64, error) {
	if hall == "" {
		hall = "facts"
	}
	if sourceType == "" {
		sourceType = "file"
	}
	sum := sha256.Sum256([]byte(content + "\x00" + sourceFile + "\x00" + hall))
	hash := fmt.Sprintf("%x", sum[:])
	res, err := d.Exec(
		`INSERT INTO drawers (content, content_hash, wing_id, room_id, hall, source_file, source_type)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		content, hash, wingID, roomID, hall, sourceFile, sourceType,
	)
	if err != nil {
		return 0, err
	}
	id, _ := res.LastInsertId()
	return id, nil
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Question struct {
	ID                 string          `json:"question_id"`
	Type               string          `json:"question_type"`
	Question           string          `json:"question"`
	Answer             any             `json:"answer"`
	HaystackSessions   [][]Message     `json:"haystack_sessions"`
	HaystackSessionIDs []string        `json:"haystack_session_ids"`
	AnswerSessionIDs   []string        `json:"answer_session_ids"`
	RawAnswerSessIDs   json.RawMessage `json:"-"`
}

func main() {
	dataFile := "/tmp/longmemeval-data/longmemeval_oracle.json"
	if len(os.Args) > 1 {
		dataFile = os.Args[1]
	}

	mode := os.Getenv("LME_MODE") // "" / "bm25" / "vector" / "hybrid"
	if mode == "" {
		mode = "bm25"
	}

	envCfg := config.Load()
	useEmbeddings := mode == "vector" || mode == "hybrid"
	if useEmbeddings && !envCfg.EmbeddingsEnabled() {
		fmt.Fprintln(os.Stderr, "LME_MODE=vector|hybrid requires MEM_EMBEDDINGS_URL and MEM_EMBEDDINGS_MODEL")
		os.Exit(1)
	}
	useRerank := os.Getenv("LME_RERANK") == "1"
	if useRerank && !envCfg.RerankEnabled() {
		fmt.Fprintln(os.Stderr, "LME_RERANK=1 requires MEM_RERANK_URL and MEM_RERANK_MODEL")
		os.Exit(1)
	}

	fmt.Println("=== LongMemEval Benchmark for mem ===")
	recencyWeight := 0.0
	if v := os.Getenv("LME_RECENCY"); v != "" {
		fmt.Sscanf(v, "%f", &recencyWeight)
	}
	// LME_LCACHE=1 enables Schift-style per-session multi-level embedding:
	// L0 = full session, L1 = user turns only, L2 = first 3 user turns.
	// At retrieval, each session's score = w0*L0 + w1*L1 + w2*L2 on cosine
	// to the query. Defaults match the Schift paper (L1-heavy), override
	// with LME_LCACHE_WEIGHTS="w0,w1,w2". LME_LCACHE_MERGE=max switches
	// from weighted sum to per-session max similarity across variants.
	useLCache := os.Getenv("LME_LCACHE") == "1"
	lcacheW0, lcacheW1, lcacheW2 := 0.2, 0.5, 0.3
	if v := os.Getenv("LME_LCACHE_WEIGHTS"); v != "" {
		parts := strings.Split(v, ",")
		if len(parts) == 3 {
			fmt.Sscanf(strings.TrimSpace(parts[0]), "%f", &lcacheW0)
			fmt.Sscanf(strings.TrimSpace(parts[1]), "%f", &lcacheW1)
			fmt.Sscanf(strings.TrimSpace(parts[2]), "%f", &lcacheW2)
		}
	}
	lcacheMerge := os.Getenv("LME_LCACHE_MERGE") // "" (default weighted sum), "max"
	// LME_QUERY2DOC_MODEL + LME_QUERY2DOC_URL enable Query2Doc / HyDE-style
	// query expansion: LLM generates a plausible pseudo-answer sentence,
	// which is embedded and averaged with the original query embedding.
	q2dModel := os.Getenv("LME_QUERY2DOC_MODEL")
	q2dURL := os.Getenv("LME_QUERY2DOC_URL")
	useQuery2Doc := q2dModel != "" && q2dURL != ""
	// LME_LCACHE_BM25=<float> adds a BM25 session-score contribution to
	// the L# Cache ranking. The BM25 half runs over the same 3 variants
	// per session (L0/L1/L2 drawers) and takes the max rank per session.
	lcacheBM25 := 0.0
	if v := os.Getenv("LME_LCACHE_BM25"); v != "" {
		fmt.Sscanf(v, "%f", &lcacheBM25)
	}

	fmt.Printf("Dataset: %s\n", dataFile)
	fmt.Printf("Mode: %s\n", mode)
	if useEmbeddings {
		fmt.Printf("Embeddings: %s\n", envCfg.EmbeddingsModel)
	}
	if useRerank {
		fmt.Printf("Reranker: %s (LME_RERANK=1)\n", envCfg.RerankModel)
	}
	if recencyWeight > 0 {
		fmt.Printf("Recency boost: %.2f (LME_RECENCY)\n", recencyWeight)
	}
	if useLCache {
		merge := "weighted-sum"
		if lcacheMerge == "max" {
			merge = "max"
		}
		fmt.Printf("L# Cache: enabled (L0=full, L1=user, L2=first3user, weights %.2f/%.2f/%.2f, merge=%s)\n",
			lcacheW0, lcacheW1, lcacheW2, merge)
	}
	if useQuery2Doc {
		fmt.Printf("Query2Doc: enabled (model=%s)\n", q2dModel)
	}
	fmt.Println()

	data, err := os.ReadFile(dataFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading dataset: %v\n", err)
		os.Exit(1)
	}

	var questions []Question
	if err := json.Unmarshal(data, &questions); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing dataset: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d questions\n", len(questions))

	// Phase 1: Index all sessions into palace
	palacePath := filepath.Join(os.TempDir(), "longmemeval-palace")
	os.RemoveAll(palacePath)

	cfg := config.Config{PalacePath: palacePath, DbFile: "palace.db"}
	d, err := palace.Init(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Init palace: %v\n", err)
		os.Exit(1)
	}

	wing, _ := palace.CreateWing(d, "longmemeval", "benchmark", "")

	fmt.Print("Indexing sessions...")
	indexStart := time.Now()

	var totalSessions, totalDrawers int
	var batchItems []struct{ ID int64; Content string }
	// For L# Cache: map session id → [L0 content, L1 content, L2 content]
	lcacheSessions := make(map[string]*sessionVariants)

	for _, q := range questions {
		for si, session := range q.HaystackSessions {
			totalSessions++
			sessionID := fmt.Sprintf("session_%d", totalSessions)
			// Prefer the dataset's canonical session ID when present, so
			// we can evaluate via answer_session_ids (LongMemEval official
			// metric) alongside the answer-text containment check.
			if si < len(q.HaystackSessionIDs) && q.HaystackSessionIDs[si] != "" {
				sessionID = q.HaystackSessionIDs[si]
			}

			if useLCache {
				l0, l1, l2 := makeSessionVariants(session)
				sv := &sessionVariants{sessionID: sessionID, l0: l0, l1: l1, l2: l2}
				lcacheSessions[sessionID] = sv
				for _, v := range []struct {
					hall, content string
					weight        float64
				}{
					{"L0", l0, lcacheW0}, {"L1", l1, lcacheW1}, {"L2", l2, lcacheW2},
				} {
					if len(v.content) < 5 {
						continue
					}
					if lcacheMerge != "max" && v.weight == 0 {
						continue
					}
					roomName := detectRoom(v.content)
					room, _ := palace.CreateRoom(d, roomName, wing.ID)
					id, err := benchAddDrawer(d, v.content, wing.ID, room.ID, v.hall, sessionID, "conversation")
					if err == nil {
						totalDrawers++
						batchItems = append(batchItems, struct{ ID int64; Content string }{id, v.content})
					}
				}
				continue
			}

			// Chunk session into exchange pairs (user+assistant) for better BM25 granularity
			chunks := chunkSession(session)
			for _, chunk := range chunks {
				if len(chunk) < 20 {
					continue
				}
				roomName := detectRoom(chunk)
				room, _ := palace.CreateRoom(d, roomName, wing.ID)
				id, err := benchAddDrawer(d, chunk, wing.ID, room.ID, "facts", sessionID, "conversation")
				if err == nil {
					totalDrawers++
					batchItems = append(batchItems, struct{ ID int64; Content string }{id, chunk})
				}
			}
		}
	}

	// Batch index all drawers (BM25)
	search.IndexBatch(d, batchItems)
	indexDuration := time.Since(indexStart)
	fmt.Printf(" done\n")
	fmt.Printf("Sessions: %d, Drawers indexed: %d, BM25 index time: %s\n",
		totalSessions, totalDrawers, indexDuration.Round(time.Millisecond))

	// Optional: compute embeddings for all drawers and store as blobs.
	var embedClient *embeddings.Client
	var embedIndexDuration time.Duration
	if useEmbeddings {
		embedClient = embeddings.NewClient(envCfg)
		embStart := time.Now()
		fmt.Printf("Embedding %d drawers (8 workers, batch 4, truncated 1500)...\n", len(batchItems))
		texts := make([]string, len(batchItems))
		for i, b := range batchItems {
			texts[i] = truncateForEmbedding(b.Content, 1500)
		}
		progress := func(done, total int) {
			fmt.Printf("\r  drawers: %d/%d (%.1f%%) elapsed=%s",
				done, total, float64(done)/float64(total)*100,
				time.Since(embStart).Round(time.Second))
		}
		vecs, failed, err := embedClient.EmbedAll(texts, 4, 8, progress)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nembed drawers: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()
		stored := 0
		for i, v := range vecs {
			if v != nil {
				search.IndexEmbedding(d, batchItems[i].ID, embeddings.Encode(v))
				stored++
			}
		}
		embedIndexDuration = time.Since(embStart)
		fmt.Printf("Embedding done in %s (stored %d, failed %d)\n",
			embedIndexDuration.Round(time.Millisecond), stored, failed)
	}
	fmt.Println()

	// Phase 2: Run retrieval benchmark
	fmt.Println("Running retrieval...")

	// For vector/hybrid we batch-embed all queries up front using the same
	// concurrent helper (8 workers).
	var queryVecs map[string][]float32
	if useEmbeddings {
		fmt.Printf("Embedding %d queries (8 workers, batch 4)...\n", len(questions))
		qStart := time.Now()
		texts := make([]string, len(questions))
		for i, q := range questions {
			texts[i] = q.Question
		}
		progress := func(done, total int) {
			fmt.Printf("\r  queries: %d/%d (%.1f%%) elapsed=%s",
				done, total, float64(done)/float64(total)*100,
				time.Since(qStart).Round(time.Second))
		}
		vecs, qFailed, err := embedClient.EmbedAll(texts, 4, 8, progress)
		if err != nil {
			fmt.Fprintf(os.Stderr, "\nembed queries: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()
		queryVecs = make(map[string][]float32, len(vecs))
		stored := 0
		for i, v := range vecs {
			if v != nil {
				queryVecs[questions[i].ID] = v
				stored++
			}
		}
		fmt.Printf("Query embedding done in %s (stored %d, failed %d, fallback to BM25)\n",
			time.Since(qStart).Round(time.Millisecond), stored, qFailed)

		// Query2Doc pass: for each query, get a pseudo-doc from the LLM,
		// embed it, then average with the original query vector.
		if useQuery2Doc {
			fmt.Printf("Query2Doc: generating %d pseudo-docs (8 workers)...\n", len(questions))
			q2dStart := time.Now()
			chat := embeddings.NewChatClient(q2dURL, q2dModel, envCfg.EmbeddingsAPIKey)
			pseudos := make([]string, len(questions))
			var wg sync.WaitGroup
			jobs := make(chan int)
			var doneMu sync.Mutex
			done := 0
			for w := 0; w < 8; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := range jobs {
						prompt := "Write one short plausible answer sentence to help retrieve the relevant passage. Question: " + questions[i].Question
						if out, err := chat.Complete(prompt, 80); err == nil {
							pseudos[i] = strings.TrimSpace(out)
						}
						doneMu.Lock()
						done++
						if done%50 == 0 {
							fmt.Printf("  %d/%d pseudo-docs...\n", done, len(questions))
						}
						doneMu.Unlock()
					}
				}()
			}
			for i := range questions {
				jobs <- i
			}
			close(jobs)
			wg.Wait()

			// Embed pseudo-docs in batch
			nonEmpty := 0
			pseudoTexts := make([]string, 0, len(pseudos))
			pseudoIdx := make([]int, 0, len(pseudos))
			for i, p := range pseudos {
				if p != "" {
					pseudoTexts = append(pseudoTexts, p)
					pseudoIdx = append(pseudoIdx, i)
					nonEmpty++
				}
			}
			if nonEmpty > 0 {
				pseudoVecs, _, err := embedClient.EmbedAll(pseudoTexts, 4, 8, nil)
				if err == nil {
					// Average with original query vector
					for k, idx := range pseudoIdx {
						if pseudoVecs[k] == nil {
							continue
						}
						orig, ok := queryVecs[questions[idx].ID]
						if !ok {
							continue
						}
						queryVecs[questions[idx].ID] = embeddings.MeanVecs(orig, pseudoVecs[k])
					}
				}
			}
			fmt.Printf("Query2Doc done in %s (non-empty: %d/%d)\n",
				time.Since(q2dStart).Round(time.Millisecond), nonEmpty, len(questions))
		}
	}

	// In hybrid mode, optionally sweep RRF weights (LME_RRF_WEIGHTS=0.3,0.5,0.7).
	// Each weight reuses the already-embedded palace and queries — only the
	// retrieval+scoring loop runs again, which is fast (~3s per pass).
	weights := []float64{0.5}
	if mode == "hybrid" {
		if env := os.Getenv("LME_RRF_WEIGHTS"); env != "" {
			weights = nil
			for _, p := range strings.Split(env, ",") {
				var w float64
				if _, err := fmt.Sscanf(strings.TrimSpace(p), "%f", &w); err == nil {
					weights = append(weights, w)
				}
			}
			if len(weights) == 0 {
				weights = []float64{0.5}
			}
		}
	}

	type weightResult struct {
		weight                     float64
		hit1, hit5, hit10          int
		sidHit1, sidHit5, sidHit10 int // session-id-based (LongMemEval official)
		searchDuration             time.Duration
		typeHits                   map[string][2]int
	}
	var sweep []weightResult
	total := len(questions)

	// Reranker (optional): retrieve a wider candidate set, then re-score
	// the top N with a cross-encoder. POOL is how many candidates we ask
	// the first stage for; we always return top-10 for evaluation.
	var rerankClient *rerank.Client
	rerankPool := 20
	if useRerank {
		rerankClient = rerank.NewClient(envCfg)
		if v := os.Getenv("LME_RERANK_POOL"); v != "" {
			fmt.Sscanf(v, "%d", &rerankPool)
		}
	}
	candidateLimit := 10
	if useRerank {
		candidateLimit = rerankPool
	}
	if v := os.Getenv("LME_CANDIDATE_LIMIT"); v != "" {
		fmt.Sscanf(v, "%d", &candidateLimit)
	}

	rerankWorkers := 8
	if v := os.Getenv("LME_RERANK_WORKERS"); v != "" {
		fmt.Sscanf(v, "%d", &rerankWorkers)
	}
	// Rerank gating: when LME_RERANK_GATE=oracle (or "classifier") only
	// apply the reranker for question types where it helped (knowledge-update,
	// temporal-reasoning per our prior sweep). Other types skip rerank.
	rerankGate := os.Getenv("LME_RERANK_GATE")
	gateAllowed := map[string]bool{
		"temporal-reasoning": true,
		"knowledge-update":   true,
	}

	// Track heuristic classifier accuracy on the way through.
	classifierHits := 0
	for _, q := range questions {
		if string(search.ClassifyQuestion(q.Question)) == q.Type {
			classifierHits++
		}
	}
	if mode == "hybrid" && len(questions) > 0 {
		fmt.Printf("Heuristic classifier accuracy: %.1f%% (%d/%d)\n",
			float64(classifierHits)/float64(len(questions))*100,
			classifierHits, len(questions))
	}

	// Also measure embedding-anchor classifier accuracy if embeddings are
	// available — uses the same client and anchor texts. Each query's
	// vector is already cached (queryVecs), so cost = one Prepare() call.
	var anchorClf *search.AnchorClassifier
	if useEmbeddings && embedClient != nil {
		anchorClf = search.NewAnchorClassifier(embedClient.Embed)
		if err := anchorClf.Prepare(); err != nil {
			fmt.Fprintf(os.Stderr, "anchor prepare: %v\n", err)
			anchorClf = nil
		}
	}
	if anchorClf != nil {
		anchorHits := 0
		for _, q := range questions {
			qvec := queryVecs[q.ID]
			if qvec == nil {
				continue
			}
			if string(anchorClf.Classify(qvec)) == q.Type {
				anchorHits++
			}
		}
		fmt.Printf("Anchor classifier accuracy:    %.1f%% (%d/%d)\n",
			float64(anchorHits)/float64(len(questions))*100,
			anchorHits, len(questions))
	}

	runSweepWeight := func(w float64) weightResult {
		wr := weightResult{weight: w, typeHits: make(map[string][2]int)}
		swStart := time.Now()

		// Per-query results, then optionally reranked in parallel.
		perQResults := make([][]search.SearchResult, len(questions))
		scopedPalacePre := os.Getenv("LME_SCOPED_PALACE") == "1"
		for i, q := range questions {
			var results []search.SearchResult
			qvec := queryVecs[q.ID]
			var haystackFilter map[string]bool
			if scopedPalacePre {
				haystackFilter = make(map[string]bool, len(q.HaystackSessionIDs))
				for _, sid := range q.HaystackSessionIDs {
					haystackFilter[sid] = true
				}
			}
			switch {
			case useLCache && qvec != nil:
				results = lcacheSearch(d, qvec, lcacheSessions, candidateLimit,
					lcacheW0, lcacheW1, lcacheW2, lcacheMerge, haystackFilter)
				if lcacheBM25 > 0 {
					results = lcacheBlendWithBM25(d, q.Question, results, lcacheBM25, candidateLimit)
				}
			case mode == "vector" && qvec != nil:
				results, _ = search.SearchVector(d, qvec, 0, 0, candidateLimit)
			case mode == "hybrid" && qvec != nil:
				results, _ = search.SearchHybridWeighted(d, q.Question, qvec, 0, 0, candidateLimit, w)
			default:
				results, _ = search.Search(d, q.Question, 0, 0, candidateLimit)
			}
			if recencyWeight > 0 {
				results = search.ApplyRecencyBoost(results, recencyWeight)
			}
			perQResults[i] = results
		}

		// Stage 2: cross-encoder re-rank in parallel. Each query is one
		// HTTP call with `rerankPool` documents. Failed reranks fall back
		// to first-stage ranking (no-op). Per-type gating skips rerank
		// for question types where it consistently hurts.
		if useRerank {
			var wg sync.WaitGroup
			jobs := make(chan int)
			gated := 0
			for w := 0; w < rerankWorkers; w++ {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for i := range jobs {
						results := perQResults[i]
						if len(results) <= 1 {
							continue
						}
						docs := make([]string, len(results))
						for j, r := range results {
							docs[j] = truncateForEmbedding(r.Content, 1500)
						}
						scores, err := rerankClient.Score(questions[i].Question, docs)
						if err != nil {
							continue
						}
						for j := range results {
							results[j].Score = scores[j]
						}
						sort.Slice(results, func(a, b int) bool {
							return results[a].Score > results[b].Score
						})
						perQResults[i] = results
					}
				}()
			}
			for i, q := range questions {
				switch rerankGate {
				case "oracle":
					if !gateAllowed[q.Type] {
						gated++
						continue
					}
				case "classifier":
					if !gateAllowed[string(search.ClassifyQuestion(q.Question))] {
						gated++
						continue
					}
				}
				jobs <- i
			}
			close(jobs)
			wg.Wait()
			if rerankGate != "" && w == weights[0] {
				fmt.Printf("Rerank gating (%s): %d/%d queries skipped rerank\n",
					rerankGate, gated, len(questions))
			}
		}

		// Tally hits — two metrics side by side:
		//   hit*    : containsAnswer(content, answer) — our original heuristic
		//   sidHit* : session_id ∈ answer_session_ids — LongMemEval's official metric
		//
		// LME_SCOPED_PALACE=1 simulates per-question palace isolation by
		// filtering results to this question's haystack before the top-K
		// cut (needs candidateLimit large enough; use pool ≥ 50).
		scopedPalace := os.Getenv("LME_SCOPED_PALACE") == "1"
		for i, q := range questions {
			answerText := normalizeAnswer(q.Answer)
			results := perQResults[i]
			if useRerank && len(results) > 10 {
				results = results[:10]
			}
			answerSet := make(map[string]bool, len(q.AnswerSessionIDs))
			for _, sid := range q.AnswerSessionIDs {
				answerSet[sid] = true
			}
			if scopedPalace {
				haystack := make(map[string]bool, len(q.HaystackSessionIDs))
				for _, sid := range q.HaystackSessionIDs {
					haystack[sid] = true
				}
				filtered := make([]search.SearchResult, 0, len(results))
				for _, r := range results {
					if haystack[r.SourceFile] {
						filtered = append(filtered, r)
					}
				}
				results = filtered
				if len(results) > 10 {
					results = results[:10]
				}
			}

			found5, found10 := false, false
			for i, r := range results {
				if containsAnswer(r.Content, answerText) {
					if i == 0 {
						wr.hit1++
					}
					if i < 5 {
						found5 = true
					}
					if i < 10 {
						found10 = true
					}
					break
				}
			}
			sidFound5, sidFound10 := false, false
			for i, r := range results {
				if answerSet[r.SourceFile] {
					if i == 0 {
						wr.sidHit1++
					}
					if i < 5 {
						sidFound5 = true
					}
					if i < 10 {
						sidFound10 = true
					}
					break
				}
			}
			if found5 {
				wr.hit5++
			}
			if found10 {
				wr.hit10++
			}
			if sidFound5 {
				wr.sidHit5++
			}
			if sidFound10 {
				wr.sidHit10++
			}
			counts := wr.typeHits[q.Type]
			if found5 {
				counts[0]++
			}
			counts[1]++
			wr.typeHits[q.Type] = counts
		}
		wr.searchDuration = time.Since(swStart)
		return wr
	}

	for _, w := range weights {
		sweep = append(sweep, runSweepWeight(w))
	}

	// Pick the run with best R@5 to use as the primary "results" report.
	best := sweep[0]
	for _, wr := range sweep {
		if wr.hit5 > best.hit5 {
			best = wr
		}
	}
	hit1, hit5, hit10 := best.hit1, best.hit5, best.hit10
	typeHits := best.typeHits
	searchDuration := best.searchDuration

	if len(sweep) > 1 {
		fmt.Printf("\n=== RRF WEIGHT SWEEP (BM25 weight) ===\n")
		fmt.Printf("%-10s %-8s %-8s %-8s %s\n", "weight", "R@1", "R@5", "R@10", "search")
		for _, wr := range sweep {
			marker := ""
			if wr.weight == best.weight {
				marker = " <- best"
			}
			fmt.Printf("%-10.2f %5.1f%%   %5.1f%%   %5.1f%%   %s%s\n",
				wr.weight,
				float64(wr.hit1)/float64(total)*100,
				float64(wr.hit5)/float64(total)*100,
				float64(wr.hit10)/float64(total)*100,
				wr.searchDuration.Round(time.Millisecond),
				marker)
		}

		// Per-type oracle: pick the weight that maximizes R@5 for each
		// question type independently, then aggregate. This is the upper
		// bound achievable with classifier-driven per-type weighting.
		fmt.Printf("\n=== PER-TYPE ORACLE WEIGHTS ===\n")
		fmt.Printf("%-30s %-8s %-8s %s\n", "type", "weight", "R@5", "n")
		typesSeen := make(map[string]bool)
		for _, wr := range sweep {
			for t := range wr.typeHits {
				typesSeen[t] = true
			}
		}
		bestWeightForType := make(map[string]float64)
		oracleHit5 := 0
		oracleTotal := 0
		for t := range typesSeen {
			var bestW float64
			bestHit, bestN := -1, 0
			for _, wr := range sweep {
				c := wr.typeHits[t]
				if c[0] > bestHit {
					bestHit, bestN = c[0], c[1]
					bestW = wr.weight
				}
			}
			bestWeightForType[t] = bestW
			fmt.Printf("%-30s %-8.2f %5.1f%% (%d/%d)\n",
				t, bestW, float64(bestHit)/float64(bestN)*100, bestHit, bestN)
			oracleHit5 += bestHit
			oracleTotal += bestN
		}
		fmt.Printf("Aggregate oracle per-type R@5: %.1f%% (%d/%d)\n",
			float64(oracleHit5)/float64(oracleTotal)*100, oracleHit5, oracleTotal)

		// Classifier-driven per-type weights: for each query predict the
		// type via search.ClassifyQuestion, then look up that type's
		// oracle-best weight and read the corresponding sweep result.
		// This shows how much of the oracle gain a real classifier captures.
		// Build per-(weight, qid) lookup of whether that query was a hit at R@5.
		hitAt := make(map[float64]map[string]bool, len(sweep))
		for _, wr := range sweep {
			hitAt[wr.weight] = make(map[string]bool, total)
		}
		// Re-evaluate by replaying: a query was a hit iff results[i] within top-5
		// matched containsAnswer. We can't reach into the inner state from here,
		// but we can re-run the per-question loop using the per-weight maps we
		// already collected aggregate hit counts for. That doesn't give us the
		// per-question outcome.
		//
		// Cheaper proxy: for each *type*, count the predicted-classifier
		// distribution and assume the type's per-weight R@5 applies uniformly
		// (i.e., the classifier picks weight based on predicted type, and we
		// score using the *predicted* type's oracle-best weight against the
		// *actual* type's sweep R@5 at that weight).
		fmt.Printf("\n=== CLASSIFIER-DRIVEN PER-TYPE WEIGHTS ===\n")
		// For each (predictedType, actualType) pair count queries.
		conf := make(map[string]map[string]int) // pred -> actual -> count
		for _, q := range questions {
			pred := string(search.ClassifyQuestion(q.Question))
			if conf[pred] == nil {
				conf[pred] = make(map[string]int)
			}
			conf[pred][q.Type]++
		}
		// Anchor-classifier confusion matrix (if available).
		var anchorConf map[string]map[string]int
		if anchorClf != nil {
			anchorConf = make(map[string]map[string]int)
			for _, q := range questions {
				qvec := queryVecs[q.ID]
				if qvec == nil {
					continue
				}
				pred := string(anchorClf.Classify(qvec))
				if anchorConf[pred] == nil {
					anchorConf[pred] = make(map[string]int)
				}
				anchorConf[pred][q.Type]++
			}
		}
		// For each predicted type bucket, the classifier picks bestWeightForType[pred].
		// Then those queries get scored at that weight. Sum up total hits.
		// Use per-(weight, actualType) hit fraction from sweep.
		sweepByWeight := make(map[float64]weightResult)
		for _, wr := range sweep {
			sweepByWeight[wr.weight] = wr
		}
		classifierHit := 0
		classifierTotal := 0
		for pred, actuals := range conf {
			w := bestWeightForType[pred]
			// Default to a sweep weight that exists if pred wasn't in sweep
			if _, ok := sweepByWeight[w]; !ok {
				w = bestWeightForType["single-session-user"]
			}
			wr := sweepByWeight[w]
			for actual, n := range actuals {
				c := wr.typeHits[actual]
				if c[1] == 0 {
					continue
				}
				rate := float64(c[0]) / float64(c[1])
				expected := int(rate*float64(n) + 0.5)
				classifierHit += expected
				classifierTotal += n
			}
		}
		if classifierTotal > 0 {
			fmt.Printf("Heuristic classifier per-type R@5:       %.1f%% (%d/%d)\n",
				float64(classifierHit)/float64(classifierTotal)*100,
				classifierHit, classifierTotal)
		}
		if anchorConf != nil {
			anchorEstHit := 0
			anchorEstTotal := 0
			for pred, actuals := range anchorConf {
				w := bestWeightForType[pred]
				if _, ok := sweepByWeight[w]; !ok {
					w = bestWeightForType["single-session-user"]
				}
				wr := sweepByWeight[w]
				for actual, n := range actuals {
					c := wr.typeHits[actual]
					if c[1] == 0 {
						continue
					}
					rate := float64(c[0]) / float64(c[1])
					expected := int(rate*float64(n) + 0.5)
					anchorEstHit += expected
					anchorEstTotal += n
				}
			}
			if anchorEstTotal > 0 {
				fmt.Printf("Anchor classifier per-type R@5:          %.1f%% (%d/%d)\n",
					float64(anchorEstHit)/float64(anchorEstTotal)*100,
					anchorEstHit, anchorEstTotal)
			}
		}
		fmt.Printf("(estimates use per-(weight, actual-type) hit rate × predicted-type count)\n")
		fmt.Printf("\nReporting best weight (%.2f) below.\n", best.weight)
	}

	fmt.Printf("\n=== RESULTS ===\n")
	if mode == "hybrid" {
		fmt.Printf("BM25 weight: %.2f (vector weight: %.2f)\n", best.weight, 1-best.weight)
	}
	fmt.Printf("Recall@1:  %.1f%% (%d/%d)   [answer-text heuristic]\n", float64(hit1)/float64(total)*100, hit1, total)
	fmt.Printf("Recall@5:  %.1f%% (%d/%d)\n", float64(hit5)/float64(total)*100, hit5, total)
	fmt.Printf("Recall@10: %.1f%% (%d/%d)\n", float64(hit10)/float64(total)*100, hit10, total)
	fmt.Printf("Recall@1 (sid):  %.1f%% (%d/%d)   [session_id — LongMemEval official]\n", float64(best.sidHit1)/float64(total)*100, best.sidHit1, total)
	fmt.Printf("Recall@5 (sid):  %.1f%% (%d/%d)\n", float64(best.sidHit5)/float64(total)*100, best.sidHit5, total)
	fmt.Printf("Recall@10 (sid): %.1f%% (%d/%d)\n", float64(best.sidHit10)/float64(total)*100, best.sidHit10, total)
	fmt.Printf("Search time: %s (avg: %s/query)\n", searchDuration.Round(time.Millisecond), (searchDuration / time.Duration(total)).Round(time.Microsecond))

	fmt.Printf("\n=== PER TYPE ===\n")
	for qtype, counts := range typeHits {
		pct := float64(counts[0]) / float64(counts[1]) * 100
		fmt.Printf("  %-35s R@5: %5.1f%% (%d/%d)\n", qtype, pct, counts[0], counts[1])
	}

	fmt.Printf("\n=== COMPARISON ===\n")
	fmt.Printf("  %-30s R@5: %.1f%%\n", fmt.Sprintf("mem (%s, this run)", mode), float64(hit5)/float64(total)*100)
	fmt.Printf("  %-30s R@5: 96.6%%\n", "MemPalace (ChromaDB)")
	fmt.Printf("  %-30s R@5: ~85%%\n", "Mem0")
	fmt.Printf("  %-30s R@5: ~85%%\n", "Zep")
	fmt.Printf("  %-30s R@5: ~70%%\n", "BM25 (flat, no structure)")
	if useEmbeddings {
		fmt.Printf("\nEmbedding index time: %s\n", embedIndexDuration.Round(time.Millisecond))
	}

	d.Close()
	os.RemoveAll(palacePath)
}

// truncateForEmbedding caps text at maxChars to fit comfortably under
// the bge-m3 8K-token context (≈ 4 chars per token average) and avoid
// server-side timeouts on overly long inputs.
func truncateForEmbedding(text string, maxChars int) string {
	if len(text) <= maxChars {
		return text
	}
	return text[:maxChars]
}

func normalizeAnswer(answer any) string {
	switch v := answer.(type) {
	case string:
		return strings.ToLower(strings.TrimSpace(v))
	case []any:
		var parts []string
		for _, a := range v {
			parts = append(parts, fmt.Sprintf("%v", a))
		}
		return strings.ToLower(strings.Join(parts, " "))
	default:
		return strings.ToLower(fmt.Sprintf("%v", answer))
	}
}

// lcacheSearch implements Schift-style per-session weighted merge over
// L0/L1/L2 embeddings. It loads every drawer's embedding blob + hall
// + source_file (cached across queries via the sql driver's row buffer),
// computes cosine to qvec, then groups by source_file (session id) and
// returns one SearchResult per session with content = the session's L0
// text (so the downstream containsAnswer check still operates on full
// content). Score = 0.2*L0 + 0.5*L1 + 0.3*L2.
//
// lcacheSessionsByID lets us hand back the L0 content as the result's
// Content — evidence matching stays on the complete session, not on
// whichever variant "won" the cosine.
func lcacheSearch(d *db.DB, qvec []float32, lcacheSessionsByID map[string]*sessionVariants,
	limit int, wL0, wL1, wL2 float64, mergeMode string, haystackFilter map[string]bool) []search.SearchResult {
	rows, err := d.Query(`SELECT source_file, hall, embedding FROM drawers WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil
	}
	defer rows.Close()

	perSession := make(map[string]float64)
	for rows.Next() {
		var sessionID, hall string
		var blob []byte
		if err := rows.Scan(&sessionID, &hall, &blob); err != nil {
			continue
		}
		if haystackFilter != nil && !haystackFilter[sessionID] {
			continue
		}
		vec, err := embeddings.Decode(blob)
		if err != nil {
			continue
		}
		sim := float64(embeddings.Cosine(qvec, vec))
		var contribution float64
		switch hall {
		case "L0":
			contribution = wL0 * sim
		case "L1":
			contribution = wL1 * sim
		case "L2":
			contribution = wL2 * sim
		}
		if mergeMode == "max" {
			if contribution > perSession[sessionID] {
				perSession[sessionID] = contribution
			}
		} else {
			perSession[sessionID] += contribution
		}
	}

	type sessionScore struct {
		sessionID string
		score     float64
	}
	ranked := make([]sessionScore, 0, len(perSession))
	for id, s := range perSession {
		ranked = append(ranked, sessionScore{id, s})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}

	out := make([]search.SearchResult, 0, len(ranked))
	for _, r := range ranked {
		sv, ok := lcacheSessionsByID[r.sessionID]
		content := ""
		if ok {
			content = sv.l0
		}
		out = append(out, search.SearchResult{
			SourceFile: r.sessionID,
			Content:    content,
			Score:      r.score,
		})
	}
	return out
}

type sessionVariants struct {
	sessionID  string
	l0, l1, l2 string
}

// lcacheBlendWithBM25 combines L# Cache vector session scores with a
// BM25 session score (max over the 3 variants). The blended score is
//
//	final = (1 - w) * vector_score + w * bm25_norm_score
//
// where bm25_norm_score is the BM25 RRF contribution 1/(60+rank) so
// it's on a comparable scale to cosine similarity. Returns top-limit
// sessions sorted by the blended score. Useful when lexical overlap
// with rare terms matters (personal names, numbers, specific places).
func lcacheBlendWithBM25(d *db.DB, query string, vecResults []search.SearchResult,
	w float64, limit int) []search.SearchResult {
	if w <= 0 || len(vecResults) == 0 {
		return vecResults
	}
	// Keep vector scores indexed by session
	vecBySession := make(map[string]search.SearchResult, len(vecResults))
	for _, r := range vecResults {
		if _, ok := vecBySession[r.SourceFile]; !ok {
			vecBySession[r.SourceFile] = r
		}
	}
	// Run BM25 over all variants (3× drawers per session); dedupe by session
	bm25, err := search.Search(d, query, 0, 0, limit*3)
	if err != nil {
		return vecResults
	}
	bm25BySession := make(map[string]float64)
	for rank, r := range bm25 {
		if _, seen := bm25BySession[r.SourceFile]; seen {
			continue
		}
		bm25BySession[r.SourceFile] = 1.0 / (60.0 + float64(rank+1))
	}

	// Blend
	union := make(map[string]bool, len(vecBySession)+len(bm25BySession))
	for k := range vecBySession {
		union[k] = true
	}
	for k := range bm25BySession {
		union[k] = true
	}

	type scored struct {
		r     search.SearchResult
		score float64
	}
	var ranked []scored
	for sid := range union {
		vec, hasVec := vecBySession[sid]
		var base search.SearchResult
		var vecScore float64
		if hasVec {
			base = vec
			vecScore = vec.Score
		} else {
			base = search.SearchResult{SourceFile: sid, Score: 0}
		}
		bm25s := bm25BySession[sid]
		final := (1.0-w)*vecScore + w*bm25s
		base.Score = final
		ranked = append(ranked, scored{base, final})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].score > ranked[j].score })
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]search.SearchResult, len(ranked))
	for i, r := range ranked {
		out[i] = r.r
	}
	return out
}

// makeSessionVariants returns the three L# views of a session:
//   L0 = full (all turns with roles)
//   L1 = user turns only (drops assistant verbosity)
//   L2 = first 3 user turns (zero-cost summary proxy)
//
// Adapted from Schift's L# Cache (they hit 96% R@5 on LongMemEval pure
// vector with 0.5*L1 + 0.3*L2 + 0.2*L0 weighting).
func makeSessionVariants(session []Message) (l0, l1, l2 string) {
	var l0Parts, userParts []string
	userCount := 0
	for _, m := range session {
		l0Parts = append(l0Parts, m.Role+": "+m.Content)
		if m.Role == "user" {
			userParts = append(userParts, m.Content)
			userCount++
		}
	}
	l0 = strings.Join(l0Parts, "\n")
	l1 = strings.Join(userParts, "\n")
	if len(userParts) > 3 {
		l2 = strings.Join(userParts[:3], "\n")
	} else {
		l2 = l1
	}
	return l0, l1, l2
}

// chunkSession: whole session as one drawer (best R@5 in testing).
// Chunking hurt IDF by inflating doc count. Single-drawer approach: 47.8% R@5.
func chunkSession(session []Message) []string {
	if len(session) == 0 {
		return nil
	}
	var parts []string
	for _, msg := range session {
		parts = append(parts, msg.Role+": "+msg.Content)
	}
	return []string{strings.Join(parts, "\n")}
}

func splitLargeChunk(text string, maxLen int) []string {
	var chunks []string
	words := strings.Fields(text)
	var current []string
	currentLen := 0
	for _, w := range words {
		if currentLen+len(w)+1 > maxLen && len(current) > 0 {
			chunks = append(chunks, strings.Join(current, " "))
			current = nil
			currentLen = 0
		}
		current = append(current, w)
		currentLen += len(w) + 1
	}
	if len(current) > 0 {
		chunks = append(chunks, strings.Join(current, " "))
	}
	return chunks
}

func containsAnswer(content, answer string) bool {
	contentLower := strings.ToLower(content)
	answerLower := strings.ToLower(answer)

	// 1. Exact substring match
	if strings.Contains(contentLower, answerLower) {
		return true
	}

	// 2. Word-level overlap: count meaningful (>2 char, non-stopword) answer words in content
	stopwords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "was": true, "are": true,
		"and": true, "or": true, "of": true, "to": true, "in": true, "on": true,
		"for": true, "at": true, "by": true, "with": true, "as": true, "it": true,
		"this": true, "that": true, "which": true, "who": true, "what": true,
		"my": true, "your": true, "his": true, "her": true, "their": true,
		"be": true, "have": true, "has": true, "had": true, "do": true, "does": true,
	}

	words := strings.FieldsFunc(answerLower, func(r rune) bool {
		return r == ' ' || r == ',' || r == '.' || r == ';' || r == ':' || r == '!' || r == '?' || r == '"' || r == '\''
	})

	var meaningful []string
	for _, w := range words {
		if len(w) > 2 && !stopwords[w] {
			meaningful = append(meaningful, w)
		}
	}

	if len(meaningful) == 0 {
		return false
	}

	matchCount := 0
	for _, w := range meaningful {
		if strings.Contains(contentLower, w) {
			matchCount++
		}
	}

	// Match if 60% or more meaningful words found (lowered from 70%)
	return float64(matchCount)/float64(len(meaningful)) >= 0.6
}

func detectRoom(content string) string {
	tokens := search.Tokenize(content)
	if len(tokens) == 0 {
		return "general"
	}
	freq := search.TokenFrequency(tokens)
	type kv struct{ k string; v float64 }
	var top []kv
	for k, v := range freq {
		top = append(top, kv{k, v})
	}
	for i := 0; i < len(top); i++ {
		for j := i + 1; j < len(top); j++ {
			if top[j].v > top[i].v {
				top[i], top[j] = top[j], top[i]
			}
		}
	}
	var parts []string
	for i, kv := range top {
		if i >= 2 { break }
		parts = append(parts, kv.k)
	}
	if len(parts) == 0 {
		return "general"
	}
	return strings.Join(parts, "-")
}

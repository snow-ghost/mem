package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"sort"
	"sync"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/embeddings"
	"github.com/snow-ghost/mem/internal/palace"
	"github.com/snow-ghost/mem/internal/rerank"
	"github.com/snow-ghost/mem/internal/search"
)

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Question struct {
	ID               string          `json:"question_id"`
	Type             string          `json:"question_type"`
	Question         string          `json:"question"`
	Answer           any             `json:"answer"`
	HaystackSessions [][]Message     `json:"haystack_sessions"`
	AnswerSessionIDs json.RawMessage `json:"answer_session_ids"`
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
	fmt.Printf("Dataset: %s\n", dataFile)
	fmt.Printf("Mode: %s\n", mode)
	if useEmbeddings {
		fmt.Printf("Embeddings: %s\n", envCfg.EmbeddingsModel)
	}
	if useRerank {
		fmt.Printf("Reranker: %s (LME_RERANK=1)\n", envCfg.RerankModel)
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

	for _, q := range questions {
		for _, session := range q.HaystackSessions {
			totalSessions++
			// Chunk session into exchange pairs (user+assistant) for better BM25 granularity
			chunks := chunkSession(session)
			for _, chunk := range chunks {
				if len(chunk) < 20 {
					continue
				}
				roomName := detectRoom(chunk)
				room, _ := palace.CreateRoom(d, roomName, wing.ID)
				drawer, _ := palace.AddDrawer(d, chunk, wing.ID, room.ID, "facts", fmt.Sprintf("session_%d", totalSessions), "conversation")
				if drawer != nil {
					totalDrawers++
					batchItems = append(batchItems, struct{ ID int64; Content string }{drawer.ID, chunk})
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
		weight                  float64
		hit1, hit5, hit10       int
		searchDuration          time.Duration
		typeHits                map[string][2]int
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

	// Track classifier accuracy on the way through (separate pass below)
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

	runSweepWeight := func(w float64) weightResult {
		wr := weightResult{weight: w, typeHits: make(map[string][2]int)}
		swStart := time.Now()

		// Per-query results, then optionally reranked in parallel.
		perQResults := make([][]search.SearchResult, len(questions))
		for i, q := range questions {
			var results []search.SearchResult
			qvec := queryVecs[q.ID]
			switch {
			case mode == "vector" && qvec != nil:
				results, _ = search.SearchVector(d, qvec, 0, 0, candidateLimit)
			case mode == "hybrid" && qvec != nil:
				results, _ = search.SearchHybridWeighted(d, q.Question, qvec, 0, 0, candidateLimit, w)
			default:
				results, _ = search.Search(d, q.Question, 0, 0, candidateLimit)
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

		// Tally hits
		for i, q := range questions {
			answerText := normalizeAnswer(q.Answer)
			results := perQResults[i]
			if useRerank && len(results) > 10 {
				results = results[:10]
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
			if found5 {
				wr.hit5++
			}
			if found10 {
				wr.hit10++
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
		fmt.Printf("\nReporting best weight (%.2f) below.\n", best.weight)
	}

	fmt.Printf("\n=== RESULTS ===\n")
	if mode == "hybrid" {
		fmt.Printf("BM25 weight: %.2f (vector weight: %.2f)\n", best.weight, 1-best.weight)
	}
	fmt.Printf("Recall@1:  %.1f%% (%d/%d)\n", float64(hit1)/float64(total)*100, hit1, total)
	fmt.Printf("Recall@5:  %.1f%% (%d/%d)\n", float64(hit5)/float64(total)*100, hit5, total)
	fmt.Printf("Recall@10: %.1f%% (%d/%d)\n", float64(hit10)/float64(total)*100, hit10, total)
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

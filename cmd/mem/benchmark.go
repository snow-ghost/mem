package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"time"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/embeddings"
	"github.com/snow-ghost/mem/internal/palace"
	"github.com/snow-ghost/mem/internal/search"
)

// runBenchmark times the retrieval pipeline on a synthetic corpus so
// users can compare modes (BM25 / vector / hybrid / HNSW), tune
// concurrency, and validate that an embeddings endpoint is fast enough
// for their target scale. It does NOT touch the user's existing palace —
// a temporary palace is created and removed.
func runBenchmark(args []string) int {
	var (
		nDrawers   int
		nQueries   int
		seed       int64
		dim        int
		hnswCutoff int
		skipBuild  bool
	)
	fs := flag.NewFlagSet("benchmark", flag.ContinueOnError)
	fs.IntVar(&nDrawers, "drawers", 1000, "synthetic drawers to generate")
	fs.IntVar(&nQueries, "queries", 100, "search queries to time")
	fs.IntVar(&dim, "dim", 128, "embedding dimensionality (for synthetic vectors)")
	fs.Int64Var(&seed, "seed", 42, "RNG seed for reproducibility")
	fs.IntVar(&hnswCutoff, "hnsw-cutoff", 5000, "HNSW build vs full-scan crossover hint (informational)")
	fs.BoolVar(&skipBuild, "skip-real-embeds", false, "skip the real-embeddings stage even if MEM_EMBEDDINGS_* is set")
	fs.Parse(args)

	rng := rand.New(rand.NewSource(seed))
	cfg := config.Load()

	tmpDir, err := os.MkdirTemp("", "mem-bench-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: benchmark: %v\n", err)
		return 1
	}
	defer os.RemoveAll(tmpDir)

	bcfg := config.Config{PalacePath: tmpDir, DbFile: "bench.db"}
	d, err := palace.Init(bcfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: benchmark: init palace: %v\n", err)
		return 1
	}
	defer d.Close()

	wing, _ := palace.CreateWing(d, "bench", "synthetic", "")
	room, _ := palace.CreateRoom(d, "general", wing.ID)

	fmt.Printf("=== mem benchmark ===\n")
	fmt.Printf("drawers=%d  queries=%d  dim=%d  seed=%d\n\n", nDrawers, nQueries, dim, seed)

	// Phase 1: synthetic content
	contents := make([]string, nDrawers)
	queries := make([]string, nQueries)
	{
		topics := []string{"authentication", "deployment", "database", "logging", "metrics",
			"queue", "cache", "encryption", "scheduler", "telemetry", "session", "indexer"}
		for i := 0; i < nDrawers; i++ {
			t1 := topics[rng.Intn(len(topics))]
			t2 := topics[rng.Intn(len(topics))]
			contents[i] = fmt.Sprintf("Drawer %d about %s and %s with details and context for indexing.", i, t1, t2)
		}
		for i := 0; i < nQueries; i++ {
			t := topics[rng.Intn(len(topics))]
			queries[i] = fmt.Sprintf("how does our %s work", t)
		}
	}

	// Phase 2: BM25 indexing
	type indexItem struct {
		ID      int64
		Content string
	}
	items := make([]indexItem, 0, nDrawers)
	addStart := time.Now()
	for i := 0; i < nDrawers; i++ {
		drawer, _ := palace.AddDrawer(d, contents[i], wing.ID, room.ID, "facts",
			fmt.Sprintf("doc_%d", i), "synthetic")
		if drawer != nil {
			items = append(items, indexItem{drawer.ID, contents[i]})
		}
	}
	bm25Items := make([]struct {
		ID      int64
		Content string
	}, len(items))
	for i, it := range items {
		bm25Items[i] = struct {
			ID      int64
			Content string
		}{it.ID, it.Content}
	}
	indexStart := time.Now()
	if err := search.IndexBatch(d, bm25Items); err != nil {
		fmt.Fprintf(os.Stderr, "BM25 index: %v\n", err)
		return 1
	}
	indexDur := time.Since(indexStart)
	totalAdd := time.Since(addStart)
	fmt.Printf("Drawer insert + BM25 index: %s (insert=%s, bm25=%s)\n",
		totalAdd.Round(time.Millisecond),
		(totalAdd - indexDur).Round(time.Millisecond),
		indexDur.Round(time.Millisecond))

	// Phase 3: BM25 query timing
	bm25Start := time.Now()
	for _, q := range queries {
		_, _ = search.Search(d, q, 0, 0, 10)
	}
	bm25QDur := time.Since(bm25Start)
	fmt.Printf("BM25 search:                %s avg/query (%s for %d queries)\n",
		(bm25QDur / time.Duration(nQueries)).Round(time.Microsecond),
		bm25QDur.Round(time.Millisecond), nQueries)

	// Phase 4: synthetic embeddings + HNSW vs full scan
	fmt.Printf("\n--- vector path ---\n")
	fmt.Printf("Generating %d synthetic %d-d vectors...\n", nDrawers, dim)
	vecs := make([][]float32, nDrawers)
	for i := range vecs {
		v := make([]float32, dim)
		for j := range v {
			v[j] = float32(rng.NormFloat64())
		}
		vecs[i] = v
	}
	embedStoreStart := time.Now()
	for i, it := range items {
		_ = search.IndexEmbedding(d, it.ID, embeddings.Encode(vecs[i]))
	}
	fmt.Printf("Synthetic embedding store:  %s (%s/drawer)\n",
		time.Since(embedStoreStart).Round(time.Millisecond),
		(time.Since(embedStoreStart) / time.Duration(nDrawers)).Round(time.Microsecond))

	// Generate query vectors
	qvecs := make([][]float32, nQueries)
	for i := range qvecs {
		v := make([]float32, dim)
		for j := range v {
			v[j] = float32(rng.NormFloat64())
		}
		qvecs[i] = v
	}

	// Full-scan vector search
	scanStart := time.Now()
	for _, q := range qvecs {
		_, _ = search.SearchVector(d, q, 0, 0, 10)
	}
	scanDur := time.Since(scanStart)
	fmt.Printf("Vector full-scan search:    %s avg/query (%s for %d queries)\n",
		(scanDur / time.Duration(nQueries)).Round(time.Microsecond),
		scanDur.Round(time.Millisecond), nQueries)

	// HNSW build + search
	buildStart := time.Now()
	idx, err := search.BuildHNSWFromPalace(d)
	if err != nil || idx == nil {
		fmt.Fprintf(os.Stderr, "HNSW build: %v\n", err)
	} else {
		fmt.Printf("HNSW build (in-memory):     %s for %d vectors (%s/insert)\n",
			time.Since(buildStart).Round(time.Millisecond), idx.Size(),
			(time.Since(buildStart) / time.Duration(idx.Size())).Round(time.Microsecond))
		hnswStart := time.Now()
		for _, q := range qvecs {
			_, _ = search.SearchHNSW(d, idx, q, 10)
		}
		hnswDur := time.Since(hnswStart)
		fmt.Printf("HNSW search:                %s avg/query (%s for %d queries)\n",
			(hnswDur / time.Duration(nQueries)).Round(time.Microsecond),
			hnswDur.Round(time.Millisecond), nQueries)
		ratio := float64(scanDur) / float64(hnswDur)
		marker := "(HNSW slower)"
		if ratio > 1 {
			marker = fmt.Sprintf("(%.1f× faster)", ratio)
		}
		fmt.Printf("HNSW vs full scan:          %s\n", marker)
	}
	_ = hnswCutoff // kept as a flag for users tuning their own thresholds

	// Phase 5: real embeddings round-trip (if configured)
	if cfg.EmbeddingsEnabled() && !skipBuild {
		fmt.Printf("\n--- embeddings provider (%s) ---\n", cfg.EmbeddingsModel)
		client := embeddings.NewClient(cfg)
		// Single call latency
		single := time.Now()
		if _, err := client.Embed("benchmark single-call latency probe"); err != nil {
			fmt.Fprintf(os.Stderr, "embed probe: %v\n", err)
		} else {
			fmt.Printf("Single-text embed latency:  %s\n", time.Since(single).Round(time.Millisecond))
		}
		// Batch of 32
		texts := make([]string, 32)
		for i := range texts {
			texts[i] = contents[rng.Intn(len(contents))]
		}
		batch := time.Now()
		if vecs, err := client.EmbedBatchAdaptive(texts); err != nil {
			fmt.Fprintf(os.Stderr, "embed batch: %v\n", err)
		} else {
			d := time.Since(batch)
			fmt.Printf("Batch-of-32 embed latency:  %s (%s/text)\n",
				d.Round(time.Millisecond),
				(d / time.Duration(len(vecs))).Round(time.Millisecond))
		}
	} else if !cfg.EmbeddingsEnabled() {
		fmt.Printf("\n(MEM_EMBEDDINGS_URL not set — skipping real-embeddings probe)\n")
	}

	fmt.Printf("\nPalace artefact: %s (will be removed)\n", filepath.Join(tmpDir, "bench.db"))
	return 0
}

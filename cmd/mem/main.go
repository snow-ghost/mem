package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	gomcp "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/embeddings"
	"github.com/snow-ghost/mem/internal/kg"
	"github.com/snow-ghost/mem/internal/layers"
	mcpserver "github.com/snow-ghost/mem/internal/mcp"
	"github.com/snow-ghost/mem/internal/palace"
	"github.com/snow-ghost/mem/internal/search"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	if len(os.Args) < 2 {
		usage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		os.Exit(runInit(os.Args[2:]))
	case "mine":
		os.Exit(runMine(os.Args[2:]))
	case "search":
		os.Exit(runSearch(os.Args[2:]))
	case "status":
		os.Exit(runStatus(os.Args[2:]))
	case "wake-up":
		os.Exit(runWakeUp(os.Args[2:]))
	case "kg":
		os.Exit(runKG(os.Args[2:]))
	case "mcp":
		os.Exit(runMCP(os.Args[2:]))
	case "reindex":
		os.Exit(runReindex(os.Args[2:]))
	case "benchmark":
		os.Exit(runBenchmark(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "mem: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: mem <command> [flags]

Commands:
  init         Initialize memory palace
  mine         Ingest files into palace
  search       Search memories (--mode bm25|vector|hybrid)
  status       Palace overview
  wake-up      Load compact context for AI session
  kg           Knowledge graph operations
  reindex      Compute embeddings for all drawers (requires MEM_EMBEDDINGS_*)
  benchmark    Time BM25/vector/hybrid/HNSW on synthetic data
  mcp          Start MCP server`)
}

func runStub(cmd string) int {
	fmt.Fprintf(os.Stderr, "mem: %s: not yet implemented\n", cmd)
	return 1
}

func resolveConfig(args []string) config.Config {
	cfg := config.Load()
	fs := flag.NewFlagSet("", flag.ContinueOnError)
	var palaceFlag string
	fs.StringVar(&palaceFlag, "palace", "", "override palace path")
	fs.Parse(args)
	if palaceFlag != "" {
		cfg.PalacePath = palaceFlag
	}
	return cfg
}

func runInit(args []string) int {
	cfg := resolveConfig(args)
	d, err := palace.Init(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: init: %v\n", err)
		return 1
	}
	d.Close()
	fmt.Printf("Initialized palace at %s\n", cfg.PalacePath)
	return 0
}

func runStatus(args []string) int {
	cfg := resolveConfig(args)
	d, err := palace.Init(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: status: %v\n", err)
		return 1
	}
	defer d.Close()

	s, err := palace.GetStatus(d, cfg.PalacePath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: status: %v\n", err)
		return 1
	}

	fmt.Printf("Palace: %s\n", s.PalacePath)
	fmt.Printf("  Wings:   %d\n", s.Wings)
	fmt.Printf("  Rooms:   %d\n", s.Rooms)
	fmt.Printf("  Drawers: %d\n", s.Drawers)
	return 0
}

func runMine(args []string) int {
	var wingFlag, modeFlag string
	var noEmbed bool
	fs := flag.NewFlagSet("mine", flag.ContinueOnError)
	fs.StringVar(&wingFlag, "wing", "", "wing name")
	fs.StringVar(&modeFlag, "mode", "files", "mining mode: files or convos")
	fs.BoolVar(&noEmbed, "no-embed", false, "skip embedding new drawers even if MEM_EMBEDDINGS_* is set")
	var palaceFlag string
	fs.StringVar(&palaceFlag, "palace", "", "override palace path")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "mem: mine: directory argument required")
		return 1
	}
	dir := fs.Arg(0)

	cfg := config.Load()
	if palaceFlag != "" {
		cfg.PalacePath = palaceFlag
	}

	d, err := palace.Init(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: mine: %v\n", err)
		return 1
	}
	defer d.Close()

	if wingFlag == "" {
		wingFlag = filepath.Base(dir)
	}
	wing, err := palace.CreateWing(d, wingFlag, "project", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: mine: %v\n", err)
		return 1
	}

	var filesProcessed, drawersCreated, duplicatesSkipped int

	textExts := map[string]bool{
		".go": true, ".py": true, ".js": true, ".ts": true, ".md": true,
		".txt": true, ".yaml": true, ".yml": true, ".json": true, ".toml": true,
		".rs": true, ".java": true, ".c": true, ".h": true, ".cpp": true,
		".rb": true, ".php": true, ".sh": true, ".css": true, ".html": true,
		".sql": true, ".xml": true, ".csv": true, ".log": true, ".env": true,
		".cfg": true, ".ini": true, ".conf": true, ".jsonl": true,
	}

	type pendingIndex struct {
		ID      int64
		Content string
	}
	var toIndex []pendingIndex

	filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		if info.Size() > 10*1024*1024 || info.Size() == 0 {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !textExts[ext] && ext != "" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		content := string(data)
		if len(content) < 10 {
			return nil
		}

		filesProcessed++

		roomName := detectRoom(content)
		room, err := palace.CreateRoom(d, roomName, wing.ID)
		if err != nil {
			return nil
		}

		drawer, err := palace.AddDrawer(d, content, wing.ID, room.ID, "facts", path, "file")
		if err != nil {
			return nil
		}
		if drawer == nil {
			duplicatesSkipped++
			return nil
		}

		drawersCreated++
		toIndex = append(toIndex, pendingIndex{ID: drawer.ID, Content: content})
		return nil
	})

	// Batch index all new drawers in a single transaction
	items := make([]struct{ ID int64; Content string }, len(toIndex))
	for i, p := range toIndex {
		items[i] = struct{ ID int64; Content string }{p.ID, p.Content}
	}
	if err := search.IndexBatch(d, items); err != nil {
		fmt.Fprintf(os.Stderr, "mem: mine: index: %v\n", err)
	}

	fmt.Printf("Mined %s into wing %q\n", dir, wingFlag)
	fmt.Printf("  Files processed: %d\n", filesProcessed)
	fmt.Printf("  Drawers created: %d\n", drawersCreated)
	fmt.Printf("  Duplicates skipped: %d\n", duplicatesSkipped)

	// Auto-embed new drawers if an embeddings provider is configured.
	// Skip with --no-embed to keep mining offline.
	if !noEmbed && cfg.EmbeddingsEnabled() && len(toIndex) > 0 {
		client := embeddings.NewClient(cfg)
		texts := make([]string, len(toIndex))
		for i, p := range toIndex {
			texts[i] = p.Content
		}
		fmt.Printf("  Embedding %d new drawers (model: %s)...\n", len(toIndex), cfg.EmbeddingsModel)
		start := time.Now()
		vecs, failed, err := client.EmbedAll(texts, 4, 8, nil)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: mine: embed: %v\n", err)
			return 0 // Mining still succeeded; embedding can be retried via `mem reindex`.
		}
		stored := 0
		for i, v := range vecs {
			if v != nil {
				if err := search.IndexEmbedding(d, toIndex[i].ID, embeddings.Encode(v)); err == nil {
					stored++
				}
			}
		}
		fmt.Printf("  Embedded: %d (failed: %d) in %s\n",
			stored, failed, time.Since(start).Round(time.Millisecond))
	}
	return 0
}

func runSearch(args []string) int {
	var wingFlag, roomFlag, modeFlag string
	var limitFlag int
	var hnswFlag, perTypeFlag bool
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.StringVar(&wingFlag, "wing", "", "filter by wing")
	fs.StringVar(&roomFlag, "room", "", "filter by room")
	fs.StringVar(&modeFlag, "mode", "bm25", "search mode: bm25, vector, or hybrid")
	fs.IntVar(&limitFlag, "limit", 5, "max results")
	fs.BoolVar(&hnswFlag, "hnsw", false, "use HNSW index for vector search (faster on >5k drawers)")
	fs.BoolVar(&perTypeFlag, "per-type", false, "in hybrid mode, classify the query and pick the per-type RRF weight")
	var palaceFlag string
	fs.StringVar(&palaceFlag, "palace", "", "override palace path")
	fs.Parse(args)

	if fs.NArg() < 1 {
		fmt.Fprintln(os.Stderr, "mem: search: query argument required")
		return 1
	}
	query := strings.Join(fs.Args(), " ")

	cfg := config.Load()
	if palaceFlag != "" {
		cfg.PalacePath = palaceFlag
	}

	d, err := palace.Init(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: search: %v\n", err)
		return 1
	}
	defer d.Close()

	var wingID, roomID int64
	if wingFlag != "" {
		w, err := palace.GetWing(d, wingFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: search: wing %q not found\n", wingFlag)
			return 1
		}
		wingID = w.ID
	}
	if roomFlag != "" && wingID > 0 {
		r, err := palace.GetRoom(d, roomFlag, wingID)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: search: room %q not found\n", roomFlag)
			return 1
		}
		roomID = r.ID
	}

	var results []search.SearchResult
	switch modeFlag {
	case "vector", "hybrid":
		if !cfg.EmbeddingsEnabled() {
			fmt.Fprintln(os.Stderr, "mem: search: MEM_EMBEDDINGS_URL and MEM_EMBEDDINGS_MODEL must be set for vector/hybrid mode")
			return 1
		}
		client := embeddings.NewClient(cfg)
		qvec, err := client.Embed(query)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: search: embed query: %v\n", err)
			return 1
		}
		switch {
		case hnswFlag && modeFlag == "vector":
			idx, ierr := search.LoadOrBuildHNSW(d, "default")
			if ierr != nil || idx == nil {
				fmt.Fprintln(os.Stderr, "mem: search: HNSW build failed, falling back to full scan")
				results, err = search.SearchVector(d, qvec, wingID, roomID, limitFlag)
			} else {
				results, err = search.SearchHNSW(d, idx, qvec, limitFlag)
			}
		case modeFlag == "vector":
			results, err = search.SearchVector(d, qvec, wingID, roomID, limitFlag)
		case modeFlag == "hybrid" && perTypeFlag:
			results, err = search.SearchHybridAuto(d, query, qvec, wingID, roomID, limitFlag, nil)
		default:
			results, err = search.SearchHybrid(d, query, qvec, wingID, roomID, limitFlag)
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: search: %v\n", err)
			return 1
		}
	default:
		results, err = search.Search(d, query, wingID, roomID, limitFlag)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: search: %v\n", err)
			return 1
		}
	}

	if len(results) == 0 {
		fmt.Printf("No results for %q\n", query)
		return 0
	}

	fmt.Printf("Results for %q (%d found):\n\n", query, len(results))
	for i, r := range results {
		fmt.Printf("[%d] %s / %s  (score: %.3f)\n", i+1, r.WingName, r.RoomName, r.Score)
		fmt.Printf("    Source: %s\n", filepath.Base(r.SourceFile))
		snippet := r.Content
		if len(snippet) > 200 {
			snippet = snippet[:200] + "..."
		}
		snippet = strings.ReplaceAll(snippet, "\n", " ")
		fmt.Printf("    %s\n\n", snippet)
	}
	return 0
}

func detectRoom(content string) string {
	tokens := search.Tokenize(content)
	if len(tokens) == 0 {
		return "general"
	}
	freq := search.TokenFrequency(tokens)

	type kv struct {
		k string
		v float64
	}
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
		if i >= 2 {
			break
		}
		parts = append(parts, kv.k)
	}
	if len(parts) == 0 {
		return "general"
	}
	return strings.Join(parts, "-")
}

func runWakeUp(args []string) int {
	var wingFlag, palaceFlag string
	fs := flag.NewFlagSet("wake-up", flag.ContinueOnError)
	fs.StringVar(&wingFlag, "wing", "", "filter by wing")
	fs.StringVar(&palaceFlag, "palace", "", "override palace path")
	fs.Parse(args)

	cfg := config.Load()
	if palaceFlag != "" {
		cfg.PalacePath = palaceFlag
	}
	d, err := palace.Init(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: wake-up: %v\n", err)
		return 1
	}
	defer d.Close()

	fmt.Print(layers.WakeUp(d, cfg, wingFlag))
	fmt.Println()
	return 0
}

func runKG(args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(os.Stderr, "mem: kg: subcommand required (add, query, timeline, invalidate, stats)")
		return 1
	}

	subcmd := args[0]
	subargs := args[1:]

	cfg := config.Load()
	// Check for --palace flag in remaining args
	for i, a := range subargs {
		if a == "--palace" && i+1 < len(subargs) {
			cfg.PalacePath = subargs[i+1]
		}
	}

	d, err := palace.Init(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: kg: %v\n", err)
		return 1
	}
	defer d.Close()

	switch subcmd {
	case "add":
		if len(subargs) < 3 {
			fmt.Fprintln(os.Stderr, "mem: kg add <subject> <predicate> <object> [--from DATE] [--to DATE]")
			return 1
		}
		var from, to string
		fs := flag.NewFlagSet("kg-add", flag.ContinueOnError)
		fs.StringVar(&from, "from", "", "valid from date")
		fs.StringVar(&to, "to", "", "valid to date")
		fs.Parse(subargs[3:])

		conflicts, _ := kg.CheckContradiction(d, subargs[0], subargs[1], subargs[2])
		if len(conflicts) > 0 {
			for _, c := range conflicts {
				fmt.Fprintf(os.Stderr, "  CONFLICT: %s %s %s (existing: %s)\n", c.Subject, c.Predicate, c.NewObj, c.ExistingObj)
			}
		}

		id, err := kg.AddTriple(d, subargs[0], subargs[1], subargs[2], from, to)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: kg add: %v\n", err)
			return 1
		}
		fmt.Printf("Added: %s → %s → %s (id: %s)\n", subargs[0], subargs[1], subargs[2], id[:12])

	case "query":
		if len(subargs) < 1 {
			fmt.Fprintln(os.Stderr, "mem: kg query <entity> [--as-of DATE]")
			return 1
		}
		var asOf string
		fs := flag.NewFlagSet("kg-query", flag.ContinueOnError)
		fs.StringVar(&asOf, "as-of", "", "query as of date")
		fs.Parse(subargs[1:])

		results, err := kg.QueryEntity(d, subargs[0], asOf, "both")
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: kg query: %v\n", err)
			return 1
		}
		if len(results) == 0 {
			fmt.Printf("No facts found for %q\n", subargs[0])
			return 0
		}
		for _, r := range results {
			status := "current"
			if !r.Current {
				status = "ended:" + r.ValidTo
			}
			fmt.Printf("  %s → %s → %s  [%s] (%s)\n", r.SubjName, r.Predicate, r.ObjName, status, r.ValidFrom)
		}

	case "timeline":
		if len(subargs) < 1 {
			fmt.Fprintln(os.Stderr, "mem: kg timeline <entity>")
			return 1
		}
		results, err := kg.Timeline(d, subargs[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: kg timeline: %v\n", err)
			return 1
		}
		for _, r := range results {
			period := r.ValidFrom
			if r.ValidTo != "" {
				period += " → " + r.ValidTo
			} else {
				period += " → now"
			}
			fmt.Printf("  [%s] %s %s %s\n", period, r.SubjName, r.Predicate, r.ObjName)
		}

	case "invalidate":
		if len(subargs) < 3 {
			fmt.Fprintln(os.Stderr, "mem: kg invalidate <subject> <predicate> <object> --ended DATE")
			return 1
		}
		var ended string
		fs := flag.NewFlagSet("kg-inv", flag.ContinueOnError)
		fs.StringVar(&ended, "ended", "", "end date")
		fs.Parse(subargs[3:])
		if err := kg.Invalidate(d, subargs[0], subargs[1], subargs[2], ended); err != nil {
			fmt.Fprintf(os.Stderr, "mem: kg invalidate: %v\n", err)
			return 1
		}
		fmt.Printf("Invalidated: %s → %s → %s\n", subargs[0], subargs[1], subargs[2])

	case "stats":
		entities, triples, current, _ := kg.Stats(d)
		fmt.Printf("Knowledge Graph:\n  Entities: %d\n  Triples: %d (current: %d, expired: %d)\n",
			entities, triples, current, triples-current)

	default:
		fmt.Fprintf(os.Stderr, "mem: kg: unknown subcommand %q\n", subcmd)
		return 1
	}
	return 0
}

func runReindex(args []string) int {
	var palaceFlag string
	var batchSize int
	fs := flag.NewFlagSet("reindex", flag.ContinueOnError)
	fs.StringVar(&palaceFlag, "palace", "", "override palace path")
	fs.IntVar(&batchSize, "batch", 64, "embeddings batch size")
	fs.Parse(args)

	cfg := config.Load()
	if palaceFlag != "" {
		cfg.PalacePath = palaceFlag
	}
	if !cfg.EmbeddingsEnabled() {
		fmt.Fprintln(os.Stderr, "mem: reindex: set MEM_EMBEDDINGS_URL and MEM_EMBEDDINGS_MODEL (optional MEM_EMBEDDINGS_API_KEY)")
		return 1
	}

	d, err := palace.Init(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: reindex: %v\n", err)
		return 1
	}
	defer d.Close()

	client := embeddings.NewClient(cfg)
	alreadyDone, _ := search.CountDrawersWithEmbeddings(d)

	var embedded int
	for {
		pending, err := search.ListDrawersWithoutEmbeddings(d, batchSize)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: reindex: list: %v\n", err)
			return 1
		}
		if len(pending) == 0 {
			break
		}

		texts := make([]string, len(pending))
		for i, p := range pending {
			texts[i] = p.Content
		}
		vecs, err := client.EmbedBatch(texts)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: reindex: embed: %v\n", err)
			return 1
		}

		for i, p := range pending {
			blob := embeddings.Encode(vecs[i])
			if err := search.IndexEmbedding(d, p.ID, blob); err != nil {
				fmt.Fprintf(os.Stderr, "mem: reindex: store %d: %v\n", p.ID, err)
				return 1
			}
			embedded++
		}
		fmt.Printf("  embedded %d drawers...\n", embedded)
	}

	fmt.Printf("Reindex complete: %d new embeddings (was %d, now %d)\n",
		embedded, alreadyDone, alreadyDone+embedded)
	return 0
}

func runMCP(args []string) int {
	var palaceFlag string
	fs := flag.NewFlagSet("mcp", flag.ContinueOnError)
	fs.StringVar(&palaceFlag, "palace", "", "override palace path")
	fs.Parse(args)

	cfg := config.Load()
	if palaceFlag != "" {
		cfg.PalacePath = palaceFlag
	}
	d, err := palace.Init(cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: mcp: %v\n", err)
		return 1
	}
	defer d.Close()

	s := mcpserver.NewServer(d, cfg)
	if err := s.Run(context.Background(), &gomcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "mem: mcp: %v\n", err)
		return 1
	}
	return 0
}

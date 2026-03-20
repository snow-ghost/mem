package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/snow-ghost/mem/internal/agent"
	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/consolidation"
	"github.com/snow-ghost/mem/internal/episode"
	"github.com/snow-ghost/mem/internal/principle"
	"github.com/snow-ghost/mem/internal/runner"
	"github.com/snow-ghost/mem/internal/skill"
	"github.com/snow-ghost/mem/internal/store"
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
	case "extract":
		os.Exit(runExtract(os.Args[2:]))
	case "consolidate":
		os.Exit(runConsolidate(os.Args[2:]))
	case "inject":
		os.Exit(runInject(os.Args[2:]))
	case "status":
		os.Exit(runStatus(os.Args[2:]))
	default:
		fmt.Fprintf(os.Stderr, "mem: unknown command %q\n", os.Args[1])
		usage()
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `Usage: mem <command> [flags]

Commands:
  init         Initialize memory store
  extract      Capture events from last session
  consolidate  Consolidate episodes into principles
  inject       Output memory context for new session
  status       Show memory store statistics`)
}

func runInit(args []string) int {
	cfg := config.Load()
	var pathFlag string
	fs := flag.NewFlagSet("init", flag.ContinueOnError)
	fs.StringVar(&pathFlag, "path", "", "override memory store path")
	fs.Parse(args)
	if pathFlag != "" {
		cfg.MemPath = pathFlag
	}
	absPath, _ := filepath.Abs(cfg.MemPath)
	s := store.New(absPath)

	if err := s.Init(); err != nil {
		fmt.Fprintf(os.Stderr, "mem: init: %v\n", err)
		return 1
	}
	fmt.Printf("Initialized memory store at %s\n", s.Root)
	fmt.Println("Created: episodes.jsonl, principles.md, skills/, consolidation-log.md, prompts/")
	fmt.Println()
	fmt.Println("To enable automatic extraction after each session, add a hook to your agent's config:")
	fmt.Println(`  command: "mem extract"`)
	fmt.Println()
	fmt.Println("Supported backends: claude, opencode, codex (auto-detected)")
	fmt.Println("Set MEM_BACKEND to choose explicitly, or mem will auto-detect.")
	return 0
}

func runStatus(args []string) int {
	var jsonFlag bool
	var pathFlag, backendFlag string
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.BoolVar(&jsonFlag, "json", false, "output as JSON")
	fs.StringVar(&pathFlag, "path", "", "override memory store path")
	fs.StringVar(&backendFlag, "backend", "", "override backend")
	fs.Parse(args)

	cfg := config.Load()
	if pathFlag != "" {
		cfg.MemPath = pathFlag
	}
	absPath, _ := filepath.Abs(cfg.MemPath)
	s := store.New(absPath)

	epCount, err := episode.Count(s.EpisodesPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: status: %v\n", err)
		return 2
	}

	principles, err := principle.Parse(s.PrinciplesPath())
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: status: %v\n", err)
		return 2
	}
	princCount := principle.Count(principles)

	skills, err := skill.List(s.SkillsDir())
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: status: %v\n", err)
		return 2
	}

	sessCount, _ := s.ReadSessionCount()
	storeSize, _ := s.StoreSize()
	lastEntry, _ := consolidation.ReadLast(s.ConsolidationLogPath())

	backendName := "(none)"
	backendSource := ""
	if b, err := agent.Resolve(cfg, backendFlag); err == nil {
		backendName = b.Name
		backendSource = b.Source
	}

	if jsonFlag {
		out := map[string]any{
			"path":               s.Root,
			"episodes":           map[string]any{"count": epCount, "max": cfg.EpisodesMax},
			"principles":         map[string]any{"count": princCount, "max": cfg.PrinciplesMax},
			"skills":             len(skills),
			"session_count":      map[string]any{"current": sessCount, "threshold": cfg.SessionThreshold},
			"last_consolidation": lastEntry.Date,
			"store_size_bytes":   storeSize,
			"backend":            map[string]any{"name": backendName, "source": backendSource},
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		enc.Encode(out)
		return 0
	}

	fmt.Printf("Memory Store: %s\n", s.Root)
	fmt.Printf("  Episodes:       %d / %d\n", epCount, cfg.EpisodesMax)
	fmt.Printf("  Principles:     %d / %d\n", princCount, cfg.PrinciplesMax)
	fmt.Printf("  Skills:         %d\n", len(skills))
	fmt.Printf("  Session count:  %d / %d (next consolidation at %d)\n", sessCount, cfg.SessionThreshold, cfg.SessionThreshold)
	if lastEntry.Date != "" {
		fmt.Printf("  Last consolidation: %s\n", lastEntry.Date)
	}
	fmt.Printf("  Store size:     %d bytes\n", storeSize)
	if backendSource != "" {
		fmt.Printf("  Backend:        %s (%s)\n", backendName, backendSource)
	}
	return 0
}

func runExtract(args []string) int {
	var sessionFlag, modelFlag, pathFlag, backendFlag string
	var dryRunFlag bool
	fs := flag.NewFlagSet("extract", flag.ContinueOnError)
	fs.StringVar(&sessionFlag, "session", "", "session ID")
	fs.StringVar(&modelFlag, "model", "haiku", "LLM model")
	fs.BoolVar(&dryRunFlag, "dry-run", false, "print without writing")
	fs.StringVar(&pathFlag, "path", "", "override memory store path")
	fs.StringVar(&backendFlag, "backend", "", "override backend (claude, opencode, codex, custom)")
	fs.Parse(args)

	cfg := config.Load()
	if pathFlag != "" {
		cfg.MemPath = pathFlag
	}
	absPath, _ := filepath.Abs(cfg.MemPath)
	s := store.New(absPath)

	if sessionFlag == "" {
		sessionFlag = runner.GetGitShortHash()
	}

	backend, err := agent.Resolve(cfg, backendFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: extract: %v\n", err)
		return 1
	}
	inv := agent.NewInvoker(backend)

	result, err := runner.RunExtract(cfg, s, inv, sessionFlag, modelFlag, dryRunFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: extract: %v\n", err)
		return 1
	}

	if result.NewCount == 0 {
		fmt.Println("No significant events found.")
	} else {
		action := "Extracted"
		if dryRunFlag {
			action = "Would extract"
		}
		fmt.Printf("%s %d episodes from session %s\n", action, result.NewCount, sessionFlag)
		for _, ep := range result.Episodes {
			fmt.Printf("  [%s] %s\n", ep.Type, ep.Summary)
		}
	}
	fmt.Printf("Session count: %d/%d", result.SessionCount, cfg.SessionThreshold)
	if result.ThresholdReached {
		fmt.Print(" — consolidation recommended (run: mem consolidate)")
	}
	fmt.Println()
	return 0
}

func runConsolidate(args []string) int {
	var modelFlag, pathFlag, backendFlag string
	var dryRunFlag, forceFlag bool
	fs := flag.NewFlagSet("consolidate", flag.ContinueOnError)
	fs.StringVar(&modelFlag, "model", "sonnet", "LLM model")
	fs.BoolVar(&dryRunFlag, "dry-run", false, "show proposed changes without applying")
	fs.BoolVar(&forceFlag, "force", false, "run even if thresholds not reached")
	fs.StringVar(&pathFlag, "path", "", "override memory store path")
	fs.StringVar(&backendFlag, "backend", "", "override backend (claude, opencode, codex, custom)")
	fs.Parse(args)

	cfg := config.Load()
	if pathFlag != "" {
		cfg.MemPath = pathFlag
	}
	absPath, _ := filepath.Abs(cfg.MemPath)
	s := store.New(absPath)

	backend, err := agent.Resolve(cfg, backendFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: consolidate: %v\n", err)
		return 1
	}
	inv := agent.NewInvoker(backend)

	result, err := runner.RunConsolidate(cfg, s, inv, modelFlag, dryRunFlag, forceFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: consolidate: %v\n", err)
		return 1
	}

	if result.Skipped {
		fmt.Fprintln(os.Stderr, "mem: consolidate: thresholds not reached (use --force to override)")
		return 3
	}

	action := "Consolidation"
	if dryRunFlag {
		action = "Dry run"
	}
	fmt.Printf("%s complete\n", action)
	fmt.Printf("  Episodes processed: %d\n", result.EpisodesProcessed)
	fmt.Printf("  Principles added: %d\n", result.PrinciplesAdded)
	fmt.Printf("  Episodes removed: %d\n", result.EpisodesRemoved)
	fmt.Printf("  Skills created: %d\n", result.SkillsCreated)
	if len(result.SkillCandidates) > 0 {
		fmt.Printf("  Skill candidates: %s\n", strings.Join(result.SkillCandidates, ", "))
	}
	if len(result.Conflicts) > 0 {
		fmt.Printf("  Conflicts: %d (review recommended)\n", len(result.Conflicts))
	}
	return 0
}

func runInject(args []string) int {
	var episodesFlag int
	var formatFlag, pathFlag string
	fs := flag.NewFlagSet("inject", flag.ContinueOnError)
	fs.IntVar(&episodesFlag, "episodes", 10, "number of recent episodes")
	fs.StringVar(&formatFlag, "format", "markdown", "output format: markdown or json")
	fs.StringVar(&pathFlag, "path", "", "override memory store path")
	fs.Parse(args)

	cfg := config.Load()
	if pathFlag != "" {
		cfg.MemPath = pathFlag
	}
	absPath, _ := filepath.Abs(cfg.MemPath)
	s := store.New(absPath)

	ctx, err := runner.RunInject(cfg, s, episodesFlag)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mem: inject: %v\n", err)
		return 1
	}

	switch formatFlag {
	case "json":
		out, err := runner.FormatJSON(ctx)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mem: inject: %v\n", err)
			return 1
		}
		fmt.Println(out)
	default:
		fmt.Print(runner.FormatMarkdown(ctx))
	}
	return 0
}

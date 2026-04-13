package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/palace"
	"github.com/snow-ghost/mem/internal/search"
)

type Message struct {
	Speaker string `json:"speaker"`
	Text    string `json:"text"`
}

type Conversation struct {
	ID               string    `json:"id"`
	ContainsEvidence bool      `json:"containsEvidence"`
	Messages         []Message `json:"messages"`
}

type EvidenceItem struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
	Category string `json:"category"`
}

type TestCase struct {
	EvidenceItems []EvidenceItem `json:"evidenceItems"`
	Conversations []Conversation `json:"conversations"`
	ContextSize   int            `json:"contextSize"`
}

type sizeMetrics struct {
	hit1, hit5, hit10, total            int
	strictHit1, strictHit5, strictHit10 int // "latest evidence conv only" — meaningful for changing_evidence
	indexTime                           time.Duration
	searchTime                          time.Duration
	totalDrawers                        int
}

func main() {
	dataDir := "/tmp/convomem-data"
	if len(os.Args) > 1 {
		dataDir = os.Args[1]
	}

	recencyWeight := 0.0
	if v := os.Getenv("CONVOMEM_RECENCY"); v != "" {
		fmt.Sscanf(v, "%f", &recencyWeight)
	}

	fmt.Println("=== ConvoMem Benchmark for mem ===")
	fmt.Printf("Dataset dir: %s\n", dataDir)
	if recencyWeight != 0 {
		fmt.Printf("Recency boost: %.2f (CONVOMEM_RECENCY)\n", recencyWeight)
	}
	fmt.Println()

	entries, err := os.ReadDir(dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading dir: %v\n", err)
		os.Exit(1)
	}

	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), "batch") && strings.HasSuffix(e.Name(), ".json") {
			files = append(files, filepath.Join(dataDir, e.Name()))
		}
	}
	sort.Strings(files)
	fmt.Printf("Found %d batch files\n\n", len(files))

	metricsBySize := make(map[int]*sizeMetrics)

	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			fmt.Fprintf(os.Stderr, "read %s: %v\n", f, err)
			continue
		}
		var tests []TestCase
		if err := json.Unmarshal(data, &tests); err != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", f, err)
			continue
		}
		if len(tests) == 0 {
			continue
		}

		cs := tests[0].ContextSize
		m, ok := metricsBySize[cs]
		if !ok {
			m = &sizeMetrics{}
			metricsBySize[cs] = m
		}

		fmt.Printf("  %s (contextSize=%d, %d tests)...", filepath.Base(f), cs, len(tests))
		fileStart := time.Now()

		for tcIdx, tc := range tests {
			palacePath := filepath.Join(os.TempDir(), fmt.Sprintf("convomem-palace-%d", tcIdx))
			os.RemoveAll(palacePath)

			cfg := config.Config{PalacePath: palacePath, DbFile: "palace.db"}
			d, err := palace.Init(cfg)
			if err != nil {
				fmt.Fprintf(os.Stderr, "init palace: %v\n", err)
				continue
			}
			wing, _ := palace.CreateWing(d, "convomem", "benchmark", "")

			// Build conv_id → containsEvidence map. For changing_evidence
			// cases there are multiple containsEvidence=true conversations
			// (each holds an older or newer version of the fact); the
			// LATEST one is the canonical answer. We track both the full
			// set and the last conv id so the bench can report a strict
			// "latest only" hit alongside the lenient "any evidence conv".
			evidenceConvs := make(map[string]bool)
			var latestEvidence string
			for _, c := range tc.Conversations {
				if c.ContainsEvidence {
					evidenceConvs[c.ID] = true
					latestEvidence = c.ID
				}
			}
			_ = latestEvidence // also used below in stricter accounting

			indexStart := time.Now()
			var batchItems []struct {
				ID      int64
				Content string
			}

			// One drawer per conversation (whole-conv chunking wins per LongMemEval/LoCoMo)
			for _, c := range tc.Conversations {
				var parts []string
				for _, msg := range c.Messages {
					parts = append(parts, msg.Speaker+": "+msg.Text)
				}
				content := strings.Join(parts, "\n")
				if len(content) < 5 {
					continue
				}
				roomName := detectRoom(content)
				room, _ := palace.CreateRoom(d, roomName, wing.ID)
				drawer, _ := palace.AddDrawer(d, content, wing.ID, room.ID, "facts", c.ID, "conversation")
				if drawer != nil {
					batchItems = append(batchItems, struct {
						ID      int64
						Content string
					}{drawer.ID, content})
				}
			}
			search.IndexBatch(d, batchItems)
			m.indexTime += time.Since(indexStart)
			m.totalDrawers += len(batchItems)

			// Run all evidence questions for this test case. Two metrics:
			//   - hit*       : any conv that has the evidence (lenient)
			//   - strictHit* : only the LATEST evidence conv counts
			//                  (correct metric for changing_evidence)
			searchStart := time.Now()
			for _, ev := range tc.EvidenceItems {
				m.total++
				results, _ := search.Search(d, ev.Question, 0, 0, 10)
				if recencyWeight != 0 {
					results = search.ApplyRecencyBoost(results, recencyWeight)
				}

				lenientHit := -1
				strictHit := -1
				for i, r := range results {
					if lenientHit < 0 && evidenceConvs[r.SourceFile] {
						lenientHit = i
					}
					if strictHit < 0 && r.SourceFile == latestEvidence {
						strictHit = i
					}
				}
				if lenientHit >= 0 {
					if lenientHit == 0 {
						m.hit1++
					}
					if lenientHit < 5 {
						m.hit5++
					}
					if lenientHit < 10 {
						m.hit10++
					}
				}
				if strictHit >= 0 {
					if strictHit == 0 {
						m.strictHit1++
					}
					if strictHit < 5 {
						m.strictHit5++
					}
					if strictHit < 10 {
						m.strictHit10++
					}
				}
			}
			m.searchTime += time.Since(searchStart)

			d.Close()
			os.RemoveAll(palacePath)
		}
		fmt.Printf(" %s\n", time.Since(fileStart).Round(time.Millisecond))
	}

	// Sort context sizes for report
	var sizes []int
	for cs := range metricsBySize {
		sizes = append(sizes, cs)
	}
	sort.Ints(sizes)

	fmt.Printf("\n=== RESULTS BY CONTEXT SIZE (lenient: any evidence conv) ===\n")
	fmt.Printf("%-12s %-8s %-8s %-8s %-12s %-12s %s\n", "ContextSize", "R@1", "R@5", "R@10", "Index", "Search", "N")
	fmt.Printf("%-12s %-8s %-8s %-8s %-12s %-12s %s\n", "-----------", "---", "---", "----", "-----", "------", "-")

	var grandHit1, grandHit5, grandHit10, grandTotal int
	var grandStrictHit1, grandStrictHit5, grandStrictHit10 int
	var grandIndex, grandSearch time.Duration
	for _, cs := range sizes {
		m := metricsBySize[cs]
		fmt.Printf("%-12d %5.1f%%  %5.1f%%  %5.1f%%  %-12s %-12s %d\n",
			cs,
			pct(m.hit1, m.total),
			pct(m.hit5, m.total),
			pct(m.hit10, m.total),
			m.indexTime.Round(time.Millisecond),
			m.searchTime.Round(time.Millisecond),
			m.total,
		)
		grandHit1 += m.hit1
		grandHit5 += m.hit5
		grandHit10 += m.hit10
		grandStrictHit1 += m.strictHit1
		grandStrictHit5 += m.strictHit5
		grandStrictHit10 += m.strictHit10
		grandTotal += m.total
		grandIndex += m.indexTime
		grandSearch += m.searchTime
	}

	fmt.Printf("\n=== RESULTS BY CONTEXT SIZE (strict: latest evidence conv only) ===\n")
	fmt.Printf("%-12s %-8s %-8s %-8s %s\n", "ContextSize", "R@1", "R@5", "R@10", "N")
	for _, cs := range sizes {
		m := metricsBySize[cs]
		fmt.Printf("%-12d %5.1f%%  %5.1f%%  %5.1f%%  %d\n",
			cs,
			pct(m.strictHit1, m.total),
			pct(m.strictHit5, m.total),
			pct(m.strictHit10, m.total),
			m.total,
		)
	}

	fmt.Printf("\n=== TOTALS ===\n")
	fmt.Printf("Total test cases: %d\n", grandTotal)
	fmt.Printf("Lenient (any evidence conv):\n")
	fmt.Printf("  Recall@1:  %.1f%%   Recall@5: %.1f%%   Recall@10: %.1f%%\n",
		pct(grandHit1, grandTotal), pct(grandHit5, grandTotal), pct(grandHit10, grandTotal))
	fmt.Printf("Strict (latest evidence conv only — meaningful for changing_evidence):\n")
	fmt.Printf("  Recall@1:  %.1f%%   Recall@5: %.1f%%   Recall@10: %.1f%%\n",
		pct(grandStrictHit1, grandTotal), pct(grandStrictHit5, grandTotal), pct(grandStrictHit10, grandTotal))
	fmt.Printf("Index time: %s\n", grandIndex.Round(time.Millisecond))
	fmt.Printf("Search time: %s (avg: %s/query)\n",
		grandSearch.Round(time.Millisecond),
		(grandSearch / time.Duration(grandTotal)).Round(time.Microsecond))
}

func pct(n, d int) float64 {
	if d == 0 {
		return 0
	}
	return float64(n) / float64(d) * 100
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
	sort.Slice(top, func(i, j int) bool { return top[i].v > top[j].v })
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

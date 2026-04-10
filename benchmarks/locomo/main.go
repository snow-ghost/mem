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
	Speaker     string `json:"speaker"`
	DiaID       string `json:"dia_id"`
	Text        string `json:"text"`
	BlipCaption string `json:"blip_caption,omitempty"`
	Query       string `json:"query,omitempty"`
}

type QA struct {
	Question          string   `json:"question"`
	Answer            any      `json:"answer,omitempty"`
	AdversarialAnswer any      `json:"adversarial_answer,omitempty"`
	Evidence          []string `json:"evidence"`
	Category          int      `json:"category"`
}

type Conversation struct {
	SampleID     string                     `json:"sample_id"`
	QA           []QA                       `json:"qa"`
	Conversation map[string]json.RawMessage `json:"conversation"`
}

var categoryNames = map[int]string{
	1: "single-hop",
	2: "temporal",
	3: "multi-hop",
	4: "open-domain",
	5: "adversarial",
}

func main() {
	dataFile := "/tmp/locomo-data/locomo10.json"
	if len(os.Args) > 1 {
		dataFile = os.Args[1]
	}

	fmt.Println("=== LoCoMo Benchmark for mem ===")
	fmt.Printf("Dataset: %s\n\n", dataFile)

	data, err := os.ReadFile(dataFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error reading dataset: %v\n", err)
		os.Exit(1)
	}

	var convs []Conversation
	if err := json.Unmarshal(data, &convs); err != nil {
		fmt.Fprintf(os.Stderr, "Error parsing dataset: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("Loaded %d conversations\n", len(convs))

	totalStart := time.Now()

	// Aggregate metrics across all conversations (each conv has its own palace)
	var (
		totalQA                      int
		hit1, hit5, hit10            int
		catHits                      = make(map[int][3]int) // [hit1, hit5, total]
		totalIndexTime               time.Duration
		totalSearchTime              time.Duration
		totalDrawers, totalMessages  int
		adversarialCorrect           int
		adversarialTotal             int
	)

	for convIdx, conv := range convs {
		// Build session map: session_N -> messages, in order
		sessionKeys := sortedSessionKeys(conv.Conversation)

		// Create a fresh palace for this conversation
		palacePath := filepath.Join(os.TempDir(), fmt.Sprintf("locomo-palace-%d", convIdx))
		os.RemoveAll(palacePath)

		cfg := config.Config{PalacePath: palacePath, DbFile: "palace.db"}
		d, err := palace.Init(cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Init palace: %v\n", err)
			os.Exit(1)
		}

		wing, _ := palace.CreateWing(d, conv.SampleID, "benchmark", "")

		indexStart := time.Now()

		// Chunk into groups of CHUNK_SIZE messages. Each drawer's source_file is a CSV
		// of its constituent dia_ids so we can match any evidence dia_id.
		chunkSize := 50
		if v := os.Getenv("LOCOMO_CHUNK_SIZE"); v != "" {
			fmt.Sscanf(v, "%d", &chunkSize)
		}
		var batchItems []struct {
			ID      int64
			Content string
		}
		msgCount := 0
		for _, sk := range sessionKeys {
			var msgs []Message
			if err := json.Unmarshal(conv.Conversation[sk], &msgs); err != nil {
				continue
			}
			for i := 0; i < len(msgs); i += chunkSize {
				end := i + chunkSize
				if end > len(msgs) {
					end = len(msgs)
				}
				var parts []string
				var diaIDs []string
				for _, m := range msgs[i:end] {
					msgCount++
					line := m.Speaker + ": " + m.Text
					if m.BlipCaption != "" {
						line += " [image: " + m.BlipCaption + "]"
					}
					if m.Query != "" {
						line += " [query: " + m.Query + "]"
					}
					parts = append(parts, line)
					diaIDs = append(diaIDs, m.DiaID)
				}
				content := strings.Join(parts, "\n")
				if len(content) < 5 {
					continue
				}
				roomName := detectRoom(content)
				room, _ := palace.CreateRoom(d, roomName, wing.ID)
				sourceRef := strings.Join(diaIDs, ",")
				drawer, _ := palace.AddDrawer(d, content, wing.ID, room.ID, "facts", sourceRef, "conversation")
				if drawer != nil {
					batchItems = append(batchItems, struct {
						ID      int64
						Content string
					}{drawer.ID, content})
				}
			}
		}
		search.IndexBatch(d, batchItems)
		totalIndexTime += time.Since(indexStart)
		totalMessages += msgCount
		totalDrawers += len(batchItems)

		// Run QAs
		searchStart := time.Now()
		for _, qa := range conv.QA {
			if qa.Category == 5 {
				adversarialTotal++
				// For adversarial, we measure whether retrieval "avoids" or finds the trick.
				// We still run the query and count based on evidence match.
			}
			totalQA++

			results, _ := search.Search(d, qa.Question, 0, 0, 10)

			evSet := make(map[string]bool)
			for _, e := range qa.Evidence {
				evSet[e] = true
			}

			found1, found5, found10 := false, false, false
			for i, r := range results {
				if drawerContainsEvidence(r.SourceFile, evSet) {
					if i == 0 {
						found1 = true
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
			if found1 {
				hit1++
			}
			if found5 {
				hit5++
			}
			if found10 {
				hit10++
			}
			if qa.Category == 5 && found5 {
				adversarialCorrect++
			}

			counts := catHits[qa.Category]
			if found1 {
				counts[0]++
			}
			if found5 {
				counts[1]++
			}
			counts[2]++
			catHits[qa.Category] = counts
		}
		totalSearchTime += time.Since(searchStart)

		d.Close()
		os.RemoveAll(palacePath)
		fmt.Printf("  [%d/%d] %-10s drawers=%d QAs=%d\n", convIdx+1, len(convs), conv.SampleID, len(batchItems), len(conv.QA))
	}

	totalDuration := time.Since(totalStart)

	fmt.Printf("\n=== RESULTS ===\n")
	fmt.Printf("Total QAs: %d (across %d conversations)\n", totalQA, len(convs))
	fmt.Printf("Total messages indexed: %d (drawers: %d)\n", totalMessages, totalDrawers)
	fmt.Printf("Recall@1:  %.1f%% (%d/%d)\n", pct(hit1, totalQA), hit1, totalQA)
	fmt.Printf("Recall@5:  %.1f%% (%d/%d)\n", pct(hit5, totalQA), hit5, totalQA)
	fmt.Printf("Recall@10: %.1f%% (%d/%d)\n", pct(hit10, totalQA), hit10, totalQA)
	fmt.Printf("Index time: %s\n", totalIndexTime.Round(time.Millisecond))
	fmt.Printf("Search time: %s (avg: %s/query)\n",
		totalSearchTime.Round(time.Millisecond),
		(totalSearchTime / time.Duration(totalQA)).Round(time.Microsecond))
	fmt.Printf("Total: %s\n", totalDuration.Round(time.Millisecond))

	fmt.Printf("\n=== PER CATEGORY ===\n")
	var cats []int
	for c := range catHits {
		cats = append(cats, c)
	}
	sort.Ints(cats)
	for _, c := range cats {
		counts := catHits[c]
		name := categoryNames[c]
		fmt.Printf("  %-15s  R@1: %5.1f%%  R@5: %5.1f%%  (%d QAs)\n",
			name, pct(counts[0], counts[2]), pct(counts[1], counts[2]), counts[2])
	}

	if adversarialTotal > 0 {
		fmt.Printf("\nNote: adversarial (cat 5) retrievals counted as \"hit\" when the tricky\n")
		fmt.Printf("evidence message was retrieved — a high score here means the system was\n")
		fmt.Printf("tricked; a lower score would suggest better robustness. Current: %.1f%%\n",
			pct(adversarialCorrect, adversarialTotal))
	}

	// Excluding adversarial for fair comparison
	nonAdvTotal := totalQA - adversarialTotal
	nonAdvHit5 := hit5
	if counts, ok := catHits[5]; ok {
		nonAdvHit5 -= counts[1]
	}
	if nonAdvTotal > 0 {
		fmt.Printf("\n=== NON-ADVERSARIAL R@5 ===\n")
		fmt.Printf("  %.1f%% (%d/%d) — excluding cat 5\n",
			pct(nonAdvHit5, nonAdvTotal), nonAdvHit5, nonAdvTotal)
	}
}

func sortedSessionKeys(m map[string]json.RawMessage) []string {
	var keys []string
	for k := range m {
		if strings.HasPrefix(k, "session_") && !strings.HasSuffix(k, "_date_time") {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		return sessionIndex(keys[i]) < sessionIndex(keys[j])
	})
	return keys
}

func sessionIndex(k string) int {
	var n int
	fmt.Sscanf(k, "session_%d", &n)
	return n
}

func drawerContainsEvidence(sourceRef string, evSet map[string]bool) bool {
	if sourceRef == "" || len(evSet) == 0 {
		return false
	}
	for _, id := range strings.Split(sourceRef, ",") {
		if evSet[id] {
			return true
		}
	}
	return false
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

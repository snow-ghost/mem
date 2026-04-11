package embeddings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/snow-ghost/mem/internal/config"
)

// Client talks to an OpenAI-compatible /v1/embeddings endpoint.
// Works with OpenAI, Voyage AI, Together, Cohere (compat mode), Ollama (/api/embeddings),
// LM Studio, llama.cpp server, LocalAI, etc.
type Client struct {
	URL    string
	Model  string
	APIKey string
	HTTP   *http.Client
}

func NewClient(cfg config.Config) *Client {
	transport := &http.Transport{
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 16,
		MaxConnsPerHost:     16,
		IdleConnTimeout:     90 * time.Second,
	}
	return &Client{
		URL:    cfg.EmbeddingsURL,
		Model:  cfg.EmbeddingsModel,
		APIKey: cfg.EmbeddingsAPIKey,
		HTTP:   &http.Client{Timeout: 30 * time.Second, Transport: transport},
	}
}

type embedRequest struct {
	Input any    `json:"input"` // string or []string
	Model string `json:"model"`
}

type embedResponseData struct {
	Embedding []float32 `json:"embedding"`
	Index     int       `json:"index"`
}

type embedResponse struct {
	Data  []embedResponseData `json:"data"`
	Model string              `json:"model"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Embed returns the embedding for a single text.
func (c *Client) Embed(text string) ([]float32, error) {
	vecs, err := c.EmbedBatch([]string{text})
	if err != nil {
		return nil, err
	}
	if len(vecs) == 0 {
		return nil, fmt.Errorf("no embedding returned")
	}
	return vecs[0], nil
}

// EmbedBatch returns embeddings for a batch of texts. The response order
// matches the request order (OpenAI spec guarantees this via `index`).
//
// Robustness: retries transient failures (5xx, network, empty body) with
// exponential backoff. If a batch consistently fails, the caller should
// split it — the splitter is wired into EmbedBatchAdaptive.
func (c *Client) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	const maxAttempts = 4
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		vecs, err := c.embedOnce(texts)
		if err == nil {
			return vecs, nil
		}
		lastErr = err
		// 4xx errors that are not 408/429 are not worth retrying
		if !shouldRetry(err) {
			return nil, err
		}
		time.Sleep(time.Duration(attempt*attempt) * time.Second)
	}
	return nil, lastErr
}

// EmbedAll embeds an arbitrary number of texts using `workers` parallel
// goroutines, each processing chunks of `batchSize` items. The result order
// matches the input order. Calls EmbedBatchAdaptive per chunk so partial
// failures self-isolate.
//
// Failed chunks leave nil vectors at the corresponding indices instead of
// aborting the whole job — the caller decides whether to retry, fall back,
// or skip those entries. The total failure count is returned in `failed`.
//
// If progress is non-nil it is called after each chunk completes with
// (done, total). Useful for long-running benchmark indexing runs.
func (c *Client) EmbedAll(texts []string, batchSize, workers int, progress func(done, total int)) (out [][]float32, failed int, err error) {
	if len(texts) == 0 {
		return nil, 0, nil
	}
	if batchSize <= 0 {
		batchSize = 32
	}
	if workers <= 0 {
		workers = 1
	}

	type chunk struct {
		start int
		end   int
	}

	out = make([][]float32, len(texts))
	jobs := make(chan chunk)
	var wg sync.WaitGroup
	var doneCount int
	var failCount int
	var mu sync.Mutex

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				vecs, err := c.EmbedBatchAdaptive(texts[j.start:j.end])
				mu.Lock()
				if err != nil {
					failCount += j.end - j.start
				} else {
					for i, v := range vecs {
						out[j.start+i] = v
					}
				}
				doneCount += j.end - j.start
				doneNow := doneCount
				mu.Unlock()
				if progress != nil {
					progress(doneNow, len(texts))
				}
			}
		}()
	}

	for i := 0; i < len(texts); i += batchSize {
		end := i + batchSize
		if end > len(texts) {
			end = len(texts)
		}
		jobs <- chunk{start: i, end: end}
	}
	close(jobs)
	wg.Wait()

	return out, failCount, nil
}

// EmbedBatchAdaptive embeds texts with automatic batch splitting when a
// request fails. This handles per-input failures (e.g., one text exceeding
// the model context window) without aborting the whole job.
func (c *Client) EmbedBatchAdaptive(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	vecs, err := c.EmbedBatch(texts)
	if err == nil {
		return vecs, nil
	}
	if len(texts) == 1 {
		return nil, fmt.Errorf("single-text embed failed: %w", err)
	}
	mid := len(texts) / 2
	left, lerr := c.EmbedBatchAdaptive(texts[:mid])
	if lerr != nil {
		return nil, lerr
	}
	right, rerr := c.EmbedBatchAdaptive(texts[mid:])
	if rerr != nil {
		return nil, rerr
	}
	return append(left, right...), nil
}

func (c *Client) embedOnce(texts []string) ([][]float32, error) {
	reqBody, err := json.Marshal(embedRequest{Input: texts, Model: c.Model})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", c.URL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("embeddings request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("embeddings %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	if len(body) == 0 {
		return nil, fmt.Errorf("empty response body (likely server timeout)")
	}

	var parsed embedResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("embeddings error: %s", parsed.Error.Message)
	}

	result := make([][]float32, len(texts))
	for _, d := range parsed.Data {
		if d.Index < 0 || d.Index >= len(texts) {
			continue
		}
		result[d.Index] = d.Embedding
	}
	for i, v := range result {
		if v == nil {
			return nil, fmt.Errorf("missing embedding for input %d", i)
		}
	}
	return result, nil
}

func shouldRetry(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	if strings.Contains(msg, "empty response") ||
		strings.Contains(msg, "EOF") ||
		strings.Contains(msg, "connection") ||
		strings.Contains(msg, "timeout") ||
		strings.Contains(msg, "parse response") {
		return true
	}
	// HTTP status retry: 408, 429, 5xx
	for _, code := range []string{"408", "429", "500", "502", "503", "504"} {
		if strings.Contains(msg, "embeddings "+code) {
			return true
		}
	}
	return false
}

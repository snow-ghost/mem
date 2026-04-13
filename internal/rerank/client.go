package rerank

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/snow-ghost/mem/internal/config"
)

// Client talks to a Cohere-compatible /v1/rerank endpoint
// (Cohere, BAAI bge-reranker via cloud.ru/Voyage/etc.). The request shape
// matches the Cohere v1 reranker API: {model, query, documents}, response
// {results: [{index, relevance_score}, ...]}.
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
		URL:    cfg.RerankURL,
		Model:  cfg.RerankModel,
		APIKey: cfg.RerankAPIKey,
		HTTP:   &http.Client{Timeout: 60 * time.Second, Transport: transport},
	}
}

type rerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
}

type rerankResultItem struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

type rerankResponse struct {
	Results []rerankResultItem `json:"results"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Score returns relevance scores in the same order as the input documents.
// A failed call returns an error; the caller should fall back to the
// original ranking (e.g., hybrid scores).
func (c *Client) Score(query string, docs []string) ([]float64, error) {
	if len(docs) == 0 {
		return nil, nil
	}
	const maxAttempts = 3
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		scores, err := c.scoreOnce(query, docs)
		if err == nil {
			return scores, nil
		}
		lastErr = err
		if !shouldRetry(err) {
			return nil, err
		}
		time.Sleep(time.Duration(attempt*attempt) * time.Second)
	}
	return nil, lastErr
}

func (c *Client) scoreOnce(query string, docs []string) ([]float64, error) {
	body, err := json.Marshal(rerankRequest{Model: c.Model, Query: query, Documents: docs})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}
	req, err := http.NewRequest("POST", c.URL, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("rerank request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("rerank %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}
	if len(respBody) == 0 {
		return nil, fmt.Errorf("empty response body")
	}
	var parsed rerankResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("rerank error: %s", parsed.Error.Message)
	}

	scores := make([]float64, len(docs))
	for _, r := range parsed.Results {
		if r.Index < 0 || r.Index >= len(docs) {
			continue
		}
		scores[r.Index] = r.RelevanceScore
	}
	return scores, nil
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
	for _, code := range []string{"408", "429", "500", "502", "503", "504"} {
		if strings.Contains(msg, "rerank "+code) {
			return true
		}
	}
	return false
}

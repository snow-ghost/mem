package embeddings

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
	return &Client{
		URL:    cfg.EmbeddingsURL,
		Model:  cfg.EmbeddingsModel,
		APIKey: cfg.EmbeddingsAPIKey,
		HTTP:   &http.Client{Timeout: 60 * time.Second},
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
func (c *Client) EmbedBatch(texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}
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

	var parsed embedResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("embeddings error: %s", parsed.Error.Message)
	}

	// Sort by index (OpenAI usually returns in order, but be safe)
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

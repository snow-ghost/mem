package embeddings

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ChatClient talks to an OpenAI-compatible /v1/chat/completions endpoint.
// Used by Query2Doc / HyDE-style query expansion. Kept in the embeddings
// package to share the transport pool and retry logic, since it serves
// the same pipeline.
type ChatClient struct {
	URL    string
	Model  string
	APIKey string
	HTTP   *http.Client
}

func NewChatClient(url, model, apiKey string) *ChatClient {
	transport := &http.Transport{
		MaxIdleConns:        16,
		MaxIdleConnsPerHost: 16,
		MaxConnsPerHost:     16,
		IdleConnTimeout:     90 * time.Second,
	}
	return &ChatClient{
		URL:    url,
		Model:  model,
		APIKey: apiKey,
		HTTP:   &http.Client{Timeout: 60 * time.Second, Transport: transport},
	}
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model     string        `json:"model"`
	Messages  []chatMessage `json:"messages"`
	MaxTokens int           `json:"max_tokens,omitempty"`
}

type chatChoice struct {
	Message chatMessage `json:"message"`
}

type chatResponse struct {
	Choices []chatChoice `json:"choices"`
	Error   *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a single user message and returns the assistant's reply.
func (c *ChatClient) Complete(user string, maxTokens int) (string, error) {
	if maxTokens <= 0 {
		maxTokens = 100
	}
	body, err := json.Marshal(chatRequest{
		Model:     c.Model,
		Messages:  []chatMessage{{Role: "user", Content: user}},
		MaxTokens: maxTokens,
	})
	if err != nil {
		return "", err
	}
	req, err := http.NewRequest("POST", c.URL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("chat %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var parsed chatResponse
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", fmt.Errorf("parse: %w", err)
	}
	if parsed.Error != nil {
		return "", fmt.Errorf("chat error: %s", parsed.Error.Message)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("no choices")
	}
	return parsed.Choices[0].Message.Content, nil
}

// MeanVecs returns the element-wise mean of equal-length float vectors.
// Used by Query2Doc to blend original query embedding with pseudo-doc
// embedding. Returns nil if any vector has different length.
func MeanVecs(vecs ...[]float32) []float32 {
	if len(vecs) == 0 {
		return nil
	}
	out := make([]float32, len(vecs[0]))
	for _, v := range vecs {
		if len(v) != len(out) {
			return nil
		}
		for i, f := range v {
			out[i] += f
		}
	}
	n := float32(len(vecs))
	for i := range out {
		out[i] /= n
	}
	return out
}

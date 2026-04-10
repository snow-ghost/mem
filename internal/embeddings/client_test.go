package embeddings

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/snow-ghost/mem/internal/config"
)

func TestClient_GivenBatch_WhenEmbedBatchCalled_ThenReturnsVectors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing auth header, got %q", r.Header.Get("Authorization"))
		}
		var req embedRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode req: %v", err)
		}
		inputs, ok := req.Input.([]any)
		if !ok {
			t.Fatalf("expected []string input, got %T", req.Input)
		}
		resp := embedResponse{Model: req.Model}
		for i := range inputs {
			resp.Data = append(resp.Data, embedResponseData{
				Embedding: []float32{float32(i), float32(i) + 0.5, 1.0},
				Index:     i,
			})
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	client := NewClient(config.Config{
		EmbeddingsURL:    server.URL,
		EmbeddingsModel:  "test-model",
		EmbeddingsAPIKey: "test-key",
	})

	vecs, err := client.EmbedBatch([]string{"hello", "world", "foo"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(vecs) != 3 {
		t.Fatalf("got %d vectors want 3", len(vecs))
	}
	if vecs[0][0] != 0 || vecs[1][0] != 1 || vecs[2][0] != 2 {
		t.Errorf("wrong order: %v", vecs)
	}
}

func TestClient_GivenServerError_WhenEmbedCalled_ThenReturnsError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"rate limited"}}`, 429)
	}))
	defer server.Close()

	client := NewClient(config.Config{EmbeddingsURL: server.URL, EmbeddingsModel: "m"})
	_, err := client.Embed("hello")
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "429") {
		t.Errorf("expected 429 in error, got %v", err)
	}
}

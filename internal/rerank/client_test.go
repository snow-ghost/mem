package rerank

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/snow-ghost/mem/internal/config"
)

func TestScore_GivenDocs_WhenScored_ThenReturnsInOrder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-key" {
			t.Errorf("missing auth header, got %q", r.Header.Get("Authorization"))
		}
		var req rerankRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Query == "" {
			t.Errorf("missing query")
		}
		// Return scores in shuffled order to test index-based assembly
		resp := rerankResponse{}
		for i := len(req.Documents) - 1; i >= 0; i-- {
			resp.Results = append(resp.Results, rerankResultItem{
				Index:          i,
				RelevanceScore: float64(i) * 0.1,
			})
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(config.Config{
		RerankURL:    server.URL,
		RerankModel:  "test-rr",
		RerankAPIKey: "test-key",
	})
	scores, err := c.Score("hello", []string{"a", "b", "c"})
	if err != nil {
		t.Fatalf("score: %v", err)
	}
	if len(scores) != 3 || scores[0] != 0 || scores[1] != 0.1 || scores[2] != 0.2 {
		t.Errorf("got scores %v want [0 0.1 0.2]", scores)
	}
}

func TestScore_GivenServer400_WhenScored_ThenErrorImmediately(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"bad"}}`, 400)
	}))
	defer server.Close()
	c := NewClient(config.Config{RerankURL: server.URL, RerankModel: "x"})
	_, err := c.Score("q", []string{"a"})
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("got %v, want 400 error", err)
	}
}

func TestScore_GivenEmptyDocs_WhenScored_ThenNil(t *testing.T) {
	c := NewClient(config.Config{RerankURL: "http://unused", RerankModel: "x"})
	scores, err := c.Score("q", nil)
	if err != nil || scores != nil {
		t.Errorf("got (%v, %v) want (nil, nil)", scores, err)
	}
}

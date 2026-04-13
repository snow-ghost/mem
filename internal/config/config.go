package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	PalacePath string
	DbFile     string

	// Embeddings (optional, OpenAI-compatible /v1/embeddings endpoint)
	EmbeddingsURL    string
	EmbeddingsModel  string
	EmbeddingsAPIKey string

	// Reranker (optional, Cohere-compatible /v1/rerank endpoint)
	RerankURL    string
	RerankModel  string
	RerankAPIKey string

	// Chat LLM (optional, OpenAI-compatible /v1/chat/completions —
	// used for Query2Doc query expansion).
	LLMURL    string
	LLMModel  string
	LLMAPIKey string
}

func Load() Config {
	palacePath := os.Getenv("MEM_PALACE")
	if palacePath == "" {
		home, _ := os.UserHomeDir()
		palacePath = filepath.Join(home, ".mempalace")
	}
	rerankKey := os.Getenv("MEM_RERANK_API_KEY")
	if rerankKey == "" {
		rerankKey = os.Getenv("MEM_EMBEDDINGS_API_KEY")
	}
	llmKey := os.Getenv("MEM_LLM_API_KEY")
	if llmKey == "" {
		llmKey = os.Getenv("MEM_EMBEDDINGS_API_KEY")
	}
	return Config{
		PalacePath:       palacePath,
		DbFile:           "palace.db",
		EmbeddingsURL:    os.Getenv("MEM_EMBEDDINGS_URL"),
		EmbeddingsModel:  os.Getenv("MEM_EMBEDDINGS_MODEL"),
		EmbeddingsAPIKey: os.Getenv("MEM_EMBEDDINGS_API_KEY"),
		RerankURL:        os.Getenv("MEM_RERANK_URL"),
		RerankModel:      os.Getenv("MEM_RERANK_MODEL"),
		RerankAPIKey:     rerankKey,
		LLMURL:           os.Getenv("MEM_LLM_URL"),
		LLMModel:         os.Getenv("MEM_LLM_MODEL"),
		LLMAPIKey:        llmKey,
	}
}

func (c Config) LLMEnabled() bool {
	return c.LLMURL != "" && c.LLMModel != ""
}

func (c Config) EmbeddingsEnabled() bool {
	return c.EmbeddingsURL != "" && c.EmbeddingsModel != ""
}

func (c Config) RerankEnabled() bool {
	return c.RerankURL != "" && c.RerankModel != ""
}

func (c Config) DbPath() string {
	return filepath.Join(c.PalacePath, c.DbFile)
}

func (c Config) IdentityPath() string {
	return filepath.Join(c.PalacePath, "identity.txt")
}

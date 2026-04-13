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
	return Config{
		PalacePath:       palacePath,
		DbFile:           "palace.db",
		EmbeddingsURL:    os.Getenv("MEM_EMBEDDINGS_URL"),
		EmbeddingsModel:  os.Getenv("MEM_EMBEDDINGS_MODEL"),
		EmbeddingsAPIKey: os.Getenv("MEM_EMBEDDINGS_API_KEY"),
		RerankURL:        os.Getenv("MEM_RERANK_URL"),
		RerankModel:      os.Getenv("MEM_RERANK_MODEL"),
		RerankAPIKey:     rerankKey,
	}
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

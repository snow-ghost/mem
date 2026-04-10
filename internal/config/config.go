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
}

func Load() Config {
	palacePath := os.Getenv("MEM_PALACE")
	if palacePath == "" {
		home, _ := os.UserHomeDir()
		palacePath = filepath.Join(home, ".mempalace")
	}
	return Config{
		PalacePath:       palacePath,
		DbFile:           "palace.db",
		EmbeddingsURL:    os.Getenv("MEM_EMBEDDINGS_URL"),
		EmbeddingsModel:  os.Getenv("MEM_EMBEDDINGS_MODEL"),
		EmbeddingsAPIKey: os.Getenv("MEM_EMBEDDINGS_API_KEY"),
	}
}

func (c Config) EmbeddingsEnabled() bool {
	return c.EmbeddingsURL != "" && c.EmbeddingsModel != ""
}

func (c Config) DbPath() string {
	return filepath.Join(c.PalacePath, c.DbFile)
}

func (c Config) IdentityPath() string {
	return filepath.Join(c.PalacePath, "identity.txt")
}

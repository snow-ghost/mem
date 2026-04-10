package config

import (
	"os"
	"path/filepath"
)

type Config struct {
	PalacePath string
	DbFile     string
}

func Load() Config {
	palacePath := os.Getenv("MEM_PALACE")
	if palacePath == "" {
		home, _ := os.UserHomeDir()
		palacePath = filepath.Join(home, ".mempalace")
	}
	return Config{
		PalacePath: palacePath,
		DbFile:     "palace.db",
	}
}

func (c Config) DbPath() string {
	return filepath.Join(c.PalacePath, c.DbFile)
}

func (c Config) IdentityPath() string {
	return filepath.Join(c.PalacePath, "identity.txt")
}

package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	MemPath          string
	SessionThreshold int
	EpisodeThreshold int
	PrinciplesMax    int
	EpisodesMax      int
	EpisodesKeep     int
	AgentID          string
}

func Load() Config {
	agentID := os.Getenv("MEM_AGENT_ID")
	if agentID == "" {
		agentID = defaultAgentID()
	}
	return Config{
		MemPath:          envOrDefault("MEM_PATH", ".memory"),
		SessionThreshold: envIntOrDefault("MEM_SESSION_THRESHOLD", 10),
		EpisodeThreshold: envIntOrDefault("MEM_EPISODE_THRESHOLD", 100),
		PrinciplesMax:    envIntOrDefault("MEM_PRINCIPLES_MAX", 100),
		EpisodesMax:      envIntOrDefault("MEM_EPISODES_MAX", 200),
		EpisodesKeep:     envIntOrDefault("MEM_EPISODES_KEEP", 50),
		AgentID:          agentID,
	}
}

func defaultAgentID() string {
	host, err := os.Hostname()
	if err != nil {
		host = "unknown"
	}
	return fmt.Sprintf("%s-%d", host, os.Getpid())
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

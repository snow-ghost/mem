package layers

import (
	"os"
	"strings"
)

func LoadIdentity(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if text == "" {
		return ""
	}
	return "## L0 — IDENTITY\n" + text
}

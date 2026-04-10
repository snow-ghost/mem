package layers

import (
	"strings"

	"github.com/snow-ghost/mem/internal/config"
	"github.com/snow-ghost/mem/internal/db"
)

func WakeUp(d *db.DB, cfg config.Config, wingFilter string) string {
	var parts []string

	l0 := LoadIdentity(cfg.IdentityPath())
	if l0 != "" {
		parts = append(parts, l0)
	}

	l1 := CompressL1(d, wingFilter)
	parts = append(parts, l1)

	return strings.Join(parts, "\n\n")
}

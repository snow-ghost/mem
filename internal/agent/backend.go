package agent

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/snow-ghost/mem/internal/config"
)

type Backend struct {
	Name          string
	Binary        string
	SupportsModel bool
	BuildArgs     func(prompt, model string) []string
	Source        string // "flag", "env", "auto-detected"
}

var builtinBackends = []Backend{
	{
		Name:          "claude",
		Binary:        "claude",
		SupportsModel: true,
		BuildArgs: func(prompt, model string) []string {
			return []string{"-p", prompt, "--model", model}
		},
	},
	{
		Name:          "opencode",
		Binary:        "opencode",
		SupportsModel: false,
		BuildArgs: func(prompt, _ string) []string {
			return []string{"-p", prompt, "-q"}
		},
	},
	{
		Name:          "codex",
		Binary:        "codex",
		SupportsModel: true,
		BuildArgs: func(prompt, model string) []string {
			return []string{"exec", prompt, "-m", model}
		},
	},
}

var validBackendNames = []string{"claude", "opencode", "codex", "custom"}

func Resolve(cfg config.Config, backendFlag string) (*Backend, error) {
	name := backendFlag
	source := "flag"
	if name == "" {
		name = cfg.Backend
		source = "env"
	}
	if name == "" {
		return autoDetect()
	}

	if name == "custom" {
		b, err := buildCustomBackend(cfg)
		if err != nil {
			return nil, err
		}
		b.Source = source
		return b, nil
	}

	for _, b := range builtinBackends {
		if b.Name == name {
			result := b
			result.Source = source
			return &result, nil
		}
	}

	return nil, fmt.Errorf(
		"invalid backend %q — valid values: %s",
		name, strings.Join(validBackendNames, ", "),
	)
}

func autoDetect() (*Backend, error) {
	for _, b := range builtinBackends {
		if _, err := exec.LookPath(b.Binary); err == nil {
			result := b
			result.Source = "auto-detected"
			return &result, nil
		}
	}
	return nil, fmt.Errorf(
		"no supported backend found\n"+
			"  Install one of: claude, opencode, codex\n"+
			"  Or configure a custom backend:\n"+
			"    MEM_BACKEND=custom\n"+
			"    MEM_BACKEND_BINARY=/path/to/binary\n"+
			"    MEM_BACKEND_ARGS=\"-p {prompt} --model {model}\"",
	)
}

func buildCustomBackend(cfg config.Config) (*Backend, error) {
	if cfg.BackendBinary == "" {
		return nil, fmt.Errorf("MEM_BACKEND_BINARY is required when MEM_BACKEND=custom")
	}
	argsTemplate := cfg.BackendArgs
	if argsTemplate == "" {
		argsTemplate = "{prompt}"
	}

	return &Backend{
		Name:          "custom",
		Binary:        cfg.BackendBinary,
		SupportsModel: strings.Contains(argsTemplate, "{model}"),
		BuildArgs: func(prompt, model string) []string {
			parts := strings.Fields(argsTemplate)
			var args []string
			for _, p := range parts {
				switch p {
				case "{prompt}":
					args = append(args, prompt)
				case "{model}":
					args = append(args, model)
				default:
					args = append(args, p)
				}
			}
			return args
		},
	}, nil
}

func NewInvoker(b *Backend) Invoker {
	return &CLIInvoker{backend: b}
}

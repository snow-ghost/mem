package agent

import (
	"bytes"
	"fmt"
	"os/exec"
)

type Invoker interface {
	Invoke(model, prompt string) (string, error)
}

type CLIInvoker struct {
	backend *Backend

	// Legacy field — kept for backward compatibility in tests.
	// If backend is nil, falls back to this binary with claude args.
	Binary string
}

func (c *CLIInvoker) Invoke(model, prompt string) (string, error) {
	var binary string
	var args []string

	if c.backend != nil {
		binary = c.backend.Binary
		args = c.backend.BuildArgs(prompt, model)
	} else {
		binary = c.Binary
		if binary == "" {
			binary = "claude"
		}
		args = []string{"-p", prompt, "--model", model}
	}

	cmd := exec.Command(binary, args...)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := stderr.String()
		if errMsg != "" {
			return "", fmt.Errorf("%s: %w\n%s", binary, err, errMsg)
		}
		return "", fmt.Errorf("%s: %w", binary, err)
	}

	return stdout.String(), nil
}

type StubInvoker struct {
	Response string
	Err      error
}

func (s *StubInvoker) Invoke(_, _ string) (string, error) {
	return s.Response, s.Err
}

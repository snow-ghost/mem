package agent

import (
	"fmt"
	"os/exec"
)

type Invoker interface {
	Invoke(model, prompt string) (string, error)
}

type CLIInvoker struct {
	Binary string
}

func (c *CLIInvoker) Invoke(model, prompt string) (string, error) {
	binary := c.Binary
	if binary == "" {
		binary = "claude"
	}
	cmd := exec.Command(binary, "-p", prompt, "--model", model)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("claude %s: %w\n%s", model, err, string(out))
	}
	return string(out), nil
}

type StubInvoker struct {
	Response string
	Err      error
}

func (s *StubInvoker) Invoke(_, _ string) (string, error) {
	return s.Response, s.Err
}

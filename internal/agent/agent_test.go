package agent

import (
	"errors"
	"testing"
)

func TestStubInvoker_GivenCannedResponse_WhenInvoked_ThenResponseReturned(t *testing.T) {
	stub := &StubInvoker{Response: `[{"type":"decision","summary":"test"}]`}
	resp, err := stub.Invoke("haiku", "test prompt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp != `[{"type":"decision","summary":"test"}]` {
		t.Errorf("response = %q, want canned response", resp)
	}
}

func TestStubInvoker_GivenError_WhenInvoked_ThenErrorReturned(t *testing.T) {
	want := errors.New("stub error")
	stub := &StubInvoker{Err: want}
	_, err := stub.Invoke("haiku", "test prompt")
	if !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

func TestCLIInvoker_GivenNonExistentBinary_WhenInvoked_ThenErrorWrapped(t *testing.T) {
	invoker := &CLIInvoker{Binary: "nonexistent-binary-xyz"}
	_, err := invoker.Invoke("haiku", "test")
	if err == nil {
		t.Error("expected error for non-existent binary")
	}
}

package miner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParseClaudeJSONL_GivenValidFile_WhenParsed_ThenExchangesExtracted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat.jsonl")
	content := `{"role":"human","content":"Hello"}
{"role":"assistant","content":"Hi there!"}
{"role":"human","content":"How are you?"}
`
	os.WriteFile(path, []byte(content), 0644)

	exchanges, err := ParseConversation(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exchanges) != 3 {
		t.Fatalf("got %d exchanges, want 3", len(exchanges))
	}
	if exchanges[0].Speaker != "human" {
		t.Errorf("speaker = %q, want human", exchanges[0].Speaker)
	}
}

func TestParsePlainText_GivenTranscript_WhenParsed_ThenExchangesExtracted(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "chat.txt")
	content := `Human: What is Go?
Assistant: Go is a programming language created by Google.
Human: Is it good?
Assistant: Yes, it's great for systems programming.
`
	os.WriteFile(path, []byte(content), 0644)

	exchanges, err := ParseConversation(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(exchanges) != 4 {
		t.Fatalf("got %d exchanges, want 4", len(exchanges))
	}
}

package filelock

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestWithLock_GivenNoLockFile_WhenCalled_ThenFnExecuted(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	executed := false
	err := WithLock(lockPath, func() error {
		executed = true
		return nil
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !executed {
		t.Error("fn was not executed")
	}
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("lock file was not created")
	}
}

func TestWithLock_GivenConcurrentAccess_WhenBothLock_ThenSerializedExecution(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	var mu sync.Mutex
	var order []int
	var wg sync.WaitGroup

	wg.Add(2)
	for i := range 2 {
		go func(id int) {
			defer wg.Done()
			WithLock(lockPath, func() error {
				mu.Lock()
				order = append(order, id)
				mu.Unlock()
				return nil
			})
		}(i)
	}
	wg.Wait()

	if len(order) != 2 {
		t.Fatalf("expected 2 executions, got %d", len(order))
	}
}

func TestWithLock_GivenFnReturnsError_WhenCalled_ThenErrorPropagated(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, ".lock")

	want := errors.New("test error")
	err := WithLock(lockPath, func() error {
		return want
	})

	if !errors.Is(err, want) {
		t.Errorf("got error %v, want %v", err, want)
	}
}

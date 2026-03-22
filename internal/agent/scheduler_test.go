package agent

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestRunSchedulerCapsConcurrentAgents(t *testing.T) {
	modules := []store.Module{
		{ID: "auth"},
		{ID: "billing"},
		{ID: "logger"},
		{ID: "config"},
	}

	input := AgentInput{
		Config: &configpkg.Config{
			Agent: configpkg.AgentConfig{Concurrency: 2},
		},
	}

	var (
		mu         sync.Mutex
		active     int
		peakActive int
	)

	runner := func(ctx context.Context, module store.Module, input AgentInput) ModuleDoc {
		_ = ctx
		_ = input

		mu.Lock()
		active++
		if active > peakActive {
			peakActive = active
		}
		mu.Unlock()

		time.Sleep(10 * time.Millisecond)

		mu.Lock()
		active--
		mu.Unlock()

		return ModuleDoc{ModuleID: module.ID, Content: module.ID}
	}

	if err := runScheduler(context.Background(), modules, input, 2, runner, t.TempDir()); err != nil {
		t.Fatalf("runScheduler() error = %v", err)
	}
	if peakActive > 2 {
		t.Fatalf("runScheduler() peakActive = %d, want <= 2", peakActive)
	}
}

func TestCollectResultsWritesSuccessfulModuleDocs(t *testing.T) {
	artifactsDir := t.TempDir()
	results := make(chan ModuleDoc, 1)
	results <- ModuleDoc{ModuleID: "auth", Content: "# Auth"}
	close(results)

	failures, err := collectResults(results, artifactsDir)
	if err != nil {
		t.Fatalf("collectResults() error = %v", err)
	}
	if len(failures) != 0 {
		t.Fatalf("len(failures) = %d, want 0", len(failures))
	}

	path := filepath.Join(artifactsDir, "module_docs", "auth.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", path, err)
	}
	if string(data) != "# Auth" {
		t.Fatalf("module doc content = %q, want %q", string(data), "# Auth")
	}
}

func TestCollectResultsSkipsFailedModuleDocs(t *testing.T) {
	artifactsDir := t.TempDir()
	results := make(chan ModuleDoc, 1)
	results <- ModuleDoc{ModuleID: "billing", Err: errors.New("boom")}
	close(results)

	failures, err := collectResults(results, artifactsDir)
	if err != nil {
		t.Fatalf("collectResults() error = %v", err)
	}
	if len(failures) != 1 {
		t.Fatalf("len(failures) = %d, want 1", len(failures))
	}
	if failures[0].ModuleID != "billing" {
		t.Fatalf("failures[0].ModuleID = %q, want billing", failures[0].ModuleID)
	}

	path := filepath.Join(artifactsDir, "module_docs", "billing.md")
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(%q) error = %v, want not-exist", path, err)
	}
}

func TestRunSchedulerProcessesAllModulesWithPartialFailures(t *testing.T) {
	artifactsDir := t.TempDir()
	modules := []store.Module{{ID: "auth"}, {ID: "billing"}, {ID: "config"}}

	runner := func(ctx context.Context, module store.Module, input AgentInput) ModuleDoc {
		_ = ctx
		_ = input

		if module.ID == "billing" {
			return ModuleDoc{ModuleID: module.ID, Err: errors.New("boom")}
		}
		return ModuleDoc{ModuleID: module.ID, Content: "# " + strings.ToUpper(module.ID[:1]) + module.ID[1:]}
	}

	err := runScheduler(context.Background(), modules, AgentInput{}, 2, runner, artifactsDir)
	if err == nil {
		t.Fatal("runScheduler() error = nil, want partial-failure error")
	}
	if !strings.Contains(err.Error(), "billing") {
		t.Fatalf("runScheduler() error = %q, want billing in summary", err.Error())
	}

	for _, moduleID := range []string{"auth", "config"} {
		path := filepath.Join(artifactsDir, "module_docs", moduleID+".md")
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("os.Stat(%q) error = %v, want success output", path, err)
		}
	}

	failedPath := filepath.Join(artifactsDir, "module_docs", "billing.md")
	if _, err := os.Stat(failedPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat(%q) error = %v, want not-exist", failedPath, err)
	}
}

func TestRunSchedulerStopsCleanlyOnContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	artifactsDir := t.TempDir()
	modules := []store.Module{{ID: "auth"}, {ID: "billing"}, {ID: "config"}}

	started := make(chan struct{}, len(modules))
	runner := func(ctx context.Context, module store.Module, input AgentInput) ModuleDoc {
		_ = input

		started <- struct{}{}
		<-ctx.Done()
		return ModuleDoc{ModuleID: module.ID, Err: ctx.Err()}
	}

	done := make(chan error, 1)
	go func() {
		done <- runScheduler(ctx, modules, AgentInput{}, 2, runner, artifactsDir)
	}()

	<-started
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("runScheduler() error = nil, want cancellation-related error")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("runScheduler() did not stop after context cancellation")
	}
}

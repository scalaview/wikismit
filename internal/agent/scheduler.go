package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

type moduleRunner func(context.Context, store.Module, AgentInput) ModuleDoc

type Phase4Error struct {
	Total    int
	Failures []ModuleDoc
}

func (e *Phase4Error) Error() string {
	if e == nil || len(e.Failures) == 0 {
		return ""
	}

	moduleIDs := make([]string, 0, len(e.Failures))
	for _, failure := range e.Failures {
		moduleIDs = append(moduleIDs, failure.ModuleID)
	}
	sort.Strings(moduleIDs)

	joined := strings.Join(moduleIDs, ", ")
	return fmt.Sprintf("Phase 4 completed with %d failures: [%s]", len(e.Failures), joined)
}

func (e *Phase4Error) Summary() string {
	if e == nil {
		return ""
	}
	return formatPhase4Summary(e.Total, e.Failures)
}

func Run(ctx context.Context, modules []store.Module, input AgentInput, client llm.Client, artifactsDir string, concurrency int) error {
	return runScheduler(ctx, modules, input, concurrency, func(ctx context.Context, module store.Module, input AgentInput) ModuleDoc {
		return runAgent(ctx, module, input, client)
	}, artifactsDir)
}

func RunFor(ctx context.Context, modules []store.Module, input AgentInput, client llm.Client, artifactsDir string, concurrency int) error {
	filtered := make([]store.Module, 0, len(modules))
	for _, module := range modules {
		if module.Owner != "agent" {
			continue
		}
		filtered = append(filtered, module)
	}
	if len(filtered) == 0 {
		return nil
	}
	return Run(ctx, filtered, input, client, artifactsDir, concurrency)
}

func runScheduler(ctx context.Context, modules []store.Module, input AgentInput, concurrency int, runner moduleRunner, artifactsDir string) error {
	if concurrency < 1 {
		concurrency = 1
	}

	sem := make(chan struct{}, concurrency)
	results := make(chan ModuleDoc, len(modules))
	var wg sync.WaitGroup

	for _, module := range modules {
		wg.Add(1)
		sem <- struct{}{}

		go func(mod store.Module) {
			defer wg.Done()
			defer func() { <-sem }()

			results <- runner(ctx, mod, input)
		}(module)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	failures, err := collectResults(results, artifactsDir)
	if err != nil {
		return err
	}
	if len(failures) > 0 {
		return formatSchedulerFailure(len(modules), failures)
	}

	wg.Wait()
	return nil
}

func collectResults(results <-chan ModuleDoc, artifactsDir string) ([]ModuleDoc, error) {
	moduleDocsDir := filepath.Join(artifactsDir, "module_docs")
	if err := os.MkdirAll(moduleDocsDir, 0o755); err != nil {
		return nil, err
	}

	var failures []ModuleDoc
	for result := range results {
		if result.Err != nil {
			failures = append(failures, result)
			continue
		}

		path := filepath.Join(moduleDocsDir, result.ModuleID+".md")
		if err := os.WriteFile(path, []byte(result.Content), 0o644); err != nil {
			return nil, err
		}
	}

	return failures, nil
}

func formatSchedulerFailure(total int, failures []ModuleDoc) error {
	if len(failures) == 0 {
		return nil
	}

	cloned := append([]ModuleDoc(nil), failures...)
	return &Phase4Error{Total: total, Failures: cloned}
}

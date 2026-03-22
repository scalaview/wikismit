package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/scalaview/wikismit/pkg/store"
)

type moduleRunner func(context.Context, store.Module, AgentInput) ModuleDoc

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
		return formatSchedulerFailure(failures)
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

func formatSchedulerFailure(failures []ModuleDoc) error {
	if len(failures) == 0 {
		return nil
	}

	moduleIDs := make([]string, 0, len(failures))
	for _, failure := range failures {
		moduleIDs = append(moduleIDs, failure.ModuleID)
	}
	sort.Strings(moduleIDs)

	joined := strings.Join(moduleIDs, ", ")
	return fmt.Errorf("Phase 4 completed with %d failures: [%s]", len(failures), joined)
}

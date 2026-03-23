package pipeline

import (
	"context"

	"github.com/scalaview/wikismit/internal/agent"
	"github.com/scalaview/wikismit/internal/analyzer"
	"github.com/scalaview/wikismit/internal/composer"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/internal/planner"
	"github.com/scalaview/wikismit/internal/preprocessor"
	"github.com/scalaview/wikismit/pkg/gitdiff"
	"github.com/scalaview/wikismit/pkg/store"
)

type IncrementalOptions struct {
	BaseRef      string
	HeadRef      string
	ChangedFiles string
}

var runGenerateFallback = runFullGenerate

var getChangedFiles = gitdiff.GetChangedFiles
var computeAffected = analyzer.ComputeAffected
var runPreprocessorFor = preprocessor.RunPreprocessorFor
var runAgentFor = agent.RunFor
var runComposer = composer.RunComposer
var reanalyzeChangedFunc = reanalyzeChanged

func runFullGenerate(ctx context.Context, cfg *configpkg.Config, client llm.Client) error {
	if err := analyzer.RunPhase1(cfg); err != nil {
		return err
	}

	idx, err := store.ReadFileIndex(cfg.ArtifactsDir)
	if err != nil {
		return err
	}
	graph, err := store.ReadDepGraph(cfg.ArtifactsDir)
	if err != nil {
		return err
	}
	plan, err := planner.RunPlanner(ctx, idx, graph, cfg, client)
	if err != nil {
		return err
	}
	if err := store.WriteNavPlan(cfg.ArtifactsDir, *plan); err != nil {
		return err
	}

	sharedCtx, err := preprocessor.RunPreprocessor(ctx, plan, idx, graph, cfg, client)
	if err != nil {
		return err
	}
	if err := agent.RunFor(ctx, plan.Modules, agent.AgentInput{FileIndex: idx, SharedContext: sharedCtx, Config: cfg}, client, cfg.ArtifactsDir, cfg.Agent.Concurrency); err != nil {
		return err
	}

	return composer.RunComposer(cfg, plan, idx, graph)
}

func RunIncremental(ctx context.Context, cfg *configpkg.Config, client llm.Client, opts IncrementalOptions) error {
	idx, err := store.ReadFileIndex(cfg.ArtifactsDir)
	if err != nil {
		if err == store.ErrArtifactNotFound {
			return runGenerateFallback(ctx, cfg, client)
		}
		return err
	}
	if cfg.RepoPath != "" {
		idx, err = analyzer.ResolveImportPaths(cfg.RepoPath, cfg.Analysis, idx)
		if err != nil {
			return err
		}
	}
	plan, err := store.ReadNavPlan(cfg.ArtifactsDir)
	if err != nil {
		return err
	}

	var changes []gitdiff.FileChange
	if opts.ChangedFiles != "" {
		changes = gitdiff.ParseChangedFiles(opts.ChangedFiles)
	} else {
		changes, err = getChangedFiles(cfg.RepoPath, opts.BaseRef, opts.HeadRef)
		if err != nil {
			return err
		}
	}

	idx, err = reanalyzeChangedFunc(changes, idx, cfg)
	if err != nil {
		return err
	}
	graph, err := store.ReadDepGraph(cfg.ArtifactsDir)
	if err != nil {
		return err
	}

	affected := computeAffected(changes, &plan, graph)
	sharedCtx, err := runPreprocessorFor(ctx, affected, &plan, idx, graph, cfg, client)
	if err != nil {
		return err
	}
	if err := runAgentFor(ctx, affected, agent.AgentInput{FileIndex: idx, SharedContext: sharedCtx, Config: cfg}, client, cfg.ArtifactsDir, cfg.Agent.Concurrency); err != nil {
		return err
	}

	return runComposer(cfg, &plan, idx, graph)

}

func reanalyzeChanged(changes []gitdiff.FileChange, idx store.FileIndex, cfg *configpkg.Config) (store.FileIndex, error) {
	next := make(store.FileIndex, len(idx))
	for path, entry := range idx {
		next[path] = entry
	}

	needsParse := false
	for _, change := range changes {
		switch change.Type {
		case gitdiff.ChangeDeleted:
			delete(next, change.Path)
		case gitdiff.ChangeRenamed:
			if change.OldPath != "" {
				delete(next, change.OldPath)
			}
			needsParse = true
		case gitdiff.ChangeAdded, gitdiff.ChangeModified:
			needsParse = true
		}
	}

	if needsParse {
		parsed, err := analyzer.RunPhase1FileIndex(cfg)
		if err != nil {
			return nil, err
		}

		for _, change := range changes {
			switch change.Type {
			case gitdiff.ChangeAdded, gitdiff.ChangeModified, gitdiff.ChangeRenamed:
				entry, ok := parsed[change.Path]
				if !ok {
					continue
				}
				next[change.Path] = entry
			}
		}
	}

	graph := analyzer.BuildDepGraph(next)
	if err := store.WriteFileIndex(cfg.ArtifactsDir, next); err != nil {
		return nil, err
	}
	if err := store.WriteDepGraph(cfg.ArtifactsDir, graph); err != nil {
		return nil, err
	}

	return next, nil
}

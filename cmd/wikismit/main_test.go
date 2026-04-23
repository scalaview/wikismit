package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/internal/log"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestMain(m *testing.M) {
	log.SetDefaultLogger(log.New(true))
	os.Exit(m.Run())
}

func writeCLIConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	content := strings.TrimSpace(body)
	if !strings.HasPrefix(content, "site:") && !strings.Contains(content, "\nsite:") {
		content += "\nsite: {}"
	}
	if err := os.WriteFile(path, []byte(content+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}

func executeCLI(t *testing.T, args ...string) (string, string, error) {
	t.Helper()
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	cmd := newRootCmd()
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func TestRootHelpListsRequiredSubcommands(t *testing.T) {
	stdout, stderr, err := executeCLI(t, "--help")
	if err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr)
	}

	for _, subcommand := range []string{"generate", "update", "plan", "validate", "build"} {
		if !strings.Contains(stdout, subcommand) {
			t.Fatalf("help output missing %q:\n%s", subcommand, stdout)
		}
	}
}

func TestGenerateCommandRunsPhase1WithRepoOverride(t *testing.T) {
	repoDir := filepath.Join("..", "..", "testdata", "sample_repo")
	artifactsDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	originalFactory := agentClientFactory
	agentClientFactory = func() llm.Client {
		return llm.NewMockClient(
			`{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/middleware.go"],"shared":false,"owner":"agent","depends_on_shared":["errors","logger"]},{"id":"api","files":["internal/api/handler.go"],"shared":false,"owner":"agent","depends_on_shared":["logger"]},{"id":"db","files":["internal/db/client.go"],"shared":false,"owner":"agent","depends_on_shared":["errors","logger"]},{"id":"cmd","files":["cmd/main.go"],"shared":false,"owner":"agent"},{"id":"errors","files":["pkg/errors/errors.go"],"shared":true,"owner":"shared_preprocessor"},{"id":"logger","files":["pkg/logger/logger.go"],"shared":true,"owner":"shared_preprocessor"}]}`,
			`{"summary":"Shared error helpers.","key_types":[],"key_functions":[]}`,
			`{"summary":"Shared logger helpers.","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() *Logger","ref":"pkg/logger/logger.go#L1"}]}`,
			"# Auth module\n",
			"# API module\n",
			"# DB module\n",
			"# CMD module\n",
		)
	}
	t.Cleanup(func() { agentClientFactory = originalFactory })

	configPath := writeCLIConfig(t, `
repo_path: "."
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(
		t,
		"generate",
		"--config", configPath,
		"--repo", repoDir,
		"--artifacts", artifactsDir,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr)
	}
	if stdout != "" {
		t.Fatalf("generate stdout = %q, want empty output", stdout)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "file_index.json")); err != nil {
		t.Fatalf("file_index.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "dep_graph.json")); err != nil {
		t.Fatalf("dep_graph.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "call_graph.json")); err != nil {
		t.Fatalf("call_graph.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "module_docs", "auth.md")); err != nil {
		t.Fatalf("auth module doc missing: %v", err)
	}
}

func TestRootCommandExposesPhase1OverrideFlags(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeCLIConfig(t, `
repo_path: "."
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(t, "generate", "--config", configPath, "--help")
	if err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr)
	}
	for _, flag := range []string{"--repo", "--output", "--artifacts"} {
		if !strings.Contains(stdout, flag) {
			t.Fatalf("generate help missing %q:\n%s", flag, stdout)
		}
	}
}

func TestApplyCLIOverridesSetsVerboseOnConfig(t *testing.T) {
	oldVerbose := verbose
	oldRepo := repoPathOverride
	oldOutput := outputDirOverride
	oldArtifacts := artifactsDirOverride
	verbose = true
	repoPathOverride = ""
	outputDirOverride = ""
	artifactsDirOverride = ""
	t.Cleanup(func() {
		verbose = oldVerbose
		repoPathOverride = oldRepo
		outputDirOverride = oldOutput
		artifactsDirOverride = oldArtifacts
	})

	cfg := &configpkg.Config{}
	applyCLIOverrides(cfg)

	if !cfg.Verbose {
		t.Fatal("cfg.Verbose = false, want true")
	}
}

func TestApplyCLIOverridesLeavesVerboseFalseByDefault(t *testing.T) {
	oldVerbose := verbose
	oldRepo := repoPathOverride
	oldOutput := outputDirOverride
	oldArtifacts := artifactsDirOverride
	verbose = false
	repoPathOverride = ""
	outputDirOverride = ""
	artifactsDirOverride = ""
	t.Cleanup(func() {
		verbose = oldVerbose
		repoPathOverride = oldRepo
		outputDirOverride = oldOutput
		artifactsDirOverride = oldArtifacts
	})

	cfg := &configpkg.Config{}
	applyCLIOverrides(cfg)

	if cfg.Verbose {
		t.Fatal("cfg.Verbose = true, want false")
	}
}

func TestGeneratePrintsConfigErrorsToStderr(t *testing.T) {
	stdout, stderr, err := executeCLI(t, "generate", "--config", "/tmp/does-not-exist.yaml")
	if err == nil {
		t.Fatalf("Execute() error = nil, stdout = %s", stdout)
	}
	if !strings.Contains(stderr, "read config") {
		t.Fatalf("stderr = %q, want config read error", stderr)
	}
}

func sampleGenerateConfig(repoDir, artifactsDir string) *configpkg.Config {
	cfg := &configpkg.Config{
		RepoPath:     repoDir,
		OutputDir:    filepath.Join(artifactsDir, "docs"),
		ArtifactsDir: artifactsDir,
		Analysis: &configpkg.AnalysisConfig{
			SharedModuleThreshold:      3,
			FunctionSummaryAgentConfig: &configpkg.FunctionSummaryAgentConfig{},
		},
		Agent: &configpkg.AgentConfig{
			Concurrency:       2,
			SkeletonMaxTokens: 3000,
		},
		LLM: &configpkg.LLMConfig{
			AgentModel: "agent-test-model",
			MaxTokens:  2048,
		},
		Site: &configpkg.SiteConfig{},
	}
	cfg.Analysis.FunctionSummaryAgentConfig.ContextBudget = 2048
	return cfg
}

func TestGenerateCommandReportsPhase4SummaryToStderr(t *testing.T) {
	repoDir := filepath.Join("..", "..", "testdata", "sample_repo")
	artifactsDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeCLIConfig(t, `
repo_path: "."
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 2
  skeleton_max_tokens: 3000
site: {}
`)

	originalFactory := agentClientFactory
	agentClientFactory = func() llm.Client {
		return llm.NewMockClient(
			`{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/middleware.go"],"shared":false,"owner":"agent","depends_on_shared":["errors","logger"]},{"id":"billing","files":["internal/api/handler.go"],"shared":false,"owner":"agent","depends_on_shared":["logger"]},{"id":"db","files":["internal/db/client.go"],"shared":false,"owner":"agent","depends_on_shared":["errors","logger"]},{"id":"cmd","files":["cmd/main.go"],"shared":false,"owner":"agent"},{"id":"errors","files":["pkg/errors/errors.go"],"shared":true,"owner":"shared_preprocessor"},{"id":"logger","files":["pkg/logger/logger.go"],"shared":true,"owner":"shared_preprocessor"}]}`,
			`{"summary":"Shared error helpers.","key_types":[],"key_functions":[]}`,
			`{"summary":"Shared logger helpers.","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() *Logger","ref":"pkg/logger/logger.go#L1"}]}`,
		).WithErrors(nil, nil, nil, errors.New("boom"), errors.New("boom"), errors.New("boom"), errors.New("boom"))
	}
	t.Cleanup(func() { agentClientFactory = originalFactory })

	stdout, stderr, err := executeCLI(
		t,
		"generate",
		"--config", configPath,
		"--repo", repoDir,
		"--artifacts", artifactsDir,
	)
	if err == nil {
		t.Fatalf("Execute() error = nil, stdout = %s, stderr = %s", stdout, stderr)
	}
	if !strings.Contains(stderr, "Phase 4 complete:") {
		t.Fatalf("stderr = %q, want phase 4 summary", stderr)
	}
	if !strings.Contains(stderr, "auth") && !strings.Contains(stderr, "billing") {
		t.Fatalf("stderr = %q, want at least one failing module in summary", stderr)
	}
}

func TestUpdateCommandExposesIncrementalFlags(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeCLIConfig(t, `
repo_path: "."
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(t, "update", "--config", configPath, "--help")
	if err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr)
	}
	for _, flag := range []string{"--base-ref", "--head-ref", "--changed-files"} {
		if !strings.Contains(stdout, flag) {
			t.Fatalf("update help missing %q:\n%s", flag, stdout)
		}
	}
}

func TestUpdateCommandFallsBackToGenerateWhenArtifactsMissing(t *testing.T) {
	repoDir := filepath.Join("..", "..", "testdata", "sample_repo")
	artifactsDir := t.TempDir()
	outputDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeCLIConfig(t, `
repo_path: "."
artifacts_dir: "./artifacts"
output_dir: "./docs"
llm:
  api_key_env: "OPENAI_API_KEY"
  planner_model: "planner-test-model"
  agent_model: "agent-test-model"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
  skeleton_max_tokens: 3000
`)

	originalFactory := updateClientFactory
	updateClientFactory = func() llm.Client {
		return llm.NewMockClient(
			`{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/middleware.go"],"shared":false,"owner":"agent","depends_on_shared":["errors","logger"]},{"id":"api","files":["internal/api/handler.go"],"shared":false,"owner":"agent","depends_on_shared":["logger"]},{"id":"db","files":["internal/db/client.go"],"shared":false,"owner":"agent","depends_on_shared":["errors","logger"]},{"id":"cmd","files":["cmd/main.go"],"shared":false,"owner":"agent"},{"id":"errors","files":["pkg/errors/errors.go"],"shared":true,"owner":"shared_preprocessor"},{"id":"logger","files":["pkg/logger/logger.go"],"shared":true,"owner":"shared_preprocessor"}]}`,
			`{"summary":"Shared error helpers.","key_types":[],"key_functions":[]}`,
			`{"summary":"Shared logger helpers.","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() *Logger","ref":"wrong.go#L1"}]}`,
			"# Auth doc",
			"# API doc",
			"# DB doc",
			"# CMD doc",
		)
	}
	t.Cleanup(func() { updateClientFactory = originalFactory })

	stdout, stderr, err := executeCLI(
		t,
		"update",
		"--config", configPath,
		"--repo", repoDir,
		"--artifacts", artifactsDir,
		"--output", outputDir,
		"--changed-files", "internal/auth/jwt.go",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stdout = %s, stderr = %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "full generate") {
		t.Fatalf("stdout = %q, want fallback message", stdout)
	}
	for _, path := range []string{
		filepath.Join(artifactsDir, "file_index.json"),
		filepath.Join(artifactsDir, "nav_plan.json"),
		filepath.Join(artifactsDir, "shared_context.json"),
		filepath.Join(artifactsDir, "module_docs", "auth.md"),
		filepath.Join(outputDir, "index.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected fallback artifact %q: %v", path, err)
		}
	}
}

func TestUpdateCommandNavPlanUsesChangedFilesOverrideForSingleModuleRerun(t *testing.T) {
	repoDir := filepath.Join("..", "..", "testdata", "sample_repo")
	artifactsDir := t.TempDir()
	outputDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeCLIConfig(t, `
repo_path: "."
artifacts_dir: "./artifacts"
output_dir: "./docs"
llm:
  api_key_env: "OPENAI_API_KEY"
  agent_model: "agent-test-model"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 2
  skeleton_max_tokens: 3000
site: {}
`)

	if err := store.WriteFileIndex(artifactsDir, store.FileIndex{
		"internal/auth/jwt.go":    {},
		"internal/api/handler.go": {},
		"pkg/logger/logger.go":    {},
	}); err != nil {
		t.Fatalf("WriteFileIndex() error = %v", err)
	}
	if err := store.WriteDepGraph(artifactsDir, store.DepGraph{
		"internal/auth/jwt.go":    {"pkg/logger/logger.go"},
		"internal/api/handler.go": {"internal/auth/jwt.go", "pkg/logger/logger.go"},
		"pkg/logger/logger.go":    {},
	}); err != nil {
		t.Fatalf("WriteDepGraph() error = %v", err)
	}
	if err := store.WriteNavPlan(artifactsDir, &store.NavPlan{Version: "planner/v2",
		Navigation: &store.Navigation{Sections: []*store.NavigationSection{{
			Type:  "generated",
			Title: "Generated Overview",
			Items: []*store.NavigationItem{{Title: "Auth", Path: "docs/modules/auth.md"}, {Title: "API", Path: "docs/modules/api.md"}, {Title: "Logger", Path: "docs/shared/logger.md"}},
		}}}, Modules: []*store.Module{
			{ID: "auth", Files: []string{"internal/auth/jwt.go"}, Owner: "agent", DependsOnShared: []string{"logger"}, NavigationRefs: []string{"generated"}},
			{ID: "api", Files: []string{"internal/api/handler.go"}, Owner: "agent", DependsOnShared: []string{"logger"}, NavigationRefs: []string{"generated"}},
			{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Owner: "shared_preprocessor", Shared: true, NavigationRefs: []string{"generated"}},
		}}); err != nil {
		t.Fatalf("WriteNavPlan() error = %v", err)
	}
	if err := store.WriteSharedContext(artifactsDir, store.SharedContext{"logger": {Summary: "Shared logger helpers."}}); err != nil {
		t.Fatalf("WriteSharedContext() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(artifactsDir, "module_docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll(module_docs) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(artifactsDir, "module_docs", "api.md"), []byte("# Existing API"), 0o644); err != nil {
		t.Fatalf("WriteFile(api.md) error = %v", err)
	}

	originalFactory := agentClientFactory
	agentClientFactory = func() llm.Client {
		return llm.NewMockClient("# Auth doc")
	}
	t.Cleanup(func() { agentClientFactory = originalFactory })

	stdout, stderr, err := executeCLI(
		t,
		"update",
		"--config", configPath,
		"--repo", repoDir,
		"--artifacts", artifactsDir,
		"--output", outputDir,
		"--changed-files", "internal/auth/jwt.go",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stdout = %s, stderr = %s", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "module_docs", "auth.md")); err != nil {
		t.Fatalf("auth.md missing: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(artifactsDir, "module_docs", "api.md"))
	if err != nil {
		t.Fatalf("ReadFile(api.md) error = %v", err)
	}
	if string(data) != "# Existing API" {
		t.Fatalf("api.md content = %q, want untouched existing doc", string(data))
	}
	if !strings.Contains(stdout, "Incremental update complete") {
		t.Fatalf("stdout = %q, want completion message", stdout)
	}
	plan, err := store.ReadNavPlan(artifactsDir)
	if err != nil {
		t.Fatalf("ReadNavPlan() error = %v", err)
	}
	if plan.Version != "planner/v2" {
		t.Fatalf("plan.Version = %q, want %q", plan.Version, "planner/v2")
	}
	if plan.Navigation == nil || len(plan.Navigation.Sections) != 1 {
		t.Fatalf("plan.Navigation = %#v, want one section", plan.Navigation)
	}
	if len(plan.Modules) != 3 || len(plan.Modules[0].NavigationRefs) != 1 || plan.Modules[0].NavigationRefs[0] != "generated" {
		t.Fatalf("plan.Modules = %#v, want navigation-compatible modules", plan.Modules)
	}
}

func TestUpdateCommandNavPlanRerunsDependentsWhenSharedModuleChanges(t *testing.T) {
	repoDir := filepath.Join("..", "..", "testdata", "sample_repo")
	artifactsDir := t.TempDir()
	outputDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeCLIConfig(t, `
repo_path: "."
artifacts_dir: "./artifacts"
output_dir: "./docs"
llm:
  api_key_env: "OPENAI_API_KEY"
  planner_model: "planner-test-model"
  agent_model: "agent-test-model"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 2
  skeleton_max_tokens: 3000
site: {}
`)

	if err := store.WriteFileIndex(artifactsDir, store.FileIndex{
		"internal/auth/jwt.go":    {},
		"internal/api/handler.go": {},
		"internal/db/client.go":   {},
		"pkg/logger/logger.go":    {},
	}); err != nil {
		t.Fatalf("WriteFileIndex() error = %v", err)
	}
	if err := store.WriteDepGraph(artifactsDir, store.DepGraph{
		"internal/auth/jwt.go":    {"pkg/logger/logger.go"},
		"internal/api/handler.go": {"pkg/logger/logger.go"},
		"internal/db/client.go":   {"pkg/logger/logger.go"},
		"pkg/logger/logger.go":    {},
	}); err != nil {
		t.Fatalf("WriteDepGraph() error = %v", err)
	}
	if err := store.WriteNavPlan(artifactsDir, &store.NavPlan{Version: "planner/v2",
		Navigation: &store.Navigation{Sections: []*store.NavigationSection{{
			Type:  "generated",
			Title: "Generated Overview",
			Items: []*store.NavigationItem{{Title: "Auth", Path: "docs/modules/auth.md"}, {Title: "API", Path: "docs/modules/api.md"}, {Title: "DB", Path: "docs/modules/db.md"}, {Title: "Logger", Path: "docs/shared/logger.md"}},
		}}}, Modules: []*store.Module{
			{ID: "auth", Files: []string{"internal/auth/jwt.go"}, Owner: "agent", DependsOnShared: []string{"logger"}, NavigationRefs: []string{"generated"}},
			{ID: "api", Files: []string{"internal/api/handler.go"}, Owner: "agent", DependsOnShared: []string{"logger"}, NavigationRefs: []string{"generated"}},
			{ID: "db", Files: []string{"internal/db/client.go"}, Owner: "agent", DependsOnShared: []string{"logger"}, NavigationRefs: []string{"generated"}},
			{ID: "logger", Files: []string{"pkg/logger/logger.go"}, Owner: "shared_preprocessor", Shared: true, NavigationRefs: []string{"generated"}},
		}}); err != nil {
		t.Fatalf("WriteNavPlan() error = %v", err)
	}
	if err := store.WriteSharedContext(artifactsDir, store.SharedContext{"logger": {Summary: "stale logger summary"}}); err != nil {
		t.Fatalf("WriteSharedContext() error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(artifactsDir, "module_docs"), 0o755); err != nil {
		t.Fatalf("MkdirAll(module_docs) error = %v", err)
	}
	for _, moduleDoc := range []struct {
		name    string
		content string
	}{
		{name: "auth.md", content: "# Existing Auth"},
		{name: "api.md", content: "# Existing API"},
		{name: "db.md", content: "# Existing DB"},
	} {
		if err := os.WriteFile(filepath.Join(artifactsDir, "module_docs", moduleDoc.name), []byte(moduleDoc.content), 0o644); err != nil {
			t.Fatalf("WriteFile(%s) error = %v", moduleDoc.name, err)
		}
	}

	originalFactory := agentClientFactory
	agentClientFactory = func() llm.Client {
		return llm.NewMockClient(
			`{"summary":"updated logger summary","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() Logger","ref":"pkg/logger/logger.go#L5"}]}`,
			"# Auth doc",
			"# API doc",
			"# DB doc",
		)
	}
	t.Cleanup(func() { agentClientFactory = originalFactory })

	stdout, stderr, err := executeCLI(
		t,
		"update",
		"--config", configPath,
		"--repo", repoDir,
		"--artifacts", artifactsDir,
		"--output", outputDir,
		"--changed-files", "pkg/logger/logger.go",
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stdout = %s, stderr = %s", err, stdout, stderr)
	}
	sharedCtx, err := store.ReadSharedContext(artifactsDir)
	if err != nil {
		t.Fatalf("ReadSharedContext() error = %v", err)
	}
	if sharedCtx["logger"].Summary != "updated logger summary" {
		t.Fatalf("logger summary = %q, want updated summary", sharedCtx["logger"].Summary)
	}
	for _, moduleID := range []string{"auth", "api", "db"} {
		if _, err := os.Stat(filepath.Join(artifactsDir, "module_docs", moduleID+".md")); err != nil {
			t.Fatalf("%s.md missing: %v", moduleID, err)
		}
	}
	if !strings.Contains(stdout, "Incremental update complete") {
		t.Fatalf("stdout = %q, want completion message", stdout)
	}
	plan, err := store.ReadNavPlan(artifactsDir)
	if err != nil {
		t.Fatalf("ReadNavPlan() error = %v", err)
	}
	if plan.Version != "planner/v2" {
		t.Fatalf("plan.Version = %q, want %q", plan.Version, "planner/v2")
	}
	if plan.Navigation == nil || len(plan.Navigation.Sections) != 1 {
		t.Fatalf("plan.Navigation = %#v, want one section", plan.Navigation)
	}
	if len(plan.Modules) != 4 || len(plan.Modules[0].NavigationRefs) != 1 || plan.Modules[0].NavigationRefs[0] != "generated" {
		t.Fatalf("plan.Modules = %#v, want navigation-compatible modules", plan.Modules)
	}
}

func TestPlanCommandWritesNavPlanArtifact(t *testing.T) {
	repoDir := filepath.Join("..", "..", "testdata", "sample_repo")
	artifactsDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	originalFactory := plannerClientFactory
	plannerClientFactory = func() llm.Client {
		return llm.NewMockClient(`{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/middleware.go"],"shared":false,"owner":"agent"},{"id":"api","files":["internal/api/handler.go"],"shared":false,"owner":"agent"},{"id":"db","files":["internal/db/client.go"],"shared":false,"owner":"agent"},{"id":"cmd","files":["cmd/main.go"],"shared":false,"owner":"agent"},{"id":"errors","files":["pkg/errors/errors.go"],"shared":true,"owner":"shared_preprocessor"},{"id":"logger","files":["pkg/logger/logger.go"],"shared":true,"owner":"shared_preprocessor"}]}`)
	}
	t.Cleanup(func() { plannerClientFactory = originalFactory })

	configPath := writeCLIConfig(t, `
repo_path: "."
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
  planner_model: "planner-test-model"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
  skeleton_max_tokens: 3000
`)

	stdout, stderr, err := executeCLI(
		t,
		"plan",
		"--config", configPath,
		"--repo", repoDir,
		"--artifacts", artifactsDir,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "nav_plan.json")); err != nil {
		t.Fatalf("nav_plan.json missing: %v", err)
	}
	if !strings.Contains(stdout, "nav_plan.json written to") {
		t.Fatalf("plan stdout = %q, want nav_plan output", stdout)
	}
}

func TestPlanCommandReportsNavPlanLocation(t *testing.T) {
	repoDir := filepath.Join("..", "..", "testdata", "sample_repo")
	artifactsDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	originalFactory := plannerClientFactory
	plannerClientFactory = func() llm.Client {
		return llm.NewMockClient(`{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/middleware.go"],"shared":false,"owner":"agent"},{"id":"api","files":["internal/api/handler.go"],"shared":false,"owner":"agent"},{"id":"db","files":["internal/db/client.go"],"shared":false,"owner":"agent"},{"id":"cmd","files":["cmd/main.go"],"shared":false,"owner":"agent"},{"id":"errors","files":["pkg/errors/errors.go"],"shared":true,"owner":"shared_preprocessor"},{"id":"logger","files":["pkg/logger/logger.go"],"shared":true,"owner":"shared_preprocessor"}]}`)
	}
	t.Cleanup(func() { plannerClientFactory = originalFactory })

	configPath := writeCLIConfig(t, `
repo_path: "."
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
  planner_model: "planner-test-model"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
  skeleton_max_tokens: 3000
`)

	stdout, stderr, err := executeCLI(
		t,
		"plan",
		"--config", configPath,
		"--repo", repoDir,
		"--artifacts", artifactsDir,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr)
	}
	if !strings.Contains(stdout, artifactsDir) {
		t.Fatalf("plan stdout = %q, want artifact dir %q", stdout, artifactsDir)
	}
	if strings.Contains(stdout, "not implemented") {
		t.Fatalf("plan stdout = %q, still using stub output", stdout)
	}
}

func TestValidateCommandWritesValidationReportArtifact(t *testing.T) {
	outputDir := t.TempDir()
	artifactsDir := t.TempDir()
	modulesDir := filepath.Join(outputDir, "modules")
	sharedDir := filepath.Join(outputDir, "shared")
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(modulesDir) error = %v", err)
	}
	if err := os.MkdirAll(sharedDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(sharedDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulesDir, "auth.md"), []byte("See [Logger](../shared/missing.md)\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}

	t.Setenv("OPENAI_API_KEY", "secret-token")
	configPath := writeCLIConfig(t, `
repo_path: "."
output_dir: "./docs"
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(
		t,
		"validate",
		"--config", configPath,
		"--output", outputDir,
		"--artifacts", artifactsDir,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stdout = %s, stderr = %s", err, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(artifactsDir, "validation_report.json")); err != nil {
		t.Fatalf("validation_report.json missing: %v", err)
	}
}

func TestValidateCommandPrintsBrokenLinkSummaryAndExitsZero(t *testing.T) {
	outputDir := t.TempDir()
	artifactsDir := t.TempDir()
	modulesDir := filepath.Join(outputDir, "modules")
	if err := os.MkdirAll(modulesDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(modulesDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(modulesDir, "auth.md"), []byte("See [Logger](../shared/missing.md)\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(auth.md) error = %v", err)
	}

	t.Setenv("OPENAI_API_KEY", "secret-token")
	configPath := writeCLIConfig(t, `
repo_path: "."
output_dir: "./docs"
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(
		t,
		"validate",
		"--config", configPath,
		"--output", outputDir,
		"--artifacts", artifactsDir,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stdout = %s, stderr = %s", err, stdout, stderr)
	}
	if !strings.Contains(stdout, "Validation complete:") {
		t.Fatalf("stdout = %q, want validation summary", stdout)
	}
	if !strings.Contains(stdout, "1 broken links found") {
		t.Fatalf("stdout = %q, want broken link count", stdout)
	}
	if stderr != "" {
		t.Fatalf("stderr = %q, want empty stderr", stderr)
	}
}

func TestGenerateCommandFallsBackToFullGenerateWhenPlanArtifactsAreMissing(t *testing.T) {
	repoDir := filepath.Join("..", "..", "testdata", "sample_repo")
	artifactsDir := t.TempDir()
	outputDir := t.TempDir()
	cfg := sampleGenerateConfig(repoDir, artifactsDir)
	cfg.OutputDir = outputDir
	client := llm.NewMockClient(
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/middleware.go"],"shared":false,"owner":"agent","depends_on_shared":["errors","logger"]},{"id":"api","files":["internal/api/handler.go"],"shared":false,"owner":"agent","depends_on_shared":["logger"]},{"id":"db","files":["internal/db/client.go"],"shared":false,"owner":"agent","depends_on_shared":["errors","logger"]},{"id":"cmd","files":["cmd/main.go"],"shared":false,"owner":"agent"},{"id":"errors","files":["pkg/errors/errors.go"],"shared":true,"owner":"shared_preprocessor"},{"id":"logger","files":["pkg/logger/logger.go"],"shared":true,"owner":"shared_preprocessor"}]}`,
		`{"summary":"Shared error helpers.","key_types":[],"key_functions":[]}`,
		`{"summary":"Shared logger helpers.","key_types":["Logger"],"key_functions":[{"name":"New","signature":"func New() *Logger","ref":"wrong.go#L1"}]}`,
		"# Auth doc\n\n## Overview\n",
		"# API doc\n\n## Overview\n",
		"# DB doc\n\n## Overview\n",
		"# CMD doc\n\n## Overview\n",
	)

	if err := runGenerate(newGenerateCmd(), cfg, client); err != nil {
		t.Fatalf("runGenerate() error = %v", err)
	}

	for _, path := range []string{
		filepath.Join(artifactsDir, "file_index.json"),
		filepath.Join(artifactsDir, "dep_graph.json"),
		filepath.Join(artifactsDir, "call_graph.json"),
		filepath.Join(artifactsDir, "nav_plan.json"),
		filepath.Join(artifactsDir, "shared_context.json"),
		filepath.Join(artifactsDir, "module_docs", "auth.md"),
		filepath.Join(outputDir, "index.md"),
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected generated artifact %q: %v", path, err)
		}
	}
	if _, err := os.Stat(filepath.Join(outputDir, "shared", "logger.md")); err != nil {
		t.Fatalf("shared/logger.md missing: %v", err)
	}
}

func TestGenerateCommandDelegatesToSharedFullGeneratePath(t *testing.T) {
	repoDir := filepath.Join("..", "..", "testdata", "sample_repo")
	artifactsDir := t.TempDir()
	cfg := sampleGenerateConfig(repoDir, artifactsDir)
	client := llm.NewMockClient()

	called := 0
	original := runFullGenerate
	runFullGenerate = func(ctx context.Context, gotCfg *configpkg.Config, gotClient llm.Client) error {
		called++
		if gotCfg != cfg {
			t.Fatalf("cfg pointer mismatch")
		}
		if gotClient != client {
			t.Fatalf("client mismatch")
		}
		return nil
	}
	t.Cleanup(func() { runFullGenerate = original })

	if err := runGenerate(newGenerateCmd(), cfg, client); err != nil {
		t.Fatalf("runGenerate() error = %v", err)
	}
	if called != 1 {
		t.Fatalf("runFullGenerate called %d times, want 1", called)
	}
}

func TestBuildCommandErrorsWhenVitePressConfigIsMissing(t *testing.T) {
	outputDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")
	configPath := writeCLIConfig(t, `
repo_path: "."
output_dir: "./docs"
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(
		t,
		"build",
		"--config", configPath,
		"--output", outputDir,
	)
	if err == nil {
		t.Fatalf("Execute() error = nil, stdout = %s, stderr = %s", stdout, stderr)
	}
	if !strings.Contains(stderr, "run wikismit generate first") {
		t.Fatalf("stderr = %q, want missing config guidance", stderr)
	}
}

func TestBuildCommandErrorsWhenNodeIsUnavailable(t *testing.T) {
	outputDir := t.TempDir()
	vitepressDir := filepath.Join(outputDir, ".vitepress")
	if err := os.MkdirAll(vitepressDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(vitepressDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(vitepressDir, "config.mts"), []byte("export default {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config.mts) error = %v", err)
	}

	t.Setenv("OPENAI_API_KEY", "secret-token")
	t.Setenv("PATH", "")
	configPath := writeCLIConfig(t, `
repo_path: "."
output_dir: "./docs"
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(
		t,
		"build",
		"--config", configPath,
		"--output", outputDir,
	)
	if err == nil {
		t.Fatalf("Execute() error = nil, stdout = %s, stderr = %s", stdout, stderr)
	}
	if !strings.Contains(stderr, "Node.js 20+") {
		t.Fatalf("stderr = %q, want Node.js prerequisite message", stderr)
	}
}

func TestBuildCommandInstallsVitePressWhenNodeModulesMissing(t *testing.T) {
	outputDir := t.TempDir()
	vitepressDir := filepath.Join(outputDir, ".vitepress")
	if err := os.MkdirAll(vitepressDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(vitepressDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(vitepressDir, "config.mts"), []byte("export default {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config.mts) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "package.json"), []byte(`{"scripts":{"docs:build":"vitepress build"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(package.json) error = %v", err)
	}

	originalLookPath := lookPath
	lookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	t.Cleanup(func() { lookPath = originalLookPath })

	commands := make([]string, 0)
	originalRunner := runCommand
	runCommand = func(dir string, name string, args ...string) error {
		commands = append(commands, dir+" :: "+name+" "+strings.Join(args, " "))
		return nil
	}
	t.Cleanup(func() { runCommand = originalRunner })

	t.Setenv("OPENAI_API_KEY", "secret-token")
	configPath := writeCLIConfig(t, `
repo_path: "."
output_dir: "./docs"
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(
		t,
		"build",
		"--config", configPath,
		"--output", outputDir,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stdout = %s, stderr = %s", err, stdout, stderr)
	}
	if len(commands) != 2 {
		t.Fatalf("command count = %d, want 2; commands = %#v", len(commands), commands)
	}
	if !strings.Contains(commands[0], "npm install") || strings.Contains(commands[0], "vitepress") {
		t.Fatalf("first command = %q, want plain npm install", commands[0])
	}
	if !strings.Contains(commands[1], "npm run docs:build") {
		t.Fatalf("second command = %q, want npm run docs:build", commands[1])
	}
}

func TestBuildCommandSkipsInstallWhenNodeModulesAlreadyExist(t *testing.T) {
	outputDir := t.TempDir()
	vitepressDir := filepath.Join(outputDir, ".vitepress")
	if err := os.MkdirAll(vitepressDir, 0o755); err != nil {
		t.Fatalf("MkdirAll(vitepressDir) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(vitepressDir, "config.ts"), []byte("export default {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile(config.ts) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(outputDir, "package.json"), []byte(`{"scripts":{"docs:build":"vitepress build"}}`), 0o644); err != nil {
		t.Fatalf("WriteFile(package.json) error = %v", err)
	}
	if err := os.MkdirAll(filepath.Join(outputDir, "node_modules"), 0o755); err != nil {
		t.Fatalf("MkdirAll(node_modules) error = %v", err)
	}

	originalLookPath := lookPath
	lookPath = func(file string) (string, error) { return "/usr/bin/" + file, nil }
	t.Cleanup(func() { lookPath = originalLookPath })

	commands := make([]string, 0)
	originalRunner := runCommand
	runCommand = func(dir string, name string, args ...string) error {
		commands = append(commands, dir+" :: "+name+" "+strings.Join(args, " "))
		return nil
	}
	t.Cleanup(func() { runCommand = originalRunner })

	t.Setenv("OPENAI_API_KEY", "secret-token")
	configPath := writeCLIConfig(t, `
repo_path: "."
output_dir: "./docs"
artifacts_dir: "./artifacts"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(
		t,
		"build",
		"--config", configPath,
		"--output", outputDir,
	)
	if err != nil {
		t.Fatalf("Execute() error = %v, stdout = %s, stderr = %s", err, stdout, stderr)
	}
	if len(commands) != 1 {
		t.Fatalf("command count = %d, want 1; commands = %#v", len(commands), commands)
	}
	if !strings.Contains(commands[0], "npm run docs:build") {
		t.Fatalf("command = %q, want npm run docs:build", commands[0])
	}
}

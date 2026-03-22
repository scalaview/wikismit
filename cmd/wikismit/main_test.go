package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/scalaview/wikismit/internal/llm"
)

func writeCLIConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
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

func TestGeneratePrintsConfigErrorsToStderr(t *testing.T) {
	stdout, stderr, err := executeCLI(t, "generate", "--config", "/tmp/does-not-exist.yaml")
	if err == nil {
		t.Fatalf("Execute() error = nil, stdout = %s", stdout)
	}
	if !strings.Contains(stderr, "read config") {
		t.Fatalf("stderr = %q, want config read error", stderr)
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

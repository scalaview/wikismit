package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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

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

func TestGenerateLoadsConfigAndPrintsResolvedConfig(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeCLIConfig(t, `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(t, "generate", "--config", configPath)
	if err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr)
	}
	if !strings.Contains(stdout, "repo_path") {
		t.Fatalf("generate output missing repo_path:\n%s", stdout)
	}
	if !strings.Contains(stdout, repoDir) {
		t.Fatalf("generate output missing repo path %q:\n%s", repoDir, stdout)
	}
	if !strings.Contains(stdout, "api_key_env") {
		t.Fatalf("generate output missing api_key_env:\n%s", stdout)
	}
}

func TestStubCommandPrintsNotImplementedAfterConfigBootstrap(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeCLIConfig(t, `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	stdout, stderr, err := executeCLI(t, "update", "--config", configPath)
	if err != nil {
		t.Fatalf("Execute() error = %v, stderr = %s", err, stderr)
	}
	if !strings.Contains(stdout, "not implemented") {
		t.Fatalf("update output = %q, want not implemented", stdout)
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

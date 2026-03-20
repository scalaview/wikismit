package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestConfig(t *testing.T, dir string, body string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(strings.TrimSpace(body)+"\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path

}

func TestLoadConfigLoadsValidYAML(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeTestConfig(t, t.TempDir(), `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}

	if cfg.RepoPath != repoDir {
		t.Fatalf("RepoPath = %q, want %q", cfg.RepoPath, repoDir)
	}
	if cfg.LLM.APIKey() != "secret-token" {
		t.Fatalf("APIKey() = %q, want %q", cfg.LLM.APIKey(), "secret-token")
	}
	if cfg.OutputDir != "./docs" {
		t.Fatalf("OutputDir = %q, want ./docs", cfg.OutputDir)
	}
	if cfg.ArtifactsDir != "./artifacts" {
		t.Fatalf("ArtifactsDir = %q, want ./artifacts", cfg.ArtifactsDir)
	}
	if cfg.LLM.BaseURL != "https://api.openai.com/v1" {
		t.Fatalf("BaseURL = %q", cfg.LLM.BaseURL)
	}
	if cfg.Agent.Concurrency != 4 {
		t.Fatalf("Concurrency = %d, want 4", cfg.Agent.Concurrency)
	}
	if cfg.Analysis.SharedModuleThreshold != 3 {
		t.Fatalf("SharedModuleThreshold = %d, want 3", cfg.Analysis.SharedModuleThreshold)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

func TestLoadConfigErrorsWhenConfiguredEnvVarMissing(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "")

	configPath := writeTestConfig(t, t.TempDir(), `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Fatal("LoadConfig() error = nil, want env var error")
	}
	if !strings.Contains(err.Error(), "env var OPENAI_API_KEY is not set") {
		t.Fatalf("LoadConfig() error = %v, want missing env var message", err)
	}
}

func TestValidateRejectsMissingAPIKeyEnv(t *testing.T) {
	cfg := &Config{
		RepoPath: t.TempDir(),
		Analysis: AnalysisConfig{SharedModuleThreshold: 1},
		Agent:    AgentConfig{Concurrency: 4},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "LLM.APIKeyEnv") {
		t.Fatalf("Validate() error = %v, want APIKeyEnv violation", err)
	}
}

func TestValidateRejectsBadConcurrency(t *testing.T) {
	cfg := &Config{
		RepoPath: t.TempDir(),
		LLM:      LLMConfig{APIKeyEnv: "OPENAI_API_KEY"},
		Analysis: AnalysisConfig{SharedModuleThreshold: 1},
		Agent:    AgentConfig{Concurrency: 0},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "Agent.Concurrency") {
		t.Fatalf("Validate() error = %v, want concurrency violation", err)
	}
}

func TestLoadConfigReadsResolvedAPIKey(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("CUSTOM_API_KEY", "resolved-value")

	configPath := writeTestConfig(t, t.TempDir(), `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "CUSTOM_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if got := cfg.LLM.APIKey(); got != "resolved-value" {
		t.Fatalf("APIKey() = %q, want resolved-value", got)
	}
}

func TestLoadConfigPreservesExplicitFalseCacheEnabled(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeTestConfig(t, t.TempDir(), `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
cache:
  enabled: false
`)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Cache.Enabled {
		t.Fatal("Cache.Enabled = true, want false when explicitly configured")
	}
}

func TestValidateRejectsMissingRepoPath(t *testing.T) {
	cfg := &Config{
		RepoPath: filepath.Join(t.TempDir(), "missing"),
		LLM:      LLMConfig{APIKeyEnv: "OPENAI_API_KEY"},
		Analysis: AnalysisConfig{SharedModuleThreshold: 1},
		Agent:    AgentConfig{Concurrency: 4},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "RepoPath") {
		t.Fatalf("Validate() error = %v, want RepoPath violation", err)
	}
}

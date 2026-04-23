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
		LLM:      &LLMConfig{},
		Analysis: &AnalysisConfig{SharedModuleThreshold: 1},
		Agent:    &AgentConfig{Concurrency: 4},
		Cache:    &CacheConfig{},
		Site:     &SiteConfig{},
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
		LLM:      &LLMConfig{APIKeyEnv: "OPENAI_API_KEY"},
		Analysis: &AnalysisConfig{SharedModuleThreshold: 1},
		Agent:    &AgentConfig{Concurrency: 0},
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

func TestLoadConfigReadsPreprocessorModel(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeTestConfig(t, t.TempDir(), `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
  planner_model: "planner-model"
  preprocessor_model: "preprocessor-model"
analysis:
  shared_module_threshold: 3
agent:
  concurrency: 4
`)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.LLM.PreprocessorModel != "preprocessor-model" {
		t.Fatalf("PreprocessorModel = %q, want preprocessor-model", cfg.LLM.PreprocessorModel)
	}
}

func TestValidateRejectsMissingRepoPath(t *testing.T) {
	cfg := &Config{
		RepoPath: filepath.Join(t.TempDir(), "missing"),
		LLM:      &LLMConfig{APIKeyEnv: "OPENAI_API_KEY"},
		Analysis: &AnalysisConfig{SharedModuleThreshold: 1},
		Agent:    &AgentConfig{Concurrency: 4},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Validate() error = nil, want validation error")
	}
	if !strings.Contains(err.Error(), "RepoPath") {
		t.Fatalf("Validate() error = %v, want RepoPath violation", err)
	}
}

func TestValidateRejectsBadImportanceThreshold(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		value float64
	}{
		{"negative", -0.1},
		{"above one", 1.5},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				RepoPath: t.TempDir(),
				LLM:      &LLMConfig{APIKeyEnv: "OPENAI_API_KEY"},
				Analysis: &AnalysisConfig{SharedModuleThreshold: 1, ImportanceThreshold: tt.value},
				Agent:    &AgentConfig{Concurrency: 4},
			}
			err := cfg.Validate()
			if err == nil {
				t.Fatal("Validate() error = nil, want validation error")
			}
			if !strings.Contains(err.Error(), "ImportanceThreshold") {
				t.Fatalf("Validate() error = %v, want ImportanceThreshold violation", err)
			}
		})
	}
}

func TestLoadConfigAppliesDefaultImportanceThreshold(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeTestConfig(t, t.TempDir(), `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
agent:
  concurrency: 4
`)
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Analysis.ImportanceThreshold != 0.1 {
		t.Fatalf("ImportanceThreshold = %.2f, want 0.10", cfg.Analysis.ImportanceThreshold)
	}
}

func TestLoadConfigReadsExplicitImportanceThreshold(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeTestConfig(t, t.TempDir(), `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
analysis:
  importance_threshold: 0.5
agent:
  concurrency: 4
`)
	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.Analysis.ImportanceThreshold != 0.5 {
		t.Fatalf("ImportanceThreshold = %.2f, want 0.50", cfg.Analysis.ImportanceThreshold)
	}
}

func TestLoadConfigAppliesEventFlowDefaults(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeTestConfig(t, t.TempDir(), `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
agent:
  concurrency: 4
`)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.EventFlow == nil {
		t.Fatal("EventFlow = nil, want defaults")
	}
	if cfg.EventFlow.Enabled {
		t.Fatal("EventFlow.Enabled = true, want false by default")
	}
	if cfg.EventFlow.IncludeHintsInRound1 {
		t.Fatal("EventFlow.IncludeHintsInRound1 = true, want false by default")
	}
}

func TestLoadConfigReadsExplicitEventFlowSettings(t *testing.T) {
	repoDir := t.TempDir()
	t.Setenv("OPENAI_API_KEY", "secret-token")

	configPath := writeTestConfig(t, t.TempDir(), `
repo_path: "`+repoDir+`"
llm:
  api_key_env: "OPENAI_API_KEY"
event_flow:
  enabled: true
  include_hints_in_round1: true
agent:
  concurrency: 4
`)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.EventFlow == nil {
		t.Fatal("EventFlow = nil, want explicit config")
	}
	if !cfg.EventFlow.Enabled {
		t.Fatal("EventFlow.Enabled = false, want true")
	}
	if !cfg.EventFlow.IncludeHintsInRound1 {
		t.Fatal("EventFlow.IncludeHintsInRound1 = false, want true")
	}
}

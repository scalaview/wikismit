package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

type Config struct {
	RepoPath     string         `yaml:"repo_path"`
	OutputDir    string         `yaml:"output_dir"`
	ArtifactsDir string         `yaml:"artifacts_dir"`
	LLM          LLMConfig      `yaml:"llm"`
	Analysis     AnalysisConfig `yaml:"analysis"`
	Agent        AgentConfig    `yaml:"agent"`
	Cache        CacheConfig    `yaml:"cache"`
	Site         SiteConfig     `yaml:"site"`
}

type LLMConfig struct {
	BaseURL           string  `yaml:"base_url"`
	APIKeyEnv         string  `yaml:"api_key_env"`
	PlannerModel      string  `yaml:"planner_model"`
	PreprocessorModel string  `yaml:"preprocessor_model"`
	AgentModel        string  `yaml:"agent_model"`
	MaxTokens         int     `yaml:"max_tokens"`
	Temperature       float32 `yaml:"temperature"`
	TimeoutSeconds    int     `yaml:"timeout_seconds"`

	resolvedAPIKey string
}

type AnalysisConfig struct {
	Languages             []string `yaml:"languages"`
	ExcludePatterns       []string `yaml:"exclude_patterns"`
	SharedModuleThreshold int      `yaml:"shared_module_threshold"`
}

type AgentConfig struct {
	Concurrency       int `yaml:"concurrency"`
	SkeletonMaxTokens int `yaml:"skeleton_max_tokens"`
}

type CacheConfig struct {
	Enabled bool   `yaml:"enabled"`
	Dir     string `yaml:"dir"`
}

type SiteConfig struct {
	Title   string `yaml:"title"`
	RepoURL string `yaml:"repo_url"`
	Logo    string `yaml:"logo"`
}

func defaultConfig() Config {
	return Config{
		RepoPath:     ".",
		OutputDir:    "./docs",
		ArtifactsDir: "./artifacts",
		LLM: LLMConfig{
			BaseURL:           "https://api.openai.com/v1",
			PlannerModel:      "gpt-4o-mini",
			PreprocessorModel: "gpt-4o-mini",
			AgentModel:        "gpt-4o",
			MaxTokens:         4096,
			Temperature:       0.2,
			TimeoutSeconds:    120,
		},
		Analysis: AnalysisConfig{
			Languages: []string{"go", "python", "typescript", "rust", "java"},
			ExcludePatterns: []string{
				"*_test.go",
				"vendor/**",
				"node_modules/**",
				"**/*.pb.go",
			},
			SharedModuleThreshold: 3,
		},
		Agent: AgentConfig{
			Concurrency:       4,
			SkeletonMaxTokens: 3000,
		},
		Cache: CacheConfig{
			Enabled: true,
			Dir:     "./artifacts/cache",
		},
	}
}

func applyDefaults(cfg *Config) {
	defaults := defaultConfig()

	if cfg.RepoPath == "" {
		cfg.RepoPath = defaults.RepoPath
	}
	if cfg.OutputDir == "" {
		cfg.OutputDir = defaults.OutputDir
	}
	if cfg.ArtifactsDir == "" {
		cfg.ArtifactsDir = defaults.ArtifactsDir
	}

	if cfg.LLM.BaseURL == "" {
		cfg.LLM.BaseURL = defaults.LLM.BaseURL
	}
	if cfg.LLM.PlannerModel == "" {
		cfg.LLM.PlannerModel = defaults.LLM.PlannerModel
	}
	if cfg.LLM.PreprocessorModel == "" {
		cfg.LLM.PreprocessorModel = cfg.LLM.PlannerModel
	}
	if cfg.LLM.AgentModel == "" {
		cfg.LLM.AgentModel = defaults.LLM.AgentModel
	}
	if cfg.LLM.MaxTokens == 0 {
		cfg.LLM.MaxTokens = defaults.LLM.MaxTokens
	}
	if cfg.LLM.Temperature == 0 {
		cfg.LLM.Temperature = defaults.LLM.Temperature
	}
	if cfg.LLM.TimeoutSeconds == 0 {
		cfg.LLM.TimeoutSeconds = defaults.LLM.TimeoutSeconds
	}

	if len(cfg.Analysis.Languages) == 0 {
		cfg.Analysis.Languages = append([]string(nil), defaults.Analysis.Languages...)
	}
	if len(cfg.Analysis.ExcludePatterns) == 0 {
		cfg.Analysis.ExcludePatterns = append([]string(nil), defaults.Analysis.ExcludePatterns...)
	}
	if cfg.Analysis.SharedModuleThreshold == 0 {
		cfg.Analysis.SharedModuleThreshold = defaults.Analysis.SharedModuleThreshold
	}

	if cfg.Agent.Concurrency == 0 {
		cfg.Agent.Concurrency = defaults.Agent.Concurrency
	}
	if cfg.Agent.SkeletonMaxTokens == 0 {
		cfg.Agent.SkeletonMaxTokens = defaults.Agent.SkeletonMaxTokens
	}

	if cfg.Cache.Dir == "" {
		cfg.Cache.Dir = defaults.Cache.Dir
	}
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config %q: %w", path, err)
	}

	cfg := defaultConfig()
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config %q: %w", path, err)
	}
	applyDefaults(&cfg)

	if cfg.LLM.APIKeyEnv != "" {
		cfg.LLM.resolvedAPIKey = os.Getenv(cfg.LLM.APIKeyEnv)
		if cfg.LLM.resolvedAPIKey == "" {
			return nil, fmt.Errorf("env var %s is not set", cfg.LLM.APIKeyEnv)
		}
	}

	return &cfg, nil
}

func (c LLMConfig) APIKey() string {
	return c.resolvedAPIKey
}

func (c *Config) Validate() error {
	var errs []error

	if c.RepoPath == "" {
		errs = append(errs, errors.New("RepoPath must not be empty"))
	} else {
		info, err := os.Stat(c.RepoPath)
		if err != nil {
			errs = append(errs, fmt.Errorf("RepoPath must be an existing directory: %w", err))
		} else if !info.IsDir() {
			errs = append(errs, fmt.Errorf("RepoPath must be a directory: %s", c.RepoPath))
		}
	}

	if c.LLM.APIKeyEnv == "" {
		errs = append(errs, errors.New("LLM.APIKeyEnv must not be empty"))
	}
	if c.Agent.Concurrency < 1 || c.Agent.Concurrency > 32 {
		errs = append(errs, fmt.Errorf("Agent.Concurrency must be between 1 and 32, got %d", c.Agent.Concurrency))
	}
	if c.Analysis.SharedModuleThreshold < 1 {
		errs = append(errs, fmt.Errorf("Analysis.SharedModuleThreshold must be >= 1, got %d", c.Analysis.SharedModuleThreshold))
	}

	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}

func (c *Config) RepoAbsolutePath() (string, error) {
	return filepath.Abs(c.RepoPath)
}

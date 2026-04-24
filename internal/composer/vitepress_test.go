package composer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/log"
	"github.com/scalaview/wikismit/pkg/store"
)

func TestMain(m *testing.M) {
	log.SetDefaultLogger(log.New(true))
	os.Exit(m.Run())
}

func TestGenerateVitePressConfigBuildsModulesAndSharedSidebarGroups(t *testing.T) {
	plan := &store.NavPlan{Modules: []*store.Module{
		{ID: "auth", Shared: false},
		{ID: "api", Shared: false},
		{ID: "logger", Shared: true},
	}}
	cfg := &configpkg.Config{RepoPath: "/tmp/wikismit", Site: &configpkg.SiteConfig{Title: "WikiSmit Docs"}}

	result, err := GenerateVitePressConfig(plan, store.DepGraph{}, cfg)
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() error = %v", err)
	}

	if !strings.Contains(result, "text: 'Modules'") {
		t.Fatalf("config missing Modules sidebar group:\n%s", result)
	}
	if !strings.Contains(result, "text: 'Shared'") {
		t.Fatalf("config missing Shared sidebar group:\n%s", result)
	}
	modulesIndex := strings.Index(result, "text: 'Modules'")
	sharedIndex := strings.Index(result, "text: 'Shared'")
	if modulesIndex == -1 || sharedIndex == -1 || modulesIndex > sharedIndex {
		t.Fatalf("config sidebar groups out of order:\n%s", result)
	}
	if !strings.Contains(result, "'/modules/auth.md'") || !strings.Contains(result, "'/shared/logger.md'") {
		t.Fatalf("config missing expected module/shared links:\n%s", result)
	}
}

func TestGenerateVitePressConfigIncludesNavigationSectionsBeforeModulesAndShared(t *testing.T) {
	plan := &store.NavPlan{
		Navigation: &store.Navigation{Sections: []*store.NavigationSection{
			{Title: "Generated Docs", Type: "generated"},
			{Title: "Business Flows", Type: "business"},
			{Title: "Event Flows", Type: "events"},
			{Title: "Architecture", Type: "architecture"},
		}},
		Modules: []*store.Module{
			{ID: "auth", Shared: false},
			{ID: "logger", Shared: true},
		},
	}

	result, err := GenerateVitePressConfig(plan, store.DepGraph{}, &configpkg.Config{RepoPath: "/tmp/wikismit"})
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() error = %v", err)
	}

	for _, want := range []string{
		"text: 'Generated Docs'",
		"'/generated/generated.md'",
		"text: 'Business Flows'",
		"'/generated/business.md'",
		"text: 'Event Flows'",
		"'/generated/events.md'",
		"text: 'Architecture'",
		"'/generated/architecture.md'",
		"text: 'Modules'",
		"text: 'Shared'",
	} {
		if !strings.Contains(result, want) {
			t.Fatalf("config missing %q:\n%s", want, result)
		}
	}

	generatedIndex := strings.Index(result, "text: 'Generated Docs'")
	businessIndex := strings.Index(result, "text: 'Business Flows'")
	eventsIndex := strings.Index(result, "text: 'Event Flows'")
	architectureIndex := strings.Index(result, "text: 'Architecture'")
	modulesIndex := strings.Index(result, "text: 'Modules'")
	sharedIndex := strings.Index(result, "text: 'Shared'")
	if !(generatedIndex < businessIndex && businessIndex < eventsIndex && eventsIndex < architectureIndex && architectureIndex < modulesIndex && modulesIndex < sharedIndex) {
		t.Fatalf("config sidebar ordering incorrect:\n%s", result)
	}
}

func TestGenerateVitePressConfigNavigationLegacyModulesOnlyStillWorks(t *testing.T) {
	plan := &store.NavPlan{Modules: []*store.Module{{ID: "auth", Shared: false}}}

	result, err := GenerateVitePressConfig(plan, store.DepGraph{}, &configpkg.Config{RepoPath: "/tmp/wikismit"})
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() error = %v", err)
	}

	if strings.Contains(result, "'/generated/") {
		t.Fatalf("legacy modules-only config unexpectedly contains generated nav links:\n%s", result)
	}
	if !strings.Contains(result, "'/modules/auth.md'") {
		t.Fatalf("legacy modules-only config missing module link:\n%s", result)
	}
}

func TestGenerateVitePressConfigNavigationToleratesNilSite(t *testing.T) {
	plan := &store.NavPlan{Modules: []*store.Module{{ID: "auth", Shared: false}}}

	result, err := GenerateVitePressConfig(plan, store.DepGraph{}, &configpkg.Config{RepoPath: "/tmp/wikismit"})
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() error = %v", err)
	}

	if !strings.Contains(result, "title: 'wikismit'") {
		t.Fatalf("config missing repo fallback title when site is nil:\n%s", result)
	}
}

func TestGenerateVitePressConfigUsesSiteTitleOrRepoNameFallback(t *testing.T) {
	plan := &store.NavPlan{}

	withTitle, err := GenerateVitePressConfig(plan, store.DepGraph{}, &configpkg.Config{
		RepoPath: "/tmp/wikismit",
		Site:     &configpkg.SiteConfig{Title: "Custom Title"},
	})
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() with title error = %v", err)
	}
	if !strings.Contains(withTitle, "title: 'Custom Title'") {
		t.Fatalf("config missing explicit site title:\n%s", withTitle)
	}

	withFallback, err := GenerateVitePressConfig(plan, store.DepGraph{}, &configpkg.Config{RepoPath: "/tmp/wikismit"})
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() fallback error = %v", err)
	}
	if !strings.Contains(withFallback, "title: 'wikismit'") {
		t.Fatalf("config missing repo-name fallback title:\n%s", withFallback)
	}
}

func TestGenerateVitePressConfigIncludesEditLinkOnlyWhenRepoURLPresent(t *testing.T) {
	plan := &store.NavPlan{}

	withRepoURL, err := GenerateVitePressConfig(plan, store.DepGraph{}, &configpkg.Config{
		RepoPath: "/tmp/wikismit",
		Site:     &configpkg.SiteConfig{RepoURL: "https://github.com/scalaview/wikismit"},
	})
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() with repo URL error = %v", err)
	}
	if !strings.Contains(withRepoURL, "editLink") {
		t.Fatalf("config missing editLink when repo URL present:\n%s", withRepoURL)
	}

	withoutRepoURL, err := GenerateVitePressConfig(plan, store.DepGraph{}, &configpkg.Config{RepoPath: "/tmp/wikismit"})
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() without repo URL error = %v", err)
	}
	if strings.Contains(withoutRepoURL, "editLink") {
		t.Fatalf("config unexpectedly includes editLink without repo URL:\n%s", withoutRepoURL)
	}
}

func TestWriteVitePressAssetsCopiesLogoWhenConfigured(t *testing.T) {
	docsDir := t.TempDir()
	logoPath := filepath.Join(t.TempDir(), "logo.png")
	if err := os.WriteFile(logoPath, []byte("logo-bytes"), 0o644); err != nil {
		t.Fatalf("WriteFile(logoPath) error = %v", err)
	}

	cfg := &configpkg.Config{
		RepoPath:  "/tmp/wikismit",
		OutputDir: docsDir,
		Site: &configpkg.SiteConfig{
			Logo: logoPath,
		},
	}
	configText := "export default {}\n"

	if err := WriteVitePressAssets(docsDir, configText, cfg); err != nil {
		t.Fatalf("WriteVitePressAssets() error = %v", err)
	}

	publicLogo := filepath.Join(docsDir, "public", "logo.png")
	data, err := os.ReadFile(publicLogo)
	if err != nil {
		t.Fatalf("ReadFile(publicLogo) error = %v", err)
	}
	if string(data) != "logo-bytes" {
		t.Fatalf("public logo content = %q, want %q", string(data), "logo-bytes")
	}
	if _, err := os.Stat(filepath.Join(docsDir, ".vitepress", "config.mts")); err != nil {
		t.Fatalf("config.mts missing: %v", err)
	}
}

func TestWriteVitePressAssetsWritesDocsPackageScripts(t *testing.T) {
	docsDir := t.TempDir()

	if err := WriteVitePressAssets(docsDir, "export default {}\n", &configpkg.Config{OutputDir: docsDir}); err != nil {
		t.Fatalf("WriteVitePressAssets() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(docsDir, "package.json"))
	if err != nil {
		t.Fatalf("ReadFile(package.json) error = %v", err)
	}
	content := string(data)
	for _, want := range []string{
		`"docs:build": "vitepress build"`,
		`"docs:preview": "vitepress preview"`,
		`"docs:dev": "vitepress dev"`,
	} {
		if !strings.Contains(content, want) {
			t.Fatalf("package.json missing %q:\n%s", want, content)
		}
	}
}

func TestGenerateVitePressConfigOmitsTemplateArtifactsFromOutput(t *testing.T) {
	result, err := GenerateVitePressConfig(&store.NavPlan{}, store.DepGraph{}, &configpkg.Config{RepoPath: "/tmp/wikismit"})
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() error = %v", err)
	}
	if strings.Contains(result, "{{") || strings.Contains(result, "}}") {
		t.Fatalf("config contains leftover template markers:\n%s", result)
	}
}

func TestGenerateVitePressConfigIncludesIgnoreDeadLinks(t *testing.T) {
	plan := &store.NavPlan{}
	cfg := &configpkg.Config{
		RepoPath: "/tmp/wikismit",
		Site:     &configpkg.SiteConfig{Title: "Test Docs"},
	}

	result, err := GenerateVitePressConfig(plan, store.DepGraph{}, cfg)
	if err != nil {
		t.Fatalf("GenerateVitePressConfig() error = %v", err)
	}

	if !strings.Contains(result, "ignoreDeadLinks: true") {
		t.Fatalf("config missing ignoreDeadLinks: true:\n%s", result)
	}
}

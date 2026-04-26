package composer

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"

	"github.com/scalaview/wikismit/internal/composer/tmpl"
	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

type vitepressSidebarItem struct {
	Text string
	Link string
}

type vitepressSidebarGroup struct {
	Text  string
	Items []vitepressSidebarItem
}

type vitepressTemplateData struct {
	Title              string
	NavigationSections []vitepressSidebarGroup
	Modules            []vitepressSidebarItem
	Shared             []vitepressSidebarItem
	HasEditLink        bool
	RepoURL            string
}

const docsPackageJSON = `{
  "private": true,
  "scripts": {
    "docs:build": "vitepress build",
    "docs:preview": "vitepress preview",
    "docs:dev": "vitepress dev"
  },
  "devDependencies": {
    "vitepress": "^1.6.4"
  }
}
`

func GenerateVitePressConfig(plan *store.NavPlan, graph store.DepGraph, cfg *configpkg.Config) (string, error) {
	_ = graph

	navigationSections := make([]vitepressSidebarGroup, 0)
	if plan != nil && plan.Navigation != nil {
		for _, section := range plan.Navigation.Sections {
			if section == nil {
				continue
			}
			navigationSections = append(navigationSections, vitepressSidebarGroup{
				Text: section.Title,
				Items: []vitepressSidebarItem{{
					Text: section.Title,
					Link: "/generated/" + GenerateSectionFilename(section) + ".md",
				}},
			})
		}
	}

	modules := make([]vitepressSidebarItem, 0)
	shared := make([]vitepressSidebarItem, 0)
	for _, module := range plan.Modules {
		item := vitepressSidebarItem{
			Text: module.ID,
			Link: "/modules/" + module.ID + ".md",
		}
		if module.Shared {
			item.Link = "/shared/" + module.ID + ".md"
			shared = append(shared, item)
			continue
		}
		modules = append(modules, item)
	}

	sort.Slice(modules, func(i int, j int) bool { return modules[i].Text < modules[j].Text })
	sort.Slice(shared, func(i int, j int) bool { return shared[i].Text < shared[j].Text })

	var site *configpkg.SiteConfig
	if cfg != nil {
		site = cfg.Site
	}

	title := ""
	if site != nil {
		title = site.Title
	}
	if title == "" {
		title = filepath.Base(cfg.RepoPath)
	}

	data := vitepressTemplateData{
		Title:              title,
		NavigationSections: navigationSections,
		Modules:            modules,
		Shared:             shared,
		HasEditLink:        site != nil && site.RepoURL != "",
		RepoURL:            repoURL(site),
	}

	var buf bytes.Buffer
	if err := tmpl.VitepressConfigTmplParsed.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func repoURL(site *configpkg.SiteConfig) string {
	if site == nil {
		return ""
	}
	return site.RepoURL
}

func WriteVitePressAssets(docsDir string, configText string, cfg *configpkg.Config) error {
	vitepressDir := filepath.Join(docsDir, ".vitepress")
	if err := os.MkdirAll(vitepressDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(vitepressDir, "config.mts"), []byte(configText), 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(docsDir, "package.json"), []byte(docsPackageJSON), 0o644); err != nil {
		return err
	}

	if cfg == nil || cfg.Site == nil || cfg.Site.Logo == "" {
		return nil
	}

	publicDir := filepath.Join(docsDir, "public")
	if err := os.MkdirAll(publicDir, 0o755); err != nil {
		return err
	}
	logoData, err := os.ReadFile(cfg.Site.Logo)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(publicDir, "logo.png"), logoData, 0o644)
}

func GenerateVitePressMermaidConfig(docsDir string) error {
	var buf bytes.Buffer
	if err := tmpl.VitepressMermaidTmplParsed.Execute(&buf, nil); err != nil {
		return err
	}

	vitepressDir := filepath.Join(docsDir, ".vitepress/theme/")
	if err := os.MkdirAll(vitepressDir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(vitepressDir, "index.ts"), buf.Bytes(), 0o644); err != nil {
		return err
	}

	return nil
}

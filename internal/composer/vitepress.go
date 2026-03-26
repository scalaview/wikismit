package composer

import (
	"bytes"
	"os"
	"path/filepath"
	"sort"
	"text/template"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/pkg/store"
)

type vitepressSidebarItem struct {
	Text string
	Link string
}

type vitepressTemplateData struct {
	Title       string
	Modules     []vitepressSidebarItem
	Shared      []vitepressSidebarItem
	HasEditLink bool
	RepoURL     string
}

var vitepressConfigTemplate = template.Must(template.New("vitepress-config").Parse(`import { defineConfig } from 'vitepress'

export default defineConfig({
  title: '{{ .Title }}',
  themeConfig: {
{{- if .HasEditLink }}
    editLink: {
      pattern: '{{ .RepoURL }}/edit/main/:path',
    },
{{- end }}
    sidebar: [
      {
        text: 'Modules',
        items: [
{{- range .Modules }}
          { text: '{{ .Text }}', link: '{{ .Link }}' },
{{- end }}
        ],
      },
      {
        text: 'Shared',
        items: [
{{- range .Shared }}
          { text: '{{ .Text }}', link: '{{ .Link }}' },
{{- end }}
        ],
      },
    ],
  },
})
`))

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

	title := cfg.Site.Title
	if title == "" {
		title = filepath.Base(cfg.RepoPath)
	}

	data := vitepressTemplateData{
		Title:       title,
		Modules:     modules,
		Shared:      shared,
		HasEditLink: cfg.Site.RepoURL != "",
		RepoURL:     cfg.Site.RepoURL,
	}

	var buf bytes.Buffer
	if err := vitepressConfigTemplate.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
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

	if cfg.Site.Logo == "" {
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

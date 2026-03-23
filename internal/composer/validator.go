package composer

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/scalaview/wikismit/pkg/store"
)

var markdownLinkRegex = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

func ValidateDocs(docsDir string) (store.ValidationReport, error) {
	report := store.ValidationReport{GeneratedAt: time.Now().UTC()}

	err := filepath.WalkDir(docsDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}

		report.TotalFiles++

		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		matches := markdownLinkRegex.FindAllStringSubmatchIndex(string(content), -1)
		for _, match := range matches {
			linkText := string(content[match[2]:match[3]])
			target := string(content[match[4]:match[5]])

			if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "#") {
				continue
			}

			report.TotalLinks++
			cleanTarget, _, _ := strings.Cut(target, "#")
			resolved := filepath.Clean(filepath.Join(filepath.Dir(path), cleanTarget))
			if _, err := os.Stat(resolved); err == nil {
				continue
			}

			report.BrokenLinks = append(report.BrokenLinks, store.BrokenLink{
				SourceFile: path,
				LinkText:   linkText,
				LinkTarget: target,
				Line:       0,
			})
		}

		return nil
	})

	if err != nil {
		return store.ValidationReport{}, err
	}

	return report, nil
}

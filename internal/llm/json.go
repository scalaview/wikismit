package llm

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
)

func ParseJSON[T any](s string, target *T) error {
	tmpPath := filepath.Join(os.TempDir(), "parse_json_debug.txt")
	if err := os.WriteFile(tmpPath, []byte(s), 0644); err != nil {
		return fmt.Errorf("write temp file: %w", err)
	}
	fmt.Println("temp file:", tmpPath)

	re := regexp.MustCompile("(?s)```(?:json)?(.*)```")
	match := re.FindStringSubmatch(s)

	var raw string
	if match == nil {
		raw = s
	} else {
		raw = match[1]
	}
	slog.Info("%s", raw)
	return json.Unmarshal([]byte(raw), target)
}

package llm

import (
	"encoding/json"
	"regexp"
)

func ParseJSON[T any](s string, target *T) error {
	re := regexp.MustCompile("(?s)```(?:json)?(.*)```")
	match := re.FindStringSubmatch(s)

	var raw string
	if match == nil {
		raw = s
	} else {
		raw = match[1]
	}

	return json.Unmarshal([]byte(raw), target)
}

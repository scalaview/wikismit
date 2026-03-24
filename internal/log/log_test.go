package log

import (
	"bytes"
	"strings"
	"testing"
)

func TestNewVerboseLoggerEmitsDebugOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newWithWriter(true, buf)

	logger.Debug("debug message", "key", "value")

	out := buf.String()
	if !strings.Contains(out, "level=DEBUG") {
		t.Fatalf("output = %q, want debug level", out)
	}
	if !strings.Contains(out, "msg=\"debug message\"") {
		t.Fatalf("output = %q, want debug message", out)
	}
	if !strings.Contains(out, "key=value") {
		t.Fatalf("output = %q, want structured field", out)
	}
}

func TestNewNonVerboseLoggerSuppressesDebugOutput(t *testing.T) {
	buf := &bytes.Buffer{}
	logger := newWithWriter(false, buf)

	logger.Debug("debug message")

	if got := buf.String(); got != "" {
		t.Fatalf("output = %q, want empty output", got)
	}
}

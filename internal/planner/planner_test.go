package planner

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	configpkg "github.com/scalaview/wikismit/internal/config"
	"github.com/scalaview/wikismit/internal/llm"
	"github.com/scalaview/wikismit/pkg/store"
)

func samplePlannerConfig(t *testing.T) *configpkg.Config {
	t.Helper()
	return &configpkg.Config{
		RepoPath:     t.TempDir(),
		ArtifactsDir: t.TempDir(),
		LLM: configpkg.LLMConfig{
			PlannerModel: "planner-test-model",
			MaxTokens:    512,
			Temperature:  0.2,
		},
		Analysis: configpkg.AnalysisConfig{
			SharedModuleThreshold: 3,
		},
		Agent: configpkg.AgentConfig{
			SkeletonMaxTokens: 1_000,
		},
	}
}

func samplePlannerIndex() store.FileIndex {
	return store.FileIndex{
		"internal/auth/jwt.go": {
			Functions: []store.FunctionDecl{{
				Name:      "GenerateToken",
				Signature: "func GenerateToken() string",
				LineStart: 10,
				Exported:  true,
			}},
		},
	}
}

func samplePlannerGraph() store.DepGraph {
	return store.DepGraph{
		"internal/auth/jwt.go": {},
	}
}

func TestBuildPlannerPromptIncludesRulesThresholdAndSkeleton(t *testing.T) {
	skeleton := "// === internal/auth/jwt.go ===\nfunc GenerateToken() string  // internal/auth/jwt.go:10"

	got := buildPlannerPrompt(skeleton, 3)

	for _, want := range []string{
		"You are a software architect.",
		"3+ modules",
		"Respond ONLY with valid JSON.",
		"referenced_by",
		skeleton,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("buildPlannerPrompt() missing %q:\n%s", want, got)
		}
	}
}

func TestRunPlannerSucceedsWithValidJSONResponse(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`)

	got, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPlanner() error = %v", err)
	}
	if got.Modules[0].ID != "auth" {
		t.Fatalf("RunPlanner() module ID = %q, want auth", got.Modules[0].ID)
	}
	if client.CallCount() != 1 {
		t.Fatalf("MockClient.CallCount() = %d, want 1", client.CallCount())
	}
}

func TestRunPlannerRetriesAfterJSONParseFailure(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
	)

	got, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPlanner() error = %v", err)
	}
	if got.Modules[0].ID != "auth" {
		t.Fatalf("RunPlanner() module ID = %q, want auth", got.Modules[0].ID)
	}
	if client.CallCount() != 2 {
		t.Fatalf("MockClient.CallCount() = %d, want 2", client.CallCount())
	}
}

func TestRunPlannerFailsAfterThreeInvalidResponses(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(`not-json`, `still-not-json`, `{"modules":[}`)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if client.CallCount() != 3 {
		t.Fatalf("MockClient.CallCount() = %d, want 3", client.CallCount())
	}
	if !strings.Contains(err.Error(), "parse nav plan") {
		t.Fatalf("RunPlanner() error = %v, want parse nav plan context", err)
	}
}

func TestRunPlannerRejectsMissingFileAssignments(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[]}`,
		`{"modules":[]}`,
		`{"modules":[]}`,
	)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "missing file assignment") {
		t.Fatalf("RunPlanner() error = %v, want missing file assignment context", err)
	}
}

func TestRunPlannerRejectsDuplicateFileAssignments(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"},{"id":"dup","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"},{"id":"dup","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"},{"id":"dup","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
	)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "duplicate file assignment") {
		t.Fatalf("RunPlanner() error = %v, want duplicate file assignment context", err)
	}
}

func TestRunPlannerRejectsInvalidOwnerValue(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"planner"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"planner"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"planner"}]}`,
	)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "invalid owner") {
		t.Fatalf("RunPlanner() error = %v, want invalid owner context", err)
	}
}

func TestRunPlannerRejectsEmptyModuleID(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[{"id":"","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
	)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "empty module id") {
		t.Fatalf("RunPlanner() error = %v, want empty module id context", err)
	}
}

func TestRunPlannerRejectsDuplicateModuleIDs(t *testing.T) {
	cfg := samplePlannerConfig(t)
	idx := store.FileIndex{
		"internal/auth/jwt.go": samplePlannerIndex()["internal/auth/jwt.go"],
		"internal/auth/session.go": {
			Functions: []store.FunctionDecl{{
				Name:      "StartSession",
				Signature: "func StartSession() string",
				LineStart: 12,
				Exported:  true,
			}},
		},
	}
	graph := store.DepGraph{
		"internal/auth/jwt.go":     {},
		"internal/auth/session.go": {},
	}
	client := llm.NewMockClient(
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"},{"id":"auth","files":["internal/auth/session.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"},{"id":"auth","files":["internal/auth/session.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"},{"id":"auth","files":["internal/auth/session.go"],"shared":false,"owner":"agent"}]}`,
	)

	_, err := RunPlanner(context.Background(), idx, graph, cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "duplicate module id") {
		t.Fatalf("RunPlanner() error = %v, want duplicate module id context", err)
	}
}

func TestRunPlannerRejectsFilesMissingFromIndex(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/missing.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/missing.go"],"shared":false,"owner":"agent"}]}`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go","internal/auth/missing.go"],"shared":false,"owner":"agent"}]}`,
	)

	_, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err == nil {
		t.Fatal("RunPlanner() error = nil, want error")
	}
	if !strings.Contains(err.Error(), "not found in file index") {
		t.Fatalf("RunPlanner() error = %v, want missing file index context", err)
	}
}

func TestRunPlannerSetsGeneratedAtOnSuccess(t *testing.T) {
	cfg := samplePlannerConfig(t)
	client := llm.NewMockClient(`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`)

	got, err := RunPlanner(context.Background(), samplePlannerIndex(), samplePlannerGraph(), cfg, client)
	if err != nil {
		t.Fatalf("RunPlanner() error = %v", err)
	}
	if got.GeneratedAt.Equal(time.Time{}) {
		t.Fatal("RunPlanner() GeneratedAt is zero, want non-zero time")
	}
}

func TestRunPlannerVerboseLoggingIncludesPromptSizingAndAttemptMetadataBeforeEachLLMCall(t *testing.T) {
	cfg := samplePlannerConfig(t)
	cfg.Verbose = true
	idx := samplePlannerIndex()
	graph := samplePlannerGraph()
	buf := capturePlannerLogOutput(t, true)
	skeleton := BuildFullSkeleton(idx, cfg.Agent.SkeletonMaxTokens)
	expectedSkeletonTokens := estimateTokens(skeleton)
	responses := []string{
		`{"modules":[`,
		`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`,
	}
	callCount := 0

	client := stubPlannerClient{complete: func(ctx context.Context, req llm.CompletionRequest) (string, error) {
		_ = ctx
		callCount++

		out := buf.String()
		if got := strings.Count(out, `msg="starting planner completion request"`); got != callCount {
			t.Fatalf("starting planner log count = %d, want %d; output=%q", got, callCount, out)
		}
		for _, want := range []string{
			`level=DEBUG`,
			`msg="starting planner completion request"`,
			fmt.Sprintf("skeleton_token_estimate=%d", expectedSkeletonTokens),
			fmt.Sprintf("prompt_length=%d", len(req.UserMsg)),
			fmt.Sprintf("planner_attempt=%d", callCount),
			`model=planner-test-model`,
		} {
			if !strings.Contains(out, want) {
				t.Fatalf("planner log output missing %q in %q", want, out)
			}
		}

		return responses[callCount-1], nil
	}}

	got, err := RunPlanner(context.Background(), idx, graph, cfg, client)
	if err != nil {
		t.Fatalf("RunPlanner() error = %v", err)
	}
	if got.Modules[0].ID != "auth" {
		t.Fatalf("RunPlanner() module ID = %q, want auth", got.Modules[0].ID)
	}
	if callCount != 2 {
		t.Fatalf("planner call count = %d, want 2", callCount)
	}
	for _, unwanted := range []string{"GenerateToken", "internal/auth/jwt.go:10"} {
		if strings.Contains(buf.String(), unwanted) {
			t.Fatalf("planner log output = %q, should not contain prompt body fragment %q", buf.String(), unwanted)
		}
	}
}

func TestRunPlannerVerboseLoggingUsesVerboseLoggerWithoutTestOverride(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("stderr pipe capture is unix-focused")
	}

	previous := logger
	logger = nil
	t.Cleanup(func() {
		logger = previous
	})

	cfg := samplePlannerConfig(t)
	cfg.Verbose = true
	idx := samplePlannerIndex()
	graph := samplePlannerGraph()
	client := llm.NewMockClient(`{"modules":[{"id":"auth","files":["internal/auth/jwt.go"],"shared":false,"owner":"agent"}]}`)

	out := captureStderrOutput(t, func() {
		_, err := RunPlanner(context.Background(), idx, graph, cfg, client)
		if err != nil {
			t.Fatalf("RunPlanner() error = %v", err)
		}
	})

	for _, want := range []string{
		`level=DEBUG`,
		`msg="starting planner completion request"`,
		`skeleton_token_estimate=`,
		`prompt_length=`,
		`planner_attempt=1`,
		`model=planner-test-model`,
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("planner log output missing %q in %q", want, out)
		}
	}
	for _, unwanted := range []string{"GenerateToken", "internal/auth/jwt.go:10"} {
		if strings.Contains(out, unwanted) {
			t.Fatalf("planner log output = %q, should not contain prompt body fragment %q", out, unwanted)
		}
	}
}

func captureStderrOutput(t *testing.T, fn func()) string {
	t.Helper()

	originalStderr := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stderr = w

	outputCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = buf.ReadFrom(r)
		outputCh <- buf.String()
	}()

	fn()

	if err := w.Close(); err != nil {
		t.Fatalf("stderr close error = %v", err)
	}
	os.Stderr = originalStderr
	t.Cleanup(func() {
		os.Stderr = originalStderr
	})

	return <-outputCh
}

func capturePlannerLogOutput(t *testing.T, verbose bool) *bytes.Buffer {
	t.Helper()

	buf := &bytes.Buffer{}
	previous := logger
	logger = newPlannerBufferLogger(verbose, buf)
	t.Cleanup(func() {
		logger = previous
	})
	return buf
}

func newPlannerBufferLogger(verbose bool, buf *bytes.Buffer) *plannerBufferLogger {
	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	return &plannerBufferLogger{
		inner: slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: level})),
	}
}

type plannerBufferLogger struct {
	inner *slog.Logger
}

func (l *plannerBufferLogger) Debug(msg string, fields ...any) {
	l.inner.DebugContext(context.Background(), msg, fields...)
}

func (l *plannerBufferLogger) Info(msg string, fields ...any) {
	l.inner.InfoContext(context.Background(), msg, fields...)
}

func (l *plannerBufferLogger) Warn(msg string, fields ...any) {
	l.inner.WarnContext(context.Background(), msg, fields...)
}

func (l *plannerBufferLogger) Error(msg string, fields ...any) {
	l.inner.ErrorContext(context.Background(), msg, fields...)
}

type stubPlannerClient struct {
	complete func(context.Context, llm.CompletionRequest) (string, error)
}

func (s stubPlannerClient) Complete(ctx context.Context, req llm.CompletionRequest) (string, error) {
	return s.complete(ctx, req)
}

package gitdiff

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestParseChangedFilesReturnsModifiedEntries(t *testing.T) {
	got := ParseChangedFiles("internal/auth/jwt.go,pkg/logger/logger.go")
	want := []FileChange{
		{Path: "internal/auth/jwt.go", Type: ChangeModified},
		{Path: "pkg/logger/logger.go", Type: ChangeModified},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ParseChangedFiles() mismatch (-want +got):\n%s", diff)
	}
}

func TestParseChangedFilesTrimsWhitespaceAndSkipsEmptyValues(t *testing.T) {
	got := ParseChangedFiles(" internal/auth/jwt.go , , pkg/logger/logger.go ,, ")
	want := []FileChange{
		{Path: "internal/auth/jwt.go", Type: ChangeModified},
		{Path: "pkg/logger/logger.go", Type: ChangeModified},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("ParseChangedFiles() mismatch (-want +got):\n%s", diff)
	}
}

func TestParseChangedFilesEmptyInputReturnsEmptySlice(t *testing.T) {
	got := ParseChangedFiles("")
	if len(got) != 0 {
		t.Fatalf("len(ParseChangedFiles()) = %d, want 0", len(got))
	}
}

func TestGetChangedFilesReturnsModifiedFileBetweenTwoCommits(t *testing.T) {
	repoDir := initGitRepo(t)
	writeTrackedFile(t, repoDir, "internal/auth/jwt.go", "package auth\n\nfunc Token() string { return \"a\" }\n")
	commitAll(t, repoDir, "initial commit")

	writeTrackedFile(t, repoDir, "internal/auth/jwt.go", "package auth\n\nfunc Token() string { return \"b\" }\n")
	commitAll(t, repoDir, "modify jwt")

	got, err := GetChangedFiles(repoDir, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("GetChangedFiles() error = %v", err)
	}

	want := []FileChange{{Path: "internal/auth/jwt.go", Type: ChangeModified}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("GetChangedFiles() mismatch (-want +got):\n%s", diff)
	}
}

func TestGetChangedFilesReportsAddedDeletedAndRenamedFiles(t *testing.T) {
	repoDir := initGitRepo(t)
	writeTrackedFile(t, repoDir, "old.go", "package sample\n")
	writeTrackedFile(t, repoDir, "keep.go", "package sample\n")
	commitAll(t, repoDir, "initial commit")

	if err := os.Rename(filepath.Join(repoDir, "old.go"), filepath.Join(repoDir, "new.go")); err != nil {
		t.Fatalf("Rename() error = %v", err)
	}
	if err := os.Remove(filepath.Join(repoDir, "keep.go")); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	writeTrackedFile(t, repoDir, "added.go", "package sample\n\nconst Added = true\n")
	commitAll(t, repoDir, "rename add delete")

	got, err := GetChangedFiles(repoDir, "HEAD~1", "HEAD")
	if err != nil {
		t.Fatalf("GetChangedFiles() error = %v", err)
	}

	want := []FileChange{
		{Path: "added.go", Type: ChangeAdded},
		{Path: "keep.go", Type: ChangeDeleted},
		{Path: "new.go", OldPath: "old.go", Type: ChangeRenamed},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("GetChangedFiles() mismatch (-want +got):\n%s", diff)
	}
}

func TestGetChangedFilesDefaultsToHeadRangeWhenRefsAreEmpty(t *testing.T) {
	repoDir := initGitRepo(t)
	writeTrackedFile(t, repoDir, "internal/auth/jwt.go", "package auth\n\nfunc Token() string { return \"a\" }\n")
	commitAll(t, repoDir, "initial commit")

	writeTrackedFile(t, repoDir, "internal/auth/jwt.go", "package auth\n\nfunc Token() string { return \"b\" }\n")
	commitAll(t, repoDir, "modify jwt")

	got, err := GetChangedFiles(repoDir, "", "")
	if err != nil {
		t.Fatalf("GetChangedFiles() error = %v", err)
	}

	want := []FileChange{{Path: "internal/auth/jwt.go", Type: ChangeModified}}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("GetChangedFiles() mismatch (-want +got):\n%s", diff)
	}
}

func TestGetChangedFilesReturnsAllFilesForInitialCommit(t *testing.T) {
	repoDir := initGitRepo(t)
	writeTrackedFile(t, repoDir, "internal/auth/jwt.go", "package auth\n")
	writeTrackedFile(t, repoDir, "pkg/logger/logger.go", "package logger\n")
	commitAll(t, repoDir, "initial commit")

	got, err := GetChangedFiles(repoDir, "", "HEAD")
	if err != nil {
		t.Fatalf("GetChangedFiles() error = %v", err)
	}

	want := []FileChange{
		{Path: "internal/auth/jwt.go", Type: ChangeAdded},
		{Path: "pkg/logger/logger.go", Type: ChangeAdded},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("GetChangedFiles() mismatch (-want +got):\n%s", diff)
	}
}

func initGitRepo(t *testing.T) string {
	t.Helper()

	repoDir := t.TempDir()
	runGit(t, repoDir, "init")
	runGit(t, repoDir, "config", "user.name", "Wikismit Test")
	runGit(t, repoDir, "config", "user.email", "wikismit@example.com")
	return repoDir
}

func writeTrackedFile(t *testing.T, repoDir string, relPath string, content string) {
	t.Helper()

	path := filepath.Join(repoDir, relPath)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func commitAll(t *testing.T, repoDir string, message string) {
	t.Helper()
	runGit(t, repoDir, "add", "-A")
	runGit(t, repoDir, "commit", "-m", message)
}

func runGit(t *testing.T, repoDir string, args ...string) {
	t.Helper()

	cmd := exec.Command("git", args...)
	cmd.Dir = repoDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, string(output))
	}
}

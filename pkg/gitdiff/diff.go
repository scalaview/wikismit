package gitdiff

import (
	"context"
	"errors"
	"io"
	"sort"
	"strings"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/utils/merkletrie"
)

type ChangeType string

const (
	ChangeModified ChangeType = "modified"
	ChangeAdded    ChangeType = "added"
	ChangeDeleted  ChangeType = "deleted"
	ChangeRenamed  ChangeType = "renamed"
)

type FileChange struct {
	Path    string
	OldPath string
	Type    ChangeType
}

func ParseChangedFiles(input string) []FileChange {
	if strings.TrimSpace(input) == "" {
		return []FileChange{}
	}

	parts := strings.Split(input, ",")
	changes := make([]FileChange, 0, len(parts))
	for _, part := range parts {
		path := strings.TrimSpace(part)
		if path == "" {
			continue
		}
		changes = append(changes, FileChange{Path: path, Type: ChangeModified})
	}

	return changes
}

func GetChangedFiles(repoPath string, baseRef string, headRef string) ([]FileChange, error) {
	if strings.TrimSpace(headRef) == "" {
		headRef = "HEAD"
	}
	if strings.TrimSpace(baseRef) == "" {
		baseRef = "HEAD~1"
	}

	repo, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	headTree, err := resolveTree(repo, headRef)
	if err != nil {
		return nil, err
	}

	baseTree, err := resolveBaseTree(repo, baseRef)
	if err != nil {
		return nil, err
	}

	changes, err := object.DiffTreeWithOptions(context.Background(), baseTree, headTree, object.DefaultDiffTreeOptions)
	if err != nil {
		return nil, err
	}

	result := make([]FileChange, 0, len(changes))
	for _, change := range changes {
		action, err := change.Action()
		if err != nil {
			return nil, err
		}

		switch action {
		case merkletrie.Insert:
			result = append(result, FileChange{Path: change.To.Name, Type: ChangeAdded})
		case merkletrie.Delete:
			result = append(result, FileChange{Path: change.From.Name, Type: ChangeDeleted})
		case merkletrie.Modify:
			if change.From.Name != change.To.Name {
				result = append(result, FileChange{Path: change.To.Name, OldPath: change.From.Name, Type: ChangeRenamed})
				continue
			}
			result = append(result, FileChange{Path: change.To.Name, Type: ChangeModified})
		}
	}

	sort.Slice(result, func(i int, j int) bool {
		if result[i].Path != result[j].Path {
			return result[i].Path < result[j].Path
		}
		if result[i].OldPath != result[j].OldPath {
			return result[i].OldPath < result[j].OldPath
		}
		return result[i].Type < result[j].Type
	})

	return result, nil
}

func resolveTree(repo *git.Repository, ref string) (*object.Tree, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		return nil, err
	}

	commit, err := repo.CommitObject(*hash)
	if err != nil {
		return nil, err
	}

	return commit.Tree()
}

func resolveBaseTree(repo *git.Repository, ref string) (*object.Tree, error) {
	hash, err := repo.ResolveRevision(plumbing.Revision(ref))
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) || errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, err
	}

	commit, err := repo.CommitObject(*hash)
	if err != nil {
		return nil, err
	}

	return commit.Tree()
}

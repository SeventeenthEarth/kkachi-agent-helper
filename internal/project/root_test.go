package project

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestDiscoverRootFindsGitAncestor(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	nested := filepath.Join(repo, "a", "b")
	mustMkdir(t, nested)

	root, err := DiscoverRoot(nested)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	want := canonicalPath(t, repo)
	if root.Path != want {
		t.Fatalf("root.Path = %q, want %q", root.Path, want)
	}
}

func TestDiscoverRootFindsKkachiAncestor(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".kkachi"))
	nested := filepath.Join(repo, "subdir")
	mustMkdir(t, nested)

	root, err := DiscoverRoot(nested)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	want := canonicalPath(t, repo)
	if root.Path != want {
		t.Fatalf("root.Path = %q, want %q", root.Path, want)
	}
}

func TestDiscoverRootFailsClosedWithoutMarker(t *testing.T) {
	_, err := DiscoverRoot(t.TempDir())
	if err == nil {
		t.Fatal("DiscoverRoot() error = nil, want error")
	}

	var problem *Problem
	if !errors.As(err, &problem) {
		t.Fatalf("error = %T, want *Problem", err)
	}
	if problem.Code != "repo_root_not_found" {
		t.Fatalf("problem.Code = %q, want repo_root_not_found", problem.Code)
	}
	if problem.Hint == "" {
		t.Fatal("problem.Hint is empty")
	}
}

func TestResolveRelativePathRejectsAbsolutePath(t *testing.T) {
	repo := t.TempDir()

	_, err := ResolveRelativePath(Root{Path: repo}, filepath.Join(repo, ".kkachi", "status.json"))
	assertProblemCode(t, err, "absolute_path")
}

func TestResolveRelativePathRejectsParentEscape(t *testing.T) {
	repo := t.TempDir()

	_, err := ResolveRelativePath(Root{Path: repo}, "../status.json")
	assertProblemCode(t, err, "path_escape")
}

func TestResolveRelativePathRejectsRepositoryRootAlias(t *testing.T) {
	repo := t.TempDir()

	_, err := ResolveRelativePath(Root{Path: repo}, ".")
	assertProblemCode(t, err, "repo_root_path")
}

func TestResolveRelativePathRequiresRoot(t *testing.T) {
	_, err := ResolveRelativePath(Root{}, ".kkachi/status.json")
	assertProblemCode(t, err, "repo_root_required")
}

func TestResolveRelativePathNormalizesSafePath(t *testing.T) {
	repo := t.TempDir()

	path, err := ResolveRelativePath(Root{Path: repo}, ".kkachi/../.kkachi/status.json")
	if err != nil {
		t.Fatalf("ResolveRelativePath() error = %v", err)
	}
	if path.Relative != ".kkachi/status.json" {
		t.Fatalf("Relative = %q, want .kkachi/status.json", path.Relative)
	}
	if path.Absolute != filepath.Join(canonicalPath(t, repo), ".kkachi", "status.json") {
		t.Fatalf("Absolute = %q, want repo-confined path", path.Absolute)
	}
}

func TestResolveRelativePathRejectsEscapingSymlink(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()

	link := filepath.Join(repo, ".kkachi")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	_, err := ResolveRelativePath(Root{Path: repo}, ".kkachi/status.json")
	assertProblemCode(t, err, "symlink_escape")
}

func TestResolveRelativePathAllowsInternalSymlink(t *testing.T) {
	repo := t.TempDir()
	realDir := filepath.Join(repo, "real")
	mustMkdir(t, realDir)

	if err := os.Symlink(realDir, filepath.Join(repo, ".kkachi")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	path, err := ResolveRelativePath(Root{Path: repo}, ".kkachi/status.json")
	if err != nil {
		t.Fatalf("ResolveRelativePath() error = %v", err)
	}
	if path.Relative != ".kkachi/status.json" {
		t.Fatalf("Relative = %q, want .kkachi/status.json", path.Relative)
	}
}

func assertProblemCode(t *testing.T, err error, want string) {
	t.Helper()

	if err == nil {
		t.Fatalf("error = nil, want %s", want)
	}
	var problem *Problem
	if !errors.As(err, &problem) {
		t.Fatalf("error = %T, want *Problem", err)
	}
	if problem.Code != want {
		t.Fatalf("problem.Code = %q, want %q", problem.Code, want)
	}
	if problem.Hint == "" {
		t.Fatal("problem.Hint is empty")
	}
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("eval symlinks %s: %v", path, err)
	}
	return resolved
}

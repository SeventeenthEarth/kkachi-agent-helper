package project

import (
	"os"
	"path/filepath"
)

const (
	gitMarker    = ".git"
	kkachiMarker = ".kkachi"
)

// Root is the canonical repository root discovered for helper state.
type Root struct {
	Path string
}

// DiscoverRoot walks upward from start and returns the first directory that
// looks like a project root. Existing symlinks in start are resolved first so
// later path checks share one canonical root.
func DiscoverRoot(start string) (Root, error) {
	if start == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Root{}, problem("working_directory_unavailable", "cannot read current working directory", "Run the command from a readable repository directory.")
		}
		start = wd
	}

	absoluteStart, err := filepath.Abs(start)
	if err != nil {
		return Root{}, (&Problem{
			Code:     "invalid_start_path",
			Message:  "cannot resolve start path",
			Hint:     "Use an existing repository directory.",
			Path:     start,
			Field:    "start",
			Expected: "existing directory",
			Actual:   err.Error(),
		})
	}

	resolvedStart, err := filepath.EvalSymlinks(absoluteStart)
	if err != nil {
		return Root{}, (&Problem{
			Code:     "invalid_start_path",
			Message:  "cannot resolve start path",
			Hint:     "Use an existing repository directory and avoid broken symlinks.",
			Path:     absoluteStart,
			Field:    "start",
			Expected: "existing directory",
			Actual:   err.Error(),
		})
	}

	info, err := os.Stat(resolvedStart)
	if err != nil {
		return Root{}, (&Problem{
			Code:     "invalid_start_path",
			Message:  "cannot inspect start path",
			Hint:     "Use an existing repository directory.",
			Path:     resolvedStart,
			Field:    "start",
			Expected: "directory",
			Actual:   err.Error(),
		})
	}
	if !info.IsDir() {
		resolvedStart = filepath.Dir(resolvedStart)
	}

	for current := filepath.Clean(resolvedStart); ; current = filepath.Dir(current) {
		if hasRootMarker(current) {
			return Root{Path: current}, nil
		}
		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}

	return Root{}, (&Problem{
		Code:     "repo_root_not_found",
		Message:  "repository root was not found",
		Hint:     "Run the command from a Git repository or an initialized .kkachi project.",
		Path:     resolvedStart,
		Field:    "repository_root",
		Expected: ".git or .kkachi marker in this directory or an ancestor",
		Actual:   "no marker found",
	})
}

func hasRootMarker(dir string) bool {
	for _, marker := range []string{gitMarker, kkachiMarker} {
		_, err := os.Lstat(filepath.Join(dir, marker))
		if err == nil {
			return true
		}
	}
	return false
}

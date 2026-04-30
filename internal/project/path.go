package project

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

// SafePath is a repository-confined, normalized relative path and its absolute
// location under the canonical root.
type SafePath struct {
	Relative string
	Absolute string
}

// ResolveRelativePath rejects absolute paths, parent escapes, root aliases, and
// symlinks that would move the resolved path outside the repository root.
func ResolveRelativePath(root Root, value string) (SafePath, error) {
	relative, err := normalizeRelativePath(value)
	if err != nil {
		return SafePath{}, err
	}

	if strings.TrimSpace(root.Path) == "" {
		return SafePath{}, problem("repo_root_required", "repository root is required", "Discover the repository root before resolving project paths.")
	}
	rootPath := filepath.Clean(root.Path)
	if resolvedRoot, err := filepath.EvalSymlinks(rootPath); err == nil {
		rootPath = resolvedRoot
	}

	absolute := filepath.Join(rootPath, relative)
	if !isWithinRoot(rootPath, absolute) {
		return SafePath{}, (&Problem{
			Code:     "path_escape",
			Message:  "path must stay within the repository root",
			Hint:     "Use a repository-relative path without parent-directory traversal.",
			Path:     value,
			Field:    "path",
			Expected: "repository-confined relative path",
			Actual:   value,
		})
	}

	if err := rejectEscapingSymlinks(rootPath, relative); err != nil {
		return SafePath{}, err
	}

	return SafePath{
		Relative: filepath.ToSlash(relative),
		Absolute: absolute,
	}, nil
}

func normalizeRelativePath(value string) (string, error) {
	original := value
	value = strings.TrimSpace(value)
	if value == "" {
		return "", (&Problem{
			Code:     "empty_path",
			Message:  "path must not be empty",
			Hint:     "Use a repository-relative path such as .kkachi/status.json.",
			Path:     original,
			Field:    "path",
			Expected: "non-empty relative path",
			Actual:   "empty",
		})
	}

	path := filepath.Clean(filepath.FromSlash(value))
	if filepath.IsAbs(path) {
		return "", (&Problem{
			Code:     "absolute_path",
			Message:  "absolute paths are not allowed",
			Hint:     "Use a path relative to the repository root.",
			Path:     original,
			Field:    "path",
			Expected: "repository-relative path",
			Actual:   original,
		})
	}
	if path == "." {
		return "", (&Problem{
			Code:     "repo_root_path",
			Message:  "path must not resolve to the repository root",
			Hint:     "Use a child path under .kkachi/ for helper-managed state.",
			Path:     original,
			Field:    "path",
			Expected: "repository child path",
			Actual:   path,
		})
	}
	if path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", (&Problem{
			Code:     "path_escape",
			Message:  "path must stay within the repository root",
			Hint:     "Remove parent-directory traversal from the path.",
			Path:     original,
			Field:    "path",
			Expected: "repository-confined relative path",
			Actual:   original,
		})
	}

	return path, nil
}

func rejectEscapingSymlinks(rootPath string, relative string) error {
	current := rootPath
	for _, part := range splitPath(relative) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return (&Problem{
				Code:     "path_inspection_failed",
				Message:  "cannot inspect path component",
				Hint:     "Check file permissions and remove unreadable path components.",
				Path:     current,
				Field:    "path",
				Expected: "readable path component",
				Actual:   err.Error(),
			})
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}

		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return (&Problem{
				Code:     "symlink_resolution_failed",
				Message:  "cannot resolve symlink path",
				Hint:     "Remove broken symlinks from helper-managed paths.",
				Path:     current,
				Field:    "path",
				Expected: "resolvable symlink within repository root",
				Actual:   err.Error(),
			})
		}
		if !isWithinRoot(rootPath, resolved) {
			return (&Problem{
				Code:     "symlink_escape",
				Message:  "symlink path escapes the repository root",
				Hint:     "Replace the symlink with a real directory or a symlink whose target stays inside the repository.",
				Path:     current,
				Field:    "path",
				Expected: "symlink target within repository root",
				Actual:   resolved,
			})
		}
	}
	return nil
}

func splitPath(path string) []string {
	parts := strings.Split(path, string(filepath.Separator))
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func isWithinRoot(rootPath string, path string) bool {
	relative, err := filepath.Rel(rootPath, path)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

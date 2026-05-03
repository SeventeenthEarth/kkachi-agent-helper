package project

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

const (
	ConfigPath = ".kkachi/config.yaml"
	StatusPath = ".kkachi/status.json"
	EventsPath = ".kkachi/events.jsonl"

	initialEventID = "evt-000001"
)

var statePaths = []string{
	ConfigPath,
	StatusPath,
	EventsPath,
}

var schemaPaths = []string{
	".kkachi/schemas/config.schema.json",
	".kkachi/schemas/status.schema.json",
	".kkachi/schemas/event.schema.json",
	".kkachi/schemas/run-metadata.schema.json",
	".kkachi/schemas/selected-cli.schema.json",
	".kkachi/schemas/bridge-session-snapshot.schema.json",
	".kkachi/schemas/install-manifest.schema.json",
}

// InitOptions controls project initialization. Tests may inject deterministic
// time and entropy while production uses the helper clock and crypto randomness.
type InitOptions struct {
	Now       func() time.Time
	RandomHex func(int) (string, error)
}

// InitResult summarizes the files created by project initialization.
type InitResult struct {
	RootPath       string
	ProjectID      string
	ProjectName    string
	InitialEventID string
	CreatedPaths   []string
	SchemaPaths    []string
}

// InitProject creates the initial .kkachi project state without overwriting any
// helper-managed files.
func InitProject(root Root, options InitOptions) (InitResult, error) {
	if strings.TrimSpace(root.Path) == "" {
		return InitResult{}, problem("repo_root_required", "repository root is required", "Discover the repository root before initializing project state.")
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if options.RandomHex == nil {
		options.RandomHex = randomHex
	}

	paths, err := resolveInitPaths(root)
	if err != nil {
		return InitResult{}, err
	}
	if err := rejectExistingManagedFiles(paths); err != nil {
		return InitResult{}, err
	}

	projectName := slugify(filepath.Base(root.Path))
	if projectName == "" {
		projectName = "project"
	}
	suffix, err := options.RandomHex(6)
	if err != nil {
		return InitResult{}, (&Problem{
			Code:     "project_id_generation_failed",
			Message:  "cannot generate project identity",
			Hint:     "Retry initialization and preserve stderr if the problem repeats.",
			Field:    "project_id",
			Expected: "crypto-random identifier suffix",
			Actual:   err.Error(),
		})
	}
	projectID := fmt.Sprintf("kkachi-project-%s-%s", projectName, suffix)
	occurredAt := options.Now().UTC().Format(time.RFC3339)

	files := initFiles(projectName, projectID, occurredAt)
	for _, path := range paths {
		content, ok := files[path.Relative]
		if !ok {
			return InitResult{}, (&Problem{
				Code:     "init_file_missing",
				Message:  "internal initialization content is missing",
				Hint:     "Rerun with the same arguments and preserve stderr for diagnosis.",
				Path:     path.Relative,
				Field:    "path",
				Expected: "managed initialization content",
				Actual:   "missing",
			})
		}
		if err := writeNewFile(path, content); err != nil {
			return InitResult{}, err
		}
	}

	return InitResult{
		RootPath:       root.Path,
		ProjectID:      projectID,
		ProjectName:    projectName,
		InitialEventID: initialEventID,
		CreatedPaths:   append([]string(nil), statePaths...),
		SchemaPaths:    append([]string(nil), schemaPaths...),
	}, nil
}

func resolveInitPaths(root Root) ([]SafePath, error) {
	relativePaths := append(append([]string(nil), statePaths...), schemaPaths...)
	paths := make([]SafePath, 0, len(relativePaths))
	for _, relative := range relativePaths {
		path, err := ResolveRelativePath(root, relative)
		if err != nil {
			return nil, err
		}
		paths = append(paths, path)
	}
	return paths, nil
}

func rejectExistingManagedFiles(paths []SafePath) error {
	for _, path := range paths {
		info, err := os.Lstat(path.Absolute)
		if err == nil {
			return helperStateExistsProblem(path.Relative, info)
		}
		if !errors.Is(err, os.ErrNotExist) {
			return &Problem{
				Code:     "path_inspection_failed",
				Message:  "cannot inspect managed state path",
				Hint:     "Check file permissions and remove unreadable helper-managed paths.",
				Path:     path.Relative,
				Field:    "path",
				Expected: "readable path state",
				Actual:   err.Error(),
			}
		}
	}
	return nil
}

func writeNewFile(path SafePath, content []byte) error {
	return writeNewFileAtomically(path, content)
}

func helperStateExistsProblem(relative string, info os.FileInfo) *Problem {
	actual := "file"
	if info != nil && info.IsDir() {
		actual = "directory"
	}
	return &Problem{
		Code:     "helper_state_exists",
		Message:  "helper-managed state already exists",
		Hint:     "Use a fresh repository or a future migration/reset command; project init never overwrites helper state.",
		Path:     relative,
		Field:    "path",
		Expected: "path does not exist",
		Actual:   actual,
	}
}

func initFiles(projectName string, projectID string, occurredAt string) map[string][]byte {
	status := map[string]any{
		"version":          "0.1",
		"project_id":       projectID,
		"active_run_id":    nil,
		"active_run_state": nil,
		"last_event_id":    initialEventID,
		"updated_at":       occurredAt,
		"gate_summary":     map[string]any{},
	}
	event := map[string]any{
		"version":     "0.1",
		"event_id":    initialEventID,
		"occurred_at": occurredAt,
		"run_id":      nil,
		"type":        "project.initialized",
		"actor":       "helper",
		"payload": map[string]any{
			"project_id":   projectID,
			"project_name": projectName,
		},
	}

	files := map[string][]byte{
		ConfigPath: []byte(configYAML(projectName)),
		StatusPath: append(mustJSON(status), '\n'),
		EventsPath: append(mustCompactJSON(event), '\n'),
	}
	for _, path := range schemaPaths {
		name, err := ResolveSchemaName(path)
		if err != nil {
			panic(err)
		}
		content, err := SchemaDocument(name)
		if err != nil {
			panic(err)
		}
		files[path] = content
	}
	return files
}

func configYAML(projectName string) string {
	return fmt.Sprintf(`version: "0.1"
project:
  name: "%s"
  root_policy: "repository_confined_no_symlink_escape"
paths:
  run_root: ".kkachi/runs"
  status_file: ".kkachi/status.json"
  events_file: ".kkachi/events.jsonl"
locks:
  one_active_write_run: true
schemas:
  mode: "both"
compat:
  required_skills: null
  required_bridge: null
`, projectName)
}

func mustJSON(value any) []byte {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		panic(err)
	}
	return data
}

func mustCompactJSON(value any) []byte {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return data
}

func randomHex(bytes int) (string, error) {
	data := make([]byte, bytes)
	if _, err := rand.Read(data); err != nil {
		return "", err
	}
	return hex.EncodeToString(data), nil
}

func slugify(value string) string {
	var builder strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			builder.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash && builder.Len() > 0 {
			builder.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

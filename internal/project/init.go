package project

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
	"unicode"
)

const (
	ConfigPath         = ".kkachi/config.yaml"
	StatusPath         = ".kkachi/status.json"
	EventsPath         = ".kkachi/events.jsonl"
	ProjectOverlayPath = ".kkachi/project-overlay.yaml"
	DocsMapPath        = "docs/kkachi-docs-map.yaml"

	initialEventID = "evt-000001"
)

var statePaths = []string{
	ConfigPath,
	StatusPath,
	EventsPath,
	ProjectOverlayPath,
	DocsMapPath,
}

var schemaPaths = []string{
	".kkachi/schemas/config.schema.json",
	".kkachi/schemas/status.schema.json",
	".kkachi/schemas/event.schema.json",
	".kkachi/schemas/run-metadata.schema.json",
	".kkachi/schemas/selected-cli.schema.json",
	".kkachi/schemas/bridge-session-snapshot.schema.json",
	".kkachi/schemas/token-economy-evidence.schema.json",
	".kkachi/schemas/multi-agent-review-evidence.schema.json",
	".kkachi/schemas/policy-promotion-evidence.schema.json",
	".kkachi/schemas/design-evidence.schema.json",
}

// InitOptions controls project initialization. Tests may inject deterministic
// time and entropy while production uses the helper clock and crypto randomness.
type InitOptions struct {
	Now       func() time.Time
	RandomHex func(int) (string, error)
	Force     bool
	Bootstrap InitBootstrapOptions
}

// InitBootstrapOptions captures the project bootstrap contract normally supplied
// by KHS/Hermes callers through project init flags.
type InitBootstrapOptions struct {
	ProjectName         string
	Stack               string
	RepoPath            string
	Commander           string
	Redteam             string
	DocsMapRoadmap      string
	DocsMapSpec         string
	DocsMapArchitecture string
	DocsMapADRDir       string
	DocsMapTODODir      string
	DocsMapSpecDir      string
	TestCommands        []string
	BackendPolicy       string
	ExecutionMode       string
	SOTPolicy           string
}

// InitResult summarizes the files created by project initialization.
type InitResult struct {
	RootPath            string
	ProjectID           string
	ProjectName         string
	InitialEventID      string
	ReconfiguredEventID string
	Forced              bool
	CreatedPaths        []string
	SchemaPaths         []string
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
	bootstrap, err := normalizeInitBootstrap(options.Bootstrap, root.Path)
	if err != nil {
		return InitResult{}, err
	}
	if options.Force {
		return reconfigureProject(root, options, bootstrap)
	}

	paths, err := resolveInitPaths(root)
	if err != nil {
		return InitResult{}, err
	}
	if err := rejectExistingManagedFiles(paths); err != nil {
		return InitResult{}, err
	}

	projectID, err := newProjectID(bootstrap.ProjectName, options.RandomHex)
	if err != nil {
		return InitResult{}, err
	}
	occurredAt := options.Now().UTC().Format(time.RFC3339)

	files := initFiles(bootstrap, projectID, occurredAt)
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
		ProjectName:    bootstrap.ProjectName,
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

func initFiles(bootstrap InitBootstrapOptions, projectID string, occurredAt string) map[string][]byte {
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
		"payload":     bootstrapEventPayload(bootstrap, projectID),
	}

	files := map[string][]byte{
		ConfigPath:         []byte(configYAML(bootstrap)),
		StatusPath:         append(mustJSON(status), '\n'),
		EventsPath:         append(mustCompactJSON(event), '\n'),
		ProjectOverlayPath: []byte(projectOverlayYAML(bootstrap)),
		DocsMapPath:        []byte(docsMapYAML(bootstrap)),
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

func configYAML(bootstrap InitBootstrapOptions) string {
	return fmt.Sprintf(`version: "0.1"
project:
  name: "%s"
  root_policy: "repository_confined_no_symlink_escape"
paths:
  run_root: ".kkachi/runs"
  status_file: ".kkachi/status.json"
  events_file: ".kkachi/events.jsonl"
  project_overlay_file: ".kkachi/project-overlay.yaml"
  docs_map_file: "docs/kkachi-docs-map.yaml"
locks:
  one_active_write_run: true
schemas:
  mode: "both"
compat:
  required_skills: null
  required_bridge: null
`, yamlQuote(bootstrap.ProjectName))
}

func projectOverlayYAML(bootstrap InitBootstrapOptions) string {
	return fmt.Sprintf(`# kkachi-agent-helper:managed
project: "%s"
stack: "%s"
repo_path: "%s"
commander: "%s"
redteam: "%s"
test_commands:%s
backend_policy: "%s"
execution_mode: "%s"
sot_policy: "%s"
`, yamlQuote(bootstrap.ProjectName), yamlQuote(bootstrap.Stack), yamlQuote(bootstrap.RepoPath), yamlQuote(bootstrap.Commander), yamlQuote(bootstrap.Redteam), yamlStringList(bootstrap.TestCommands), yamlQuote(bootstrap.BackendPolicy), yamlQuote(bootstrap.ExecutionMode), yamlQuote(bootstrap.SOTPolicy))
}

func docsMapYAML(bootstrap InitBootstrapOptions) string {
	sotDocs := []string{bootstrap.DocsMapSpec, bootstrap.DocsMapArchitecture}
	return fmt.Sprintf(`# kkachi-agent-helper:managed
project: "%s"
roadmap: "%s"
sot_docs:%s
adr_dir: "%s"
todo_dir: "%s"
spec_dir: "%s"
test_commands:%s
`, yamlQuote(bootstrap.ProjectName), yamlQuote(bootstrap.DocsMapRoadmap), yamlStringList(sotDocs), yamlQuote(bootstrap.DocsMapADRDir), yamlQuote(bootstrap.DocsMapTODODir), yamlQuote(bootstrap.DocsMapSpecDir), yamlStringList(bootstrap.TestCommands))
}

func yamlStringList(values []string) string {
	if len(values) == 0 {
		return " []"
	}
	var builder strings.Builder
	for _, value := range values {
		builder.WriteString("\n  - \"")
		builder.WriteString(yamlQuote(value))
		builder.WriteString("\"")
	}
	return builder.String()
}

func yamlQuote(value string) string {
	value = strings.ReplaceAll(value, `\\`, `\\\\`)
	value = strings.ReplaceAll(value, `"`, `\\"`)
	return value
}

func normalizeInitBootstrap(options InitBootstrapOptions, rootPath string) (InitBootstrapOptions, error) {
	options.ProjectName = strings.TrimSpace(options.ProjectName)
	options.Stack = strings.TrimSpace(options.Stack)
	options.RepoPath = strings.TrimSpace(options.RepoPath)
	options.Commander = strings.TrimSpace(options.Commander)
	options.Redteam = strings.TrimSpace(options.Redteam)
	options.DocsMapRoadmap = strings.TrimSpace(options.DocsMapRoadmap)
	options.DocsMapSpec = strings.TrimSpace(options.DocsMapSpec)
	options.DocsMapArchitecture = strings.TrimSpace(options.DocsMapArchitecture)
	options.DocsMapADRDir = strings.TrimSpace(options.DocsMapADRDir)
	options.DocsMapTODODir = strings.TrimSpace(options.DocsMapTODODir)
	options.DocsMapSpecDir = strings.TrimSpace(options.DocsMapSpecDir)
	options.BackendPolicy = strings.TrimSpace(options.BackendPolicy)
	options.ExecutionMode = strings.TrimSpace(options.ExecutionMode)
	options.SOTPolicy = strings.TrimSpace(options.SOTPolicy)
	trimmedCommands := make([]string, 0, len(options.TestCommands))
	for _, command := range options.TestCommands {
		if trimmed := strings.TrimSpace(command); trimmed != "" {
			trimmedCommands = append(trimmedCommands, trimmed)
		}
	}
	options.TestCommands = trimmedCommands
	if options.RepoPath == "" {
		options.RepoPath = rootPath
	}
	for _, field := range []struct{ name, value string }{
		{"project_name", options.ProjectName},
		{"stack", options.Stack},
		{"repo_path", options.RepoPath},
		{"commander", options.Commander},
		{"redteam", options.Redteam},
		{"docs_map_roadmap", options.DocsMapRoadmap},
		{"docs_map_spec", options.DocsMapSpec},
		{"docs_map_architecture", options.DocsMapArchitecture},
		{"docs_map_adr_dir", options.DocsMapADRDir},
		{"docs_map_todo_dir", options.DocsMapTODODir},
		{"docs_map_spec_dir", options.DocsMapSpecDir},
		{"backend_policy", options.BackendPolicy},
		{"execution_mode", options.ExecutionMode},
		{"sot_policy", options.SOTPolicy},
	} {
		if field.value == "" {
			return InitBootstrapOptions{}, &Problem{Code: "project_init_parameter_required", Message: "project init requires bootstrap parameters", Hint: "Pass all required project init bootstrap options, including project identity, docs-map paths, backend policy, execution mode, and SOT policy.", Field: field.name, Expected: "non-empty value", Actual: "missing"}
		}
	}
	if len(options.TestCommands) == 0 {
		return InitBootstrapOptions{}, &Problem{Code: "project_init_parameter_required", Message: "project init requires bootstrap parameters", Hint: "Pass --test-commands with one or more comma-separated commands.", Field: "test_commands", Expected: "one or more commands", Actual: "missing"}
	}
	return options, nil
}

func newProjectID(projectName string, randomHex func(int) (string, error)) (string, error) {
	suffix, err := randomHex(6)
	if err != nil {
		return "", (&Problem{Code: "project_id_generation_failed", Message: "cannot generate project identity", Hint: "Retry initialization and preserve stderr if the problem repeats.", Field: "project_id", Expected: "crypto-random identifier suffix", Actual: err.Error()})
	}
	return fmt.Sprintf("kkachi-project-%s-%s", slugify(projectName), suffix), nil
}

func reconfigureProject(root Root, options InitOptions, bootstrap InitBootstrapOptions) (InitResult, error) {
	paths := []SafePath{}
	for _, relative := range append([]string{ConfigPath, ProjectOverlayPath, DocsMapPath}, schemaPaths...) {
		path, err := ResolveRelativePath(root, relative)
		if err != nil {
			return InitResult{}, err
		}
		paths = append(paths, path)
	}
	projectID, err := readProjectID(root)
	if err != nil {
		return InitResult{}, err
	}
	files := map[string][]byte{ConfigPath: []byte(configYAML(bootstrap)), ProjectOverlayPath: []byte(projectOverlayYAML(bootstrap)), DocsMapPath: []byte(docsMapYAML(bootstrap))}
	for _, schemaPath := range schemaPaths {
		name, err := ResolveSchemaName(schemaPath)
		if err != nil {
			return InitResult{}, err
		}
		content, err := SchemaDocument(name)
		if err != nil {
			return InitResult{}, err
		}
		files[schemaPath] = content
	}
	var event AppendEventResult
	err = withProjectWriteLock(root, "project init --force", "", func() error {
		for _, path := range paths {
			content, ok := files[path.Relative]
			if !ok {
				return &Problem{Code: "init_file_missing", Message: "internal initialization content is missing", Hint: "Rerun with the same arguments and preserve stderr for diagnosis.", Path: path.Relative, Field: "path", Expected: "managed initialization content", Actual: "missing"}
			}
			if err := writeExistingFileAtomically(path, content); err != nil {
				return err
			}
		}
		var err error
		event, err = appendEventWithStatusMutation(root, AppendEventOptions{Type: "project.reconfigured", Payload: bootstrapEventPayload(bootstrap, projectID), Now: options.Now}, nil)
		return err
	})
	if err != nil {
		return InitResult{}, err
	}
	return InitResult{RootPath: root.Path, ProjectID: projectID, ProjectName: bootstrap.ProjectName, Forced: true, ReconfiguredEventID: event.EventID, CreatedPaths: append([]string(nil), statePaths...), SchemaPaths: append([]string(nil), schemaPaths...)}, nil
}

func readProjectID(root Root) (string, error) {
	path, err := ResolveRelativePath(root, StatusPath)
	if err != nil {
		return "", err
	}
	status, err := readStatus(path)
	if err != nil {
		return "", err
	}
	id, ok := status["project_id"].(string)
	if !ok || strings.TrimSpace(id) == "" {
		return "", &Problem{Code: "status_project_id_invalid", Message: "project status is missing a project id", Hint: "Restore status.json before running project init --force.", Path: path.Relative, Field: "project_id", Expected: "non-empty string", Actual: "missing"}
	}
	return id, nil
}

func bootstrapEventPayload(bootstrap InitBootstrapOptions, projectID string) map[string]any {
	return map[string]any{"project_id": projectID, "project_name": bootstrap.ProjectName, "stack": bootstrap.Stack, "repo_path": bootstrap.RepoPath, "commander": bootstrap.Commander, "redteam": bootstrap.Redteam, "docs_map": map[string]any{"roadmap": bootstrap.DocsMapRoadmap, "spec": bootstrap.DocsMapSpec, "architecture": bootstrap.DocsMapArchitecture, "adr_dir": bootstrap.DocsMapADRDir, "todo_dir": bootstrap.DocsMapTODODir, "spec_dir": bootstrap.DocsMapSpecDir}, "test_commands": bootstrap.TestCommands, "backend_policy": bootstrap.BackendPolicy, "execution_mode": bootstrap.ExecutionMode, "sot_policy": bootstrap.SOTPolicy}
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

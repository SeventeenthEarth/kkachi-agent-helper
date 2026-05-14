package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const DiagnosticsVersion = "0.1"

var diagnosticArtifactPaths = []string{
	"run-metadata.json",
	"phase-plan.yaml",
	"intake-classification.md",
	"selected-cli.json",
	"capability-check.md",
	"bridge-session-snapshot.json",
	"bridge-events.md",
	"test-log.md",
	"verification.md",
	"docs-update.md",
	"final-report.md",
}

type DiagnosticsExportOptions struct {
	RunID  string
	Output string
	Now    func() time.Time
}

type DiagnosticsBundle struct {
	Version           string               `json:"version"`
	GeneratedAt       string               `json:"generated_at"`
	RootPath          string               `json:"root_path"`
	Redaction         DiagnosticsRedaction `json:"redaction"`
	Project           DiagnosticsProject   `json:"project"`
	SchemaVersions    []DiagnosticsSchema  `json:"schema_versions"`
	RunID             string               `json:"run_id,omitempty"`
	GateReports       []DiagnosticsFile    `json:"gate_reports"`
	SelectedArtifacts []DiagnosticsFile    `json:"selected_artifacts"`
	OutputPath        string               `json:"output_path,omitempty"`
}

type DiagnosticsRedaction struct {
	Enabled     bool   `json:"enabled"`
	Placeholder string `json:"placeholder"`
}

type DiagnosticsProject struct {
	Config DiagnosticsFile `json:"config"`
	Status DiagnosticsFile `json:"status"`
	Events DiagnosticsFile `json:"events"`
}

type DiagnosticsSchema struct {
	Schema  string `json:"schema"`
	Path    string `json:"path"`
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
	Error   string `json:"error,omitempty"`
}

type DiagnosticsFile struct {
	Path    string `json:"path"`
	Status  string `json:"status"`
	Bytes   int64  `json:"bytes,omitempty"`
	Content any    `json:"content,omitempty"`
	Error   string `json:"error,omitempty"`
}

func ExportDiagnostics(root Root, options DiagnosticsExportOptions) (DiagnosticsBundle, error) {
	if strings.TrimSpace(root.Path) == "" {
		return DiagnosticsBundle{}, problem("repo_root_required", "repository root is required", "Discover the repository root before exporting diagnostics.")
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}

	runID := ""
	if strings.TrimSpace(options.RunID) != "" {
		resolved, err := ResolveRunID(root, options.RunID)
		if err != nil {
			return DiagnosticsBundle{}, err
		}
		runID = resolved
	} else if active := activeRunIDForDiagnostics(root); active != "" {
		runID = active
	}

	bundle := DiagnosticsBundle{
		Version:     DiagnosticsVersion,
		GeneratedAt: options.Now().UTC().Format(time.RFC3339),
		RootPath:    root.Path,
		Redaction:   DiagnosticsRedaction{Enabled: true, Placeholder: RedactedPlaceholder},
		RunID:       runID,
	}
	bundle.Project = DiagnosticsProject{
		Config: diagnosticTextFile(root, ConfigPath, true),
		Status: diagnosticJSONFile(root, StatusPath),
		Events: diagnosticEventsFile(root),
	}
	bundle.SchemaVersions = diagnosticSchemaVersions(root)
	if runID != "" {
		bundle.GateReports = diagnosticGateReports(root, runID)
		bundle.SelectedArtifacts = diagnosticSelectedArtifacts(root, runID)
	} else {
		bundle.GateReports = []DiagnosticsFile{}
		bundle.SelectedArtifacts = []DiagnosticsFile{}
	}

	if strings.TrimSpace(options.Output) != "" {
		path, err := ResolveRelativePath(root, options.Output)
		if err != nil {
			return DiagnosticsBundle{}, err
		}
		if _, err := os.Lstat(path.Absolute); err == nil {
			return DiagnosticsBundle{}, &Problem{Code: "diagnostics_output_exists", Message: "diagnostics output already exists", Hint: "Choose a new repository-relative output path so an older support bundle is not overwritten.", Path: path.Relative, Field: "output", Expected: "absent file path", Actual: "exists"}
		} else if !os.IsNotExist(err) {
			return DiagnosticsBundle{}, &Problem{Code: "path_inspection_failed", Message: "cannot inspect diagnostics output path", Hint: "Check output path permissions before exporting diagnostics.", Path: path.Relative, Field: "output", Expected: "inspectable output path", Actual: err.Error()}
		}
		data, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return DiagnosticsBundle{}, &Problem{Code: "diagnostics_encode_failed", Message: "cannot encode diagnostics bundle", Hint: "Retry diagnostics export and preserve stderr if the problem repeats.", Field: "diagnostics", Expected: "JSON object", Actual: err.Error()}
		}
		data = append(data, '\n')
		if err := writeNewFileAtomically(path, data); err != nil {
			return DiagnosticsBundle{}, err
		}
		bundle.OutputPath = path.Relative
	}
	return bundle, nil
}

func activeRunIDForDiagnostics(root Root) string {
	path, err := ResolveRelativePath(root, StatusPath)
	if err != nil {
		return ""
	}
	status, err := readStatus(path)
	if err != nil {
		return ""
	}
	value, _ := optionalString(status, "active_run_id")
	if value == nil {
		return ""
	}
	return *value
}

func diagnosticSchemaVersions(root Root) []DiagnosticsSchema {
	results := make([]DiagnosticsSchema, 0, len(canonicalSchemaNames))
	for _, name := range canonicalSchemaNames {
		relative, err := SchemaPathForName(name)
		if err != nil {
			results = append(results, DiagnosticsSchema{Schema: name, Status: "invalid", Error: RedactString(err.Error())})
			continue
		}
		path, err := ResolveRelativePath(root, relative)
		if err != nil {
			results = append(results, DiagnosticsSchema{Schema: name, Path: relative, Status: "invalid", Error: RedactString(err.Error())})
			continue
		}
		data, err := os.ReadFile(path.Absolute)
		if os.IsNotExist(err) {
			results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "missing", Error: "schema file is missing"})
			continue
		}
		if err != nil {
			results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "unreadable", Error: RedactString(err.Error())})
			continue
		}
		var payload map[string]any
		if err := json.Unmarshal(data, &payload); err != nil {
			results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "invalid", Error: RedactString(err.Error())})
			continue
		}
		version, _ := payload["version"].(string)
		if strings.TrimSpace(version) == "" {
			results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "invalid", Error: "schema version is missing"})
			continue
		}
		// Schema files are project-local input; redact defensively even though
		// canonical schema versions are ordinary short semver-like strings.
		results = append(results, DiagnosticsSchema{Schema: name, Path: path.Relative, Status: "present", Version: RedactString(version)})
	}
	return results
}

func diagnosticGateReports(root Root, runID string) []DiagnosticsFile {
	dir, err := ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "gate-reports")))
	if err != nil {
		return []DiagnosticsFile{{Path: filepath.ToSlash(filepath.Join(RunRootPath, runID, "gate-reports")), Status: "invalid", Error: RedactString(err.Error())}}
	}
	entries, err := os.ReadDir(dir.Absolute)
	if os.IsNotExist(err) {
		return []DiagnosticsFile{}
	}
	if err != nil {
		return []DiagnosticsFile{{Path: dir.Relative, Status: "unreadable", Error: RedactString(err.Error())}}
	}
	names := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	files := make([]DiagnosticsFile, 0, len(names))
	for _, name := range names {
		files = append(files, diagnosticJSONFile(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "gate-reports", name))))
	}
	return files
}

func diagnosticSelectedArtifacts(root Root, runID string) []DiagnosticsFile {
	files := make([]DiagnosticsFile, 0, len(diagnosticArtifactPaths))
	for _, artifact := range diagnosticArtifactPaths {
		relative := filepath.ToSlash(filepath.Join(RunRootPath, runID, artifact))
		if strings.HasSuffix(artifact, ".json") {
			files = append(files, diagnosticJSONFile(root, relative))
			continue
		}
		files = append(files, diagnosticTextFile(root, relative, true))
	}
	return files
}

func diagnosticTextFile(root Root, relative string, redact bool) DiagnosticsFile {
	path, err := ResolveRelativePath(root, relative)
	if err != nil {
		return DiagnosticsFile{Path: relative, Status: "invalid", Error: RedactString(err.Error())}
	}
	data, info, err := readRegularDiagnosticFile(path)
	if err != nil {
		return diagnosticFileError(path.Relative, err)
	}
	content := string(data)
	if redact {
		content = RedactString(content)
	}
	return DiagnosticsFile{Path: path.Relative, Status: "present", Bytes: info.Size(), Content: content}
}

func diagnosticJSONFile(root Root, relative string) DiagnosticsFile {
	path, err := ResolveRelativePath(root, relative)
	if err != nil {
		return DiagnosticsFile{Path: relative, Status: "invalid", Error: RedactString(err.Error())}
	}
	data, info, err := readRegularDiagnosticFile(path)
	if err != nil {
		return diagnosticFileError(path.Relative, err)
	}
	var payload any
	if err := json.Unmarshal(data, &payload); err != nil {
		return DiagnosticsFile{Path: path.Relative, Status: "invalid", Bytes: info.Size(), Error: RedactString(err.Error())}
	}
	return DiagnosticsFile{Path: path.Relative, Status: "present", Bytes: info.Size(), Content: RedactValue(payload)}
}

func diagnosticEventsFile(root Root) DiagnosticsFile {
	path, err := ResolveRelativePath(root, EventsPath)
	if err != nil {
		return DiagnosticsFile{Path: EventsPath, Status: "invalid", Error: RedactString(err.Error())}
	}
	data, info, err := readRegularDiagnosticFile(path)
	if err != nil {
		return diagnosticFileError(path.Relative, err)
	}
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), MaxEventLineBytes)
	var events []any
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			return DiagnosticsFile{Path: path.Relative, Status: "invalid", Bytes: info.Size(), Error: fmt.Sprintf("blank event line at %d", line)}
		}
		var payload any
		if err := json.Unmarshal([]byte(text), &payload); err != nil {
			return DiagnosticsFile{Path: path.Relative, Status: "invalid", Bytes: info.Size(), Error: RedactString(fmt.Sprintf("line %d: %v", line, err))}
		}
		events = append(events, RedactValue(payload))
	}
	if err := scanner.Err(); err != nil {
		return DiagnosticsFile{Path: path.Relative, Status: "invalid", Bytes: info.Size(), Error: RedactString(err.Error())}
	}
	return DiagnosticsFile{Path: path.Relative, Status: "present", Bytes: info.Size(), Content: events}
}

func readRegularDiagnosticFile(path SafePath) ([]byte, os.FileInfo, error) {
	info, err := os.Lstat(path.Absolute)
	if err != nil {
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("not a regular file")
	}
	data, err := os.ReadFile(path.Absolute)
	return data, info, err
}

func diagnosticFileError(relative string, err error) DiagnosticsFile {
	status := "unreadable"
	if os.IsNotExist(err) {
		status = "missing"
	}
	return DiagnosticsFile{Path: relative, Status: status, Error: RedactString(err.Error())}
}

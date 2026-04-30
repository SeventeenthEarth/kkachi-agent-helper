package project

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	HealthOK      = "ok"
	HealthWarning = "warning"
	HealthFail    = "fail"

	CheckPass = "pass"
	CheckWarn = "warn"
	CheckFail = "fail"
)

// DiagnosticCheck is one read-only project doctor finding.
type DiagnosticCheck struct {
	Name     string
	Status   string
	Path     string
	Message  string
	Hint     string
	Field    string
	Expected string
	Actual   string
}

// DoctorSummary counts check outcomes.
type DoctorSummary struct {
	Passed   int
	Warnings int
	Failed   int
}

// DoctorReport is the read-only project diagnostic report.
type DoctorReport struct {
	RootPath string
	Health   string
	Summary  DoctorSummary
	Checks   []DiagnosticCheck
}

// ProjectStatus is a read-only summary of the initialized .kkachi project state.
type ProjectStatus struct {
	RootPath       string
	Health         string
	ProjectID      string
	ProjectName    string
	ActiveRunID    *string
	ActiveRunState *string
	LastEventID    string
	EventTailID    string
	EventCount     int
	UpdatedAt      string
	GateSummary    map[string]any
	Issues         []DiagnosticCheck
}

type configInfo struct {
	ProjectName string
}

type statusInfo struct {
	ProjectID      string
	ActiveRunID    *string
	ActiveRunState *string
	LastEventID    string
	UpdatedAt      string
	GateSummary    map[string]any
}

type eventLogInfo struct {
	TailID string
	Count  int
}

// InspectProjectStatus reads helper state and returns a summary without mutating .kkachi/.
func InspectProjectStatus(root Root) (ProjectStatus, error) {
	if strings.TrimSpace(root.Path) == "" {
		return ProjectStatus{}, problem("repo_root_required", "repository root is required", "Discover the repository root before inspecting project status.")
	}

	checks, facts := inspectProject(root)
	status := ProjectStatus{
		RootPath:       root.Path,
		Health:         healthFromChecks(checks),
		ProjectName:    facts.config.ProjectName,
		ProjectID:      facts.status.ProjectID,
		ActiveRunID:    facts.status.ActiveRunID,
		ActiveRunState: facts.status.ActiveRunState,
		LastEventID:    facts.status.LastEventID,
		EventTailID:    facts.events.TailID,
		EventCount:     facts.events.Count,
		UpdatedAt:      facts.status.UpdatedAt,
		GateSummary:    facts.status.GateSummary,
	}
	if status.GateSummary == nil {
		status.GateSummary = map[string]any{}
	}
	for _, check := range checks {
		if check.Status != CheckPass {
			status.Issues = append(status.Issues, check)
		}
	}
	return status, nil
}

// Doctor runs read-only checks over .kkachi/ state and never repairs or records events.
func Doctor(root Root) (DoctorReport, error) {
	if strings.TrimSpace(root.Path) == "" {
		return DoctorReport{}, problem("repo_root_required", "repository root is required", "Discover the repository root before running project doctor.")
	}
	checks, _ := inspectProject(root)
	summary := summarizeChecks(checks)
	return DoctorReport{RootPath: root.Path, Health: healthFromSummary(summary), Summary: summary, Checks: checks}, nil
}

type inspectionFacts struct {
	config configInfo
	status statusInfo
	events eventLogInfo
}

func inspectProject(root Root) ([]DiagnosticCheck, inspectionFacts) {
	var checks []DiagnosticCheck
	var facts inspectionFacts

	config, check := inspectConfig(root)
	facts.config = config
	checks = append(checks, check)

	status, check := inspectStatus(root)
	facts.status = status
	checks = append(checks, check)

	events, check := inspectEvents(root)
	facts.events = events
	checks = append(checks, check)

	checks = append(checks, inspectPaths(root))
	checks = append(checks, inspectSchemas(root)...)
	checks = append(checks, inspectLocks(root)...)

	if facts.status.LastEventID != "" && facts.events.TailID != "" && facts.status.LastEventID != facts.events.TailID {
		checks = append(checks, DiagnosticCheck{
			Name:     "coherence",
			Status:   CheckFail,
			Path:     StatusPath,
			Message:  "status last_event_id does not match the event log tail",
			Hint:     "Restore status.json and events.jsonl from a coherent backup before running mutating commands.",
			Field:    "last_event_id",
			Expected: facts.events.TailID,
			Actual:   facts.status.LastEventID,
		})
	}

	return checks, facts
}

func inspectConfig(root Root) (configInfo, DiagnosticCheck) {
	path, err := ResolveRelativePath(root, ConfigPath)
	if err != nil {
		return configInfo{}, checkFromError("config", err)
	}
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return configInfo{}, failCheck("config", path.Relative, "project config is missing or unreadable", "Run project init or restore .kkachi/config.yaml from backup.", "path", "readable config file", err.Error())
	}
	values := parseSimpleConfig(data)
	required := []struct {
		key      string
		expected string
		exact    bool
	}{
		{key: "version", expected: "0.1", exact: true},
		{key: "project.name", expected: "non-empty project name"},
		{key: "paths.run_root", expected: ".kkachi/runs", exact: true},
		{key: "paths.status_file", expected: StatusPath, exact: true},
		{key: "paths.events_file", expected: EventsPath, exact: true},
	}
	for _, field := range required {
		actual := values[field.key]
		if strings.TrimSpace(actual) == "" {
			return configInfo{}, failCheck("config", path.Relative, "project config is missing a required declaration", "Restore the config generated by project init or rerun initialization in a fresh repository.", field.key, field.expected, "missing")
		}
		if field.exact && actual != field.expected {
			return configInfo{}, failCheck("config", path.Relative, "project config declares an unexpected value", "Restore the canonical config path declarations before using helper commands.", field.key, field.expected, actual)
		}
	}
	return configInfo{ProjectName: values["project.name"]}, passCheck("config", path.Relative, "project config is readable and declares the required fields")
}

func parseSimpleConfig(data []byte) map[string]string {
	values := map[string]string{}
	section := ""
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") || !strings.Contains(trimmed, ":") {
			continue
		}
		parts := strings.SplitN(trimmed, ":", 2)
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if value == "" {
			section = key
			continue
		}
		value = strings.Trim(value, `"'`)
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if section != "" {
				values[section+"."+key] = value
			}
			continue
		}
		values[key] = value
		section = ""
	}
	return values
}

func inspectStatus(root Root) (statusInfo, DiagnosticCheck) {
	path, err := ResolveRelativePath(root, StatusPath)
	if err != nil {
		return statusInfo{}, checkFromError("status", err)
	}
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return statusInfo{}, failCheck("status", path.Relative, "project status is missing or unreadable", "Run project init or restore .kkachi/status.json from backup.", "path", "readable JSON object", err.Error())
	}
	var raw any
	if err := json.Unmarshal(data, &raw); err != nil {
		return statusInfo{}, failCheck("status", path.Relative, "project status is not valid JSON", "Restore .kkachi/status.json from a coherent backup.", "json", "JSON object", err.Error())
	}
	object, ok := raw.(map[string]any)
	if !ok || object == nil {
		return statusInfo{}, failCheck("status", path.Relative, "project status must be a JSON object", "Restore .kkachi/status.json from a coherent backup.", "json", "JSON object", fmt.Sprintf("%T", raw))
	}

	version, ok := object["version"].(string)
	if !ok || strings.TrimSpace(version) == "" {
		return statusInfo{}, failCheck("status", path.Relative, "project status is missing a valid version", "Restore status.json from the initialized helper state.", "version", "non-empty string", fmt.Sprintf("%v", object["version"]))
	}
	projectID, ok := object["project_id"].(string)
	if !ok || strings.TrimSpace(projectID) == "" {
		return statusInfo{}, failCheck("status", path.Relative, "project status is missing a valid project_id", "Restore status.json from the initialized helper state.", "project_id", "non-empty string", fmt.Sprintf("%v", object["project_id"]))
	}
	lastEventID, ok := object["last_event_id"].(string)
	if !ok || !eventIDPattern.MatchString(lastEventID) {
		return statusInfo{}, failCheck("status", path.Relative, "project status is missing a valid last_event_id", "Restore status.json from a coherent backup before mutating helper state.", "last_event_id", "evt- followed by six digits", fmt.Sprintf("%v", object["last_event_id"]))
	}
	updatedAt, ok := object["updated_at"].(string)
	if !ok || strings.TrimSpace(updatedAt) == "" {
		return statusInfo{}, failCheck("status", path.Relative, "project status is missing a valid updated_at", "Restore status.json from the initialized helper state.", "updated_at", "RFC3339 timestamp string", fmt.Sprintf("%v", object["updated_at"]))
	}
	if _, err := time.Parse(time.RFC3339, updatedAt); err != nil {
		return statusInfo{}, failCheck("status", path.Relative, "project status updated_at is not RFC3339", "Restore or rewrite status.json using the helper timestamp format.", "updated_at", "RFC3339 timestamp string", updatedAt)
	}
	gateSummary, ok := object["gate_summary"].(map[string]any)
	if !ok || gateSummary == nil {
		return statusInfo{}, failCheck("status", path.Relative, "project status is missing a valid gate_summary", "Restore status.json from the initialized helper state.", "gate_summary", "JSON object", fmt.Sprintf("%v", object["gate_summary"]))
	}

	activeRunID, err := optionalString(object, "active_run_id")
	if err != nil {
		return statusInfo{}, failCheck("status", path.Relative, "project status active_run_id must be null or a string", "Restore status.json or wait for run workflow migration support.", "active_run_id", "null or string", fmt.Sprintf("%v", object["active_run_id"]))
	}
	activeRunState, err := optionalString(object, "active_run_state")
	if err != nil {
		return statusInfo{}, failCheck("status", path.Relative, "project status active_run_state must be null or a string", "Restore status.json or wait for run workflow migration support.", "active_run_state", "null or string", fmt.Sprintf("%v", object["active_run_state"]))
	}

	return statusInfo{ProjectID: projectID, ActiveRunID: activeRunID, ActiveRunState: activeRunState, LastEventID: lastEventID, UpdatedAt: updatedAt, GateSummary: gateSummary}, passCheck("status", path.Relative, "project status is readable and structurally valid")
}

func optionalString(object map[string]any, key string) (*string, error) {
	value, ok := object[key]
	if !ok || value == nil {
		return nil, nil
	}
	text, ok := value.(string)
	if !ok {
		return nil, fmt.Errorf("not a string")
	}
	return &text, nil
}

func inspectEvents(root Root) (eventLogInfo, DiagnosticCheck) {
	path, err := ResolveRelativePath(root, EventsPath)
	if err != nil {
		return eventLogInfo{}, checkFromError("events", err)
	}
	file, err := os.Open(path.Absolute)
	if err != nil {
		return eventLogInfo{}, failCheck("events", path.Relative, "event log is missing or unreadable", "Run project init or restore .kkachi/events.jsonl from backup.", "path", "readable JSONL event log", err.Error())
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), MaxEventLineBytes)
	lineNumber := 0
	lastID := ""
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return eventLogInfo{}, failCheck("events", path.Relative, "event log contains an empty line", "Remove blank lines only by restoring a coherent backup; doctor is read-only.", fmt.Sprintf("line_%d", lineNumber), "non-empty JSON object line", "empty line")
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(line), &object); err != nil {
			return eventLogInfo{}, failCheck("events", path.Relative, "event log contains invalid JSON", "Restore .kkachi/events.jsonl from a coherent backup.", fmt.Sprintf("line_%d", lineNumber), "valid JSON object line", err.Error())
		}
		eventID, ok := object["event_id"].(string)
		if !ok || !eventIDPattern.MatchString(eventID) {
			return eventLogInfo{}, failCheck("events", path.Relative, "event log contains an invalid event_id", "Restore the event log from a coherent backup.", fmt.Sprintf("line_%d.event_id", lineNumber), "evt- followed by six digits", fmt.Sprintf("%v", object["event_id"]))
		}
		want := fmt.Sprintf("evt-%06d", lineNumber)
		if eventID != want {
			return eventLogInfo{}, failCheck("events", path.Relative, "event log event ids are not sequential", "Restore the event log from the last coherent backup before running mutating commands.", fmt.Sprintf("line_%d.event_id", lineNumber), want, eventID)
		}
		lastID = eventID
	}
	if err := scanner.Err(); err != nil {
		actual := err.Error()
		if strings.Contains(actual, "token too long") {
			actual = fmt.Sprintf("line exceeds %d bytes", MaxEventLineBytes)
		}
		return eventLogInfo{}, failCheck("events", path.Relative, "cannot scan event log", "Check event log permissions and restore from backup if the file is damaged.", "events", "readable JSONL event log", actual)
	}
	if lineNumber == 0 {
		return eventLogInfo{}, failCheck("events", path.Relative, "event log is empty", "Restore .kkachi/events.jsonl from a coherent backup.", "events", "at least one event line", "empty")
	}
	return eventLogInfo{TailID: lastID, Count: lineNumber}, passCheck("events", path.Relative, "event log is readable, non-empty, and sequential")
}

func inspectPaths(root Root) DiagnosticCheck {
	paths := append(append([]string(nil), statePaths...), schemaPaths...)
	paths = append(paths, lockPaths...)
	for _, relative := range paths {
		if _, err := ResolveRelativePath(root, relative); err != nil {
			check := checkFromError("paths", err)
			if check.Path == "" {
				check.Path = relative
			}
			return check
		}
	}
	return passCheck("paths", ".kkachi", "canonical helper paths stay inside the repository and do not symlink-escape")
}

func inspectSchemas(root Root) []DiagnosticCheck {
	checks := make([]DiagnosticCheck, 0, len(schemaPaths))
	for _, schemaPath := range schemaPaths {
		path, err := ResolveRelativePath(root, schemaPath)
		if err != nil {
			check := checkFromError("schemas", err)
			check.Path = schemaPath
			checks = append(checks, check)
			continue
		}
		data, err := os.ReadFile(path.Absolute)
		if err != nil {
			checks = append(checks, failCheck("schemas", path.Relative, "schema file is missing or unreadable", "Run project init in a fresh repository or restore the schema file from backup.", "path", "readable JSON schema file", err.Error()))
			continue
		}
		var object map[string]any
		if err := json.Unmarshal(data, &object); err != nil || object == nil {
			actual := "not a JSON object"
			if err != nil {
				actual = err.Error()
			}
			checks = append(checks, failCheck("schemas", path.Relative, "schema file is not a valid JSON object", "Restore the generated schema file from backup.", "json", "JSON object schema", actual))
			continue
		}
		if !schemaRequiresVersion(object) {
			checks = append(checks, failCheck("schemas", path.Relative, "schema does not require the version property", "Restore the generated schema file so versioned state remains explicit.", "required", "array containing version", fmt.Sprintf("%v", object["required"])))
			continue
		}
		checks = append(checks, passCheck("schemas", path.Relative, "schema is readable and requires version"))
	}
	return checks
}

func schemaRequiresVersion(object map[string]any) bool {
	required, ok := object["required"].([]any)
	if !ok {
		return false
	}
	for _, value := range required {
		if value == "version" {
			return true
		}
	}
	return false
}

func inspectLocks(root Root) []DiagnosticCheck {
	checks := make([]DiagnosticCheck, 0, len(lockPaths))
	for _, lockPath := range lockPaths {
		path, err := ResolveRelativePath(root, lockPath)
		if err != nil {
			check := checkFromError("locks", err)
			check.Path = lockPath
			checks = append(checks, check)
			continue
		}
		name := ProjectWriteLockName
		if lockPath == activeRunLockPath {
			name = ActiveRunLockName
		}
		info, err := os.Lstat(path.Absolute)
		if os.IsNotExist(err) {
			checks = append(checks, passCheck("locks", path.Relative, "lock file is absent"))
			continue
		}
		if err != nil {
			checks = append(checks, failCheck("locks", path.Relative, "cannot inspect lock file", "Check permissions and remove unsafe lock paths only after preserving diagnostics.", "path", "inspectable absent or readable lock", err.Error()))
			continue
		}
		if info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			checks = append(checks, failCheck("locks", path.Relative, "lock path is not a regular file", "Preserve the path for diagnosis; do not remove it silently.", "path", "absent or readable regular lock", info.Mode().String()))
			continue
		}
		inspection, err := inspectLockFile(root, name, time.Now().UTC())
		if err != nil {
			checks = append(checks, checkFromError("locks", err))
			continue
		}
		actual := lockIdentity(inspection.metadata)
		if inspection.stale {
			checks = append(checks, DiagnosticCheck{Name: "locks", Status: CheckWarn, Path: path.Relative, Message: "stale lock file is present", Hint: "Run lock recover for this stale lock target with --reason before retrying mutating commands.", Field: "lock", Expected: "absent lock", Actual: actual + " stale: " + inspection.reason})
			continue
		}
		checks = append(checks, DiagnosticCheck{Name: "locks", Status: CheckWarn, Path: path.Relative, Message: "fresh lock file is present", Hint: "Wait for the active writer to finish before mutating helper state.", Field: "lock", Expected: "absent when no helper run is active", Actual: actual})
	}
	return checks
}

func passCheck(name, path, message string) DiagnosticCheck {
	return DiagnosticCheck{Name: name, Status: CheckPass, Path: path, Message: message}
}

func failCheck(name, path, message, hint, field, expected, actual string) DiagnosticCheck {
	return DiagnosticCheck{Name: name, Status: CheckFail, Path: path, Message: message, Hint: hint, Field: field, Expected: expected, Actual: actual}
}

func checkFromError(name string, err error) DiagnosticCheck {
	var p *Problem
	if errors.As(err, &p) {
		return failCheck(name, p.Path, p.Message, p.Hint, p.Field, p.Expected, p.Actual)
	}
	return failCheck(name, "", "unexpected diagnostic error", "Rerun with the same arguments and preserve stderr for diagnosis.", "error", "diagnostic check", err.Error())
}

func summarizeChecks(checks []DiagnosticCheck) DoctorSummary {
	var summary DoctorSummary
	for _, check := range checks {
		switch check.Status {
		case CheckPass:
			summary.Passed++
		case CheckWarn:
			summary.Warnings++
		case CheckFail:
			summary.Failed++
		}
	}
	return summary
}

func healthFromChecks(checks []DiagnosticCheck) string {
	return healthFromSummary(summarizeChecks(checks))
}

func healthFromSummary(summary DoctorSummary) string {
	if summary.Failed > 0 {
		return HealthFail
	}
	if summary.Warnings > 0 {
		return HealthWarning
	}
	return HealthOK
}

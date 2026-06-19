package project

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	SchemaVersion = "0.1"

	SchemaConfig                   = "config"
	SchemaStatus                   = "status"
	SchemaEvent                    = "event"
	SchemaRunMetadata              = "run-metadata"
	SchemaSelectedCLI              = "selected-cli"
	SchemaBridgeSessionSnapshot    = "bridge-session-snapshot"
	SchemaTokenEconomyEvidence     = "token-economy-evidence"
	SchemaMultiAgentReviewEvidence = "multi-agent-review-evidence"
	SchemaPolicyPromotionEvidence  = "policy-promotion-evidence"

	schemaExportedEventType = "schema.exported"

	schemaExportStateAbsent    = "absent"
	schemaExportStateChanged   = "changed"
	schemaExportStateUnchanged = "unchanged"
)

var canonicalSchemaNames = []string{
	SchemaConfig,
	SchemaStatus,
	SchemaEvent,
	SchemaRunMetadata,
	SchemaSelectedCLI,
	SchemaBridgeSessionSnapshot,
	SchemaTokenEconomyEvidence,
	SchemaMultiAgentReviewEvidence,
	SchemaPolicyPromotionEvidence,
}

type SchemaCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Path     string `json:"path"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
	Line     int    `json:"line,omitempty"`
}

type SchemaValidateOptions struct {
	File   string
	Schema string
}

type SchemaValidateResult struct {
	Schema   string        `json:"schema"`
	FilePath string        `json:"file_path"`
	Status   string        `json:"status"`
	Checks   []SchemaCheck `json:"checks"`
}

type SchemaExportOptions struct {
	Schema string
	All    bool
	DryRun bool
	Now    func() time.Time
}

type SchemaExportResult struct {
	DryRun     bool     `json:"dry_run"`
	Schemas    []string `json:"schemas"`
	Written    []string `json:"written"`
	Unchanged  []string `json:"unchanged"`
	WouldWrite []string `json:"would_write"`
	EventID    string   `json:"event_id,omitempty"`
}

func CanonicalSchemaNames() []string {
	return append([]string(nil), canonicalSchemaNames...)
}

func SchemaPathForName(name string) (string, error) {
	name, err := ResolveSchemaName(name)
	if err != nil {
		return "", err
	}
	return ".kkachi/schemas/" + name + ".schema.json", nil
}

func ResolveSchemaName(input string) (string, error) {
	original := input
	input = strings.TrimSpace(input)
	if input == "" {
		return "", &Problem{Code: "schema_required", Message: "schema name is required", Hint: "Pass --schema with one of: " + strings.Join(canonicalSchemaNames, ", ") + ".", Field: "schema", Expected: strings.Join(canonicalSchemaNames, ","), Actual: "empty"}
	}
	input = filepath.ToSlash(input)
	base := filepath.Base(input)
	if strings.HasSuffix(base, ".schema.json") {
		input = strings.TrimSuffix(base, ".schema.json")
	}
	input = strings.TrimSpace(input)
	for _, name := range canonicalSchemaNames {
		if input == name {
			return name, nil
		}
	}
	return "", &Problem{Code: "schema_unknown", Message: "schema is not registered", Hint: "Use one of: " + strings.Join(canonicalSchemaNames, ", ") + ".", Field: "schema", Expected: strings.Join(canonicalSchemaNames, ","), Actual: original}
}

func SchemaDocument(name string) ([]byte, error) {
	resolved, err := ResolveSchemaName(name)
	if err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(schemaObject(resolved), "", "  ")
	if err != nil {
		return nil, &Problem{Code: "schema_encode_failed", Message: "cannot encode embedded schema", Hint: "Rerun with the same arguments and preserve stderr for diagnosis.", Field: "schema", Expected: "JSON schema object", Actual: err.Error()}
	}
	return append(data, '\n'), nil
}

func ValidateSchemaFile(root Root, options SchemaValidateOptions) (SchemaValidateResult, error) {
	if strings.TrimSpace(root.Path) == "" {
		return SchemaValidateResult{}, problem("repo_root_required", "repository root is required", "Discover the repository root before validating a schema file.")
	}
	schemaName, err := resolveSchemaOption(root, options.Schema)
	if err != nil {
		return SchemaValidateResult{}, err
	}
	path, err := ResolveRelativePath(root, options.File)
	if err != nil {
		return SchemaValidateResult{}, err
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		return SchemaValidateResult{}, &Problem{Code: "schema_validation_read_failed", Message: "cannot read file for schema validation", Hint: "Check the path and file permissions before validating again.", Path: path.Relative, Field: "path", Expected: "readable file", Actual: err.Error()}
	}
	checks := validateContentAgainstSchema(schemaName, path.Relative, content)
	return SchemaValidateResult{Schema: schemaName, FilePath: path.Relative, Status: schemaStatus(checks), Checks: checks}, nil
}

func ExportSchemas(root Root, options SchemaExportOptions) (SchemaExportResult, error) {
	if strings.TrimSpace(root.Path) == "" {
		return SchemaExportResult{}, problem("repo_root_required", "repository root is required", "Discover the repository root before exporting schemas.")
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	names, err := exportSchemaNames(options)
	if err != nil {
		return SchemaExportResult{}, err
	}
	if options.DryRun {
		return exportSchemasUnlocked(root, options, names, true)
	}
	var result SchemaExportResult
	err = withProjectWriteLock(root, "schema export", "", func() error {
		var err error
		result, err = exportSchemasUnlocked(root, options, names, false)
		return err
	})
	return result, err
}

func exportSchemaNames(options SchemaExportOptions) ([]string, error) {
	if options.All && strings.TrimSpace(options.Schema) != "" {
		return nil, &Problem{Code: "schema_export_selector_conflict", Message: "schema export accepts either --all or --schema, not both", Hint: "Use schema export --all or schema export --schema <name>.", Field: "schema", Expected: "one selector", Actual: "--all and --schema"}
	}
	if options.All || strings.TrimSpace(options.Schema) == "" {
		return CanonicalSchemaNames(), nil
	}
	name, err := ResolveSchemaName(options.Schema)
	if err != nil {
		return nil, err
	}
	return []string{name}, nil
}

func exportSchemasUnlocked(root Root, options SchemaExportOptions, names []string, dryRun bool) (SchemaExportResult, error) {
	if !dryRun {
		if err := preflightEventCoherence(root); err != nil {
			return SchemaExportResult{}, err
		}
	}
	result := SchemaExportResult{DryRun: dryRun, Schemas: append([]string(nil), names...)}
	writtenPaths := []string{}
	for _, name := range names {
		relative, err := SchemaPathForName(name)
		if err != nil {
			return SchemaExportResult{}, err
		}
		path, err := ResolveRelativePath(root, relative)
		if err != nil {
			return SchemaExportResult{}, err
		}
		content, err := SchemaDocument(name)
		if err != nil {
			return SchemaExportResult{}, err
		}
		state, err := schemaExportState(path, content)
		if err != nil {
			return SchemaExportResult{}, err
		}
		switch state {
		case schemaExportStateUnchanged:
			result.Unchanged = append(result.Unchanged, path.Relative)
		case schemaExportStateAbsent, schemaExportStateChanged:
			if dryRun {
				result.WouldWrite = append(result.WouldWrite, path.Relative)
				continue
			}
			if err := writeSchemaFile(path, content); err != nil {
				return SchemaExportResult{}, err
			}
			result.Written = append(result.Written, path.Relative)
			writtenPaths = append(writtenPaths, path.Relative)
		}
	}
	if !dryRun && len(writtenPaths) > 0 {
		appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: schemaExportedEventType, Payload: map[string]any{"schemas": names, "written": writtenPaths}, Now: options.Now}, nil)
		if err != nil {
			return SchemaExportResult{}, err
		}
		result.EventID = appendResult.EventID
	}
	return result, nil
}

func schemaExportState(path SafePath, content []byte) (string, error) {
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		return schemaExportStateAbsent, nil
	}
	if err != nil {
		return "", &Problem{Code: "schema_export_inspection_failed", Message: "cannot inspect schema export path", Hint: "Check .kkachi/schemas permissions before exporting schemas.", Path: path.Relative, Field: "path", Expected: "inspectable schema path", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		actual := "non-regular"
		if info.IsDir() {
			actual = "directory"
		}
		return "", &Problem{Code: "schema_export_conflict", Message: "schema export path must be a regular file", Hint: "Move the conflicting path before exporting schemas.", Path: path.Relative, Field: "path", Expected: "regular file or absent path", Actual: actual}
	}
	existing, err := os.ReadFile(path.Absolute)
	if err != nil {
		return "", &Problem{Code: "schema_export_read_failed", Message: "cannot read existing schema export path", Hint: "Check .kkachi/schemas permissions before exporting schemas.", Path: path.Relative, Field: "path", Expected: "readable existing schema", Actual: err.Error()}
	}
	if bytes.Equal(existing, content) {
		return schemaExportStateUnchanged, nil
	}
	return schemaExportStateChanged, nil
}

func writeSchemaFile(path SafePath, content []byte) error {
	if _, err := os.Lstat(path.Absolute); os.IsNotExist(err) {
		return writeNewFileAtomically(path, content)
	}
	return writeExistingFileAtomically(path, content)
}

func resolveSchemaOption(root Root, value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if strings.Contains(trimmed, "/") || strings.HasSuffix(trimmed, ".schema.json") {
		path, err := ResolveRelativePath(root, trimmed)
		if err != nil {
			return "", err
		}
		content, err := os.ReadFile(path.Absolute)
		if err != nil {
			return "", &Problem{Code: "schema_read_failed", Message: "cannot read schema reference", Hint: "Use an embedded schema name or a readable .kkachi/schemas/*.schema.json path.", Path: path.Relative, Field: "schema", Expected: "readable schema file", Actual: err.Error()}
		}
		var object map[string]any
		if err := json.Unmarshal(content, &object); err != nil || object == nil {
			actual := "not an object"
			if err != nil {
				actual = err.Error()
			}
			return "", &Problem{Code: "schema_reference_invalid", Message: "schema reference is not a valid JSON object", Hint: "Restore the exported schema or pass an embedded schema name.", Path: path.Relative, Field: "schema", Expected: "JSON schema object", Actual: actual}
		}
		name, err := ResolveSchemaName(path.Relative)
		if err != nil {
			return "", err
		}
		if !schemaDeclaresIdentity(object, name) {
			return "", &Problem{Code: "schema_reference_invalid", Message: "schema reference does not declare the expected embedded schema identity", Hint: "Run schema export --schema " + name + " to refresh the local schema copy.", Path: path.Relative, Field: "$id", Expected: "https://kkachi.local/schemas/" + name + ".schema.json", Actual: fmt.Sprintf("%v", object["$id"])}
		}
		return name, nil
	}
	return ResolveSchemaName(trimmed)
}

func schemaDeclaresIdentity(object map[string]any, name string) bool {
	id, _ := object["$id"].(string)
	return id == "https://kkachi.local/schemas/"+name+".schema.json"
}

func validateContentAgainstSchema(schemaName, relative string, content []byte) []SchemaCheck {
	if schemaName == SchemaConfig {
		return validateConfigSchema(relative, content)
	}
	if schemaName == SchemaEvent && strings.HasSuffix(relative, ".jsonl") {
		return validateEventJSONL(relative, content)
	}
	var payload any
	if err := json.Unmarshal(content, &payload); err != nil {
		return []SchemaCheck{schemaFail("json", relative, "file is not valid JSON", "Fix the file so it contains one JSON object before validating again.", "json", "valid JSON object", err.Error())}
	}
	object, ok := payload.(map[string]any)
	if !ok || object == nil {
		return []SchemaCheck{schemaFail("json_object", relative, "file must contain a JSON object", "Record helper state as a JSON object, not null, an array, or a scalar.", "json", "JSON object", fmt.Sprintf("%T", payload))}
	}
	switch schemaName {
	case SchemaStatus:
		return validateStatusObject(relative, object)
	case SchemaEvent:
		return validateEventObject(relative, object, 0)
	case SchemaRunMetadata:
		return validateRunMetadataObject(relative, object)
	case SchemaSelectedCLI:
		return validateSelectedCLIObject(relative, object)
	case SchemaBridgeSessionSnapshot:
		return validateBridgeSnapshotObject(relative, object)
	case SchemaTokenEconomyEvidence:
		return validateTokenEconomyEvidenceSchema(relative, content)
	case SchemaMultiAgentReviewEvidence:
		return validateMultiAgentReviewEvidenceSchema(relative, content)
	case SchemaPolicyPromotionEvidence:
		return validatePolicyPromotionEvidenceSchema(relative, content)
	default:
		return []SchemaCheck{schemaPass("schema", relative, "schema is registered")}
	}
}

func validateConfigSchema(relative string, content []byte) []SchemaCheck {
	values := parseSimpleConfig(content)
	required := []struct{ field, expected string }{
		{"version", SchemaVersion},
		{"project.name", "non-empty project name"},
		{"project.root_policy", "repository_confined_no_symlink_escape"},
		{"paths.run_root", RunRootPath},
		{"paths.status_file", StatusPath},
		{"paths.events_file", EventsPath},
		{"locks.one_active_write_run", "true"},
		{"schemas.mode", "embedded, local, or both"},
		{"compat.required_skills", "declared value or null"},
		{"compat.required_bridge", "declared value or null"},
	}
	checks := make([]SchemaCheck, 0, len(required)+1)
	for _, field := range required {
		actual := strings.TrimSpace(values[field.field])
		if actual == "" {
			checks = append(checks, schemaFail("required", relative, "config is missing a required field", "Restore the generated config or rerun project init in a fresh repository.", field.field, field.expected, "missing"))
			continue
		}
		switch field.field {
		case "project.name":
			checks = append(checks, schemaPass(field.field, relative, "config field is present"))
		case "schemas.mode":
			if !allowed(actual, "embedded", "local", "both") {
				checks = append(checks, schemaFail(field.field, relative, "config schema mode is invalid", "Use schemas.mode: embedded, local, or both.", field.field, field.expected, actual))
			} else {
				checks = append(checks, schemaPass(field.field, relative, "config field is valid"))
			}
		default:
			if actual != field.expected && field.expected != "declared value or null" {
				checks = append(checks, schemaFail(field.field, relative, "config field has an unexpected value", "Restore the generated config value before validating helper state.", field.field, field.expected, actual))
			} else {
				checks = append(checks, schemaPass(field.field, relative, "config field is valid"))
			}
		}
	}
	return checks
}

func validateStatusObject(relative string, object map[string]any) []SchemaCheck {
	checks := []SchemaCheck{}
	checks = append(checks, requireStringField(relative, object, "version", SchemaVersion, true))
	checks = append(checks, requireStringField(relative, object, "project_id", "non-empty string", false))
	checks = append(checks, requireNullableStringField(relative, object, "active_run_id"))
	checks = append(checks, requireNullableStringField(relative, object, "active_run_state"))
	checks = append(checks, requirePatternField(relative, object, "last_event_id", eventIDPattern.String(), eventIDPattern.MatchString))
	checks = append(checks, requireRFC3339Field(relative, object, "updated_at"))
	checks = append(checks, requireObjectField(relative, object, "gate_summary"))
	return checks
}

func validateEventJSONL(relative string, content []byte) []SchemaCheck {
	checks := []SchemaCheck{}
	scanner := bufio.NewScanner(bytes.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), MaxEventLineBytes)
	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			checks = append(checks, schemaFailLine("jsonl_line", relative, line, "event JSONL line must not be empty", "Remove blank lines from events.jsonl.", "line", "JSON object", "empty"))
			continue
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(text), &object); err != nil || object == nil {
			actual := "not an object"
			if err != nil {
				actual = err.Error()
			}
			checks = append(checks, schemaFailLine("jsonl_line", relative, line, "event JSONL line must be a valid JSON object", "Restore the malformed event line from a coherent backup.", "json", "JSON object", actual))
			continue
		}
		for _, check := range validateEventObject(relative, object, line) {
			checks = append(checks, check)
		}
	}
	if err := scanner.Err(); err != nil {
		checks = append(checks, schemaFail("jsonl_scan", relative, "cannot scan event JSONL", "Check events.jsonl for over-large lines or read errors.", "events", "readable JSONL", err.Error()))
	}
	if line == 0 {
		checks = append(checks, schemaFail("jsonl_line", relative, "event JSONL must contain at least one event", "Restore events.jsonl from initialized helper state.", "line", "at least one event", "empty"))
	}
	return checks
}

func validateEventObject(relative string, object map[string]any, line int) []SchemaCheck {
	checks := []SchemaCheck{}
	checks = append(checks, requireStringFieldLine(relative, object, "version", SchemaVersion, true, line))
	checks = append(checks, requirePatternFieldLine(relative, object, "event_id", eventIDPattern.String(), eventIDPattern.MatchString, line))
	checks = append(checks, requireRFC3339FieldLine(relative, object, "occurred_at", line))
	checks = append(checks, requireNullableStringFieldLine(relative, object, "run_id", line))
	checks = append(checks, requireStringFieldLine(relative, object, "type", "non-empty string", false, line))
	checks = append(checks, requireEnumFieldLine(relative, object, "actor", []string{"helper", "commander", "bridge", "reviewer", "operator"}, line))
	checks = append(checks, requireObjectFieldLine(relative, object, "payload", line))
	return checks
}

func validateRunMetadataObject(relative string, object map[string]any) []SchemaCheck {
	checks := []SchemaCheck{}
	checks = append(checks, requireStringField(relative, object, "version", RunMetadataVersion, true))
	checks = append(checks, requirePatternField(relative, object, "run_id", runIDPattern.String(), runIDPattern.MatchString))
	checks = append(checks, requireNullableStringField(relative, object, "task_id"))
	checks = append(checks, requireStringField(relative, object, "title", "non-empty string", false))
	checks = append(checks, requireEnumField(relative, object, "work_path", []string{"A_development_execution", "B_discovery_shaping"}))
	checks = append(checks, requireEnumField(relative, object, "work_mode", []string{"standard", "light"}))
	checks = append(checks, requireEnumField(relative, object, "urgency", []string{"normal", "urgent", "critical"}))
	checks = append(checks, requireEnumField(relative, object, "sot_policy", []string{"existing_sot_basis", "minimal_sot_before_code", "full_sot_before_code"}))
	checks = append(checks, requireEnumField(relative, object, "execution_mode", []string{"production_write", "adapter_qa", "readiness_hardening", "research", "verification", "docs_only"}))
	if _, ok := object["backend_evidence"]; ok {
		checks = append(checks, requireEnumField(relative, object, "backend_evidence", []string{BackendEvidenceRequired, BackendEvidenceNotApplicable}))
	}
	checks = append(checks, requireStringField(relative, object, "commander", "non-empty string", false))
	checks = append(checks, requireNullableStringField(relative, object, "redteam"))
	checks = append(checks, requireRFC3339Field(relative, object, "created_at"))
	checks = append(checks, requireEnumField(relative, object, "state", []string{RunStateCreated, RunStateActive, RunStateClosed, RunStateAborted}))
	checks = append(checks, requireStringArrayField(relative, object, "required_artifacts"))
	checks = append(checks, requireObjectField(relative, object, "gate_state"))
	return checks
}

func validateSelectedCLIObject(relative string, object map[string]any) []SchemaCheck {
	checks := []SchemaCheck{}
	checks = append(checks, requireStringField(relative, object, "version", SchemaVersion, true))
	checks = append(checks, requireEnumField(relative, object, "status", []string{"supported", "degraded", "unsupported", "pending"}))
	checks = append(checks, requireStringField(relative, object, "backend_type", "non-empty string", false))
	checks = append(checks, requireStringField(relative, object, "adapter_type", "non-empty string", false))
	checks = append(checks, requireStringField(relative, object, "source_ledger_ref", "non-empty string", false))
	checks = append(checks, requireStringArrayField(relative, object, "caveats"))
	return checks
}

func validateBridgeSnapshotObject(relative string, object map[string]any) []SchemaCheck {
	checks := []SchemaCheck{}
	checks = append(checks, requireStringField(relative, object, "session_id", "non-empty string", false))
	checks = append(checks, requireStringField(relative, object, "backend_type", "non-empty string", false))
	checks = append(checks, requireStringField(relative, object, "adapter_type", "non-empty string", false))
	checks = append(checks, requireStringField(relative, object, "state", "non-empty string", false))
	checks = append(checks, requireStringField(relative, object, "lifecycle_class", "non-empty string", false))
	checks = append(checks, requireIntegerField(relative, object, "open_pendings"))
	return checks
}

func requireStringField(relative string, object map[string]any, field, expected string, exact bool) SchemaCheck {
	return requireStringFieldLine(relative, object, field, expected, exact, 0)
}

func requireStringFieldLine(relative string, object map[string]any, field, expected string, exact bool, line int) SchemaCheck {
	value, ok := object[field].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return schemaFailLine(field, relative, line, "required string field is missing or invalid", "Record a non-empty string for this schema field.", field, expected, fmt.Sprintf("%v", object[field]))
	}
	if exact && value != expected {
		return schemaFailLine(field, relative, line, "string field has an unsupported version", "Use the schema version generated by this helper release.", field, expected, value)
	}
	return schemaPassLine(field, relative, line, "string field is valid")
}

func requireNullableStringField(relative string, object map[string]any, field string) SchemaCheck {
	return requireNullableStringFieldLine(relative, object, field, 0)
}

func requireNullableStringFieldLine(relative string, object map[string]any, field string, line int) SchemaCheck {
	value, ok := object[field]
	if !ok {
		return schemaFailLine(field, relative, line, "nullable string field is missing", "Record the field with a string value or null.", field, "string or null", "missing")
	}
	if value == nil {
		return schemaPassLine(field, relative, line, "nullable string field is valid")
	}
	if _, ok := value.(string); !ok {
		return schemaFailLine(field, relative, line, "nullable string field has an invalid type", "Record the field with a string value or null.", field, "string or null", fmt.Sprintf("%T", value))
	}
	return schemaPassLine(field, relative, line, "nullable string field is valid")
}

func requirePatternField(relative string, object map[string]any, field, expected string, match func(string) bool) SchemaCheck {
	return requirePatternFieldLine(relative, object, field, expected, match, 0)
}

func requirePatternFieldLine(relative string, object map[string]any, field, expected string, match func(string) bool, line int) SchemaCheck {
	value, ok := object[field].(string)
	if !ok || !match(value) {
		return schemaFailLine(field, relative, line, "field does not match the required pattern", "Restore the generated identifier format before validating helper state.", field, expected, fmt.Sprintf("%v", object[field]))
	}
	return schemaPassLine(field, relative, line, "pattern field is valid")
}

func requireRFC3339Field(relative string, object map[string]any, field string) SchemaCheck {
	return requireRFC3339FieldLine(relative, object, field, 0)
}

func requireRFC3339FieldLine(relative string, object map[string]any, field string, line int) SchemaCheck {
	value, ok := object[field].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return schemaFailLine(field, relative, line, "timestamp field is missing or invalid", "Record the timestamp in RFC3339 UTC form.", field, "RFC3339 timestamp", fmt.Sprintf("%v", object[field]))
	}
	if _, err := time.Parse(time.RFC3339, value); err != nil {
		return schemaFailLine(field, relative, line, "timestamp field is not RFC3339", "Record the timestamp in RFC3339 UTC form.", field, "RFC3339 timestamp", value)
	}
	return schemaPassLine(field, relative, line, "timestamp field is valid")
}

func requireObjectField(relative string, object map[string]any, field string) SchemaCheck {
	return requireObjectFieldLine(relative, object, field, 0)
}

func requireObjectFieldLine(relative string, object map[string]any, field string, line int) SchemaCheck {
	value, ok := object[field]
	if !ok {
		return schemaFailLine(field, relative, line, "object field is missing", "Record an object for this schema field.", field, "object", "missing")
	}
	if _, ok := value.(map[string]any); !ok {
		return schemaFailLine(field, relative, line, "object field has an invalid type", "Record an object for this schema field.", field, "object", fmt.Sprintf("%T", value))
	}
	return schemaPassLine(field, relative, line, "object field is valid")
}

func requireEnumField(relative string, object map[string]any, field string, values []string) SchemaCheck {
	return requireEnumFieldLine(relative, object, field, values, 0)
}

func requireEnumFieldLine(relative string, object map[string]any, field string, values []string, line int) SchemaCheck {
	value, ok := object[field].(string)
	if !ok || !allowed(value, values...) {
		return schemaFailLine(field, relative, line, "enum field has an invalid value", "Use one of the schema-supported values.", field, strings.Join(values, ","), fmt.Sprintf("%v", object[field]))
	}
	return schemaPassLine(field, relative, line, "enum field is valid")
}

func requireStringArrayField(relative string, object map[string]any, field string) SchemaCheck {
	value, ok := object[field]
	if !ok {
		return schemaFail(field, relative, "string array field is missing", "Record this field as an array of strings; use [] when empty.", field, "array of strings", "missing")
	}
	array, ok := value.([]any)
	if !ok {
		return schemaFail(field, relative, "string array field has an invalid type", "Record this field as an array of strings; use [] when empty.", field, "array of strings", fmt.Sprintf("%T", value))
	}
	for _, item := range array {
		if _, ok := item.(string); !ok {
			return schemaFail(field, relative, "string array field contains a non-string item", "Record this field as an array of strings only.", field, "array of strings", fmt.Sprintf("%T", item))
		}
	}
	return schemaPass(field, relative, "string array field is valid")
}

func requireIntegerField(relative string, object map[string]any, field string) SchemaCheck {
	value, ok := object[field]
	if !ok {
		return schemaFail(field, relative, "integer field is missing", "Record an integer for this schema field.", field, "integer", "missing")
	}
	if _, ok := jsonNumberAsInt(value); !ok {
		return schemaFail(field, relative, "integer field has an invalid type", "Record an integer value for this schema field.", field, "integer", fmt.Sprintf("%v", value))
	}
	return schemaPass(field, relative, "integer field is valid")
}

func schemaStatus(checks []SchemaCheck) string {
	for _, check := range checks {
		if check.Status == "fail" {
			return "fail"
		}
	}
	return "pass"
}

func schemaPass(name, relative, message string) SchemaCheck {
	return schemaPassLine(name, relative, 0, message)
}

func schemaPassLine(name, relative string, line int, message string) SchemaCheck {
	return SchemaCheck{Name: name, Status: "pass", Path: relative, Message: message, Line: line}
}

func schemaFail(name, relative, message, hint, field, expected, actual string) SchemaCheck {
	return schemaFailLine(name, relative, 0, message, hint, field, expected, actual)
}

func schemaFailLine(name, relative string, line int, message, hint, field, expected, actual string) SchemaCheck {
	return SchemaCheck{Name: name, Status: "fail", Path: relative, Message: message, Hint: hint, Field: field, Expected: expected, Actual: actual, Line: line}
}

func schemaObject(name string) map[string]any {
	object := map[string]any{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"$id":     "https://kkachi.local/schemas/" + name + ".schema.json",
		"title":   name,
		"version": SchemaVersion,
		"type":    "object",
	}
	switch name {
	case SchemaConfig:
		object["description"] = "kkachi-agent-helper config.yaml contract; helper validates this deterministic YAML shape."
		object["required"] = []string{"version", "project", "paths", "locks", "schemas", "compat"}
	case SchemaStatus:
		object["required"] = []string{"version", "project_id", "active_run_id", "active_run_state", "last_event_id", "updated_at", "gate_summary"}
		object["properties"] = map[string]any{
			"version":          map[string]any{"const": SchemaVersion},
			"project_id":       map[string]any{"type": "string", "minLength": 1},
			"active_run_id":    map[string]any{"type": []string{"string", "null"}},
			"active_run_state": map[string]any{"type": []string{"string", "null"}},
			"last_event_id":    map[string]any{"type": "string", "pattern": "^evt-[0-9]{6}$"},
			"updated_at":       map[string]any{"type": "string", "format": "date-time"},
			"gate_summary":     map[string]any{"type": "object"},
		}
	case SchemaEvent:
		object["required"] = []string{"version", "event_id", "occurred_at", "run_id", "type", "actor", "payload"}
		object["properties"] = map[string]any{
			"version":     map[string]any{"const": SchemaVersion},
			"event_id":    map[string]any{"type": "string", "pattern": "^evt-[0-9]{6}$"},
			"occurred_at": map[string]any{"type": "string", "format": "date-time"},
			"run_id":      map[string]any{"type": []string{"string", "null"}},
			"type":        map[string]any{"type": "string", "minLength": 1},
			"actor":       map[string]any{"enum": []string{"helper", "commander", "bridge", "reviewer", "operator"}},
			"payload":     map[string]any{"type": "object"},
		}
	case SchemaRunMetadata:
		object["required"] = []string{"version", "run_id", "task_id", "title", "work_path", "work_mode", "urgency", "sot_policy", "execution_mode", "commander", "redteam", "created_at", "state", "required_artifacts", "gate_state"}
		object["properties"] = map[string]any{
			"version":            map[string]any{"const": RunMetadataVersion},
			"run_id":             map[string]any{"type": "string", "pattern": "^run-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$"},
			"task_id":            map[string]any{"type": []string{"string", "null"}},
			"title":              map[string]any{"type": "string", "minLength": 1},
			"work_path":          map[string]any{"enum": []string{"A_development_execution", "B_discovery_shaping"}},
			"work_mode":          map[string]any{"enum": []string{"standard", "light"}},
			"urgency":            map[string]any{"enum": []string{"normal", "urgent", "critical"}},
			"sot_policy":         map[string]any{"enum": []string{"existing_sot_basis", "minimal_sot_before_code", "full_sot_before_code"}},
			"execution_mode":     map[string]any{"enum": []string{"production_write", "adapter_qa", "readiness_hardening", "research", "verification", "docs_only"}},
			"backend_evidence":   map[string]any{"enum": []string{BackendEvidenceRequired, BackendEvidenceNotApplicable}},
			"commander":          map[string]any{"type": "string", "minLength": 1},
			"redteam":            map[string]any{"type": []string{"string", "null"}},
			"created_at":         map[string]any{"type": "string", "format": "date-time"},
			"state":              map[string]any{"enum": []string{RunStateCreated, RunStateActive, RunStateClosed, RunStateAborted}},
			"required_artifacts": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			"gate_state":         map[string]any{"type": "object"},
		}
	case SchemaSelectedCLI:
		object["required"] = []string{"version", "status", "backend_type", "adapter_type", "source_ledger_ref", "caveats"}
		object["properties"] = map[string]any{
			"version":           map[string]any{"const": SchemaVersion},
			"status":            map[string]any{"enum": []string{"supported", "degraded", "unsupported", "pending"}},
			"backend_type":      map[string]any{"type": "string", "minLength": 1},
			"adapter_type":      map[string]any{"type": "string", "minLength": 1},
			"source_ledger_ref": map[string]any{"type": "string", "minLength": 1},
			"caveats":           map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
		}
	case SchemaBridgeSessionSnapshot:
		object["required"] = []string{"version", "session_id", "backend_type", "adapter_type", "state", "lifecycle_class", "open_pendings"}
		object["properties"] = map[string]any{
			"version":         map[string]any{"const": SchemaVersion},
			"session_id":      map[string]any{"type": "string", "minLength": 1},
			"backend_type":    map[string]any{"type": "string", "minLength": 1},
			"adapter_type":    map[string]any{"type": "string", "minLength": 1},
			"state":           map[string]any{"type": "string", "minLength": 1},
			"lifecycle_class": map[string]any{"type": "string", "minLength": 1},
			"open_pendings":   map[string]any{"type": "integer", "minimum": 0},
		}
	case SchemaTokenEconomyEvidence:
		object["description"] = "token-001/token-002 token-economy and English-output mechanical evidence artifact."
		object["required"] = []string{"schema_version", "run_id", "task_id", "task_class", "scope", "compact_output_policy", "artifact_first_detail", "agent_instruction_evidence", "final_report_evidence", "kas_lifecycle_evidence", "mutation_approval_evidence"}
		statusProperty := map[string]any{"enum": []string{GateStatusPass, GateStatusNotApplicable}}
		refProperty := map[string]any{
			"type":     "object",
			"required": []string{"path"},
			"properties": map[string]any{
				"path":     map[string]any{"type": "string", "minLength": 1},
				"checksum": map[string]any{"type": "string", "pattern": "^sha256:[0-9a-fA-F]{64}$"},
				"markers":  map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}},
			},
			"additionalProperties": true,
		}
		sectionProperty := map[string]any{
			"type":     "object",
			"required": []string{"status"},
			"properties": map[string]any{
				"status":        statusProperty,
				"reason":        map[string]any{"type": "string"},
				"evidence_refs": map[string]any{"type": "array", "items": refProperty},
				"detail_ref":    refProperty,
			},
			"additionalProperties": true,
		}
		object["properties"] = map[string]any{
			"schema_version":             map[string]any{"const": tokenEconomySchemaVersion},
			"run_id":                     map[string]any{"type": "string", "pattern": "^run-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$"},
			"task_id":                    map[string]any{"const": tokenEconomyTaskID},
			"task_class":                 map[string]any{"type": "string", "minLength": 1},
			"scope":                      sectionProperty,
			"compact_output_policy":      sectionProperty,
			"artifact_first_detail":      sectionProperty,
			"agent_instruction_evidence": sectionProperty,
			"final_report_evidence":      sectionProperty,
			"kas_lifecycle_evidence":     map[string]any{"type": "object"},
			"mutation_approval_evidence": map[string]any{"type": "object"},
		}
	case SchemaMultiAgentReviewEvidence:
		object["description"] = "KAS MAR role-first review coverage and provider-attempt evidence artifact."
		object["required"] = []string{"schema_version", "run_id", "task_id", "status", "reason", "coverage", "provider_attempts", "blue_disposition_ref"}
		refProperty := map[string]any{
			"type":     "object",
			"required": []string{"path"},
			"properties": map[string]any{
				"path":     map[string]any{"type": "string", "minLength": 1},
				"checksum": map[string]any{"type": "string", "pattern": "^sha256:[0-9a-fA-F]{64}$"},
				"markers":  map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}},
			},
			"additionalProperties": true,
		}
		attemptProperty := map[string]any{
			"type":     "object",
			"required": []string{"schema_version", "run_id", "task_id", "attempt_id", "role_id", "provider_id", "provider_candidate", "terminal_status"},
			"properties": map[string]any{
				"schema_version":          map[string]any{"const": "mar.provider_attempt.v1"},
				"run_id":                  map[string]any{"type": "string", "pattern": "^run-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$"},
				"task_id":                 map[string]any{"type": "string", "minLength": 1},
				"attempt_id":              map[string]any{"type": "string", "minLength": 1},
				"role_id":                 map[string]any{"type": "string", "minLength": 1},
				"provider_id":             map[string]any{"type": "string", "minLength": 1},
				"provider_candidate":      map[string]any{"enum": multiAgentReviewProviderCandidates},
				"terminal_status":         map[string]any{"enum": multiAgentReviewStatuses},
				"parser_status":           map[string]any{"type": "string"},
				"provider_failure_reason": map[string]any{"type": "string"},
				"raw_output_path":         map[string]any{"type": "string"},
				"parsed_finding_path":     map[string]any{"type": "string"},
				"mutation_check":          map[string]any{"type": "object"},
			},
			"additionalProperties": true,
		}
		object["properties"] = map[string]any{
			"schema_version": map[string]any{"const": multiAgentReviewSchemaVersion},
			"run_id":         map[string]any{"type": "string", "pattern": "^run-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$"},
			"task_id":        map[string]any{"type": "string", "minLength": 1},
			"status":         map[string]any{"enum": multiAgentReviewStatuses},
			"reason":         map[string]any{"type": "string", "minLength": 1},
			"coverage": map[string]any{"type": "object", "required": []string{"required_roles", "covered_roles", "by_role"}, "properties": map[string]any{
				"required_roles":            map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}},
				"observed_roles":            map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"covered_roles":             map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"minimum_met":               map[string]any{"type": "boolean"},
				"unresolved_required_roles": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
				"by_role":                   map[string]any{"type": "object"},
			}, "additionalProperties": true},
			"provider_attempts":      map[string]any{"type": "array", "items": attemptProperty},
			"blue_disposition_ref":   refProperty,
			"red_adjudication_ref":   refProperty,
			"alternate_approval_ref": refProperty,
			"waiver_ref":             refProperty,
			"premium_approval_ref":   refProperty,
			"premium_review_used":    map[string]any{"type": "boolean"},
			"blue_reason":            map[string]any{"type": "string"},
		}
	case SchemaPolicyPromotionEvidence:
		object["description"] = "POLPR-007 policy-promotion helper evidence artifact; KAH validates deterministic evidence presence and shape only."
		object["required"] = policyPromotionRequiredFields
		statusProperty := map[string]any{"enum": []string{GateStatusPass, GateStatusFail, GateStatusNotApplicable}}
		refProperty := map[string]any{"type": "object", "required": []string{"path"}, "properties": map[string]any{"path": map[string]any{"type": "string", "minLength": 1}, "checksum": map[string]any{"type": "string", "pattern": "^sha256:[0-9a-fA-F]{64}$"}, "markers": map[string]any{"type": "array", "items": map[string]any{"type": "string", "minLength": 1}}}, "additionalProperties": true}
		sectionProperty := map[string]any{"type": "object", "required": []string{"status"}, "properties": map[string]any{"status": statusProperty, "reason": map[string]any{"type": "string"}, "evidence_refs": map[string]any{"type": "array", "items": refProperty}, "detail_ref": refProperty}, "additionalProperties": true}
		object["properties"] = map[string]any{
			"schema_version":               map[string]any{"const": policyPromotionSchemaVersion},
			"run_id":                       map[string]any{"type": "string", "pattern": "^run-[0-9]{8}T[0-9]{6}Z-[0-9a-f]{12}$"},
			"task_id":                      map[string]any{"const": policyPromotionTaskID},
			"task_class":                   map[string]any{"type": "string", "minLength": 1},
			"scope":                        sectionProperty,
			"document_impact_map":          sectionProperty,
			"project_gray_coverage":        sectionProperty,
			"test_layer_evidence":          sectionProperty,
			"failed_test_repair_ownership": sectionProperty,
			"final_stale_status_check":     sectionProperty,
			"boundary_evidence":            sectionProperty,
			"mutation_approval_evidence":   sectionProperty,
		}
	}
	if _, ok := object["properties"]; !ok {
		object["properties"] = map[string]any{"version": map[string]any{"type": "string"}}
	}
	object["additionalProperties"] = true
	return object
}

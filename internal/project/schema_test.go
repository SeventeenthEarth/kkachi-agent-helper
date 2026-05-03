package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestSchemaValidateInitializedProjectFiles(t *testing.T) {
	repo, root, result := initSchemaTestProject(t)

	cases := []struct {
		name   string
		file   string
		schema string
	}{
		{name: "config", file: ConfigPath, schema: SchemaConfig},
		{name: "status", file: StatusPath, schema: SchemaStatus},
		{name: "events", file: EventsPath, schema: SchemaEvent},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			validated, err := ValidateSchemaFile(root, SchemaValidateOptions{File: tt.file, Schema: tt.schema})
			if err != nil {
				t.Fatalf("ValidateSchemaFile() error = %v", err)
			}
			if validated.Status != "pass" || validated.FilePath != tt.file || validated.Schema != tt.schema {
				t.Fatalf("validated = %#v, want pass for %s", validated, tt.name)
			}
		})
	}

	created, err := CreateRun(root, CreateRunOptions{
		Title:         "Schema validation run",
		WorkPath:      "A_development_execution",
		WorkMode:      "standard",
		Urgency:       "normal",
		SOTPolicy:     "existing_sot_basis",
		ExecutionMode: "adapter_qa",
		Commander:     "Gongmyeong",
		TaskID:        "packg-001",
		Now:           deterministicInitOptions().Now,
		RandomHex:     deterministicInitOptions().RandomHex,
	})
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	validArtifacts := map[string][]byte{
		".kkachi/selected-cli.json":            []byte(`{"version":"0.1","status":"supported","backend_type":"codex","adapter_type":"openai-codex","source_ledger_ref":"docs/ledger.md#codex","caveats":[]}` + "\n"),
		".kkachi/bridge-session-snapshot.json": []byte(`{"session_id":"session-123","backend_type":"codex","adapter_type":"openai-codex","state":"running","lifecycle_class":"interactive","open_pendings":0}` + "\n"),
	}
	for relative, content := range validArtifacts {
		if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(relative)), content, 0o600); err != nil {
			t.Fatalf("write %s: %v", relative, err)
		}
	}
	moreCases := []struct {
		name   string
		file   string
		schema string
	}{
		{name: "run metadata", file: ".kkachi/runs/" + created.Metadata.RunID + "/run-metadata.json", schema: SchemaRunMetadata},
		{name: "selected cli", file: ".kkachi/selected-cli.json", schema: SchemaSelectedCLI},
		{name: "bridge snapshot", file: ".kkachi/bridge-session-snapshot.json", schema: SchemaBridgeSessionSnapshot},
	}
	for _, tt := range moreCases {
		t.Run(tt.name, func(t *testing.T) {
			validated, err := ValidateSchemaFile(root, SchemaValidateOptions{File: tt.file, Schema: tt.schema})
			if err != nil {
				t.Fatalf("ValidateSchemaFile() error = %v", err)
			}
			if validated.Status != "pass" {
				t.Fatalf("validated = %#v, want pass", validated)
			}
		})
	}
	if len(result.SchemaPaths) != 7 || !containsString(result.SchemaPaths, ".kkachi/schemas/config.schema.json") || !containsString(result.SchemaPaths, ".kkachi/schemas/install-manifest.schema.json") {
		t.Fatalf("schema paths = %#v, want seven canonical schema paths including config and install-manifest", result.SchemaPaths)
	}
}

func TestSchemaRegistryContracts(t *testing.T) {
	names := CanonicalSchemaNames()
	if len(names) != 7 {
		t.Fatalf("CanonicalSchemaNames() = %#v, want seven schemas", names)
	}
	names[0] = "mutated"
	if CanonicalSchemaNames()[0] == "mutated" {
		t.Fatalf("CanonicalSchemaNames() returned mutable backing slice")
	}

	for _, name := range CanonicalSchemaNames() {
		t.Run(name, func(t *testing.T) {
			object := schemaObject(name)
			if object["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
				t.Fatalf("$schema = %#v, want draft 2020-12", object["$schema"])
			}
			if object["$id"] != "https://kkachi.local/schemas/"+name+".schema.json" {
				t.Fatalf("$id = %#v, want canonical schema id", object["$id"])
			}
			if object["version"] != SchemaVersion {
				t.Fatalf("version = %#v, want %s", object["version"], SchemaVersion)
			}
			if object["type"] != "object" {
				t.Fatalf("type = %#v, want object", object["type"])
			}
			if !schemaRequiresField(object, "version") {
				t.Fatalf("required = %#v, want version required", object["required"])
			}
			properties, ok := object["properties"].(map[string]any)
			if !ok || len(properties) == 0 {
				t.Fatalf("properties = %#v, want non-empty object", object["properties"])
			}
			if object["additionalProperties"] != true {
				t.Fatalf("additionalProperties = %#v, want true", object["additionalProperties"])
			}
		})
	}
}

func TestSchemaValidateRejectsMalformedStatusAndBackendEvidence(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)

	badStatus := filepath.Join(repo, ".kkachi", "bad-status.json")
	if err := os.WriteFile(badStatus, []byte(`{"version":"0.1","project_id":"p","active_run_id":null,"active_run_state":null,"last_event_id":"evt-bad","updated_at":"2026-04-30T01:02:03Z","gate_summary":{}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write bad status: %v", err)
	}
	validated, err := ValidateSchemaFile(root, SchemaValidateOptions{File: ".kkachi/bad-status.json", Schema: ".kkachi/schemas/status.schema.json"})
	if err != nil {
		t.Fatalf("ValidateSchemaFile() error = %v", err)
	}
	if validated.Status != "fail" || !schemaTestCheck(validated.Checks, "last_event_id", "fail") {
		t.Fatalf("validated = %#v, want last_event_id failure", validated)
	}

	badSelected := filepath.Join(repo, ".kkachi", "selected-cli.json")
	if err := os.WriteFile(badSelected, []byte(`{"version":"0.1","status":"supported","backend_type":"codex","adapter_type":"openai-codex","source_ledger_ref":"docs/ledger.md#codex","caveats":"none"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write bad selected cli: %v", err)
	}
	validated, err = ValidateSchemaFile(root, SchemaValidateOptions{File: ".kkachi/selected-cli.json", Schema: SchemaSelectedCLI})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(selected-cli) error = %v", err)
	}
	if validated.Status != "fail" || !schemaTestCheck(validated.Checks, "caveats", "fail") {
		t.Fatalf("validated = %#v, want caveats failure", validated)
	}
}

func TestSchemaExportDryRunAndRealExport(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	statusSchema := filepath.Join(repo, ".kkachi", "schemas", "status.schema.json")
	if err := os.WriteFile(statusSchema, []byte(`{"$id":"https://kkachi.local/schemas/status.schema.json","required":["version"]}`+"\n"), 0o600); err != nil {
		t.Fatalf("write older schema: %v", err)
	}

	dryRun, err := ExportSchemas(root, SchemaExportOptions{Schema: SchemaStatus, DryRun: true, Now: deterministicInitOptions().Now})
	if err != nil {
		t.Fatalf("ExportSchemas(dry-run) error = %v", err)
	}
	if !dryRun.DryRun || len(dryRun.WouldWrite) != 1 || dryRun.EventID != "" {
		t.Fatalf("dryRun = %#v, want would-write without event", dryRun)
	}
	if !strings.Contains(readText(t, statusSchema), `"required":["version"]`) {
		t.Fatalf("dry-run modified schema unexpectedly")
	}

	exported, err := ExportSchemas(root, SchemaExportOptions{Schema: SchemaStatus, Now: deterministicInitOptions().Now})
	if err != nil {
		t.Fatalf("ExportSchemas() error = %v", err)
	}
	if exported.EventID != "evt-000002" || len(exported.Written) != 1 {
		t.Fatalf("exported = %#v, want written schema and evt-000002", exported)
	}
	var schema map[string]any
	readJSONFile(t, statusSchema, &schema)
	if schema["version"] != SchemaVersion || !schemaRequiresField(schema, "version") {
		t.Fatalf("schema = %#v, want full canonical schema", schema)
	}
	validated, err := ValidateSchemaFile(root, SchemaValidateOptions{File: StatusPath, Schema: ".kkachi/schemas/status.schema.json"})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(local schema) error = %v", err)
	}
	if validated.Status != "pass" {
		data, _ := json.MarshalIndent(validated, "", "  ")
		t.Fatalf("validated = %s, want pass", data)
	}
}

func TestSchemaExportAllIdempotentAndConflictFailures(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)

	unchanged, err := ExportSchemas(root, SchemaExportOptions{All: true, Now: deterministicInitOptions().Now})
	if err != nil {
		t.Fatalf("ExportSchemas(all unchanged) error = %v", err)
	}
	if len(unchanged.Schemas) != 7 || len(unchanged.Unchanged) != 7 || len(unchanged.Written) != 0 || unchanged.EventID != "" {
		t.Fatalf("unchanged = %#v, want all schemas unchanged without event", unchanged)
	}

	freshLock := LockMetadata{Version: LockVersion, LockName: ProjectWriteLockName, OwnerPID: os.Getpid(), Hostname: mustHostname(t), Command: "fresh schema export", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	writeLockMetadata(t, repo, ProjectWriteLockName, freshLock)
	_, err = ExportSchemas(root, SchemaExportOptions{Schema: SchemaStatus})
	assertProblemCode(t, err, "lock_conflict")
	if err := os.Remove(filepath.Join(repo, ".kkachi", "project_write.lock")); err != nil {
		t.Fatalf("remove project write lock: %v", err)
	}

	configSchema := filepath.Join(repo, ".kkachi", "schemas", "config.schema.json")
	if err := os.Remove(configSchema); err != nil {
		t.Fatalf("remove config schema: %v", err)
	}
	if err := os.Mkdir(configSchema, 0o755); err != nil {
		t.Fatalf("mkdir config schema conflict: %v", err)
	}
	_, err = ExportSchemas(root, SchemaExportOptions{Schema: SchemaConfig})
	assertProblemCode(t, err, "schema_export_conflict")
}

func TestSchemaValidateRejectsBadSchemaReferences(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	badRef := filepath.Join(repo, ".kkachi", "schemas", "status.schema.json")
	if err := os.WriteFile(badRef, []byte(`{"$id":"https://kkachi.local/schemas/event.schema.json","version":"0.1"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write bad schema ref: %v", err)
	}
	_, err := ValidateSchemaFile(root, SchemaValidateOptions{File: StatusPath, Schema: ".kkachi/schemas/status.schema.json"})
	assertProblemCode(t, err, "schema_reference_invalid")

	_, err = ValidateSchemaFile(root, SchemaValidateOptions{File: StatusPath, Schema: "unknown"})
	assertProblemCode(t, err, "schema_unknown")
}

func TestSchemaValidateRejectsSymlinkEscapes(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "status.json"), []byte(`{"version":"0.1"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write outside status: %v", err)
	}
	escapeFile := filepath.Join(repo, ".kkachi", "escape-status.json")
	if err := os.Symlink(filepath.Join(outside, "status.json"), escapeFile); err != nil {
		t.Fatalf("create escaping file symlink: %v", err)
	}

	_, err := ValidateSchemaFile(root, SchemaValidateOptions{File: ".kkachi/escape-status.json", Schema: SchemaStatus})
	assertProblemCode(t, err, "symlink_escape")

	escapeSchemaDir := filepath.Join(repo, ".kkachi", "schema-escape")
	if err := os.Symlink(outside, escapeSchemaDir); err != nil {
		t.Fatalf("create escaping schema symlink: %v", err)
	}

	_, err = ValidateSchemaFile(root, SchemaValidateOptions{File: StatusPath, Schema: ".kkachi/schema-escape/status.schema.json"})
	assertProblemCode(t, err, "symlink_escape")
}

func initSchemaTestProject(t *testing.T) (string, Root, InitResult) {
	t.Helper()
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	result, err := InitProject(root, deterministicInitOptions())
	if err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}
	return repo, root, result
}

func schemaTestCheck(checks []SchemaCheck, name, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func schemaRequiresField(object map[string]any, field string) bool {
	switch required := object["required"].(type) {
	case []string:
		for _, value := range required {
			if value == field {
				return true
			}
		}
	case []any:
		for _, value := range required {
			if value == field {
				return true
			}
		}
	}
	return false
}

func TestSchemaMigrationDryRunIsReadOnlyAndUnknownSourceFails(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	beforeEvents := readSchemaTestText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))
	beforeStatus := readSchemaTestText(t, filepath.Join(repo, ".kkachi", "status.json"))

	result, err := MigrateSchemaState(root, SchemaMigrationOptions{From: SchemaVersion, To: SchemaVersion, DryRun: true, Now: deterministicInitOptions().Now})
	if err != nil {
		t.Fatalf("MigrateSchemaState(dry-run) error = %v", err)
	}
	if !result.DryRun || result.Status != "pass" || result.EventID != "" || result.BackupPath != "" || len(result.BackedUp) != 0 || len(result.WouldBackup) == 0 || len(result.Unchanged) == 0 {
		t.Fatalf("dry-run result = %#v, want read-only summary", result)
	}
	if got := readSchemaTestText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); got != beforeEvents {
		t.Fatalf("events changed on dry-run\nbefore=%s\nafter=%s", beforeEvents, got)
	}
	if got := readSchemaTestText(t, filepath.Join(repo, ".kkachi", "status.json")); got != beforeStatus {
		t.Fatalf("status changed on dry-run\nbefore=%s\nafter=%s", beforeStatus, got)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "backups")); !os.IsNotExist(err) {
		t.Fatalf("backup dir exists after dry-run: %v", err)
	}

	_, err = MigrateSchemaState(root, SchemaMigrationOptions{From: "9.9", To: SchemaVersion})
	assertProblemCode(t, err, "schema_migration_unknown_source_version")
}

func TestSchemaMigrationNoopCreatesBackupAndEvent(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	result, err := MigrateSchemaState(root, SchemaMigrationOptions{From: SchemaVersion, To: SchemaVersion, Now: deterministicInitOptions().Now})
	if err != nil {
		t.Fatalf("MigrateSchemaState() error = %v", err)
	}
	if result.DryRun || result.Status != "pass" || result.EventID != "evt-000002" || result.Migration == "" || result.BackupPath == "" || len(result.BackedUp) == 0 || len(result.Migrated) != 0 {
		t.Fatalf("result = %#v, want no-op backup and event", result)
	}
	for _, relative := range []string{ConfigPath, StatusPath, EventsPath, ".kkachi/schemas/status.schema.json"} {
		backup := filepath.Join(repo, filepath.FromSlash(result.BackupPath), filepath.FromSlash(relative))
		if _, err := os.Stat(backup); err != nil {
			t.Fatalf("backup %s missing: %v", backup, err)
		}
	}
	events := readSchemaTestText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))
	if !strings.Contains(events, `"type":"schema.migrated"`) || !strings.Contains(events, `"backup_path":"`+result.BackupPath+`"`) {
		t.Fatalf("events missing schema.migrated backup evidence: %s", events)
	}
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["last_event_id"] != result.EventID {
		t.Fatalf("status last_event_id = %v, want %s", status["last_event_id"], result.EventID)
	}
}

func TestSchemaMigrationRejectsStateVersionMismatchAndLockConflict(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	statusPath := filepath.Join(repo, ".kkachi", "status.json")
	var status map[string]any
	readJSONFile(t, statusPath, &status)
	status["version"] = "0.2"
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}
	_, err = MigrateSchemaState(root, SchemaMigrationOptions{From: SchemaVersion, To: SchemaVersion, DryRun: true})
	assertProblemCode(t, err, "schema_migration_source_version_mismatch")

	status["version"] = SchemaVersion
	data, _ = json.MarshalIndent(status, "", "  ")
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("restore status: %v", err)
	}
	freshLock := LockMetadata{Version: LockVersion, LockName: ProjectWriteLockName, OwnerPID: os.Getpid(), Hostname: mustHostname(t), Command: "fresh schema migrate", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	writeLockMetadata(t, repo, ProjectWriteLockName, freshLock)
	_, err = MigrateSchemaState(root, SchemaMigrationOptions{From: SchemaVersion, To: SchemaVersion})
	assertProblemCode(t, err, "lock_conflict")
}

func TestSchemaMigrationRejectsStatusEventTailMismatch(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	eventsPath := filepath.Join(repo, ".kkachi", "events.jsonl")
	file, err := os.OpenFile(eventsPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open events: %v", err)
	}
	if _, err := file.WriteString(`{"version":"0.1","event_id":"evt-000002","occurred_at":"2026-04-30T02:00:00Z","run_id":null,"type":"schema.migrated","actor":"helper","payload":{}}` + "\n"); err != nil {
		t.Fatalf("append incoherent event: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close events: %v", err)
	}

	_, err = MigrateSchemaState(root, SchemaMigrationOptions{From: SchemaVersion, To: SchemaVersion, DryRun: true})
	assertProblemCode(t, err, "last_event_id_mismatch")
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "backups")); !os.IsNotExist(err) {
		t.Fatalf("backup dir exists after incoherent migration refusal: %v", err)
	}
}

func readSchemaTestText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

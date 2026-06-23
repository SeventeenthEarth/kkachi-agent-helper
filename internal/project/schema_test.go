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
	if len(result.SchemaPaths) != len(canonicalSchemaNames) || !containsString(result.SchemaPaths, ".kkachi/schemas/config.schema.json") || !containsString(result.SchemaPaths, ".kkachi/schemas/bridge-session-snapshot.schema.json") || !containsString(result.SchemaPaths, ".kkachi/schemas/token-economy-evidence.schema.json") || !containsString(result.SchemaPaths, ".kkachi/schemas/multi-agent-review-evidence.schema.json") || !containsString(result.SchemaPaths, ".kkachi/schemas/policy-promotion-evidence.schema.json") || !containsString(result.SchemaPaths, ".kkachi/schemas/design-evidence.schema.json") {
		t.Fatalf("schema paths = %#v, want canonical schema paths including config, bridge-session-snapshot, token-economy-evidence, multi-agent-review-evidence, policy-promotion-evidence, and design-evidence", result.SchemaPaths)
	}
}

func TestSchemaRegistryContracts(t *testing.T) {
	names := CanonicalSchemaNames()
	if len(names) != 10 {
		t.Fatalf("CanonicalSchemaNames() = %#v, want ten schemas", names)
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
			requiredVersionField := "version"
			if name == SchemaTokenEconomyEvidence || name == SchemaMultiAgentReviewEvidence || name == SchemaPolicyPromotionEvidence || name == SchemaDesignEvidence {
				requiredVersionField = "schema_version"
			}
			if !schemaRequiresField(object, requiredVersionField) {
				t.Fatalf("required = %#v, want %s required", object["required"], requiredVersionField)
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

func TestRunMetadataSchemaAcceptsOptionalWorkflowManagedMarkers(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	metadata.WorkflowManaged = true
	metadata.StrictWorkflowOrder = true
	metadata.SelectedWorkflowID = optionalTrimmedString("development_full")
	metadata.WorkflowSource = optionalTrimmedString(".kkachi/runs/" + created.Metadata.RunID + "/workflow/workflow.yaml")
	writeRunMetadataForTest(t, repo, metadata)

	validated, err := ValidateSchemaFile(root, SchemaValidateOptions{File: ".kkachi/runs/" + created.Metadata.RunID + "/run-metadata.json", Schema: SchemaRunMetadata})
	if err != nil {
		t.Fatalf("ValidateSchemaFile() error = %v", err)
	}
	if validated.Status != "pass" || !schemaTestCheck(validated.Checks, "workflow_managed", "pass") || !schemaTestCheck(validated.Checks, "strict_workflow_order", "pass") || !schemaTestCheck(validated.Checks, "selected_workflow_id", "pass") || !schemaTestCheck(validated.Checks, "workflow_source", "pass") {
		t.Fatalf("validated = %#v, want workflow marker schema checks to pass", validated)
	}

	object := schemaObject(SchemaRunMetadata)
	properties := object["properties"].(map[string]any)
	for _, field := range []string{"workflow_managed", "strict_workflow_order", "selected_workflow_id", "workflow_source"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("run metadata schema missing property %s", field)
		}
	}
}

func TestPolicyPromotionSchemaObjectDeclaresRequiredSurface(t *testing.T) {
	object := schemaObject(SchemaPolicyPromotionEvidence)
	for _, field := range policyPromotionRequiredFields {
		if !schemaRequiresField(object, field) {
			t.Fatalf("policy promotion schema required = %#v, missing %s", object["required"], field)
		}
	}
	properties, ok := object["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v, want object", object["properties"])
	}
	for _, field := range []string{"schema_version", "task_id", "project_gray_coverage", "test_layer_evidence", "boundary_evidence", "mutation_approval_evidence"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("policy promotion schema properties missing %s: %#v", field, properties)
		}
	}
}

func TestDesignEvidenceSchemaObjectDeclaresRequiredSurface(t *testing.T) {
	object := schemaObject(SchemaDesignEvidence)
	for _, field := range designEvidenceRequiredFields {
		if !schemaRequiresField(object, field) {
			t.Fatalf("design evidence schema required = %#v, missing %s", object["required"], field)
		}
	}
	properties, ok := object["properties"].(map[string]any)
	if !ok {
		t.Fatalf("properties = %#v, want object", object["properties"])
	}
	for _, field := range []string{"schema_version", "teal_applicability", "design_plan_evidence", "design_fidelity_evidence", "color_review_evidence", "boundary_evidence"} {
		if _, ok := properties[field]; !ok {
			t.Fatalf("design evidence schema properties missing %s: %#v", field, properties)
		}
	}
	tealApplicability, ok := properties["teal_applicability"].(map[string]any)
	if !ok {
		t.Fatalf("teal_applicability property = %#v, want object schema", properties["teal_applicability"])
	}
	for _, field := range []string{"project_has_teal_lane", "ui_ux_change", "teal_required", "derivation", "ui_ux_classification_owner", "teal_skip_reason", "teal_owner", "teal_waiver_approved", "teal_waiver_approval_ref", "teal_waiver_scope", "teal_waiver_expires_at", "required_when_teal_required", "missing_required_status"} {
		if !schemaRequiresField(tealApplicability, field) {
			t.Fatalf("teal_applicability required = %#v, missing %s", tealApplicability["required"], field)
		}
	}
	tealProperties, ok := tealApplicability["properties"].(map[string]any)
	if !ok {
		t.Fatalf("teal_applicability properties = %#v, want object", tealApplicability["properties"])
	}
	for _, field := range []string{"project_has_teal_lane", "ui_ux_change", "teal_required", "teal_waiver_approved"} {
		property, ok := tealProperties[field].(map[string]any)
		if !ok || property["type"] != "boolean" {
			t.Fatalf("teal_applicability.%s property = %#v, want boolean", field, tealProperties[field])
		}
	}
	derivation, ok := tealProperties["derivation"].(map[string]any)
	if !ok || derivation["const"] != "project_has_teal_lane && ui_ux_change" {
		t.Fatalf("teal_applicability.derivation property = %#v, want derivation const", tealProperties["derivation"])
	}
	for _, field := range []string{"required_when_teal_required"} {
		property, ok := tealProperties[field].(map[string]any)
		if !ok || property["type"] != "array" {
			t.Fatalf("teal_applicability.%s property = %#v, want string array", field, tealProperties[field])
		}
	}
	boundary, ok := properties["boundary_evidence"].(map[string]any)
	if !ok {
		t.Fatalf("boundary_evidence property = %#v, want object schema", properties["boundary_evidence"])
	}
	for _, field := range []string{"status", "policy_owner", "kah_validation_role"} {
		if !schemaRequiresField(boundary, field) {
			t.Fatalf("boundary_evidence required = %#v, missing %s", boundary["required"], field)
		}
	}
	boundaryProperties, ok := boundary["properties"].(map[string]any)
	if !ok {
		t.Fatalf("boundary_evidence properties = %#v, want object", boundary["properties"])
	}
	policyOwner, ok := boundaryProperties["policy_owner"].(map[string]any)
	if !ok || policyOwner["const"] != "KAS" {
		t.Fatalf("boundary_evidence.policy_owner property = %#v, want KAS const", boundaryProperties["policy_owner"])
	}
	validationRole, ok := boundaryProperties["kah_validation_role"].(map[string]any)
	if !ok || validationRole["const"] != "deterministic_shape_only" {
		t.Fatalf("boundary_evidence.kah_validation_role property = %#v, want deterministic role const", boundaryProperties["kah_validation_role"])
	}
}

func TestSchemaValidateDesignEvidenceRelativePathShapeRejectsUnsafeBranches(t *testing.T) {
	rejected := map[string]string{
		"empty":               "",
		"whitespace":          "   ",
		"absolute":            "/tmp/evidence.md",
		"dot":                 ".",
		"dotdot":              "..",
		"dotdot prefix":       "../escape.md",
		"dotdot infix":        "safe/../escape.md",
		"backslash traversal": `foo\..\..\bar`,
	}
	for name, value := range rejected {
		t.Run(name, func(t *testing.T) {
			if designRelativePathShape(value) {
				t.Fatalf("designRelativePathShape(%q) = true, want false", value)
			}
		})
	}

	accepted := []string{
		".kkachi/runs/run-20260622T082204Z-8aa46fa42bff/plan.md",
		"docs/sot/teal-ui-evidence-gates.md",
	}
	for _, value := range accepted {
		t.Run("accept "+value, func(t *testing.T) {
			if !designRelativePathShape(value) {
				t.Fatalf("designRelativePathShape(%q) = false, want true", value)
			}
		})
	}
}

func TestSchemaValidateDesignEvidence(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	runID := "run-20260622T082204Z-8aa46fa42bff"
	path := filepath.Join(repo, ".kkachi", "runs", runID, designEvidenceArtifact)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir design evidence dir: %v", err)
	}

	writeDesignEvidence(t, path, validDesignEvidence(runID, true))
	validated, err := ValidateSchemaFile(root, SchemaValidateOptions{File: tokenRunPath(runID, designEvidenceArtifact), Schema: SchemaDesignEvidence})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(design-evidence teal-required) error = %v", err)
	}
	if validated.Status != "pass" || !schemaTestCheck(validated.Checks, "teal_applicability.teal_required", "pass") || !schemaTestCheck(validated.Checks, "design_plan_evidence.status", "pass") {
		t.Fatalf("validated = %#v, want Teal-required design evidence schema pass", validated)
	}

	writeDesignEvidence(t, path, validDesignEvidence(runID, false))
	skipped, err := ValidateSchemaFile(root, SchemaValidateOptions{File: tokenRunPath(runID, designEvidenceArtifact), Schema: ".kkachi/schemas/design-evidence.schema.json"})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(design-evidence non-UI skip) error = %v", err)
	}
	if skipped.Status != "pass" || !schemaTestCheck(skipped.Checks, "teal_applicability.teal_required", "pass") || !schemaTestCheck(skipped.Checks, "design_plan_evidence.status", "pass") {
		t.Fatalf("skipped = %#v, want non-UI skip design evidence schema pass", skipped)
	}

	tests := []struct {
		name  string
		setup func(map[string]any)
		want  string
	}{
		{
			name: "missing skip reason",
			setup: func(payload map[string]any) {
				payload["teal_applicability"].(map[string]any)["teal_skip_reason"] = ""
			},
			want: "teal_applicability.teal_skip_reason",
		},
		{
			name: "invalid derivation",
			setup: func(payload map[string]any) {
				payload["teal_applicability"].(map[string]any)["project_has_teal_lane"] = true
				payload["teal_applicability"].(map[string]any)["ui_ux_change"] = true
				payload["teal_applicability"].(map[string]any)["teal_required"] = false
			},
			want: "teal_applicability.teal_required",
		},
		{
			name: "malformed ref",
			setup: func(payload map[string]any) {
				payload["design_plan_evidence"].(map[string]any)["detail_ref"] = map[string]any{"path": "../escape.md", "checksum": "sha256:bad"}
			},
			want: "design_plan_evidence.detail_ref.path",
		},
		{
			name: "backslash traversal ref",
			setup: func(payload map[string]any) {
				payload["design_plan_evidence"].(map[string]any)["detail_ref"] = map[string]any{"path": `foo\..\..\bar`}
			},
			want: "design_plan_evidence.detail_ref.path",
		},
		{
			name: "invalid boundary policy owner",
			setup: func(payload map[string]any) {
				payload["boundary_evidence"].(map[string]any)["policy_owner"] = "KAH"
			},
			want: "boundary_evidence.policy_owner",
		},
		{
			name: "invalid boundary validation role",
			setup: func(payload map[string]any) {
				payload["boundary_evidence"].(map[string]any)["kah_validation_role"] = "ui_classification"
			},
			want: "boundary_evidence.kah_validation_role",
		},
		{
			name: "missing required section",
			setup: func(payload map[string]any) {
				delete(payload, "design_plan_evidence")
			},
			want: "design_plan_evidence",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			payload := validDesignEvidence(runID, false)
			tt.setup(payload)
			writeDesignEvidence(t, path, payload)
			failed, err := ValidateSchemaFile(root, SchemaValidateOptions{File: tokenRunPath(runID, designEvidenceArtifact), Schema: SchemaDesignEvidence})
			if err != nil {
				t.Fatalf("ValidateSchemaFile(design-evidence negative) error = %v", err)
			}
			if failed.Status != "fail" || !schemaTestCheck(failed.Checks, tt.want, "fail") {
				t.Fatalf("failed = %#v, want fail check %s", failed, tt.want)
			}
		})
	}
}

func TestSchemaValidateTokenEconomyEvidence(t *testing.T) {
	repo, root, runID := tokenEconomyRunWithArtifacts(t)
	writeTokenEconomyReferencedArtifacts(t, repo, runID)
	writeTokenEconomyEvidence(t, repo, runID, validTokenEconomyEvidence(t, repo, runID))

	validated, err := ValidateSchemaFile(root, SchemaValidateOptions{File: tokenRunPath(runID, tokenEconomyArtifact), Schema: SchemaTokenEconomyEvidence})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(token-economy) error = %v", err)
	}
	if validated.Status != "pass" || !schemaTestCheck(validated.Checks, "schema_version", "pass") || !schemaTestCheck(validated.Checks, "mutation_approval_evidence.status", "pass") {
		t.Fatalf("validated = %#v, want token-economy schema pass", validated)
	}

	repo2, root2, runID2 := token002RunWithArtifacts(t)
	writeToken002ReferencedArtifacts(t, repo2, runID2)
	writeTokenEconomyEvidence(t, repo2, runID2, validToken002EconomyEvidence(t, repo2, runID2))
	token002Validated, err := ValidateSchemaFile(root2, SchemaValidateOptions{File: tokenRunPath(runID2, tokenEconomyArtifact), Schema: SchemaTokenEconomyEvidence})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(token-economy token-002) error = %v", err)
	}
	if token002Validated.Status != "pass" || !schemaTestCheck(token002Validated.Checks, "verification_profile_evidence.status", "pass") || !schemaTestCheck(token002Validated.Checks, "change_verification_matrix_evidence.status", "pass") {
		t.Fatalf("token002Validated = %#v, want token-002 schema pass", token002Validated)
	}

	writeTokenEconomyEvidence(t, repo, runID, notApplicableTokenEconomyEvidence(runID, false))
	failed, err := ValidateSchemaFile(root, SchemaValidateOptions{File: tokenRunPath(runID, tokenEconomyArtifact), Schema: ".kkachi/schemas/token-economy-evidence.schema.json"})
	if err != nil {
		t.Fatalf("ValidateSchemaFile(token-economy local schema) error = %v", err)
	}
	if failed.Status != "fail" || !schemaTestCheck(failed.Checks, "scope.reason", "fail") {
		t.Fatalf("failed = %#v, want missing not_applicable reason failure", failed)
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
	if len(unchanged.Schemas) != len(canonicalSchemaNames) || len(unchanged.Unchanged) != len(canonicalSchemaNames) || len(unchanged.Written) != 0 || unchanged.EventID != "" {
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

func writeDesignEvidence(t *testing.T, path string, payload map[string]any) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal design evidence: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write design evidence: %v", err)
	}
}

func validDesignEvidence(runID string, tealRequired bool) map[string]any {
	applicability := map[string]any{
		"project_has_teal_lane":       tealRequired,
		"ui_ux_change":                tealRequired,
		"teal_required":               tealRequired,
		"derivation":                  "project_has_teal_lane && ui_ux_change",
		"ui_ux_classification_owner":  "KAS workflow-route",
		"teal_skip_reason":            nil,
		"teal_owner":                  "teal_reviewer",
		"teal_waiver_approved":        false,
		"teal_waiver_approval_ref":    "",
		"teal_waiver_scope":           "",
		"teal_waiver_expires_at":      "",
		"required_when_teal_required": []string{"DESIGN_PLAN_GATE", "DESIGN_FIDELITY_REVIEW"},
		"missing_required_status":     "required_teal_verdict_missing",
	}
	sectionStatus := GateStatusPass
	sectionReason := ""
	if !tealRequired {
		applicability["project_has_teal_lane"] = false
		applicability["ui_ux_change"] = false
		applicability["teal_required"] = false
		applicability["teal_skip_reason"] = "No UI/UX surface in this project/task."
		applicability["teal_owner"] = nil
		sectionStatus = GateStatusNotApplicable
		sectionReason = "No UI/UX surface in this project/task."
	}
	return map[string]any{
		"schema_version":     designEvidenceSchemaVersion,
		"run_id":             runID,
		"task_id":            "DESIGN-004",
		"task_class":         "design-evidence-schema-bootstrap",
		"teal_applicability": applicability,
		"design_plan_evidence": map[string]any{
			"status": sectionStatus,
			"reason": sectionReason,
			"detail_ref": map[string]any{
				"path":     tokenRunPath(runID, "plan.md"),
				"checksum": "sha256:" + strings.Repeat("a", 64),
				"markers":  []string{"DESIGN_PLAN_GATE"},
			},
		},
		"design_fidelity_evidence": map[string]any{
			"status": sectionStatus,
			"reason": sectionReason,
			"evidence_refs": []map[string]any{{
				"path":     tokenRunPath(runID, "verification.md"),
				"checksum": "sha256:" + strings.Repeat("b", 64),
			}},
		},
		"color_review_evidence": map[string]any{
			"status": sectionStatus,
			"reason": sectionReason,
			"evidence_refs": []map[string]any{{
				"path":     tokenRunPath(runID, "review.md"),
				"checksum": "sha256:" + strings.Repeat("c", 64),
			}},
		},
		"boundary_evidence": map[string]any{
			"status":                  GateStatusPass,
			"policy_owner":            "KAS",
			"kah_validation_role":     "deterministic_shape_only",
			"kah_forbidden_decisions": []string{"classify UI", "select Teal owner", "judge design quality", "score screenshots", "approve waiver", "waive gates"},
		},
	}
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

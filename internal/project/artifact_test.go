package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitArtifactsCreatesCanonicalFilesAndUpdatesMetadata(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	result, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	if result.RunID != created.Metadata.RunID || result.EventID != "evt-000003" {
		t.Fatalf("result = %#v, want run id and artifact event", result)
	}
	if len(result.Created) != len(canonicalArtifactPaths) || len(result.RequiredArtifacts) == 0 {
		t.Fatalf("created=%d required=%d, want canonical files and required manifest", len(result.Created), len(result.RequiredArtifacts))
	}
	for _, artifact := range canonicalArtifactPaths {
		path := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, filepath.FromSlash(artifact))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("missing artifact %s: %v", artifact, err)
		}
		if info.Size() == 0 {
			t.Fatalf("artifact %s is empty, want baseline content", artifact)
		}
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	if len(metadata.RequiredArtifacts) != len(result.RequiredArtifacts) || !containsString(metadata.RequiredArtifacts, "diff.patch") || !containsString(metadata.RequiredArtifacts, "task-brief.md") || !containsString(metadata.RequiredArtifacts, "redteam/final-gate-review.md") {
		t.Fatalf("required_artifacts = %#v, want production write manifest", metadata.RequiredArtifacts)
	}
	lines := runEventLines(t, repo)
	if len(lines) != 3 || !strings.Contains(lines[2], `"type":"artifact.written"`) || !strings.Contains(lines[2], `"artifact_count"`) {
		t.Fatalf("events = %#v, want artifact.written", lines)
	}
}

func TestInitArtifactsPreservesNonEmptyAndReinitializesEmpty(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runDir := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "plan.md"), []byte("custom plan\n"), 0o600); err != nil {
		t.Fatalf("write custom plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "checklist.md"), nil, 0o600); err != nil {
		t.Fatalf("write empty checklist: %v", err)
	}

	result, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	if !containsArtifactPath(result.Preserved, "plan.md") || !containsArtifactPath(result.Reinitialized, "checklist.md") {
		t.Fatalf("preserved=%#v reinitialized=%#v, want non-empty preserved and empty reinitialized", result.Preserved, result.Reinitialized)
	}
	if got := readText(t, filepath.Join(runDir, "plan.md")); got != "custom plan\n" {
		t.Fatalf("plan.md = %q, want preserved custom content", got)
	}
	if got := readText(t, filepath.Join(runDir, "checklist.md")); !strings.Contains(got, "Status: pending") {
		t.Fatalf("checklist.md = %q, want baseline content", got)
	}
}

func TestListArtifactsUsesManifestBeforeAndAfterInit(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.WorkPath = "B_discovery_shaping"
	options.SOTPolicy = "minimal_sot_before_code"
	options.ExecutionMode = "research"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	before, err := ListArtifacts(root, created.Metadata.RunID)
	if err != nil {
		t.Fatalf("ListArtifacts(before) error = %v", err)
	}
	if before.RunID != created.Metadata.RunID {
		t.Fatalf("before.RunID = %q, want %q", before.RunID, created.Metadata.RunID)
	}
	if len(before.Artifacts) != len(canonicalArtifactPaths) || !artifactRequired(before.Artifacts, "discovery/handoff-to-development.md") || !artifactRequired(before.Artifacts, "discovery/research-notes.md") {
		t.Fatalf("before = %#v, want Path B research required discovery artifacts", before)
	}
	if before.Artifacts[0].Exists {
		t.Fatalf("before.Artifacts[0] = %#v, want missing before init", before.Artifacts[0])
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	after, err := ListArtifacts(root, created.Metadata.RunID)
	if err != nil {
		t.Fatalf("ListArtifacts(after) error = %v", err)
	}
	if !after.Artifacts[0].Exists || after.Artifacts[0].Empty {
		t.Fatalf("after.Artifacts[0] = %#v, want initialized artifact", after.Artifacts[0])
	}
}

func TestInitArtifactsRejectsFinishedRun(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := CloseRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("CloseRun() error = %v", err)
	}
	_, err = InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	assertProblemCode(t, err, "run_artifact_init_invalid_state")
}

func TestInitArtifactsRejectsAbortedRun(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := AbortRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("AbortRun() error = %v", err)
	}
	_, err = InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	assertProblemCode(t, err, "run_artifact_init_invalid_state")
}

func TestInitArtifactsSecondRunPreservesAllExistingArtifacts(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	first, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("InitArtifacts(first) error = %v", err)
	}
	second, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("InitArtifacts(second) error = %v", err)
	}
	if len(first.Created) != len(canonicalArtifactPaths) {
		t.Fatalf("first.Created=%d, want canonical artifact count", len(first.Created))
	}
	if len(second.Created) != 0 || len(second.Reinitialized) != 0 || len(second.Preserved) != len(canonicalArtifactPaths) {
		t.Fatalf("second created=%d reinitialized=%d preserved=%d, want all preserved", len(second.Created), len(second.Reinitialized), len(second.Preserved))
	}
	if second.EventID != "evt-000004" {
		t.Fatalf("second.EventID = %q, want evt-000004", second.EventID)
	}
}

func TestInitArtifactsRejectsSymlinkArtifactPath(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runDir := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID)
	if err := os.Symlink(filepath.Join(repo, ".kkachi", "status.json"), filepath.Join(runDir, "plan.md")); err != nil {
		t.Fatalf("create artifact symlink: %v", err)
	}
	_, err = InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	assertProblemCode(t, err, "artifact_path_invalid")
}

func TestArtifactManifestExecutionModes(t *testing.T) {
	tests := []struct {
		name            string
		executionMode   string
		backendEvidence string
		want            []string
	}{
		{name: "production write", executionMode: "production_write", want: []string{"diff.patch", "impl-log.md", "review.md", "redteam/impl-review.md", "redteam/test-review.md", "redteam/final-gate-review.md"}},
		{name: "production write with backend evidence", executionMode: "production_write", backendEvidence: BackendEvidenceRequired, want: []string{"diff.patch", "impl-log.md", "review.md", "redteam/impl-review.md", "redteam/test-review.md", "redteam/final-gate-review.md", "selected-cli.json", "capability-check.md", "bridge-session-snapshot.json", "bridge-events.md"}},
		{name: "readiness hardening", executionMode: "readiness_hardening", want: []string{"diff.patch", "impl-log.md", "review.md", "redteam/impl-review.md", "redteam/test-review.md", "redteam/final-gate-review.md"}},
		{name: "adapter qa", executionMode: "adapter_qa", want: []string{"selected-cli.json", "capability-check.md", "bridge-session-snapshot.json", "bridge-events.md", "cli-output.md", "redteam/qa-review.md"}},
		{name: "research", executionMode: "research", want: []string{"discovery/research-notes.md", "discovery/strategy-options.md", "discovery/selected-strategy.md"}},
		{name: "docs only", executionMode: "docs_only", want: []string{"sot-update.md", "roadmap-update.md"}},
		{name: "verification", executionMode: "verification", want: []string{"review.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := RunMetadata{
				WorkPath:        "A_development_execution",
				WorkMode:        "standard",
				ExecutionMode:   tt.executionMode,
				BackendEvidence: tt.backendEvidence,
			}
			got := ArtifactManifest(metadata)
			for _, want := range tt.want {
				if !containsString(got, want) {
					t.Fatalf("ArtifactManifest(%s) = %#v, missing %s", tt.executionMode, got, want)
				}
			}
			if hasDuplicates(got) {
				t.Fatalf("ArtifactManifest(%s) = %#v, want no duplicates", tt.executionMode, got)
			}
		})
	}
}

func TestArtifactManifestTokenEconomyArtifactRequiredForSupportedTokenTasks(t *testing.T) {
	token001Task := tokenEconomyTaskID
	token002Task := tokenEconomyToken002TaskID
	otherTask := "token-003"
	base := RunMetadata{
		WorkPath:        "A_development_execution",
		WorkMode:        "standard",
		ExecutionMode:   "adapter_qa",
		BackendEvidence: BackendEvidenceNotApplicable,
	}
	base.TaskID = &token001Task
	if got := ArtifactManifest(base); !containsString(got, tokenEconomyArtifact) {
		t.Fatalf("ArtifactManifest(token-001) = %#v, missing %s", got, tokenEconomyArtifact)
	}
	base.TaskID = &token002Task
	if got := ArtifactManifest(base); !containsString(got, tokenEconomyArtifact) {
		t.Fatalf("ArtifactManifest(token-002) = %#v, missing %s", got, tokenEconomyArtifact)
	}
	base.TaskID = &otherTask
	if got := ArtifactManifest(base); containsString(got, tokenEconomyArtifact) {
		t.Fatalf("ArtifactManifest(token-003) = %#v, want no token-economy artifact", got)
	}
	base.TaskID = nil
	if got := ArtifactManifest(base); containsString(got, tokenEconomyArtifact) {
		t.Fatalf("ArtifactManifest(nil task) = %#v, want no token-economy artifact", got)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasDuplicates(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

func containsArtifactPath(values []ArtifactStatus, target string) bool {
	for _, value := range values {
		if value.Path == target {
			return true
		}
	}
	return false
}

func artifactRequired(values []ArtifactStatus, target string) bool {
	for _, value := range values {
		if value.Path == target {
			return value.Required
		}
	}
	return false
}

func TestArtifactWriteAppendAndSetStatusMutateCanonically(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "plan-source.md"), []byte("# Plan\n\nStatus: pending\nBody\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	written, err := WriteArtifact(root, ArtifactMutateOptions{RunID: created.Metadata.RunID[:24], Artifact: "plan.md", From: "plan-source.md", Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("WriteArtifact() error = %v", err)
	}
	if written.Operation != artifactOperationWrite || written.ArtifactKind != artifactKindCanonical || written.SourcePath != "plan-source.md" || written.EventID != "evt-000004" {
		t.Fatalf("written = %#v, want canonical write result", written)
	}
	planPath := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "plan.md")
	if got := readText(t, planPath); got != "# Plan\n\nStatus: pending\nBody\n" {
		t.Fatalf("plan.md after write = %q", got)
	}

	if err := os.WriteFile(filepath.Join(repo, "append.md"), []byte("- [x] done\n"), 0o600); err != nil {
		t.Fatalf("write append source: %v", err)
	}
	appended, err := AppendArtifact(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "checklist.md", From: "append.md", Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("AppendArtifact() error = %v", err)
	}
	if appended.Operation != artifactOperationAppend || appended.EventID != "evt-000005" {
		t.Fatalf("appended = %#v, want append event", appended)
	}
	checklist := readText(t, filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "checklist.md"))
	if !strings.Contains(checklist, "Status: pending") || !strings.HasSuffix(checklist, "- [x] done\n") {
		t.Fatalf("checklist after append = %q, want baseline plus appended bytes", checklist)
	}

	updated, err := SetArtifactStatus(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "checklist.md", Status: "complete", Now: testRunNow(7)})
	if err != nil {
		t.Fatalf("SetArtifactStatus(markdown) error = %v", err)
	}
	if updated.Operation != artifactOperationSetStatus || updated.Status != "complete" || updated.EventID != "evt-000006" {
		t.Fatalf("updated = %#v, want set-status event", updated)
	}
	if got := readText(t, filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "checklist.md")); !strings.Contains(got, "Status: complete") || strings.Contains(got, "Status: pending") {
		t.Fatalf("checklist after set-status = %q, want complete status", got)
	}

	lines := runEventLines(t, repo)
	if len(lines) != 6 || !strings.Contains(lines[3], `"operation":"write"`) || !strings.Contains(lines[4], `"operation":"append"`) || !strings.Contains(lines[5], `"operation":"set-status"`) || !strings.Contains(lines[5], `"artifact_kind":"canonical"`) {
		t.Fatalf("events = %#v, want artifact mutation payloads", lines)
	}
}

func TestArtifactSetStatusRejectsSchemaOwnedBackendJSON(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.BackendEvidence = BackendEvidenceRequired
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	runDir := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID)
	selectedPath := filepath.Join(runDir, "selected-cli.json")
	selectedContent := []byte(`{"version":"0.1","status":"supported","backend_type":"local_cli","adapter_type":"kkachi-agent-bridge","source_ledger_ref":"ledger://selected","caveats":[]}` + "\n")
	if err := os.WriteFile(selectedPath, selectedContent, 0o600); err != nil {
		t.Fatalf("write selected-cli: %v", err)
	}
	snapshotPath := filepath.Join(runDir, "bridge-session-snapshot.json")
	snapshotContent := []byte(`{"version":"0.1","session_id":"session-1","backend_type":"local_cli","adapter_type":"kkachi-agent-bridge","state":"closed","lifecycle_class":"completed","open_pendings":0}` + "\n")
	if err := os.WriteFile(snapshotPath, snapshotContent, 0o600); err != nil {
		t.Fatalf("write bridge snapshot: %v", err)
	}

	beforeEvents := runEventLines(t, repo)
	_, err = SetArtifactStatus(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "selected-cli.json", Status: "complete", Now: testRunNow(5)})
	assertProblemCode(t, err, "artifact_status_not_applicable")
	if got := readText(t, selectedPath); got != string(selectedContent) {
		t.Fatalf("selected-cli.json after rejected set-status = %q, want unchanged", got)
	}
	if afterEvents := runEventLines(t, repo); len(afterEvents) != len(beforeEvents) {
		t.Fatalf("events changed after selected-cli rejection: before=%d after=%d", len(beforeEvents), len(afterEvents))
	}

	_, err = SetArtifactStatus(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "bridge-session-snapshot.json", Status: "complete", Now: testRunNow(6)})
	assertProblemCode(t, err, "artifact_status_not_applicable")
	if got := readText(t, snapshotPath); got != string(snapshotContent) {
		t.Fatalf("bridge-session-snapshot.json after rejected set-status = %q, want unchanged", got)
	}
	if afterEvents := runEventLines(t, repo); len(afterEvents) != len(beforeEvents) {
		t.Fatalf("events changed after bridge snapshot rejection: before=%d after=%d", len(beforeEvents), len(afterEvents))
	}
}

func TestArtifactMutationRejectsUnsafeSupplementalAndFinishedRuns(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "source.md"), []byte("content\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	_, err = WriteArtifact(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "supplemental/note.md", From: "source.md", Now: testRunNow(4)})
	assertProblemCode(t, err, "artifact_path_invalid")
	_, err = WriteArtifact(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "../plan.md", From: "source.md", Now: testRunNow(4)})
	assertProblemCode(t, err, "artifact_path_invalid")
	_, err = WriteArtifact(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "plan.md", From: "missing.md", Now: testRunNow(4)})
	assertProblemCode(t, err, "artifact_source_missing")
	if err := os.Mkdir(filepath.Join(repo, "source-dir"), 0o755); err != nil {
		t.Fatalf("mkdir source dir: %v", err)
	}
	_, err = WriteArtifact(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "plan.md", From: "source-dir", Now: testRunNow(4)})
	assertProblemCode(t, err, "artifact_source_invalid")
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "redteam", "impl-review.md"), 0o755); err != nil {
		t.Fatalf("mkdir artifact target: %v", err)
	}
	_, err = WriteArtifact(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "redteam/impl-review.md", From: "source.md", Now: testRunNow(4)})
	assertProblemCode(t, err, "artifact_path_invalid")
	_, err = SetArtifactStatus(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "roadmap-update.md", Status: "not_applicable", Now: testRunNow(4)})
	assertProblemCode(t, err, "artifact_reason_required")
	_, err = SetArtifactStatus(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "diff.patch", Status: "complete", Now: testRunNow(4)})
	assertProblemCode(t, err, "artifact_status_unsupported")
	if after := len(runEventLines(t, repo)); after != 2 {
		t.Fatalf("events after rejected mutations = %d, want unchanged", after)
	}
	if _, err := CloseRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("CloseRun() error = %v", err)
	}
	_, err = WriteArtifact(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "plan.md", From: "source.md", Now: testRunNow(5)})
	assertProblemCode(t, err, "run_artifact_mutation_invalid_state")
}

func TestArtifactMutationRefusesCoherenceMismatch(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "source.md"), []byte("content\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}
	setStatusLastEventID(t, repo, "evt-999999")
	_, err = WriteArtifact(root, ArtifactMutateOptions{RunID: created.Metadata.RunID, Artifact: "plan.md", From: "source.md", Now: testRunNow(4)})
	assertProblemCode(t, err, "last_event_id_mismatch")
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "plan.md")); !os.IsNotExist(err) {
		t.Fatalf("plan.md exists after coherence refusal: %v", err)
	}
}

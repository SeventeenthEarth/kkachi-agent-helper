package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestGenerateRunIDFormat(t *testing.T) {
	runID, err := GenerateRunID(time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC), func(int) (string, error) { return "abcdef123456", nil })
	if err != nil {
		t.Fatalf("GenerateRunID() error = %v", err)
	}
	if got, want := runID, "run-20260430T010203Z-abcdef123456"; got != want {
		t.Fatalf("runID = %q, want %q", got, want)
	}
}

func TestCreateRunWritesMetadataAndEvent(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)

	result, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if !runIDPattern.MatchString(result.Metadata.RunID) {
		t.Fatalf("run id = %q, want helper run id", result.Metadata.RunID)
	}
	if result.Metadata.State != RunStateCreated || result.Metadata.TaskID == nil || *result.Metadata.TaskID != "runwf-001" {
		t.Fatalf("metadata = %#v, want created runwf metadata", result.Metadata)
	}
	if result.Metadata.BackendEvidence != BackendEvidenceNotApplicable {
		t.Fatalf("backend_evidence = %q, want not_applicable", result.Metadata.BackendEvidence)
	}
	metadataPath := filepath.Join(repo, ".kkachi", "runs", result.Metadata.RunID, "run-metadata.json")
	var onDisk RunMetadata
	readJSONFile(t, metadataPath, &onDisk)
	if onDisk.RunID != result.Metadata.RunID || len(onDisk.RequiredArtifacts) != 0 || len(onDisk.GateState) != 0 {
		t.Fatalf("on disk metadata = %#v, want empty artifact/gate state", onDisk)
	}
	if onDisk.BackendEvidence != BackendEvidenceNotApplicable {
		t.Fatalf("on disk backend_evidence = %q, want not_applicable", onDisk.BackendEvidence)
	}
	lines := runEventLines(t, repo)
	if len(lines) != 2 || !strings.Contains(lines[1], `"type":"run.created"`) || !strings.Contains(lines[1], result.Metadata.RunID) {
		t.Fatalf("events = %#v, want run.created", lines)
	}
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["last_event_id"] != "evt-000002" || status["active_run_id"] != nil {
		t.Fatalf("status = %#v, want event advanced and no active run", status)
	}
}

func TestCreateRunResolvesBackendEvidenceAutoForAdapterQA(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.ExecutionMode = "adapter_qa"
	options.BackendEvidence = BackendEvidenceAuto

	result, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun(adapter qa auto) error = %v", err)
	}
	if result.Metadata.BackendEvidence != BackendEvidenceRequired {
		t.Fatalf("backend_evidence = %q, want required", result.Metadata.BackendEvidence)
	}
}

func TestResolveBackendEvidenceMatrix(t *testing.T) {
	tests := []struct {
		name          string
		executionMode string
		declaration   string
		want          string
	}{
		{name: "adapter qa auto", executionMode: "adapter_qa", declaration: BackendEvidenceAuto, want: BackendEvidenceRequired},
		{name: "adapter qa empty", executionMode: "adapter_qa", declaration: "", want: BackendEvidenceRequired},
		{name: "production write auto", executionMode: "production_write", declaration: BackendEvidenceAuto, want: BackendEvidenceNotApplicable},
		{name: "production write empty", executionMode: "production_write", declaration: "", want: BackendEvidenceNotApplicable},
		{name: "production write required", executionMode: "production_write", declaration: BackendEvidenceRequired, want: BackendEvidenceRequired},
		{name: "invalid preserved for validation", executionMode: "production_write", declaration: " maybe ", want: "maybe"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ResolveBackendEvidence(tt.executionMode, tt.declaration); got != tt.want {
				t.Fatalf("ResolveBackendEvidence(%q, %q) = %q, want %q", tt.executionMode, tt.declaration, got, tt.want)
			}
		})
	}
}

func TestReadRunMetadataDefaultsLegacyMissingBackendEvidence(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	path := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "run-metadata.json")
	var raw map[string]any
	readJSONFile(t, path, &raw)
	delete(raw, "backend_evidence")
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy metadata: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write legacy metadata: %v", err)
	}

	metadata, _, err := ReadRunMetadata(root, created.Metadata.RunID)
	if err != nil {
		t.Fatalf("ReadRunMetadata(legacy) error = %v", err)
	}
	if metadata.BackendEvidence != BackendEvidenceNotApplicable {
		t.Fatalf("backend_evidence = %q, want legacy default not_applicable", metadata.BackendEvidence)
	}
}

func TestCreateRunRetriesIDCollision(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	first := deterministicCreateRunOptions()
	first.RandomHex = func(int) (string, error) { return "111111111111", nil }
	result, err := CreateRun(root, first)
	if err != nil {
		t.Fatalf("CreateRun(first) error = %v", err)
	}
	calls := 0
	second := deterministicCreateRunOptions()
	second.RandomHex = func(int) (string, error) {
		calls++
		if calls == 1 {
			return "111111111111", nil
		}
		return "222222222222", nil
	}
	second.Title = "Second"
	second.Now = func() time.Time { return time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC) }
	secondResult, err := CreateRun(root, second)
	if err != nil {
		t.Fatalf("CreateRun(second) error = %v", err)
	}
	if secondResult.Metadata.RunID == result.Metadata.RunID || !strings.HasSuffix(secondResult.Metadata.RunID, "222222222222") {
		t.Fatalf("second run id = %q, want retry suffix", secondResult.Metadata.RunID)
	}
}

func TestCreateRunValidatesRequiredAndEnums(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.Title = ""
	_, err := CreateRun(root, options)
	assertProblemCode(t, err, "run_field_required")

	tests := []struct {
		name   string
		mutate func(*CreateRunOptions)
	}{
		{name: "work path", mutate: func(o *CreateRunOptions) { o.WorkPath = "C_unknown" }},
		{name: "work mode", mutate: func(o *CreateRunOptions) { o.WorkMode = "turbo" }},
		{name: "urgency", mutate: func(o *CreateRunOptions) { o.Urgency = "eventually" }},
		{name: "sot policy", mutate: func(o *CreateRunOptions) { o.SOTPolicy = "skip_sot" }},
		{name: "execution mode", mutate: func(o *CreateRunOptions) { o.ExecutionMode = "freestyle" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := deterministicCreateRunOptions()
			options.RandomHex = func(int) (string, error) { return strings.Repeat("a", 12), nil }
			tt.mutate(&options)
			_, err := CreateRun(root, options)
			assertProblemCode(t, err, "run_metadata_invalid")
		})
	}
}

func TestCreateRunRejectsDiscoveryPathWithExistingSOTPolicy(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.WorkPath = "B_discovery_shaping"
	options.SOTPolicy = "existing_sot_basis"

	_, err := CreateRun(root, options)
	assertProblemCode(t, err, "run_field_incompatible")
	problem := err.(*Problem)
	if problem.Field != "sot_policy" || problem.Expected != "minimal_sot_before_code or full_sot_before_code" {
		t.Fatalf("problem = %#v, want sot policy compatibility guidance", problem)
	}
}

func TestRunLifecycleTransitionsUpdateMetadataStatusAndEvents(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	activated, err := ActivateRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("ActivateRun() error = %v", err)
	}
	if activated.Metadata.State != RunStateActive || activated.EventID != "evt-000003" {
		t.Fatalf("activated = %#v, want active evt-000003", activated)
	}
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["active_run_id"] != created.Metadata.RunID || status["active_run_state"] != RunStateActive {
		t.Fatalf("status = %#v, want active run fields", status)
	}
	closed, err := CloseRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(6)})
	if err != nil {
		t.Fatalf("CloseRun() error = %v", err)
	}
	if closed.Metadata.State != RunStateClosed || closed.EventID != "evt-000004" {
		t.Fatalf("closed = %#v, want closed evt-000004", closed)
	}
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["active_run_id"] != nil || status["active_run_state"] != nil || status["last_event_id"] != "evt-000004" {
		t.Fatalf("status after close = %#v, want active fields cleared", status)
	}
	lines := runEventLines(t, repo)
	if !strings.Contains(lines[2], `"type":"run.activated"`) || !strings.Contains(lines[3], `"type":"run.closed"`) {
		t.Fatalf("events = %#v, want lifecycle events", lines)
	}
}

func TestRunAbortCreatedRun(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, _ := CreateRun(root, deterministicCreateRunOptions())
	aborted, err := AbortRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("AbortRun() error = %v", err)
	}
	if aborted.Metadata.State != RunStateAborted || aborted.EventID != "evt-000003" {
		t.Fatalf("aborted = %#v, want aborted evt-000003", aborted)
	}
}

func TestRunLifecycleRejectsInvalidTransitionsAndActiveConflict(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	first, _ := CreateRun(root, deterministicCreateRunOptions())
	secondOptions := deterministicCreateRunOptions()
	secondOptions.Title = "Second"
	secondOptions.RandomHex = func(int) (string, error) { return "222222222222", nil }
	second, _ := CreateRun(root, secondOptions)
	if _, err := ActivateRun(root, RunLifecycleOptions{RunID: first.Metadata.RunID, Now: testRunNow(5)}); err != nil {
		t.Fatalf("ActivateRun(first) error = %v", err)
	}
	_, err := ActivateRun(root, RunLifecycleOptions{RunID: second.Metadata.RunID, Now: testRunNow(6)})
	assertProblemCode(t, err, "active_run_conflict")
	if _, err := CloseRun(root, RunLifecycleOptions{RunID: first.Metadata.RunID, Now: testRunNow(7)}); err != nil {
		t.Fatalf("CloseRun(first) error = %v", err)
	}
	_, err = AbortRun(root, RunLifecycleOptions{RunID: first.Metadata.RunID, Now: testRunNow(8)})
	assertProblemCode(t, err, "run_transition_invalid")
}

func TestRunLifecycleRefusesCoherenceMismatchBeforeMetadataMutation(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	metadataBefore := readRunMetadata(t, repo, created.Metadata.RunID)
	eventsPath := filepath.Join(repo, ".kkachi", "events.jsonl")
	file, err := os.OpenFile(eventsPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	if _, err := file.WriteString(`{"version":"0.1","event_id":"evt-000003","occurred_at":"2026-04-30T03:00:00Z","run_id":"` + created.Metadata.RunID + `","type":"run.created","actor":"helper","payload":{}}` + "\n"); err != nil {
		t.Fatalf("append crash event: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	_, err = ActivateRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	assertProblemCode(t, err, "last_event_id_mismatch")
	metadataAfter := readRunMetadata(t, repo, created.Metadata.RunID)
	if metadataAfter.State != metadataBefore.State {
		t.Fatalf("metadata state changed after refused transition = %q, want %q", metadataAfter.State, metadataBefore.State)
	}
}

func TestCreateRunMetadataWriteFailureLeavesFailClosedEventMismatch(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	injected := errors.New("injected metadata write failure")
	original := writeRunMetadataNewDuringEvent
	writeRunMetadataNewDuringEvent = func(SafePath, RunMetadata) error { return injected }
	defer func() { writeRunMetadataNewDuringEvent = original }()

	_, err := CreateRun(root, deterministicCreateRunOptions())
	if !errors.Is(err, injected) {
		t.Fatalf("CreateRun() error = %v, want injected failure", err)
	}

	runID := "run-20260430T010203Z-abcdef123456"
	metadataPath := filepath.Join(repo, ".kkachi", "runs", runID, "run-metadata.json")
	if _, statErr := os.Stat(metadataPath); !os.IsNotExist(statErr) {
		t.Fatalf("metadata stat err = %v, want no metadata file after injected failure", statErr)
	}
	assertStatusAndEventTail(t, repo, "evt-000001", "evt-000002")
	_, err = ListRuns(root)
	assertProblemCode(t, err, "last_event_id_mismatch")
}

func TestRunLifecycleMetadataWriteFailureLeavesFailClosedEventMismatch(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	metadataBefore := readRunMetadata(t, repo, created.Metadata.RunID)
	injected := errors.New("injected metadata write failure")
	original := writeRunMetadataExistingDuringEvent
	writeRunMetadataExistingDuringEvent = func(SafePath, RunMetadata) error { return injected }
	defer func() { writeRunMetadataExistingDuringEvent = original }()

	_, err = ActivateRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	if !errors.Is(err, injected) {
		t.Fatalf("ActivateRun() error = %v, want injected failure", err)
	}
	metadataAfter := readRunMetadata(t, repo, created.Metadata.RunID)
	if metadataAfter.State != metadataBefore.State {
		t.Fatalf("metadata state changed after failed write = %q, want %q", metadataAfter.State, metadataBefore.State)
	}
	assertStatusAndEventTail(t, repo, "evt-000002", "evt-000003")
	_, err = ListRuns(root)
	assertProblemCode(t, err, "last_event_id_mismatch")
}

func TestRunLifecycleRejectsMalformedActiveStatus(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, _ := CreateRun(root, deterministicCreateRunOptions())
	var status map[string]any
	statusPath := filepath.Join(repo, ".kkachi", "status.json")
	readJSONFile(t, statusPath, &status)
	status["active_run_state"] = 123
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("encode status: %v", err)
	}
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write malformed status: %v", err)
	}
	_, err = ActivateRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	assertProblemCode(t, err, "status_active_run_invalid")
}

func TestResolveRunIDPrefix(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	firstOptions := deterministicCreateRunOptions()
	firstOptions.RandomHex = func(int) (string, error) { return "111111111111", nil }
	first, _ := CreateRun(root, firstOptions)
	secondOptions := deterministicCreateRunOptions()
	secondOptions.RandomHex = func(int) (string, error) { return "222222222222", nil }
	secondOptions.Now = testRunNow(4)
	second, _ := CreateRun(root, secondOptions)

	resolved, err := ResolveRunID(root, first.Metadata.RunID[:24])
	if err != nil || resolved != first.Metadata.RunID {
		t.Fatalf("ResolveRunID(unique) = %q/%v, want %q", resolved, err, first.Metadata.RunID)
	}
	_, err = ResolveRunID(root, "run-20260430")
	assertProblemCode(t, err, "run_id_ambiguous")
	_, err = ResolveRunID(root, "run-19990101")
	assertProblemCode(t, err, "run_not_found")
	_ = second
}

func TestListRunsRejectsMalformedMetadata(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, _ := CreateRun(root, deterministicCreateRunOptions())
	path := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "run-metadata.json")
	if err := os.WriteFile(path, []byte("{not-json\n"), 0o600); err != nil {
		t.Fatalf("corrupt metadata: %v", err)
	}
	_, err := ListRuns(root)
	assertProblemCode(t, err, "run_metadata_invalid_json")
}

func deterministicCreateRunOptions() CreateRunOptions {
	return CreateRunOptions{
		TaskID:        "runwf-001",
		Title:         "Run workflow metadata",
		WorkPath:      "A_development_execution",
		WorkMode:      "standard",
		Urgency:       "normal",
		SOTPolicy:     "existing_sot_basis",
		ExecutionMode: "production_write",
		Commander:     "Gongmyeong",
		Redteam:       "Reviewer",
		Now:           testRunNow(3),
		RandomHex:     func(int) (string, error) { return "abcdef123456", nil },
	}
}

func testRunNow(second int) func() time.Time {
	return func() time.Time { return time.Date(2026, 4, 30, 1, 2, second, 0, time.UTC) }
}

func runEventLines(t *testing.T, repo string) []string {
	t.Helper()
	text := strings.TrimSpace(readText(t, filepath.Join(repo, ".kkachi", "events.jsonl")))
	if text == "" {
		return nil
	}
	return strings.Split(text, "\n")
}

func assertStatusAndEventTail(t *testing.T, repo string, wantStatusLast string, wantEventTail string) {
	t.Helper()
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["last_event_id"] != wantStatusLast {
		t.Fatalf("status last_event_id = %q, want %q", status["last_event_id"], wantStatusLast)
	}
	lines := runEventLines(t, repo)
	if len(lines) == 0 {
		t.Fatal("event log is empty")
	}
	var event struct {
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal([]byte(lines[len(lines)-1]), &event); err != nil {
		t.Fatalf("decode event tail: %v", err)
	}
	if event.EventID != wantEventTail {
		t.Fatalf("event tail = %q, want %q", event.EventID, wantEventTail)
	}
}

func readRunMetadata(t *testing.T, repo string, runID string) RunMetadata {
	t.Helper()
	var metadata RunMetadata
	data, err := os.ReadFile(filepath.Join(repo, ".kkachi", "runs", runID, "run-metadata.json"))
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("decode metadata: %v", err)
	}
	return metadata
}

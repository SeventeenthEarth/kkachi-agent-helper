//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBinaryEntrypointSmoke(t *testing.T) {
	cmd := exec.Command("go", "run", "../../cmd/kkachi-agent-helper", "--version")

	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go run failed: %v\n%s", err, string(output))
	}
	if !strings.HasPrefix(string(output), "kkachi-agent-helper 0.0.0-dev") {
		t.Fatalf("output = %q, want version prefix", string(output))
	}
}

func TestProjectInitCreatesStateAndRefusesOverwrite(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	cmd := exec.Command(binary, "project", "init", "--json")
	cmd.Dir = repo
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("project init failed: %v\n%s", err, string(output))
	}

	var payload struct {
		RootPath       string   `json:"root_path"`
		ProjectID      string   `json:"project_id"`
		ProjectName    string   `json:"project_name"`
		CreatedPaths   []string `json:"created_paths"`
		SchemaPaths    []string `json:"schema_paths"`
		InitialEventID string   `json:"initial_event_id"`
	}
	if err := json.Unmarshal(output, &payload); err != nil {
		t.Fatalf("output is not JSON: %v\n%s", err, string(output))
	}
	wantRoot, err := filepath.EvalSymlinks(repo)
	if err != nil {
		t.Fatalf("eval repo path: %v", err)
	}
	if payload.RootPath != wantRoot {
		t.Fatalf("root_path = %q, want %q", payload.RootPath, wantRoot)
	}
	if payload.ProjectID == "" || payload.ProjectName == "" || payload.InitialEventID != "evt-000001" {
		t.Fatalf("payload = %#v, want project identity and initial event", payload)
	}
	if len(payload.CreatedPaths) != 3 || len(payload.SchemaPaths) != 5 {
		t.Fatalf("paths = %#v/%#v, want state and schema paths", payload.CreatedPaths, payload.SchemaPaths)
	}

	statusCmd := exec.Command(binary, "project", "status", "--json")
	statusCmd.Dir = repo
	statusOutput, err := statusCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("project status failed: %v\n%s", err, string(statusOutput))
	}
	if !strings.Contains(string(statusOutput), `"health":"ok"`) || !strings.Contains(string(statusOutput), `"event_tail_id":"evt-000001"`) {
		t.Fatalf("project status output = %s, want healthy event tail", string(statusOutput))
	}

	doctorCmd := exec.Command(binary, "project", "doctor", "--json")
	doctorCmd.Dir = repo
	doctorOutput, err := doctorCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("project doctor failed: %v\n%s", err, string(doctorOutput))
	}
	if !strings.Contains(string(doctorOutput), `"health":"ok"`) || !strings.Contains(string(doctorOutput), `"failed":0`) {
		t.Fatalf("project doctor output = %s, want healthy checks", string(doctorOutput))
	}

	required := []string{
		".kkachi/config.yaml",
		".kkachi/status.json",
		".kkachi/events.jsonl",
		".kkachi/schemas/status.schema.json",
		".kkachi/schemas/run-metadata.schema.json",
		".kkachi/schemas/event.schema.json",
		".kkachi/schemas/selected-cli.schema.json",
		".kkachi/schemas/bridge-session-snapshot.schema.json",
	}
	for _, relative := range required {
		if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(relative))); err != nil {
			t.Fatalf("%s was not created: %v", relative, err)
		}
	}

	runCreate := exec.Command(binary, "run", "create", "--title", "Run workflow metadata", "--work-path", "A_development_execution", "--work-mode", "standard", "--urgency", "normal", "--sot-policy", "existing_sot_basis", "--execution-mode", "production_write", "--commander", "Gongmyeong", "--task-id", "runwf-001", "--json")
	runCreate.Dir = repo
	runCreateOutput, err := runCreate.CombinedOutput()
	if err != nil {
		t.Fatalf("run create failed: %v\n%s", err, string(runCreateOutput))
	}
	var createdRun struct {
		RunID   string `json:"run_id"`
		State   string `json:"state"`
		EventID string `json:"event_id"`
	}
	if err := json.Unmarshal(runCreateOutput, &createdRun); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(runCreateOutput))
	}
	if !strings.HasPrefix(createdRun.RunID, "run-") || createdRun.State != "created" || createdRun.EventID != "evt-000002" {
		t.Fatalf("createdRun = %#v, want created evt-000002", createdRun)
	}

	runShow := exec.Command(binary, "run", "show", createdRun.RunID, "--json")
	runShow.Dir = repo
	runShowOutput, err := runShow.CombinedOutput()
	if err != nil {
		t.Fatalf("run show failed: %v\n%s", err, string(runShowOutput))
	}
	if !strings.Contains(string(runShowOutput), `"task_id":"runwf-001"`) || !strings.Contains(string(runShowOutput), `"required_artifacts":[]`) {
		t.Fatalf("run show output = %s, want metadata", string(runShowOutput))
	}

	runActivate := exec.Command(binary, "run", "activate", createdRun.RunID, "--json")
	runActivate.Dir = repo
	runActivateOutput, err := runActivate.CombinedOutput()
	if err != nil {
		t.Fatalf("run activate failed: %v\n%s", err, string(runActivateOutput))
	}
	if !strings.Contains(string(runActivateOutput), `"state":"active"`) || !strings.Contains(string(runActivateOutput), `"event_id":"evt-000003"`) {
		t.Fatalf("run activate output = %s, want active evt-000003", string(runActivateOutput))
	}

	runClose := exec.Command(binary, "run", "close", createdRun.RunID, "--json")
	runClose.Dir = repo
	runCloseOutput, err := runClose.CombinedOutput()
	if err != nil {
		t.Fatalf("run close failed: %v\n%s", err, string(runCloseOutput))
	}
	if !strings.Contains(string(runCloseOutput), `"state":"closed"`) || !strings.Contains(string(runCloseOutput), `"event_id":"evt-000004"`) {
		t.Fatalf("run close output = %s, want closed evt-000004", string(runCloseOutput))
	}

	appendCmd := exec.Command(binary, "event", "append", "artifact.written", "--run", "run-abc", "--payload", `{"path":"impl-log.md"}`, "--json")
	appendCmd.Dir = repo
	appendOutput, err := appendCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("event append failed: %v\n%s", err, string(appendOutput))
	}
	if !strings.Contains(string(appendOutput), `"event_id":"evt-000005"`) {
		t.Fatalf("event append output = %s, want evt-000005", string(appendOutput))
	}
	statusBytes, err := os.ReadFile(filepath.Join(repo, ".kkachi", "status.json"))
	if err != nil {
		t.Fatalf("read status after event append: %v", err)
	}
	if !strings.Contains(string(statusBytes), `"last_event_id": "evt-000005"`) {
		t.Fatalf("status after event append = %s, want evt-000005", string(statusBytes))
	}

	retry := exec.Command(binary, "project", "init", "--json")
	retry.Dir = repo
	retryOutput, err := retry.CombinedOutput()
	if err == nil {
		t.Fatalf("second project init succeeded, want overwrite refusal\n%s", string(retryOutput))
	}
	if !strings.Contains(string(retryOutput), `"code":"helper_state_exists"`) {
		t.Fatalf("retry output = %s, want helper_state_exists", string(retryOutput))
	}
}

func TestGates001And002GateCheckWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Gate check workflow",
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "gates-002",
		"--json",
	)
	var created struct {
		RunID    string `json:"run_id"`
		EventID  string `json:"event_id"`
		Metadata struct {
			WorkPath  string `json:"work_path"`
			WorkMode  string `json:"work_mode"`
			SOTPolicy string `json:"sot_policy"`
			Urgency   string `json:"urgency"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	if created.EventID != "evt-000002" {
		t.Fatalf("created event id = %q, want evt-000002", created.EventID)
	}
	runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")

	pendingOutput, err := runHelperAllowError(binary, repo, "gate", "check", created.RunID, "intake", "--json")
	if err == nil {
		t.Fatalf("gate check succeeded with pending intake\n%s", string(pendingOutput))
	}
	var pending gateCheckOutput
	if err := json.Unmarshal(pendingOutput, &pending); err != nil {
		t.Fatalf("pending gate output is not JSON: %v\n%s", err, string(pendingOutput))
	}
	if pending.RunID != created.RunID || pending.Gate != "intake" || pending.Status != "fail" || pending.EventID != "evt-000004" || len(pending.MissingEvidence) == 0 || !gateCheckListed(pending.Checks, "intake_status", "fail") {
		t.Fatalf("pending gate = %#v, want intake fail with evidence and evt-000004", pending)
	}
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"gate.failed"`, "events after failed gate check")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "status.json")), `"event_id": "evt-000004"`, "status after failed gate check")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")), `"status": "fail"`, "metadata after failed gate check")

	writeIntegrationIntake(t, repo, created.RunID, created.Metadata.WorkPath, created.Metadata.WorkMode, created.Metadata.SOTPolicy, created.Metadata.Urgency, "")
	passOutput := runHelper(t, binary, repo, "gate", "check", created.RunID[:24], "intake", "--json")
	var passed gateCheckOutput
	if err := json.Unmarshal(passOutput, &passed); err != nil {
		t.Fatalf("passing gate output is not JSON: %v\n%s", err, string(passOutput))
	}
	if passed.RunID != created.RunID || passed.Gate != "intake" || passed.Status != "pass" || passed.EventID != "evt-000005" || len(passed.MissingEvidence) != 0 || !gateCheckListed(passed.Checks, "required_artifacts", "pass") {
		t.Fatalf("passed gate = %#v, want intake pass with evt-000005", passed)
	}
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"gate.passed"`, "events after passing gate check")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "status.json")), `"status": "pass"`, "status after passing gate check")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")), `"event_id": "evt-000005"`, "metadata after passing gate check")

	planPendingOutput, err := runHelperAllowError(binary, repo, "gate", "check", created.RunID, "plan", "--json")
	if err == nil {
		t.Fatalf("plan gate check succeeded with pending plan artifacts\n%s", string(planPendingOutput))
	}
	var planPending gateCheckOutput
	if err := json.Unmarshal(planPendingOutput, &planPending); err != nil {
		t.Fatalf("plan pending gate output is not JSON: %v\n%s", err, string(planPendingOutput))
	}
	if planPending.Status != "fail" || planPending.EventID != "evt-000006" || len(planPending.MissingEvidence) != 3 || !gateCheckListed(planPending.Checks, "acceptance_criteria", "fail") || !gateCheckListed(planPending.Checks, "plan_artifact", "fail") || !gateCheckListed(planPending.Checks, "checklist_artifact", "fail") {
		t.Fatalf("planPending = %#v, want failed plan gate with pending artifacts", planPending)
	}

	writeIntegrationMarkdownArtifact(t, repo, created.RunID, "sot-basis.md", "Status: complete\nSource: docs/specs.md\n")
	writeIntegrationMarkdownArtifact(t, repo, created.RunID, "acceptance-criteria.md", "Status: complete\nCriteria: pre-implementation safety\n")
	writeIntegrationMarkdownArtifact(t, repo, created.RunID, "plan.md", "Status: complete\nPlan: gates-002 deterministic validators\n")
	writeIntegrationMarkdownArtifact(t, repo, created.RunID, "checklist.md", "Status: complete\n- [x] SOT gate\n- [x] roadmap gate\n- [x] plan gate\n")

	sotOutput := runHelper(t, binary, repo, "gate", "check", created.RunID, "sot", "--json")
	var sotPassed gateCheckOutput
	if err := json.Unmarshal(sotOutput, &sotPassed); err != nil {
		t.Fatalf("sot gate output is not JSON: %v\n%s", err, string(sotOutput))
	}
	if sotPassed.Status != "pass" || sotPassed.EventID != "evt-000007" || !gateCheckListed(sotPassed.Checks, "sot_basis", "pass") {
		t.Fatalf("sotPassed = %#v, want SOT pass", sotPassed)
	}

	roadmapOutput := runHelper(t, binary, repo, "gate", "check", created.RunID, "roadmap", "--json")
	var roadmapPassed gateCheckOutput
	if err := json.Unmarshal(roadmapOutput, &roadmapPassed); err != nil {
		t.Fatalf("roadmap gate output is not JSON: %v\n%s", err, string(roadmapOutput))
	}
	if roadmapPassed.Status != "pass" || roadmapPassed.EventID != "evt-000008" || !gateCheckListed(roadmapPassed.Checks, "roadmap_trace", "pass") {
		t.Fatalf("roadmapPassed = %#v, want roadmap trace pass", roadmapPassed)
	}

	planOutput := runHelper(t, binary, repo, "gate", "check", created.RunID, "plan", "--json")
	var planPassed gateCheckOutput
	if err := json.Unmarshal(planOutput, &planPassed); err != nil {
		t.Fatalf("plan gate output is not JSON: %v\n%s", err, string(planOutput))
	}
	if planPassed.Status != "pass" || planPassed.EventID != "evt-000009" || len(planPassed.MissingEvidence) != 0 || !gateCheckListed(planPassed.Checks, "acceptance_criteria", "pass") || !gateCheckListed(planPassed.Checks, "plan_artifact", "pass") || !gateCheckListed(planPassed.Checks, "checklist_artifact", "pass") {
		t.Fatalf("planPassed = %#v, want completed plan gate pass", planPassed)
	}
}

func TestRunwf002LockWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")

	writeLockMetadata(t, repo, "project_write", lockMetadata{
		Version:   "0.1",
		LockName:  "project_write",
		OwnerPID:  os.Getpid(),
		Hostname:  mustHostname(t),
		Command:   "integration fresh writer",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	output, err := runHelperAllowError(binary, repo, runwf002CreateRunArgs("Blocked by write lock")...)
	if err == nil {
		t.Fatalf("run create succeeded under fresh project_write lock\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"lock_conflict"`, "fresh project lock conflict")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"event_id":"evt-000001"`, "events after refused create")
	removeLock(t, repo, "project_write")

	createdOutput := runHelper(t, binary, repo, runwf002CreateRunArgs("Lock workflow")...)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}

	writeLockMetadata(t, repo, "active_run", lockMetadata{
		Version:   "0.1",
		LockName:  "active_run",
		RunID:     created.RunID,
		OwnerPID:  os.Getpid(),
		Hostname:  mustHostname(t),
		Command:   "integration active lifecycle",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	output, err = runHelperAllowError(binary, repo, "run", "activate", created.RunID, "--json")
	if err == nil {
		t.Fatalf("run activate succeeded under fresh active_run lock\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"lock_conflict"`, "fresh active lock conflict")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "status.json")), `"active_run_id": null`, "status after refused activate")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")), `"state": "created"`, "metadata after refused activate")
	removeLock(t, repo, "active_run")

	old := time.Date(2026, 4, 30, 3, 4, 5, 0, time.UTC)
	writeLockMetadata(t, repo, "project_write", lockMetadata{
		Version:   "0.1",
		LockName:  "project_write",
		OwnerPID:  999999,
		Hostname:  "other-host",
		Command:   "integration stale writer",
		CreatedAt: old.Add(-31 * time.Minute).Format(time.RFC3339),
	})
	output, err = runHelperAllowError(binary, repo, runwf002CreateRunArgs("Blocked by stale lock")...)
	if err == nil {
		t.Fatalf("run create succeeded under stale project_write lock before recovery\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"lock_stale_recovery_required"`, "stale project lock refusal")

	doctorOutput, err := runHelperAllowError(binary, repo, "project", "doctor", "--json")
	if err != nil {
		t.Fatalf("project doctor failed under stale lock: %v\n%s", err, string(doctorOutput))
	}
	assertOutputContains(t, doctorOutput, `"health":"warning"`, "doctor stale lock health")
	assertOutputContains(t, doctorOutput, "lock recover", "doctor stale lock hint")

	recoverOutput := runHelper(t, binary, repo, "lock", "recover", "project-write", "--reason", "integration stale recovery", "--json")
	assertOutputContains(t, recoverOutput, `"lock_name":"project_write"`, "lock recover output")
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "project_write.lock")); !os.IsNotExist(err) {
		t.Fatalf("project_write lock stat = %v, want absent after recovery", err)
	}
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"lock.recovered"`, "events after recovery")

	runHelper(t, binary, repo, runwf002CreateRunArgs("After recovery")...)
}

func TestRunwf003ArtifactWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo, runwf003CreateRunArgs("Artifact workflow")...)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	runDir := filepath.Join(repo, ".kkachi", "runs", created.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "plan.md"), []byte("custom integration plan\n"), 0o600); err != nil {
		t.Fatalf("write custom plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "checklist.md"), nil, 0o600); err != nil {
		t.Fatalf("write empty checklist: %v", err)
	}

	initOutput := runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	var initialized struct {
		RunID             string             `json:"run_id"`
		EventID           string             `json:"event_id"`
		Created           []artifactPathOnly `json:"created"`
		Reinitialized     []artifactPathOnly `json:"reinitialized"`
		Preserved         []artifactPathOnly `json:"preserved"`
		RequiredArtifacts []string           `json:"required_artifacts"`
	}
	if err := json.Unmarshal(initOutput, &initialized); err != nil {
		t.Fatalf("artifact init output is not JSON: %v\n%s", err, string(initOutput))
	}
	if initialized.RunID != created.RunID || initialized.EventID != "evt-000003" || len(initialized.Created) == 0 || len(initialized.RequiredArtifacts) == 0 {
		t.Fatalf("initialized = %#v, want created artifacts and required manifest", initialized)
	}
	if !artifactPathListed(initialized.Preserved, "plan.md") || !artifactPathListed(initialized.Reinitialized, "checklist.md") {
		t.Fatalf("preserved=%#v reinitialized=%#v, want plan preserved and checklist reinitialized", initialized.Preserved, initialized.Reinitialized)
	}
	assertOutputContains(t, readFile(t, filepath.Join(runDir, "plan.md")), "custom integration plan", "preserved plan")
	assertOutputContains(t, readFile(t, filepath.Join(runDir, "checklist.md")), "Status: pending", "reinitialized checklist")
	for _, relative := range []string{"intake-classification.md", "sot-basis.md", "acceptance-criteria.md", "diff.patch", "impl-log.md", "test-log.md", "verification.md", "docs-update.md", "final-report.md", "redteam/final-gate-review.md"} {
		info, err := os.Stat(filepath.Join(runDir, filepath.FromSlash(relative)))
		if err != nil {
			t.Fatalf("artifact %s was not created: %v", relative, err)
		}
		if info.Size() == 0 {
			t.Fatalf("artifact %s is empty, want baseline content", relative)
		}
	}

	metadata := readFile(t, filepath.Join(runDir, "run-metadata.json"))
	assertOutputContains(t, metadata, `"required_artifacts": [`, "metadata after artifact init")
	assertOutputContains(t, metadata, `"diff.patch"`, "metadata production manifest")
	assertOutputContains(t, metadata, `"task-brief.md"`, "metadata standard mode manifest")
	assertOutputContains(t, metadata, `"redteam/final-gate-review.md"`, "metadata redteam manifest")
	assertOutputContains(t, readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"artifact.written"`, "events after artifact init")

	listOutput := runHelper(t, binary, repo, "artifact", "list", created.RunID[:24], "--json")
	var listed struct {
		RunID     string                 `json:"run_id"`
		Artifacts []artifactListedStatus `json:"artifacts"`
	}
	if err := json.Unmarshal(listOutput, &listed); err != nil {
		t.Fatalf("artifact list output is not JSON: %v\n%s", err, string(listOutput))
	}
	if listed.RunID != created.RunID || !artifactStatusListed(listed.Artifacts, "intake-classification.md", true, true) || !artifactStatusListed(listed.Artifacts, "plan.md", true, true) {
		t.Fatalf("listed = %#v, want initialized required artifacts", listed)
	}
}

func TestRunwf003ArtifactInitSafety(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo, runwf003CreateRunArgs("Artifact safety")...)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}

	writeLockMetadata(t, repo, "project_write", lockMetadata{
		Version:   "0.1",
		LockName:  "project_write",
		RunID:     created.RunID,
		OwnerPID:  os.Getpid(),
		Hostname:  mustHostname(t),
		Command:   "integration artifact init",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})
	output, err := runHelperAllowError(binary, repo, "artifact", "init", created.RunID, "--json")
	if err == nil {
		t.Fatalf("artifact init succeeded under fresh project_write lock\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"lock_conflict"`, "fresh project lock conflict")
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "runs", created.RunID, "intake-classification.md")); !os.IsNotExist(err) {
		t.Fatalf("artifact stat under lock = %v, want absent", err)
	}
	removeLock(t, repo, "project_write")

	eventsPath := filepath.Join(repo, ".kkachi", "events.jsonl")
	file, err := os.OpenFile(eventsPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	if _, err := file.WriteString(`{"version":"0.1","event_id":"evt-000003","occurred_at":"2026-04-30T03:00:00Z","run_id":"` + created.RunID + `","type":"run.created","actor":"helper","payload":{}}` + "\n"); err != nil {
		t.Fatalf("append crash event: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}
	output, err = runHelperAllowError(binary, repo, "artifact", "init", created.RunID, "--json")
	if err == nil {
		t.Fatalf("artifact init succeeded under status/event mismatch\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"last_event_id_mismatch"`, "artifact init mismatch")
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "runs", created.RunID, "intake-classification.md")); !os.IsNotExist(err) {
		t.Fatalf("artifact stat under mismatch = %v, want absent", err)
	}
}

func TestRunwf004ArtifactValidateWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo, runwf004CreateRunArgs("Validate workflow")...)
	var created struct {
		RunID    string `json:"run_id"`
		Metadata struct {
			WorkPath  string `json:"work_path"`
			WorkMode  string `json:"work_mode"`
			SOTPolicy string `json:"sot_policy"`
			Urgency   string `json:"urgency"`
		} `json:"metadata"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	beforeEvents := readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl"))

	output, err := runHelperAllowError(binary, repo, "artifact", "validate", created.RunID, "--json")
	if err == nil {
		t.Fatalf("artifact validate succeeded with pending intake\n%s", string(output))
	}
	assertOutputContains(t, output, `"status":"fail"`, "pending intake validate")
	assertOutputContains(t, output, `"name":"intake_status"`, "pending intake validate")
	if got := readFile(t, filepath.Join(repo, ".kkachi", "events.jsonl")); string(got) != string(beforeEvents) {
		t.Fatalf("artifact validate mutated events\nbefore=%s\nafter=%s", string(beforeEvents), string(got))
	}

	writeIntegrationIntake(t, repo, created.RunID, created.Metadata.WorkPath, created.Metadata.WorkMode, created.Metadata.SOTPolicy, created.Metadata.Urgency, "")
	passOutput := runHelper(t, binary, repo, "artifact", "validate", created.RunID[:24], "--gate", "intake", "--json")
	assertOutputContains(t, passOutput, `"run_id":"`+created.RunID+`"`, "passing intake validate")
	assertOutputContains(t, passOutput, `"gate":"intake"`, "passing intake validate")
	assertOutputContains(t, passOutput, `"status":"pass"`, "passing intake validate")
	assertOutputContains(t, passOutput, `"name":"required_artifacts","status":"pass"`, "passing intake validate")

	output, err = runHelperAllowError(binary, repo, "artifact", "validate", created.RunID, "--gate", "final", "--json")
	if err == nil {
		t.Fatalf("artifact validate succeeded with unsupported gate\n%s", string(output))
	}
	assertOutputContains(t, output, `"code":"unsupported_gate"`, "unsupported gate validate")
}

func TestRunwf004ArtifactValidateLightPathBWorkflow(t *testing.T) {
	repo := t.TempDir()
	if err := os.Mkdir(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	binary := buildHelperBinary(t)

	runHelper(t, binary, repo, "project", "init", "--json")
	createdOutput := runHelper(t, binary, repo,
		"run", "create",
		"--title", "Path B light validate",
		"--work-path", "B_discovery_shaping",
		"--work-mode", "light",
		"--urgency", "critical",
		"--sot-policy", "minimal_sot_before_code",
		"--execution-mode", "research",
		"--commander", "Gongmyeong",
		"--task-id", "runwf-004",
		"--json",
	)
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(createdOutput, &created); err != nil {
		t.Fatalf("run create output is not JSON: %v\n%s", err, string(createdOutput))
	}
	runHelper(t, binary, repo, "artifact", "init", created.RunID, "--json")
	writeIntegrationIntake(t, repo, created.RunID, "B_discovery_shaping", "light", "minimal_sot_before_code", "critical", "")

	output, err := runHelperAllowError(binary, repo, "artifact", "validate", created.RunID, "--json")
	if err == nil {
		t.Fatalf("artifact validate succeeded without light mode reason\n%s", string(output))
	}
	assertOutputContains(t, output, `"name":"light_mode_reason"`, "missing light reason validate")
	assertOutputContains(t, output, `"status":"fail"`, "missing light reason validate")

	writeIntegrationIntake(t, repo, created.RunID, "B_discovery_shaping", "light", "minimal_sot_before_code", "critical", "Light Mode Reason: discovery is low-risk and still records safety artifacts\n")
	passOutput := runHelper(t, binary, repo, "artifact", "validate", created.RunID, "--json")
	assertOutputContains(t, passOutput, `"status":"pass"`, "Path B light validate")
	assertOutputContains(t, passOutput, `"name":"work_path_sot_policy","status":"pass"`, "Path B light validate")
	assertOutputContains(t, passOutput, `"name":"light_mode_reason","status":"pass"`, "Path B light validate")
}

type gateCheckOutput struct {
	RunID           string      `json:"run_id"`
	Gate            string      `json:"gate"`
	Status          string      `json:"status"`
	EventID         string      `json:"event_id"`
	MissingEvidence []string    `json:"missing_evidence"`
	Checks          []gateCheck `json:"checks"`
}

type gateCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
}

func gateCheckListed(checks []gateCheck, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func buildHelperBinary(t *testing.T) string {
	t.Helper()

	binary := filepath.Join(t.TempDir(), "kkachi-agent-helper")
	cmd := exec.Command("go", "build", "-o", binary, "../../cmd/kkachi-agent-helper")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("go build failed: %v\n%s", err, string(output))
	}
	return binary
}

type lockMetadata struct {
	Version   string `json:"version"`
	LockName  string `json:"lock_name"`
	RunID     string `json:"run_id,omitempty"`
	OwnerPID  int    `json:"owner_pid"`
	Hostname  string `json:"hostname"`
	Command   string `json:"command"`
	CreatedAt string `json:"created_at"`
}

type artifactPathOnly struct {
	Path string `json:"path"`
}

type artifactListedStatus struct {
	Path     string `json:"path"`
	Required bool   `json:"required"`
	Exists   bool   `json:"exists"`
	Empty    bool   `json:"empty"`
	Bytes    int64  `json:"bytes"`
}

func runHelper(t *testing.T, binary string, repo string, args ...string) []byte {
	t.Helper()
	output, err := runHelperAllowError(binary, repo, args...)
	if err != nil {
		t.Fatalf("%s failed: %v\n%s", strings.Join(args, " "), err, string(output))
	}
	return output
}

func runHelperAllowError(binary string, repo string, args ...string) ([]byte, error) {
	cmd := exec.Command(binary, args...)
	cmd.Dir = repo
	return cmd.CombinedOutput()
}

func runwf002CreateRunArgs(title string) []string {
	return []string{
		"run", "create",
		"--title", title,
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "runwf-002",
		"--json",
	}
}

func runwf003CreateRunArgs(title string) []string {
	return []string{
		"run", "create",
		"--title", title,
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--redteam", "Reviewer",
		"--task-id", "runwf-003",
		"--json",
	}
}

func runwf004CreateRunArgs(title string) []string {
	return []string{
		"run", "create",
		"--title", title,
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
		"--task-id", "runwf-004",
		"--json",
	}
}

func writeIntegrationIntake(t *testing.T, repo string, runID string, workPath string, workMode string, sotPolicy string, urgency string, extra string) {
	t.Helper()
	content := strings.Join([]string{
		"# intake-classification.md",
		"",
		"Status: complete",
		"Work Path: " + workPath,
		"Work Mode: " + workMode,
		"SOT Policy: " + sotPolicy,
		"Urgency: " + urgency,
		strings.TrimRight(extra, "\n"),
		"",
	}, "\n")
	path := filepath.Join(repo, ".kkachi", "runs", runID, "intake-classification.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write intake classification: %v", err)
	}
}

func writeIntegrationMarkdownArtifact(t *testing.T, repo string, runID string, artifact string, body string) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "runs", runID, filepath.FromSlash(artifact))
	content := "# " + artifact + "\n\n" + strings.TrimRight(body, "\n") + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", artifact, err)
	}
}

func artifactPathListed(artifacts []artifactPathOnly, path string) bool {
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return true
		}
	}
	return false
}

func artifactStatusListed(artifacts []artifactListedStatus, path string, required bool, exists bool) bool {
	for _, artifact := range artifacts {
		if artifact.Path == path {
			return artifact.Required == required && artifact.Exists == exists && !artifact.Empty && artifact.Bytes > 0
		}
	}
	return false
}

func writeLockMetadata(t *testing.T, repo string, name string, metadata lockMetadata) {
	t.Helper()
	path := lockFilePath(repo, name)
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("marshal lock metadata: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write lock metadata: %v", err)
	}
}

func removeLock(t *testing.T, repo string, name string) {
	t.Helper()
	path := lockFilePath(repo, name)
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

func lockFilePath(repo string, name string) string {
	if name == "active_run" {
		return filepath.Join(repo, ".kkachi", "active_run.lock")
	}
	return filepath.Join(repo, ".kkachi", "project_write.lock")
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func assertOutputContains(t *testing.T, output []byte, pattern string, label string) {
	t.Helper()
	if !strings.Contains(string(output), pattern) {
		t.Fatalf("%s output = %s, want %q", label, string(output), pattern)
	}
}

func mustHostname(t *testing.T) string {
	t.Helper()
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("hostname: %v", err)
	}
	return hostname
}

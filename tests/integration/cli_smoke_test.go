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

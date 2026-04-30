//go:build integration

package integration

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
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
	if payload.RootPath != repo {
		t.Fatalf("root_path = %q, want %q", payload.RootPath, repo)
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

	appendCmd := exec.Command(binary, "event", "append", "artifact.written", "--run", "run-abc", "--payload", `{"path":"impl-log.md"}`, "--json")
	appendCmd.Dir = repo
	appendOutput, err := appendCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("event append failed: %v\n%s", err, string(appendOutput))
	}
	if !strings.Contains(string(appendOutput), `"event_id":"evt-000002"`) {
		t.Fatalf("event append output = %s, want evt-000002", string(appendOutput))
	}
	statusBytes, err := os.ReadFile(filepath.Join(repo, ".kkachi", "status.json"))
	if err != nil {
		t.Fatalf("read status after event append: %v", err)
	}
	if !strings.Contains(string(statusBytes), `"last_event_id": "evt-000002"`) {
		t.Fatalf("status after event append = %s, want evt-000002", string(statusBytes))
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

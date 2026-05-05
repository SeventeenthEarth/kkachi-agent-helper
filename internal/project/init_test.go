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

func TestInitProjectCreatesInitialState(t *testing.T) {
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

	if result.ProjectName != "kkachi-test" {
		t.Fatalf("ProjectName = %q, want kkachi-test", result.ProjectName)
	}
	if !strings.HasPrefix(result.ProjectID, "kkachi-project-"+result.ProjectName+"-") {
		t.Fatalf("ProjectID = %q, want project slug prefix", result.ProjectID)
	}
	if result.InitialEventID != "evt-000001" {
		t.Fatalf("InitialEventID = %q, want evt-000001", result.InitialEventID)
	}

	config := readText(t, filepath.Join(repo, ".kkachi", "config.yaml"))
	if !strings.Contains(config, `version: "0.1"`) || !strings.Contains(config, `name: "`+result.ProjectName+`"`) || !strings.Contains(config, `project_overlay_file`) {
		t.Fatalf("config.yaml = %q, want version and project name", config)
	}

	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["project_id"] != result.ProjectID {
		t.Fatalf("status project_id = %q, want %q", status["project_id"], result.ProjectID)
	}
	if status["last_event_id"] != "evt-000001" {
		t.Fatalf("status last_event_id = %q, want evt-000001", status["last_event_id"])
	}
	if status["updated_at"] != "2026-04-30T01:02:03Z" {
		t.Fatalf("status updated_at = %q, want fixed timestamp", status["updated_at"])
	}
	if status["active_run_id"] != nil || status["active_run_state"] != nil {
		t.Fatalf("status active run fields = %#v/%#v, want null", status["active_run_id"], status["active_run_state"])
	}

	events := strings.Split(strings.TrimSpace(readText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))), "\n")
	if len(events) != 1 {
		t.Fatalf("events line count = %d, want 1", len(events))
	}
	var event map[string]any
	if err := json.Unmarshal([]byte(events[0]), &event); err != nil {
		t.Fatalf("event line is not JSON: %v\n%s", err, events[0])
	}
	if event["event_id"] != "evt-000001" || event["type"] != "project.initialized" || event["actor"] != "helper" {
		t.Fatalf("event = %#v, want initial helper event", event)
	}

	if overlay := readText(t, filepath.Join(repo, ".kkachi", "project-overlay.yaml")); !strings.Contains(overlay, `project: "kkachi-test"`) || !strings.Contains(overlay, `backend_policy: "codex"`) {
		t.Fatalf("project-overlay.yaml = %q, want bootstrap content", overlay)
	}
	if docsMap := readText(t, filepath.Join(repo, "docs", "kkachi-docs-map.yaml")); !strings.Contains(docsMap, `roadmap: "docs/roadmap.md"`) || !strings.Contains(docsMap, `docs/architecture.md`) {
		t.Fatalf("kkachi-docs-map.yaml = %q, want docs map content", docsMap)
	}

	for _, schemaPath := range schemaPaths {
		var schema map[string]any
		readJSONFile(t, filepath.Join(repo, filepath.FromSlash(schemaPath)), &schema)
		if schema["type"] != "object" {
			t.Fatalf("%s type = %q, want object", schemaPath, schema["type"])
		}
	}
}

func TestInitProjectAllowsExistingEmptyDirectories(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	mustMkdir(t, filepath.Join(repo, ".kkachi", "schemas"))

	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	if _, err := InitProject(root, deterministicInitOptions()); err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}
}

func TestInitProjectRefusesExistingManagedFile(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	mustMkdir(t, filepath.Join(repo, ".kkachi"))
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "config.yaml"), []byte("existing\n"), 0o600); err != nil {
		t.Fatalf("write existing config: %v", err)
	}

	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	_, err = InitProject(root, deterministicInitOptions())
	assertProblemCode(t, err, "helper_state_exists")
}

func TestInitProjectRejectsEscapingKkachiSymlink(t *testing.T) {
	repo := t.TempDir()
	outside := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	if err := os.Symlink(outside, filepath.Join(repo, ".kkachi")); err != nil {
		t.Fatalf("create symlink: %v", err)
	}

	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	_, err = InitProject(root, deterministicInitOptions())
	assertProblemCode(t, err, "symlink_escape")
}

func TestInitProjectRequiresRoot(t *testing.T) {
	_, err := InitProject(Root{}, deterministicInitOptions())
	var problem *Problem
	if !errors.As(err, &problem) {
		t.Fatalf("error = %T, want *Problem", err)
	}
	if problem.Code != "repo_root_required" {
		t.Fatalf("problem.Code = %q, want repo_root_required", problem.Code)
	}
}

func deterministicInitOptions() InitOptions {
	return InitOptions{
		Now: func() time.Time {
			return time.Date(2026, 4, 30, 1, 2, 3, 0, time.UTC)
		},
		RandomHex: func(int) (string, error) {
			return "abcdef123456", nil
		},
		Bootstrap: testInitBootstrap(),
	}
}

func testInitBootstrap() InitBootstrapOptions {
	return InitBootstrapOptions{ProjectName: "kkachi-test", Stack: "go", RepoPath: "/tmp/kkachi-test", Commander: "Gongmyeong", Redteam: "Macho", DocsMapRoadmap: "docs/roadmap.md", DocsMapSpec: "docs/specs.md", DocsMapArchitecture: "docs/architecture.md", DocsMapADRDir: "docs/adr", DocsMapTODODir: "docs/todo", DocsMapSpecDir: "docs/specs", TestCommands: []string{"go test ./...", "make test"}, BackendPolicy: "codex", ExecutionMode: "production_write", SOTPolicy: "existing_sot_basis"}
}

func readText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if err := json.Unmarshal(data, target); err != nil {
		t.Fatalf("%s is not JSON: %v\n%s", path, err, string(data))
	}
}

func TestInitProjectForceReconfiguresBootstrapAndPreservesHistory(t *testing.T) {
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	initial, err := InitProject(root, deterministicInitOptions())
	if err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}
	mustMkdir(t, filepath.Join(repo, ".kkachi", "runs", "run-001"))
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "runs", "run-001", "evidence.md"), []byte("keep\n"), 0o600); err != nil {
		t.Fatalf("write run evidence: %v", err)
	}

	forcedOptions := deterministicInitOptions()
	forcedOptions.Force = true
	forcedOptions.Bootstrap.ProjectName = "kkachi-reset"
	forcedOptions.Bootstrap.Stack = "rust"
	forcedOptions.Bootstrap.TestCommands = []string{"cargo test"}
	result, err := InitProject(root, forcedOptions)
	if err != nil {
		t.Fatalf("InitProject(force) error = %v", err)
	}
	if !result.Forced || result.ReconfiguredEventID != "evt-000002" || result.ProjectID != initial.ProjectID {
		t.Fatalf("result = %#v, want forced reconfiguration preserving project id", result)
	}
	if got := readText(t, filepath.Join(repo, ".kkachi", "project-overlay.yaml")); !strings.Contains(got, `project: "kkachi-reset"`) || !strings.Contains(got, `stack: "rust"`) || !strings.Contains(got, `cargo test`) {
		t.Fatalf("project-overlay.yaml = %q, want forced bootstrap content", got)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "runs", "run-001", "evidence.md")); err != nil {
		t.Fatalf("force removed run evidence: %v", err)
	}
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["project_id"] != initial.ProjectID || status["last_event_id"] != "evt-000002" {
		t.Fatalf("status = %#v, want preserved project id and reconfigured event", status)
	}
	events := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))
	if !strings.Contains(events, `"type":"project.initialized"`) || !strings.Contains(events, `"type":"project.reconfigured"`) {
		t.Fatalf("events = %s, want initialized and reconfigured", events)
	}
}

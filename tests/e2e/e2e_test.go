package e2e

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

var helperBinary string
var projectRoot string

func TestMain(m *testing.M) {
	var err error
	projectRoot, err = filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		fmt.Fprintf(os.Stderr, "resolve project root: %v\n", err)
		os.Exit(1)
	}
	buildDir, err := os.MkdirTemp("", "kkachi-e2e-bin-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create build tempdir: %v\n", err)
		os.Exit(1)
	}
	helperBinary = filepath.Join(buildDir, "kkachi-agent-helper")
	cmd := exec.Command("go", "build", "-ldflags", "-X main.version=0.1.0", "-o", helperBinary, ".")
	cmd.Dir = projectRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Fprintf(os.Stderr, "build helper: %v\n%s\n", err, out)
		os.Exit(1)
	}
	code := m.Run()
	if err := os.RemoveAll(buildDir); err != nil {
		fmt.Fprintf(os.Stderr, "remove build tempdir: %v\n", err)
	}
	os.Exit(code)
}

type cliResult struct {
	stdout string
	stderr string
	err    error
}

func expandProjectInitArgs(args []string) []string {
	if len(args) >= 2 && args[0] == "project" && args[1] == "init" {
		for _, arg := range args[2:] {
			if arg == "--help" {
				return args
			}
		}
		for _, arg := range args[2:] {
			if arg == "--project-name" {
				return args
			}
		}
		extra := append([]string{}, args[2:]...)
		base := []string{"project", "init", "--project-name", "kkachi-test", "--stack", "go", "--repo-path", "/tmp/kkachi-test", "--commander", "Gongmyeong", "--redteam", "Macho", "--docs-map-roadmap", "docs/roadmap.md", "--docs-map-spec", "docs/specs.md", "--docs-map-architecture", "docs/architecture.md", "--docs-map-adr-dir", "docs/adr", "--docs-map-todo-dir", "docs/todo", "--docs-map-spec-dir", "docs/specs", "--test-commands", "go test ./...,make test", "--backend-policy", "codex", "--execution-mode", "production_write", "--sot-policy", "existing_sot_basis"}
		return append(base, extra...)
	}
	return args
}

func runCLI(t *testing.T, dir string, args ...string) cliResult {
	t.Helper()
	args = expandProjectInitArgs(args)
	cmd := exec.Command(helperBinary, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	stderr := ""
	if ee, ok := err.(*exec.ExitError); ok {
		stderr = string(ee.Stderr)
	}
	return cliResult{stdout: string(out), stderr: stderr, err: err}
}

func requireCLI(t *testing.T, dir string, args ...string) cliResult {
	t.Helper()
	res := runCLI(t, dir, args...)
	if res.err != nil {
		t.Fatalf("%s failed: %v\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), res.err, res.stdout, res.stderr)
	}
	return res
}

func requireFailCLI(t *testing.T, dir string, args ...string) cliResult {
	t.Helper()
	res := runCLI(t, dir, args...)
	if res.err == nil {
		t.Fatalf("%s unexpectedly succeeded\nstdout:\n%s\nstderr:\n%s", strings.Join(args, " "), res.stdout, res.stderr)
	}
	return res
}

func requireContains(t *testing.T, text, want, label string) {
	t.Helper()
	if !strings.Contains(text, want) {
		t.Fatalf("%s missing %q\n--- content ---\n%s", label, want, text)
	}
}

func requireNotContains(t *testing.T, text, want, label string) {
	t.Helper()
	if strings.Contains(text, want) {
		t.Fatalf("%s unexpectedly contained %q\n--- content ---\n%s", label, want, text)
	}
}

func TestStandardHelpUX(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{name: "top-level help command", args: []string{"help"}, want: []string{"kkachi-agent-helper", "Usage:", "JSON behavior:"}},
		{name: "top-level help flag", args: []string{"--help"}, want: []string{"kkachi-agent-helper", "capabilities", "--json"}},
		{name: "project group", args: []string{"project", "--help"}, want: []string{"kkachi-agent-helper project", "init", "status", "doctor"}},
		{name: "project init", args: []string{"project", "init", "--help"}, want: []string{"kkachi-agent-helper project init", "--project-name <name> (required)", "--force"}},
		{name: "run group", args: []string{"run", "--help"}, want: []string{"kkachi-agent-helper run", "create", "activate <run_id>"}},
		{name: "run create", args: []string{"run", "create", "--help"}, want: []string{"kkachi-agent-helper run create", "--title <title> (required)", "--backend-evidence <auto|required|not_applicable>"}},
		{name: "artifact group", args: []string{"artifact", "--help"}, want: []string{"kkachi-agent-helper artifact", "validate <run_id> [--gate intake]", "--gate intake"}},
		{name: "gate group", args: []string{"gate", "--help"}, want: []string{"kkachi-agent-helper gate", "check <run_id> <gate>", "intake, sot, roadmap"}},
		{name: "schema group", args: []string{"schema", "--help"}, want: []string{"kkachi-agent-helper schema", "validate <file> --schema <schema>", "migrate --from <version> --to <version>"}},
		{name: "event group", args: []string{"event", "--help"}, want: []string{"kkachi-agent-helper event", "append <type>", "--payload <json-object> (required)"}},
		{name: "lock group", args: []string{"lock", "--help"}, want: []string{"kkachi-agent-helper lock", "recover <active-run|project-write|all>", "--reason <text> (required)"}},
		{name: "diagnostics group", args: []string{"diagnostics", "--help"}, want: []string{"kkachi-agent-helper diagnostics", "export", "--output <repo-relative-path>"}},
		{name: "phase plan", args: []string{"phase-plan", "--help"}, want: []string{"kkachi-agent-helper phase-plan", "supported", "validate <run_id>"}},
		{name: "approval", args: []string{"approval", "--help"}, want: []string{"kkachi-agent-helper approval", "request <run_id>", "--decision <approved|rejected>"}},
		{name: "help help", args: []string{"help", "help"}, want: []string{"kkachi-agent-helper help", "[command] [subcommand]", "JSON behavior:"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := requireCLI(t, dir, tc.args...)
			if res.stderr != "" {
				t.Fatalf("%s wrote stderr: %s", strings.Join(tc.args, " "), res.stderr)
			}
			for _, want := range tc.want {
				requireContains(t, res.stdout, want, tc.name)
			}
		})
	}

	jsonHelp := requireCLI(t, dir, "--json", "phase-plan", "--help")
	var payload struct {
		Command      string `json:"command"`
		Status       string `json:"status"`
		Usage        string `json:"usage"`
		JSONBehavior string `json:"json_behavior"`
	}
	if err := json.Unmarshal([]byte(jsonHelp.stdout), &payload); err != nil {
		t.Fatalf("phase-plan help output is not JSON: %v\n%s", err, jsonHelp.stdout)
	}
	if payload.Command != "kkachi-agent-helper phase-plan" || payload.Status != "supported" || payload.Usage == "" || !strings.Contains(payload.JSONBehavior, "Failing validation exits 3") {
		t.Fatalf("payload = %#v, want supported phase-plan help JSON", payload)
	}
}

func requireFileContains(t *testing.T, path, want, label string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	requireContains(t, string(data), want, label)
}

func TestAlign008DocsCompatibilityContract(t *testing.T) {
	type docContractCase struct {
		rel   string
		wants []string
	}

	assertDocContains := func(t *testing.T, tc docContractCase) {
		t.Helper()
		text := mustRead(t, filepath.Join(projectRoot, filepath.FromSlash(tc.rel)))
		for _, want := range tc.wants {
			requireContains(t, text, want, tc.rel)
		}
	}

	for _, tc := range []docContractCase{
		{
			rel: "README.md",
			wants: []string{
				"`align-001` through `align-008`",
				"## KHS/KAH compatibility contract",
				"KHS owns workflow policy",
				"kkachi-agent-helper capabilities --json",
				"go install github.com/SeventeenthEarth/kkachi-agent-helper@latest",
				"tested/recommended KAH versions",
				"`project init` / `project init --force`",
				"never installs Hermes/KHS skill content",
			},
		},
		{
			rel: "docs/specs.md",
			wants: []string{
				"KAH owns deterministic state only after KHS or a user chooses to apply the Kkachi workflow",
				"does not decide whether KHS should trigger",
				"install Hermes/KHS skill content",
				"KHS `main` may use KAH `@latest`",
				"tested/recommended KAH versions",
				"KAH bootstrap must not install, update, or vendor Hermes skill content",
				"KHS owns workflow policy, phase applicability, phase ordering",
				"## 17. Compatibility contract",
				"`project init` and `project init --force` are the KAH bootstrap/reconfiguration contract",
			},
		},
		{
			rel: "docs/compatibility.md",
			wants: []string{
				"KHS/KAH integration",
				"KHS may consume KAH `@latest`",
				"kkachi-agent-helper capabilities --json",
				"tested/recommended KAH versions",
				"`project init --force` reconfigures bootstrap files without deleting status",
				"KAH does not install KHS/Hermes skill content, templates, registries, or evaluation assets",
				"KHS owns workflow policy",
				"KAH must not become the workflow-policy owner, planner, backend selector, code reviewer, KAB session controller, or Hermes skill installer",
			},
		},
		{
			rel: "docs/roadmap.md",
			wants: []string{
				"| align-008 | KHS/KAH compatibility contract docs | Completed |",
				"`capabilities --json` activation checks",
				"tested/recommended release versions",
				"no Hermes skill installation by KAH",
			},
		},
		{
			rel: "docs/TODO-ALIGN.md",
			wants: []string{
				"### align-008 — KHS/KAH compatibility contract docs",
				"Status: Completed",
				"README, specs, and compatibility docs now state the KHS/KAH ownership boundary consistently",
				"`capabilities --json` as the preferred KHS `@latest` activation check",
				"`project init` / `project init --force` as the bootstrap/reconfiguration contract",
			},
		},
	} {
		t.Run(tc.rel, func(t *testing.T) {
			assertDocContains(t, tc)
		})
	}
}

func jsonFieldString(t *testing.T, raw []byte, field string) string {
	t.Helper()
	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		t.Fatalf("unmarshal JSON for %s: %v\n%s", field, err, raw)
	}
	for _, part := range strings.Split(field, ".") {
		m, ok := value.(map[string]any)
		if !ok {
			t.Fatalf("field %s could not descend through %T", field, value)
		}
		value, ok = m[part]
		if !ok || value == nil {
			t.Fatalf("field %s missing/null in %s", field, raw)
		}
	}
	switch v := value.(type) {
	case string:
		return v
	case float64:
		return fmt.Sprintf("%g", v)
	case bool:
		return fmt.Sprintf("%t", v)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

func repo(t *testing.T, name string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(dir, ".git"), 0o755); err != nil {
		t.Fatalf("create repo: %v", err)
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	return abs
}

func eventCount(t *testing.T, repo string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(repo, ".kkachi", "events.jsonl"))
	if err != nil {
		t.Fatalf("read events: %v", err)
	}
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return 0
	}
	return len(strings.Split(trimmed, "\n"))
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func writeArtifact(t *testing.T, repo, runID, path, body string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, ".kkachi", "runs", runID, filepath.FromSlash(path)), body+"\n")
}

func writeCompleteArtifacts(t *testing.T, repo, runID, taskID string, includeBackend bool) {
	t.Helper()
	artifacts := map[string]string{
		"intake-classification.md":     "Status: complete\nWork Path: A_development_execution\nWork Mode: standard\nSOT Policy: existing_sot_basis\nUrgency: normal",
		"sot-basis.md":                 "Status: complete\nSource: docs/specs.md",
		"roadmap-update.md":            "Status: complete\nTrace: docs/roadmap.md " + taskID,
		"acceptance-criteria.md":       "Status: complete\nCriteria: black-box CLI flows pass",
		"plan.md":                      "Status: complete\nPlan: execute public CLI surfaces only",
		"checklist.md":                 "Status: complete\n- [x] required gates pass",
		"diff.patch":                   "diff --git a/file b/file\n+e2e evidence",
		"impl-log.md":                  "Status: complete\nImplementation: harness verified",
		"review.md":                    "Status: complete\nReview: accepted",
		"redteam/impl-review.md":       "Status: complete\nReview: no implementation blockers",
		"redteam/test-review.md":       "Status: complete\nReview: test coverage accepted",
		"redteam/final-gate-review.md": "Status: complete\nReview: final gate ready",
		"test-log.md":                  "Status: complete\nTests: e2e harness",
		"verification.md":              "Status: complete\nVerdict: pass",
		"docs-update.md":               "Status: complete\nChanged Docs: docs/roadmap.md",
		"final-report.md":              "Status: complete\nReport: black-box flow complete",
	}
	for path, body := range artifacts {
		writeArtifact(t, repo, runID, path, body)
	}
	if includeBackend {
		writeArtifact(t, repo, runID, "selected-cli.json", `{
  "version": "0.1",
  "status": "supported",
  "backend_type": "codex",
  "adapter_type": "openai-codex",
  "source_ledger_ref": "pilot-local-ledger",
  "caveats": []
}`)
		writeArtifact(t, repo, runID, "capability-check.md", "Status: complete\nBackend Type: codex\nAdapter Type: openai-codex\nCapability: local helper acceptance workflow can preserve bridge evidence.")
		writeArtifact(t, repo, runID, "bridge-session-snapshot.json", `{
  "version": "0.1",
  "session_id": "pilot-local-session",
  "backend_type": "codex",
  "adapter_type": "openai-codex",
  "state": "closed",
  "lifecycle_class": "local_acceptance",
  "open_pendings": 0
}`)
		writeArtifact(t, repo, runID, "bridge-events.md", "Status: complete\nEvent: bridge-shaped session snapshot closed with open_pendings 0.")
		writeArtifact(t, repo, runID, "cli-output.md", "Status: complete\nOutput: helper commands completed.")
	}
}

func createRun(t *testing.T, repo, taskID, executionMode string) string {
	t.Helper()
	res := requireCLI(t, repo, "run", "create", "--title", taskID+" e2e", "--work-path", "A_development_execution", "--work-mode", "standard", "--urgency", "normal", "--sot-policy", "existing_sot_basis", "--execution-mode", executionMode, "--commander", "Gongmyeong", "--task-id", taskID, "--json")
	return jsonFieldString(t, []byte(res.stdout), "run_id")
}

func copyDir(t *testing.T, src, dst string) {
	t.Helper()
	entries, err := os.ReadDir(src)
	if err != nil {
		t.Fatalf("read dir %s: %v", src, err)
	}
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dst, err)
	}
	for _, entry := range entries {
		s := filepath.Join(src, entry.Name())
		d := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			copyDir(t, s, d)
			continue
		}
		data, err := os.ReadFile(s)
		if err != nil {
			t.Fatalf("read %s: %v", s, err)
		}
		if err := os.WriteFile(d, data, 0o644); err != nil {
			t.Fatalf("write %s: %v", d, err)
		}
	}
}

func appendSyntheticEvent(t *testing.T, repo string, payload string) string {
	t.Helper()
	path := filepath.Join(repo, ".kkachi/events.jsonl")
	events := strings.TrimRight(mustRead(t, path), "\n")
	nextID := fmt.Sprintf("evt-%06d", eventCount(t, repo)+1)
	writeFile(t, path, events+"\n"+strings.Replace(payload, "${event_id}", nextID, 1)+"\n")
	return nextID
}

func writeLock(t *testing.T, repo, lockName string, ownerPID int, host, command, createdAt, runID string) {
	t.Helper()
	file := "project_write.lock"
	if lockName == "active_run" {
		file = "active_run.lock"
	}
	payload := map[string]any{
		"version":    "0.1",
		"lock_name":  lockName,
		"owner_pid":  ownerPID,
		"hostname":   host,
		"command":    command,
		"created_at": createdAt,
	}
	if runID != "" {
		payload["run_id"] = runID
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, ".kkachi", file), string(data)+"\n")
}

func TestProjectInitForceReconfiguresBootstrapE2E(t *testing.T) {
	r := repo(t, "force-reconfigure")
	init := requireCLI(t, r, "project", "init", "--json")
	projectID := jsonFieldString(t, []byte(init.stdout), "project_id")
	runID := createRun(t, r, "force-001", "production_write")
	writeArtifact(t, r, runID, "plan.md", "Status: complete\nPlan survives force reconfiguration.")

	force := requireCLI(t, r, "project", "init", "--project-name", "kkachi-reset", "--stack", "rust", "--repo-path", "/tmp/kkachi-reset", "--commander", "Sunji", "--redteam", "Macho", "--docs-map-roadmap", "docs/ROADMAP.md", "--docs-map-spec", "docs/SPEC.md", "--docs-map-architecture", "docs/ARCHITECTURE.md", "--docs-map-adr-dir", "docs/decisions", "--docs-map-todo-dir", "docs/tasks", "--docs-map-spec-dir", "docs/specifications", "--test-commands", "cargo test,make verify", "--backend-policy", "codex", "--execution-mode", "readiness_hardening", "--sot-policy", "existing_sot_basis", "--force", "--json")
	reconfiguredID := jsonFieldString(t, []byte(force.stdout), "project_id")
	if reconfiguredID != projectID {
		t.Fatalf("project_id after force = %q, want preserved %q", reconfiguredID, projectID)
	}
	requireContains(t, force.stdout, `"forced":true`, "force init JSON")
	requireContains(t, force.stdout, `"reconfigured_event_id":"evt-000003"`, "force init JSON")
	requireFileContains(t, filepath.Join(r, ".kkachi", "project-overlay.yaml"), `project: "kkachi-reset"`, "force overlay")
	requireFileContains(t, filepath.Join(r, ".kkachi", "project-overlay.yaml"), `stack: "rust"`, "force overlay")
	requireFileContains(t, filepath.Join(r, "docs", "kkachi-docs-map.yaml"), `roadmap: "docs/ROADMAP.md"`, "force docs map")
	requireFileContains(t, filepath.Join(r, ".kkachi", "runs", runID, "plan.md"), "Plan survives force reconfiguration", "force preserved run artifact")
	requireFileContains(t, filepath.Join(r, ".kkachi", "events.jsonl"), `"type":"project.reconfigured"`, "force event log")
	status := requireCLI(t, r, "project", "status", "--json")
	requireContains(t, status.stdout, `"event_tail_id":"evt-000003"`, "force status")
}

func TestProjectInitSchemaAndBootstrapFlow(t *testing.T) {
	r := repo(t, "repo")
	init := requireCLI(t, r, "project", "init", "--json")
	requireContains(t, init.stdout, `"project_name":"kkachi-test"`, "init JSON")
	for _, rel := range []string{
		".kkachi/config.yaml", ".kkachi/status.json", ".kkachi/events.jsonl", ".kkachi/project-overlay.yaml", "docs/kkachi-docs-map.yaml", ".kkachi/schemas/config.schema.json", ".kkachi/schemas/status.schema.json", ".kkachi/schemas/event.schema.json", ".kkachi/schemas/run-metadata.schema.json", ".kkachi/schemas/selected-cli.schema.json", ".kkachi/schemas/bridge-session-snapshot.schema.json",
	} {
		if _, err := os.Stat(filepath.Join(r, filepath.FromSlash(rel))); err != nil {
			t.Fatalf("missing expected file %s: %v", rel, err)
		}
	}
	status := requireCLI(t, r, "project", "status", "--json")
	requireContains(t, status.stdout, `"health":"ok"`, "status JSON")
	doctor := requireCLI(t, r, "project", "doctor", "--json")
	requireContains(t, doctor.stdout, `"failed":0`, "doctor JSON")

	requireCLI(t, r, "schema", "validate", ".kkachi/config.yaml", "--schema", "config", "--json")
	requireCLI(t, r, "schema", "validate", ".kkachi/status.json", "--schema", ".kkachi/schemas/status.schema.json", "--json")
	requireCLI(t, r, "schema", "validate", ".kkachi/events.jsonl", "--schema", "event", "--json")

	removed := requireFailCLI(t, r, "install", "templates", "--json")
	requireContains(t, removed.stderr, `unknown command`, "removed install command")

	exportDryRun := requireCLI(t, r, "schema", "export", "--all", "--dry-run", "--json")
	requireContains(t, exportDryRun.stdout, `"dry_run":true`, "schema export dry-run")
	exported := requireCLI(t, r, "schema", "export", "--all", "--json")
	requireContains(t, exported.stdout, `"unchanged":[`, "schema export unchanged")
	requireNotContains(t, exported.stdout, `"event_id"`, "schema export unchanged")
	idempotent := requireCLI(t, r, "schema", "export", "--all", "--json")
	requireContains(t, idempotent.stdout, `"written":null`, "schema export idempotent")
	requireNotContains(t, idempotent.stdout, `"event_id"`, "schema export idempotent")
	migrateDryRun := requireCLI(t, r, "schema", "migrate", "--from", "0.1", "--to", "0.1", "--dry-run", "--json")
	requireContains(t, migrateDryRun.stdout, `"would_backup":[`, "schema migrate dry-run")
	migrated := requireCLI(t, r, "schema", "migrate", "--from", "0.1", "--to", "0.1", "--json")
	requireContains(t, migrated.stdout, `"migration":"noop-0.1-to-0.1"`, "schema migrate")
	backupPath := jsonFieldString(t, []byte(migrated.stdout), "backup_path")
	if _, err := os.Stat(filepath.Join(r, filepath.FromSlash(backupPath), ".kkachi/status.json")); err != nil {
		t.Fatalf("schema migration backup missing status.json: %v", err)
	}
	unknown := requireFailCLI(t, r, "schema", "migrate", "--from", "9.9", "--to", "0.1", "--json")
	requireContains(t, unknown.stderr, `"code":"schema_migration_unknown_source_version"`, "schema migrate unknown source")
}

func TestArtifactMutationCommandsE2E(t *testing.T) {
	r := repo(t, "artifact-mutation")
	requireCLI(t, r, "project", "init", "--json")
	runID := createRun(t, r, "align-006", "production_write")
	requireCLI(t, r, "artifact", "init", runID, "--json")

	writeFile(t, filepath.Join(r, "sources", "acceptance.md"), "Status: complete\nCriteria: artifact mutation commands preserve gate evidence\n")
	writeFile(t, filepath.Join(r, "sources", "plan.md"), "Status: complete\nPlan: use KAH-managed artifact mutation commands\n")
	writeFile(t, filepath.Join(r, "sources", "checklist.md"), "- [x] write canonical artifacts\n- [x] append checklist evidence\n")

	acceptance := requireCLI(t, r, "artifact", "write", runID[:24], "acceptance-criteria.md", "--from", "sources/acceptance.md", "--json")
	requireContains(t, acceptance.stdout, `"operation":"write"`, "acceptance artifact write")
	requireContains(t, acceptance.stdout, `"artifact_kind":"canonical"`, "acceptance artifact write")
	requireContains(t, acceptance.stdout, `"event_id":"evt-000004"`, "acceptance artifact write")

	plan := requireCLI(t, r, "artifact", "write", runID, "plan.md", "--from", "sources/plan.md", "--json")
	requireContains(t, plan.stdout, `"path":"plan.md"`, "plan artifact write")
	requireContains(t, plan.stdout, `"event_id":"evt-000005"`, "plan artifact write")

	appendChecklist := requireCLI(t, r, "artifact", "append", runID, "checklist.md", "--from", "sources/checklist.md", "--json")
	requireContains(t, appendChecklist.stdout, `"operation":"append"`, "checklist append")
	requireContains(t, appendChecklist.stdout, `"event_id":"evt-000006"`, "checklist append")

	completeChecklist := requireCLI(t, r, "artifact", "set-status", runID, "checklist.md", "--status", "complete", "--json")
	requireContains(t, completeChecklist.stdout, `"operation":"set-status"`, "checklist set-status")
	requireContains(t, completeChecklist.stdout, `"status":"complete"`, "checklist set-status")
	requireContains(t, completeChecklist.stdout, `"event_id":"evt-000007"`, "checklist set-status")

	planGate := requireCLI(t, r, "gate", "check", runID, "plan", "--json")
	requireContains(t, planGate.stdout, `"status":"pass"`, "plan gate after artifact mutation")
	requireContains(t, planGate.stdout, `"event_id":"evt-000008"`, "plan gate after artifact mutation")
	requireFileContains(t, filepath.Join(r, ".kkachi", "runs", runID, "plan.md"), "KAH-managed artifact mutation commands", "mutated plan artifact")
	requireFileContains(t, filepath.Join(r, ".kkachi", "runs", runID, "checklist.md"), "Status: complete", "mutated checklist artifact")
	requireFileContains(t, filepath.Join(r, ".kkachi", "events.jsonl"), `"operation":"write"`, "artifact write event")
	requireFileContains(t, filepath.Join(r, ".kkachi", "events.jsonl"), `"operation":"append"`, "artifact append event")
	requireFileContains(t, filepath.Join(r, ".kkachi", "events.jsonl"), `"operation":"set-status"`, "artifact set-status event")

	supplemental := requireFailCLI(t, r, "artifact", "write", runID, "supplemental/note.md", "--from", "sources/plan.md", "--json")
	requireContains(t, supplemental.stderr, `"code":"artifact_path_invalid"`, "supplemental artifact rejection")
	if got := eventCount(t, r); got != 8 {
		t.Fatalf("event count after rejected supplemental mutation = %d, want 8", got)
	}
}

func TestRunArtifactGateAndCoherenceFlow(t *testing.T) {
	r := repo(t, "repo")
	requireCLI(t, r, "project", "init", "--json")
	runID := createRun(t, r, "runwf-001", "production_write")
	if !regexp.MustCompile(`^run-\d{8}T\d{6}Z-[0-9a-f]{12}$`).MatchString(runID) {
		t.Fatalf("unexpected run id: %s", runID)
	}
	prefix := runID[:24]
	show := requireCLI(t, r, "run", "show", prefix, "--json")
	requireContains(t, show.stdout, `"run_id":"`+runID+`"`, "run show")
	artifactInit := requireCLI(t, r, "artifact", "init", runID, "--json")
	requireContains(t, artifactInit.stdout, `"event_id":"evt-000003"`, "artifact init")
	pending := requireFailCLI(t, r, "artifact", "validate", runID, "--json")
	requireContains(t, pending.stdout, `"status":"fail"`, "pending artifact validate")
	writeArtifact(t, r, runID, "intake-classification.md", "Status: complete\nWork Path: A_development_execution\nWork Mode: standard\nSOT Policy: existing_sot_basis\nUrgency: normal")
	validate := requireCLI(t, r, "artifact", "validate", prefix, "--gate", "intake", "--json")
	requireContains(t, validate.stdout, `"status":"pass"`, "artifact validate")
	if got := eventCount(t, r); got != 3 {
		t.Fatalf("events after read-only artifact validate = %d, want 3", got)
	}
	intakeGate := requireCLI(t, r, "gate", "check", prefix, "intake", "--json")
	requireContains(t, intakeGate.stdout, `"event_id":"evt-000004"`, "intake gate")
	writeArtifact(t, r, runID, "sot-basis.md", "Status: complete\nSource: docs/specs.md")
	writeArtifact(t, r, runID, "acceptance-criteria.md", "Status: complete\nCriteria: deterministic gates pass")
	writeArtifact(t, r, runID, "plan.md", "Status: complete\nPlan: implement gate checks")
	writeArtifact(t, r, runID, "checklist.md", "Status: complete\n- [x] SOT gate\n- [x] Roadmap gate\n- [x] Plan gate")
	requireCLI(t, r, "gate", "check", runID, "sot", "--json")
	requireCLI(t, r, "gate", "check", runID, "roadmap", "--json")
	requireCLI(t, r, "gate", "check", runID, "plan", "--json")
	activate := requireCLI(t, r, "run", "activate", runID, "--json")
	requireContains(t, activate.stdout, `"state":"active"`, "run activate")
	closeRun := requireCLI(t, r, "run", "close", runID, "--json")
	requireContains(t, closeRun.stdout, `"state":"closed"`, "run close")
	event := requireCLI(t, r, "event", "append", "artifact.written", "--run", "run-abc", "--payload", `{"path":"impl-log.md"}`, "--json")
	requireContains(t, event.stdout, `"event_id":"evt-000010"`, "event append")

	adapterRunID := createRun(t, r, "gates-003", "adapter_qa")
	requireCLI(t, r, "artifact", "init", adapterRunID, "--json")
	backendFail := requireFailCLI(t, r, "gate", "check", adapterRunID, "backend", "--json")
	requireContains(t, backendFail.stdout, `"status":"fail"`, "backend pending")
	writeArtifact(t, r, adapterRunID, "selected-cli.json", `{"version":"0.1","status":"supported","backend_type":"codex","adapter_type":"openai-codex","source_ledger_ref":"docs/ledger.md#codex","caveats":[]}`)
	writeArtifact(t, r, adapterRunID, "capability-check.md", "Status: complete\nBackend Type: codex\nAdapter Type: openai-codex\nCapability: thread resume checked")
	writeArtifact(t, r, adapterRunID, "bridge-session-snapshot.json", `{"session_id":"session-123","backend_type":"codex","adapter_type":"openai-codex","state":"running","lifecycle_class":"interactive","open_pendings":0}`)
	writeArtifact(t, r, adapterRunID, "bridge-events.md", "Status: complete\nEvent: bridge opened a codex session and emitted output")
	backendPass := requireCLI(t, r, "gate", "check", adapterRunID, "backend", "--json")
	requireContains(t, backendPass.stdout, `"status":"pass"`, "backend pass")
	crashEventID := appendSyntheticEvent(t, r, `{"version":"0.1","event_id":"${event_id}","occurred_at":"2026-04-30T03:00:00Z","run_id":"run-abc","type":"run.created","actor":"helper","payload":{}}`)
	mismatch := requireFailCLI(t, r, "event", "append", "artifact.written", "--run", "run-abc", "--payload", `{}`, "--json")
	requireContains(t, mismatch.stderr, `"code":"last_event_id_mismatch"`, "mismatch append")
	doctorMismatch := requireFailCLI(t, r, "project", "doctor", "--json")
	requireContains(t, doctorMismatch.stdout, `"health":"fail"`, "mismatch doctor")
	requireContains(t, doctorMismatch.stdout, `"expected":"`+crashEventID+`"`, "mismatch doctor expected tail")
	retry := requireFailCLI(t, r, "project", "init", "--json")
	requireContains(t, retry.stderr, `"code":"helper_state_exists"`, "retry init")
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestLockRecoveryFlow(t *testing.T) {
	r := repo(t, "locks")
	requireCLI(t, r, "project", "init", "--json")
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown"
	}
	fresh := time.Now().UTC().Format(time.RFC3339)
	writeLock(t, r, "project_write", os.Getpid(), host, "e2e fresh writer", fresh, "")
	blocked := requireFailCLI(t, r, "run", "create", "--title", "Blocked", "--work-path", "A_development_execution", "--work-mode", "standard", "--urgency", "normal", "--sot-policy", "existing_sot_basis", "--execution-mode", "production_write", "--commander", "Gongmyeong", "--task-id", "runwf-002", "--json")
	requireContains(t, blocked.stderr, `"code":"lock_conflict"`, "fresh project lock")
	if got := eventCount(t, r); got != 1 {
		t.Fatalf("events after refused create = %d, want 1", got)
	}
	os.Remove(filepath.Join(r, ".kkachi/project_write.lock"))
	runID := createRun(t, r, "runwf-002", "production_write")
	writeLock(t, r, "active_run", os.Getpid(), host, "e2e active lifecycle", fresh, runID)
	activeBlocked := requireFailCLI(t, r, "run", "activate", runID, "--json")
	requireContains(t, activeBlocked.stderr, `"code":"lock_conflict"`, "fresh active lock")
	requireFileContains(t, filepath.Join(r, ".kkachi/status.json"), `"active_run_id": null`, "status after active lock")
	os.Remove(filepath.Join(r, ".kkachi/active_run.lock"))
	writeLock(t, r, "project_write", 999999, "other-host", "e2e stale writer", "2026-04-30T02:33:05Z", "")
	stale := requireFailCLI(t, r, "run", "create", "--title", "Blocked stale", "--work-path", "A_development_execution", "--work-mode", "standard", "--urgency", "normal", "--sot-policy", "existing_sot_basis", "--execution-mode", "production_write", "--commander", "Gongmyeong", "--task-id", "runwf-002", "--json")
	requireContains(t, stale.stderr, `"code":"lock_stale_recovery_required"`, "stale project lock")
	doctor := requireCLI(t, r, "project", "doctor", "--json")
	requireContains(t, doctor.stdout, `"health":"warning"`, "stale doctor")
	recover := requireCLI(t, r, "lock", "recover", "project-write", "--reason", "e2e stale recovery", "--json")
	requireContains(t, recover.stdout, `"lock_name":"project_write"`, "lock recover")
	if _, err := os.Stat(filepath.Join(r, ".kkachi/project_write.lock")); !os.IsNotExist(err) {
		t.Fatalf("project_write.lock still exists after recovery: %v", err)
	}
	requireContains(t, mustRead(t, filepath.Join(r, ".kkachi/events.jsonl")), `"type":"lock.recovered"`, "events after recovery")
	post := createRun(t, r, "runwf-002", "production_write")
	if post == "" {
		t.Fatal("missing run id after recovery")
	}
}

func TestPilotGoldenWorkspacesAndFailureScenarios(t *testing.T) {
	successRepo := repo(t, "success")
	requireCLI(t, successRepo, "project", "init", "--json")
	runID := createRun(t, successRepo, "pilot-001", "production_write")
	requireCLI(t, successRepo, "artifact", "init", runID, "--json")
	artifactList := requireCLI(t, successRepo, "artifact", "list", runID, "--json")
	requireContains(t, artifactList.stdout, `"run_id":"`+runID+`"`, "artifact list")
	requireContains(t, artifactList.stdout, `"path":"intake-classification.md","required":true,"exists":true`, "artifact list")
	writeCompleteArtifacts(t, successRepo, runID, "pilot-001", false)
	validate := requireCLI(t, successRepo, "artifact", "validate", runID, "--gate", "intake", "--json")
	requireContains(t, validate.stdout, `"status":"pass"`, "artifact validate")
	for _, gate := range []string{"intake", "sot", "roadmap", "plan", "implementation", "review", "verification", "docs"} {
		checked := requireCLI(t, successRepo, "gate", "check", runID, gate, "--json")
		requireContains(t, checked.stdout, `"status":"pass"`, gate+" gate")
		requireContains(t, checked.stdout, `.kkachi/runs/`+runID+`/gate-reports/`+gate+`.json`, gate+" report path")
	}
	final := requireCLI(t, successRepo, "gate", "final", runID, "--json")
	requireContains(t, final.stdout, `"status":"pass"`, "final gate")
	requireFileContains(t, filepath.Join(successRepo, ".kkachi/runs", runID, "gate-reports/final.json"), `"status": "pass"`, "final gate report")

	missingRepo := repo(t, "missing")
	requireCLI(t, missingRepo, "project", "init", "--json")
	missingRunID := createRun(t, missingRepo, "pilot-001", "production_write")
	requireCLI(t, missingRepo, "artifact", "init", missingRunID, "--json")
	writeFile(t, filepath.Join(missingRepo, ".kkachi/runs", missingRunID, "acceptance-criteria.md"), "")
	missing := requireFailCLI(t, missingRepo, "gate", "check", missingRunID, "plan", "--json")
	requireContains(t, missing.stdout, `"status":"fail"`, "missing artifacts plan gate")
	requireContains(t, missing.stdout, `acceptance-criteria.md`, "missing artifacts plan gate")

	ambiguousRepo := repo(t, "ambiguous")
	requireCLI(t, ambiguousRepo, "project", "init", "--json")
	createRun(t, ambiguousRepo, "pilot-001", "production_write")
	createRun(t, ambiguousRepo, "pilot-001", "production_write")
	ambiguous := requireFailCLI(t, ambiguousRepo, "run", "show", "run", "--json")
	requireContains(t, ambiguous.stderr, `"code":"run_id_ambiguous"`, "ambiguous run stderr")

	lockRepo := repo(t, "lock-conflict")
	requireCLI(t, lockRepo, "project", "init", "--json")
	host, _ := os.Hostname()
	writeLock(t, lockRepo, "project_write", os.Getpid(), host, "pilot-001 lock conflict", time.Now().UTC().Format(time.RFC3339), "")
	locked := requireFailCLI(t, lockRepo, "event", "append", "artifact.written", "--run", "run-pilot", "--payload", `{}`, "--json")
	requireContains(t, locked.stderr, `"code":"lock_conflict"`, "lock conflict")
	if got := eventCount(t, lockRepo); got != 1 {
		t.Fatalf("lock conflict appended events unexpectedly: %d", got)
	}

	safetyRepo := repo(t, "safety")
	requireCLI(t, safetyRepo, "project", "init", "--json")
	writeFile(t, filepath.Join(filepath.Dir(safetyRepo), "outside-status.json"), "{}\n")
	unsafe := requireFailCLI(t, safetyRepo, "schema", "validate", "../outside-status.json", "--schema", "status", "--json")
	requireContains(t, unsafe.stderr, `"code":"path_escape"`, "unsafe path")
	badJSON := requireFailCLI(t, safetyRepo, "event", "append", "artifact.written", "--run", "run-pilot", "--payload", `{`, "--json")
	requireContains(t, badJSON.stderr, `"code":"payload_invalid_json"`, "bad JSON")
	requireNotContains(t, mustRead(t, filepath.Join(safetyRepo, ".kkachi/events.jsonl")), "run-pilot", "bad JSON events")

	for _, fixture := range []struct{ name, cmd, want string }{
		{"schema-mismatch", "schema", `"name":"project_id"`},
		{"status-event-mismatch", "doctor", `"name":"coherence"`},
		{"invalid-events-jsonl", "doctor", `invalid JSON`},
	} {
		gr := repo(t, "golden-"+fixture.name)
		copyDir(t, filepath.Join(projectRoot, "tests/e2e/golden-workspaces/pilot-001", fixture.name, ".kkachi"), filepath.Join(gr, ".kkachi"))
		var res cliResult
		if fixture.cmd == "schema" {
			res = requireFailCLI(t, gr, "schema", "validate", ".kkachi/status.json", "--schema", "status", "--json")
		} else {
			res = requireFailCLI(t, gr, "project", "doctor", "--json")
		}
		requireContains(t, res.stdout, fixture.want, fixture.name)
	}
}

func TestDiagnosticsExportRedaction(t *testing.T) {
	r := repo(t, "diagnostics")
	secret := "sk-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	requireCLI(t, r, "project", "init", "--json")
	runID := createRun(t, r, "pilot-002", "adapter_qa")
	requireCLI(t, r, "artifact", "init", runID, "--json")
	requireCLI(t, r, "phase-plan", "init", runID, "--json")
	requireCLI(t, r, "phase-plan", "set", runID, "ask", "--status", "not_applicable", "--reason", "No actionable question.", "--json")
	requireCLI(t, r, "phase-plan", "validate", runID, "--json")
	writeFile(t, filepath.Join(r, ".kkachi/runs", runID, "selected-cli.json"), fmt.Sprintf(`{"version":"0.1","status":"pending","api_token":"%s"}`+"\n", secret))
	requireCLI(t, r, "event", "append", "diagnostic.secret", "--run", runID, "--payload", fmt.Sprintf(`{"access_token":"%s"}`, secret), "--json")
	requireFailCLI(t, r, "gate", "check", runID, "intake", "--json")
	bundle := requireCLI(t, r, "diagnostics", "export", "--run", runID, "--json")
	requireContains(t, bundle.stdout, `"schema_versions":`, "diagnostics JSON")
	requireContains(t, bundle.stdout, `"path":".kkachi/runs/`+runID+`/phase-plan.yaml"`, "diagnostics JSON")
	requireContains(t, bundle.stdout, `"api_token":"[REDACTED]"`, "diagnostics JSON")
	requireNotContains(t, bundle.stdout, secret, "diagnostics JSON")
	human := requireCLI(t, r, "diagnostics", "export", "--run", runID, "--output", "diagnostics/pilot-002.json")
	requireContains(t, human.stdout, "diagnostics bundle exported: diagnostics/pilot-002.json", "diagnostics human")
	written := mustRead(t, filepath.Join(r, "diagnostics/pilot-002.json"))
	requireContains(t, written, `"run_id": "`+runID+`"`, "written diagnostics")
	requireContains(t, written, `"api_token": "[REDACTED]"`, "written diagnostics")
	requireNotContains(t, written, secret, "written diagnostics")
	redacted := requireFailCLI(t, r, "diagnostics", "export", "--output", "../api_token="+secret, "--json")
	requireContains(t, redacted.stderr, `"code":"path_escape"`, "redacted diagnostics error")
	requireContains(t, redacted.stderr, `[REDACTED]`, "redacted diagnostics error")
	requireNotContains(t, redacted.stderr, secret, "redacted diagnostics error")
}

func TestApprovalRecordsEndToEnd(t *testing.T) {
	r := repo(t, "approval")
	requireCLI(t, r, "project", "init", "--json")
	runID := createRun(t, r, "align-007", "production_write")
	requireCLI(t, r, "phase-plan", "init", runID, "--json")
	set := requireCLI(t, r, "phase-plan", "set", runID, "implement", "--status", "in_progress", "--approval-required", "true", "--json")
	requireContains(t, set.stdout, `"approval_required":true`, "phase approval-required")
	request := requireCLI(t, r, "approval", "request", runID, "--phase", "implement", "--reason", "High-risk implementation phase.", "--evidence", "plan.md#approval", "--json")
	requireContains(t, request.stdout, `"type":"approval.requested"`, "approval request")
	requireContains(t, request.stdout, `"timestamp":`, "approval request")

	finalBefore := requireFailCLI(t, r, "phase-plan", "validate", runID, "--final", "--json")
	requireContains(t, finalBefore.stdout, `"name":"final_approval_records","status":"fail"`, "approval final before decision")

	record := requireCLI(t, r, "approval", "record", runID, "--phase", "implement", "--decision", "approved", "--by", "master", "--evidence", "messages/approval-123", "--json")
	requireContains(t, record.stdout, `"type":"approval.recorded"`, "approval record")
	requireContains(t, record.stdout, `"decision":"approved"`, "approval record")
	show := requireCLI(t, r, "approval", "show", runID, "--phase", "implement", "--json")
	requireContains(t, show.stdout, `"records":[`, "approval show")
	requireContains(t, show.stdout, `"type":"approval.requested"`, "approval show")
	requireContains(t, show.stdout, `"decision":"approved"`, "approval show")

	finalAfter := requireFailCLI(t, r, "phase-plan", "validate", runID, "--final", "--json")
	requireContains(t, finalAfter.stdout, `"name":"final_approval_records","status":"pass"`, "approval final after decision")
	bundle := requireCLI(t, r, "diagnostics", "export", "--run", runID, "--json")
	requireContains(t, bundle.stdout, `"approval_records":[`, "approval diagnostics")
	requireContains(t, bundle.stdout, `"type":"approval.requested"`, "approval diagnostics")
	requireContains(t, bundle.stdout, `"type":"approval.recorded"`, "approval diagnostics")
	requireFileContains(t, filepath.Join(r, ".kkachi", "events.jsonl"), `"type":"approval.requested"`, "approval event log")
	requireFileContains(t, filepath.Join(r, ".kkachi", "events.jsonl"), `"type":"approval.recorded"`, "approval event log")
}

func TestReleasePackaging(t *testing.T) {
	tmp := t.TempDir()
	dist := filepath.Join(tmp, "dist")
	prefix := filepath.Join(tmp, "prefix")
	goos, goarch := runtime.GOOS, runtime.GOARCH
	name := fmt.Sprintf("kkachi-agent-helper_0.1.0_%s_%s", goos, goarch)
	invalid := exec.Command(filepath.Join(projectRoot, "scripts/build-release.sh"))
	invalid.Dir = projectRoot
	invalid.Env = append(os.Environ(), `VERSION=0.1.0"bad`, "COMMIT=e2e", "BUILD_DATE=2026-01-01T00:00:00Z", "DIST_DIR="+filepath.Join(tmp, "invalid-dist"), "GOOS="+goos, "GOARCH="+goarch)
	if out, err := invalid.CombinedOutput(); err == nil {
		t.Fatalf("release accepted unsafe VERSION\n%s", out)
	}
	runMake(t, "VERSION=0.1.0", "COMMIT=e2e", "BUILD_DATE=2026-01-01T00:00:00Z", "DIST_DIR="+dist, "GOOS="+goos, "GOARCH="+goarch, "release")
	artifact := filepath.Join(dist, name)
	archive := artifact + ".tar.gz"
	checksums := filepath.Join(dist, "SHA256SUMS")
	for _, path := range []string{artifact, archive, checksums} {
		if info, err := os.Stat(path); err != nil || info.Size() == 0 {
			t.Fatalf("missing release artifact %s: %v", path, err)
		}
	}
	verifyChecksums(t, dist)
	f, err := os.OpenFile(artifact, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.WriteString("tamper")
	_ = f.Close()
	if err := checkChecksums(dist); err == nil {
		t.Fatal("checksum verification unexpectedly passed after tamper")
	}
	runMake(t, "VERSION=0.1.0", "COMMIT=e2e", "BUILD_DATE=2026-01-01T00:00:00Z", "DIST_DIR="+dist, "GOOS="+goos, "GOARCH="+goarch, "release")
	verifyChecksums(t, dist)
	altArch := "amd64"
	if goarch == "amd64" {
		altArch = "arm64"
	}
	runMake(t, "VERSION=0.1.0", "COMMIT=e2e", "BUILD_DATE=2026-01-01T00:00:00Z", "DIST_DIR="+dist, "GOOS="+goos, "GOARCH="+altArch, "release")
	checksumsText := mustRead(t, checksums)
	requireContains(t, checksumsText, name, "checksums")
	requireContains(t, checksumsText, fmt.Sprintf("kkachi-agent-helper_0.1.0_%s_%s", goos, altArch), "checksums")
	verifyChecksums(t, dist)

	extract := filepath.Join(tmp, "extract")
	extractTarGz(t, archive, extract)
	for _, rel := range []string{"bin/kkachi-agent-helper", "README.md", "LICENSE", "docs/specs.md", "docs/roadmap.md", "docs/compatibility.md", "docs/release-notes-template.md", "RELEASE-MANIFEST.json"} {
		if info, err := os.Stat(filepath.Join(extract, filepath.FromSlash(rel))); err != nil || info.Size() == 0 {
			t.Fatalf("missing archive member %s: %v", rel, err)
		}
	}
	manifest := mustRead(t, filepath.Join(extract, "RELEASE-MANIFEST.json"))
	for _, want := range []string{`"manifest_version": "1"`, `"name": "kkachi-agent-helper"`, `"version": "0.1.0"`, `"commit": "e2e"`, `"build_date": "2026-01-01T00:00:00Z"`, `"goos": "` + goos + `"`, `"goarch": "` + goarch + `"`, `"binary": "bin/kkachi-agent-helper"`, `"license": "LICENSE"`} {
		requireContains(t, manifest, want, "release manifest")
	}
	version := exec.Command(artifact, "version", "--json")
	out, err := version.Output()
	if err != nil {
		t.Fatalf("release artifact version: %v", err)
	}
	requireContains(t, string(out), `"version":"0.1.0"`, "version JSON")
	capabilities := exec.Command(artifact, "capabilities", "--json")
	out, err = capabilities.Output()
	if err != nil {
		t.Fatalf("release artifact capabilities: %v", err)
	}
	requireContains(t, string(out), `"version":"0.1.0"`, "capabilities helper version")
	requireContains(t, string(out), `"project_schema_version":"0.1"`, "capabilities schema version")
	requireContains(t, string(out), `"backend_evidence_requirements":true`, "capabilities backend evidence flag")
	requireContains(t, string(out), `"phase_plan":true`, "capabilities phase-plan flag")
	requireContains(t, string(out), `"approval_records":true`, "capabilities approval flag")
	requireContains(t, string(out), `"name":"install"`, "capabilities omitted install")
	help := exec.Command(artifact, "run", "create", "--help")
	out, err = help.Output()
	if err != nil {
		t.Fatalf("release artifact help: %v", err)
	}
	requireContains(t, string(out), "kkachi-agent-helper run create", "release artifact help")
	requireContains(t, string(out), "--title <title> (required)", "release artifact help")
	phaseHelp := exec.Command(artifact, "--json", "phase-plan", "--help")
	out, err = phaseHelp.Output()
	if err != nil {
		t.Fatalf("release artifact phase-plan help: %v", err)
	}
	requireContains(t, string(out), `"command":"kkachi-agent-helper phase-plan"`, "release artifact phase-plan help")
	requireContains(t, string(out), `"status":"supported"`, "release artifact phase-plan help")
	approvalHelp := exec.Command(artifact, "--json", "approval", "--help")
	out, err = approvalHelp.Output()
	if err != nil {
		t.Fatalf("release artifact approval help: %v", err)
	}
	requireContains(t, string(out), `"command":"kkachi-agent-helper approval"`, "release artifact approval help")
	requireContains(t, string(out), `"status":"supported"`, "release artifact approval help")
	runMake(t, "VERSION=0.1.0", "COMMIT=e2e", "BUILD_DATE=2026-01-01T00:00:00Z", "PREFIX="+prefix, "install-local")
	installed := filepath.Join(prefix, "bin/kkachi-agent-helper")
	out, err = exec.Command(installed, "version", "--json").Output()
	if err != nil {
		t.Fatalf("installed version: %v", err)
	}
	requireContains(t, string(out), `"commit":"e2e"`, "installed version JSON")
	out, err = exec.Command(installed, "capabilities", "--json").Output()
	if err != nil {
		t.Fatalf("installed capabilities: %v", err)
	}
	requireContains(t, string(out), `"commit":"e2e"`, "installed capabilities JSON")
}

func runMake(t *testing.T, args ...string) {
	t.Helper()
	cmd := exec.Command("make", append([]string{"-C", projectRoot}, args...)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("make %s failed: %v\n%s", strings.Join(args, " "), err, out)
	}
}

func checkChecksums(dist string) error {
	data, err := os.ReadFile(filepath.Join(dist, "SHA256SUMS"))
	if err != nil {
		return err
	}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return fmt.Errorf("bad checksum line %q", line)
		}
		path := filepath.Join(dist, strings.TrimPrefix(fields[1], "*"))
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(content)
		if got := hex.EncodeToString(sum[:]); got != fields[0] {
			return fmt.Errorf("checksum mismatch for %s", path)
		}
	}
	return nil
}

func verifyChecksums(t *testing.T, dist string) {
	t.Helper()
	if err := checkChecksums(dist); err != nil {
		t.Fatal(err)
	}
}

func extractTarGz(t *testing.T, archive, dest string) {
	t.Helper()
	file, err := os.Open(archive)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	gz, err := gzip.NewReader(file)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		clean := filepath.Clean(header.Name)
		if strings.HasPrefix(clean, "..") || filepath.IsAbs(clean) {
			t.Fatalf("unsafe tar member %s", header.Name)
		}
		path := filepath.Join(dest, clean)
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(path, 0o755); err != nil {
				t.Fatal(err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			out, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, os.FileMode(header.Mode))
			if err != nil {
				t.Fatal(err)
			}
			_, copyErr := io.Copy(out, tr)
			closeErr := out.Close()
			if copyErr != nil {
				t.Fatal(copyErr)
			}
			if closeErr != nil {
				t.Fatal(closeErr)
			}
		}
	}
}

func TestPilotMVPAcceptanceRun(t *testing.T) {
	r := repo(t, "pilot-004")
	requireCLI(t, r, "project", "init", "--json")
	run := requireCLI(t, r, "run", "create", "--title", "Pilot 004 MVP acceptance run", "--work-path", "A_development_execution", "--work-mode", "standard", "--urgency", "normal", "--sot-policy", "existing_sot_basis", "--execution-mode", "adapter_qa", "--commander", "Gongmyeong", "--task-id", "pilot-004", "--redteam", "Haneul", "--json")
	runID := jsonFieldString(t, []byte(run.stdout), "run_id")
	if !regexp.MustCompile(`^run-\d{8}T\d{6}Z-[0-9a-f]{12}$`).MatchString(runID) {
		t.Fatalf("invalid run id: %s", runID)
	}
	active := requireCLI(t, r, "run", "activate", runID, "--json")
	requireContains(t, active.stdout, `"state":"active"`, "run activate")
	requireCLI(t, r, "artifact", "init", runID, "--json")
	status := requireCLI(t, r, "project", "status", "--json")
	requireContains(t, status.stdout, `"active_run_id":"`+runID+`"`, "active status")
	writeCompleteArtifacts(t, r, runID, "pilot-004", true)
	writeArtifact(t, r, runID, "task-brief.md", "Status: complete\nBrief: pilot-004 proves MVP readiness discipline through one complete helper-managed run.")
	writeArtifact(t, r, runID, "prompt.md", "Status: complete\nPrompt: execute pilot-004 according to docs/roadmap.md and docs/specs.md.")
	writeArtifact(t, r, runID, "context-pack.md", "Status: complete\nContext: docs/specs.md gate contracts and docs/roadmap.md pilot epic.")
	writeArtifact(t, r, runID, "redteam/plan-review.md", "Status: complete\nReview: plan evidence matches docs/roadmap.md pilot-004.")
	writeArtifact(t, r, runID, "redteam/shaping-review.md", "Status: complete\nReview: no shaping blocker.")
	writeArtifact(t, r, runID, "redteam/qa-review.md", "Status: complete\nReview: bridge evidence is shape-valid.")
	for _, gate := range []string{"intake", "sot", "roadmap", "plan", "backend", "implementation", "review", "verification", "docs"} {
		checked := requireCLI(t, r, "gate", "check", runID, gate, "--json")
		requireContains(t, checked.stdout, `"status":"pass"`, gate+" gate")
		requireContains(t, checked.stdout, `.kkachi/runs/`+runID+`/gate-reports/`+gate+`.json`, gate+" report path")
	}
	final := requireCLI(t, r, "gate", "final", runID, "--json")
	requireContains(t, final.stdout, `"status":"pass"`, "final gate")
	gatedStatus := requireCLI(t, r, "project", "status", "--json")
	requireContains(t, gatedStatus.stdout, `"gate_summary"`, "gated status")
	diagnostics := requireCLI(t, r, "diagnostics", "export", "--run", runID, "--output", "diagnostics/pilot-004.json")
	requireContains(t, diagnostics.stdout, "diagnostics bundle exported: diagnostics/pilot-004.json", "diagnostics human")
	bundle := mustRead(t, filepath.Join(r, "diagnostics/pilot-004.json"))
	for _, want := range []string{`"run_id": "` + runID + `"`, `"gate_reports": [`, "gate-reports/final.json", "selected-cli.json", "bridge-session-snapshot.json", "verification.md", "docs-update.md", "final-report.md"} {
		requireContains(t, bundle, want, "pilot diagnostics bundle")
	}
	closed := requireCLI(t, r, "run", "close", runID, "--json")
	requireContains(t, closed.stdout, `"state":"closed"`, "run close")
	closedStatus := requireCLI(t, r, "project", "status", "--json")
	requireContains(t, closedStatus.stdout, `"active_run_id":null`, "closed status")
}

var pilot005LegacyShellScenarios = []string{
	"tests/e2e/project-init.sh",
	"tests/e2e/runwf-002-locks.sh",
	"tests/e2e/pilot-001-golden-workspaces.sh",
	"tests/e2e/pilot-002-diagnostics.sh",
	"tests/e2e/pilot-003-release-packaging.sh",
	"tests/e2e/pilot-004-mvp-acceptance-run.sh",
}

var pilot005HarnessScanRoots = []string{"Makefile", "scripts", "tests"}

func TestPilot005E2EEntrypointRunsGoNativeHarness(t *testing.T) {
	script := mustRead(t, filepath.Join(projectRoot, "scripts/test-e2e.sh"))
	for _, want := range []string{
		`#!/bin/sh`,
		`set -eu`,
		`cd "$project_root"`,
		`go test ./tests/e2e`,
	} {
		requireContains(t, script, want, "test-e2e entrypoint")
	}
	for _, forbidden := range append([]string{"run_scenario", "mktemp -d"}, pilot005LegacyShellScenarios...) {
		requireNotContains(t, script, forbidden, "test-e2e entrypoint")
	}

	makefile := mustRead(t, filepath.Join(projectRoot, "Makefile"))
	requireContains(t, makefile, "test-e2e:\n\t./scripts/test-e2e.sh", "Makefile test-e2e target")
}

func TestPilot005LegacyShellScenariosRemoved(t *testing.T) {
	for _, rel := range pilot005LegacyShellScenarios {
		if _, err := os.Stat(filepath.Join(projectRoot, filepath.FromSlash(rel))); !os.IsNotExist(err) {
			t.Fatalf("legacy shell scenario should be removed: %s (stat err: %v)", rel, err)
		}
	}
}

func TestNoPythonHarnessReferences(t *testing.T) {
	var offenders []string
	for _, base := range pilot005HarnessScanRoots {
		root := filepath.Join(projectRoot, base)
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return nil
			}
			if strings.Contains(string(data), "python"+"3") || strings.Contains(string(data), "python"+" ") {
				offenders = append(offenders, strings.TrimPrefix(path, projectRoot+string(os.PathSeparator)))
			}
			return nil
		})
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Fatalf("Python-harness references remain: %s", strings.Join(offenders, ", "))
	}
}

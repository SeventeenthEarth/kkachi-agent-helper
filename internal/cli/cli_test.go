package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SeventeenthEarth/kkachi-agent-helper/internal/project"
)

func TestVersionHumanOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--version"}, &stdout, &stderr, testBuildInfo())

	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	if got, want := stdout.String(), "kkachi-agent-helper 1.2.3 commit abc123 built 2026-04-30T00:00:00Z\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestVersionJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"version", "--json"}, &stdout, &stderr, testBuildInfo())

	if exitCode != 0 {
		t.Fatalf("exitCode = %d, want 0", exitCode)
	}
	assertNoHumanDecoration(t, stdout.String())

	var payload BuildInfo
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if payload != testBuildInfo() {
		t.Fatalf("payload = %#v, want %#v", payload, testBuildInfo())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
}

func TestNoCommandReturnsUsageError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run(nil, &stdout, &stderr, testBuildInfo())

	if exitCode == 0 {
		t.Fatal("exitCode = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	assertHumanError(t, stderr.String(), "no command provided")
}

func TestNoCommandJSONError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--json"}, &stdout, &stderr, testBuildInfo())

	if exitCode == 0 {
		t.Fatal("exitCode = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "no_command" {
		t.Fatalf("error code = %q, want no_command", env.Error.Code)
	}
	if env.Error.ExitCode != ExitUsage {
		t.Fatalf("exit code = %d, want %d", env.Error.ExitCode, ExitUsage)
	}
	assertNoHumanDecoration(t, stderr.String())
}

func TestUnknownCommandJSONError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"--json", "bogus"}, &stdout, &stderr, testBuildInfo())

	if exitCode == 0 {
		t.Fatal("exitCode = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "unknown_command" {
		t.Fatalf("error code = %q, want unknown_command", env.Error.Code)
	}
	if !strings.Contains(env.Error.Hint, "Usage:") {
		t.Fatalf("hint = %q, want usage guidance", env.Error.Hint)
	}
	if env.Error.ExitCode != ExitUsage {
		t.Fatalf("exit code = %d, want %d", env.Error.ExitCode, ExitUsage)
	}
	assertNoHumanDecoration(t, stderr.String())
}

func TestImplementedRunCommandValidatesCreateOptions(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	exitCode := runWithOptions([]string{"run", "create", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})

	if exitCode != ExitUsage {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "missing_required_option" {
		t.Fatalf("error code = %q, want missing_required_option", env.Error.Code)
	}
}

func TestProjectInitHumanOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions(
		[]string{"project", "init"},
		&stdout,
		&stderr,
		testBuildInfo(),
		runOptions{workingDir: repo},
	)

	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d\nstderr: %s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	output := stdout.String()
	if !strings.Contains(output, "initialized kkachi project:") || !strings.Contains(output, ".kkachi/config.yaml") || !strings.Contains(output, "initial_event_id: evt-000001") {
		t.Fatalf("stdout = %q, want init summary", output)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "status.json")); err != nil {
		t.Fatalf("status.json was not created: %v", err)
	}
}

func TestProjectInitJSONOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions(
		[]string{"project", "init", "--json"},
		&stdout,
		&stderr,
		testBuildInfo(),
		runOptions{workingDir: filepath.Join(repo, "nested")},
	)

	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d\nstderr: %s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertNoHumanDecoration(t, stdout.String())

	var payload projectInitOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if payload.RootPath == "" || payload.ProjectID == "" || payload.ProjectName == "" {
		t.Fatalf("payload = %#v, want root and project identity", payload)
	}
	if payload.InitialEventID != "evt-000001" {
		t.Fatalf("initial event id = %q, want evt-000001", payload.InitialEventID)
	}
	if len(payload.CreatedPaths) != 3 || len(payload.SchemaPaths) != 5 {
		t.Fatalf("payload paths = %#v/%#v, want created and schema paths", payload.CreatedPaths, payload.SchemaPaths)
	}
}

func TestProjectInitRefusesExistingState(t *testing.T) {
	repo := tempGitRepo(t)
	if err := os.MkdirAll(filepath.Join(repo, ".kkachi"), 0o755); err != nil {
		t.Fatalf("mkdir .kkachi: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "status.json"), []byte("{}\n"), 0o600); err != nil {
		t.Fatalf("write existing status: %v", err)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions(
		[]string{"project", "init", "--json"},
		&stdout,
		&stderr,
		testBuildInfo(),
		runOptions{workingDir: repo},
	)

	if exitCode != ExitSafety {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitSafety)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "helper_state_exists" {
		t.Fatalf("error code = %q, want helper_state_exists", env.Error.Code)
	}
}

func TestUnsupportedProjectSubcommandIsNotImplemented(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions(
		[]string{"project", "frobnicate"},
		&stdout,
		&stderr,
		testBuildInfo(),
		runOptions{workingDir: repo},
	)

	if exitCode != ExitUsage {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	assertHumanError(t, stderr.String(), "project command is not implemented yet")
}

func TestProjectStatusAndDoctorJSONOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode := runWithOptions([]string{"project", "status", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("project status exit = %d want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertNoHumanDecoration(t, stdout.String())
	var status projectStatusOutput
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("status stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if status.Health != "ok" || status.LastEventID != "evt-000001" || status.EventTailID != "evt-000001" || status.EventCount != 1 || len(status.Issues) != 0 {
		t.Fatalf("status = %#v, want healthy initialized project", status)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = runWithOptions([]string{"project", "doctor", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("project doctor exit = %d want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertNoHumanDecoration(t, stdout.String())
	var doctor projectDoctorOutput
	if err := json.Unmarshal(stdout.Bytes(), &doctor); err != nil {
		t.Fatalf("doctor stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if doctor.Health != "ok" || doctor.Summary.Failed != 0 || doctor.Summary.Warnings != 0 || len(doctor.Checks) == 0 {
		t.Fatalf("doctor = %#v, want healthy checks", doctor)
	}
}

func TestProjectStatusAndDoctorHumanOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	exitCode := runWithOptions([]string{"project", "status"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("project status exit = %d want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	statusOutput := stdout.String()
	for _, want := range []string{"project status: ok", "last_event_id: evt-000001", "event_tail_id: evt-000001", "issues: 0"} {
		if !strings.Contains(statusOutput, want) {
			t.Fatalf("status output = %q, want %q", statusOutput, want)
		}
	}

	stdout.Reset()
	stderr.Reset()
	exitCode = runWithOptions([]string{"project", "doctor"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitOK {
		t.Fatalf("project doctor exit = %d want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	doctorOutput := stdout.String()
	for _, want := range []string{"project doctor: ok", "summary:", "[pass] config .kkachi/config.yaml", "[pass] status .kkachi/status.json"} {
		if !strings.Contains(doctorOutput, want) {
			t.Fatalf("doctor output = %q, want %q", doctorOutput, want)
		}
	}
}

func TestProjectStatusAndDoctorRejectUnsupportedOptions(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	for _, args := range [][]string{
		{"project", "status", "--bogus", "--json"},
		{"project", "doctor", "--bogus", "--json"},
	} {
		stdout.Reset()
		stderr.Reset()
		exitCode := runWithOptions(args, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
		if exitCode != ExitUsage {
			t.Fatalf("%v exitCode = %d, want %d", args, exitCode, ExitUsage)
		}
		if stdout.Len() != 0 {
			t.Fatalf("stdout = %q, want empty", stdout.String())
		}
		env := decodeErrorEnvelope(t, stderr.Bytes())
		if env.Error.Code != "unknown_option" || env.Error.ExitCode != ExitUsage {
			t.Fatalf("error = %#v, want unknown_option usage", env.Error)
		}
		assertNoHumanDecoration(t, stderr.String())
	}
}

func TestProjectDoctorReportsCoherenceMismatch(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "events.jsonl"), []byte(`{"version":"0.1","event_id":"evt-000001","occurred_at":"2026-04-30T01:00:00Z","run_id":null,"type":"project.initialized","actor":"helper","payload":{}}`+"\n"+`{"version":"0.1","event_id":"evt-000002","occurred_at":"2026-04-30T02:00:00Z","run_id":null,"type":"run.created","actor":"helper","payload":{}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write divergent event log: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode := runWithOptions([]string{"project", "doctor", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitSafety {
		t.Fatalf("project doctor exit = %d want %d", exitCode, ExitSafety)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var doctor projectDoctorOutput
	if err := json.Unmarshal(stdout.Bytes(), &doctor); err != nil {
		t.Fatalf("doctor stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if doctor.Health != "fail" {
		t.Fatalf("health = %q, want fail", doctor.Health)
	}
	found := false
	for _, check := range doctor.Checks {
		if check.Name == "coherence" && check.Status == "fail" && check.Expected == "evt-000002" && check.Actual == "evt-000001" {
			found = true
		}
	}
	if !found {
		t.Fatalf("doctor checks = %#v, want coherence mismatch", doctor.Checks)
	}
}

func TestProjectStatusReportsCoherenceMismatch(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "events.jsonl"), []byte(`{"version":"0.1","event_id":"evt-000001","occurred_at":"2026-04-30T01:00:00Z","run_id":null,"type":"project.initialized","actor":"helper","payload":{}}`+"\n"+`{"version":"0.1","event_id":"evt-000002","occurred_at":"2026-04-30T02:00:00Z","run_id":null,"type":"run.created","actor":"helper","payload":{}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write divergent event log: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	exitCode := runWithOptions([]string{"project", "status", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if exitCode != ExitSafety {
		t.Fatalf("project status exit = %d want %d", exitCode, ExitSafety)
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	var status projectStatusOutput
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("status stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if status.Health != "fail" || status.LastEventID != "evt-000001" || status.EventTailID != "evt-000002" {
		t.Fatalf("status = %#v, want fail with tail mismatch", status)
	}
}

func TestKnownCommandGroupJSONError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"schema", "validate", "file", "--json"}, &stdout, &stderr, testBuildInfo())

	if exitCode == 0 {
		t.Fatal("exitCode = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "not_implemented" {
		t.Fatalf("error code = %q, want not_implemented", env.Error.Code)
	}
	if !strings.Contains(env.Error.Message, "schema") {
		t.Fatalf("message = %q, want command group name", env.Error.Message)
	}
	if env.Error.ExitCode != ExitUsage {
		t.Fatalf("exit code = %d, want %d", env.Error.ExitCode, ExitUsage)
	}
	assertNoHumanDecoration(t, stderr.String())
}

func TestCommandGroupRequiresRepositoryRoot(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions(
		[]string{"--json", "project", "status"},
		&stdout,
		&stderr,
		testBuildInfo(),
		runOptions{workingDir: t.TempDir()},
	)

	if exitCode != ExitNotFound {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitNotFound)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "repo_root_not_found" {
		t.Fatalf("error code = %q, want repo_root_not_found", env.Error.Code)
	}
	if env.Error.ExitCode != ExitNotFound {
		t.Fatalf("error exit code = %d, want %d", env.Error.ExitCode, ExitNotFound)
	}
	if env.Error.Hint == "" || env.Error.Expected == "" || env.Error.Actual == "" {
		t.Fatalf("error = %#v, want structured remediation fields", env.Error)
	}
	assertNoHumanDecoration(t, stderr.String())
}

func TestRunCreateListShowActivateCloseCLI(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	createArgs := runCreateArgs("Run workflow metadata", "--task-id", "runwf-001", "--redteam", "Reviewer", "--json")
	if code := runWithOptions(createArgs, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("create stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if !strings.HasPrefix(created.RunID, "run-") || created.State != "created" || created.EventID != "evt-000002" || created.Metadata.TaskID == nil || *created.Metadata.TaskID != "runwf-001" {
		t.Fatalf("created = %#v, want created run payload", created)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"run", "list", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run list exit = %d stderr=%s", code, stderr.String())
	}
	var list runListOutput
	if err := json.Unmarshal(stdout.Bytes(), &list); err != nil {
		t.Fatalf("list stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if len(list.Runs) != 1 || list.Runs[0].RunID != created.RunID || list.Runs[0].State != "created" {
		t.Fatalf("list = %#v, want created run summary", list)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"run", "show", created.RunID[:24], "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run show exit = %d stderr=%s", code, stderr.String())
	}
	var shown project.RunMetadata
	if err := json.Unmarshal(stdout.Bytes(), &shown); err != nil {
		t.Fatalf("show stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if shown.RunID != created.RunID || shown.RequiredArtifacts == nil || shown.GateState == nil {
		t.Fatalf("shown = %#v, want full metadata", shown)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"run", "activate", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run activate exit = %d stderr=%s", code, stderr.String())
	}
	var activated runLifecycleOutput
	if err := json.Unmarshal(stdout.Bytes(), &activated); err != nil {
		t.Fatalf("activate stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if activated.RunID != created.RunID || activated.State != "active" || activated.EventID != "evt-000003" {
		t.Fatalf("activated = %#v, want active evt", activated)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"project", "status", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project status exit = %d stderr=%s", code, stderr.String())
	}
	var status projectStatusOutput
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("status stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if status.ActiveRunID == nil || *status.ActiveRunID != created.RunID || status.ActiveRunState == nil || *status.ActiveRunState != "active" || status.LastEventID != "evt-000003" {
		t.Fatalf("status = %#v, want active fields", status)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"run", "close", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run close exit = %d stderr=%s", code, stderr.String())
	}
	var closed runLifecycleOutput
	if err := json.Unmarshal(stdout.Bytes(), &closed); err != nil {
		t.Fatalf("close stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if closed.State != "closed" || closed.EventID != "evt-000004" {
		t.Fatalf("closed = %#v, want closed evt", closed)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"project", "status", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project status after close exit = %d stderr=%s", code, stderr.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &status); err != nil {
		t.Fatalf("status stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if status.ActiveRunID != nil || status.ActiveRunState != nil || status.LastEventID != "evt-000004" || status.EventCount != 4 {
		t.Fatalf("status after close = %#v, want active cleared", status)
	}
}

func TestRunAbortCLI(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	createArgs := runCreateArgs("Abort me", "--work-mode", "light", "--urgency", "urgent", "--sot-policy", "minimal_sot_before_code", "--execution-mode", "verification", "--json")
	if code := runWithOptions(createArgs, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("create stdout is not JSON: %v\n%s", err, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"run", "abort", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run abort exit = %d stderr=%s", code, stderr.String())
	}
	var aborted runLifecycleOutput
	if err := json.Unmarshal(stdout.Bytes(), &aborted); err != nil {
		t.Fatalf("abort stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if aborted.State != "aborted" || aborted.EventID != "evt-000003" {
		t.Fatalf("aborted = %#v, want aborted evt", aborted)
	}
}

func TestRunCLIValidationAndSafetyErrors(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"run", "create", "--title", "Bad", "--work-path", "nope", "--work-mode", "standard", "--urgency", "normal", "--sot-policy", "existing_sot_basis", "--execution-mode", "production_write", "--commander", "Gongmyeong", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "run_metadata_invalid")

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"run", "show", "run-19990101T000000Z-aaaaaaaaaaaa", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "run_not_found")

	stdout.Reset()
	stderr.Reset()
	createArgs := runCreateArgs("Corrupt", "--json")
	if code := runWithOptions(createArgs, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json"), []byte("{not-json\n"), 0o600); err != nil {
		t.Fatalf("corrupt metadata: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"run", "list", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "run_metadata_invalid_json")
}

func TestRunCLIHumanOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	createArgs := runCreateArgs("Human run", "--task-id", "runwf-001")
	if code := runWithOptions(createArgs, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	createOutput := stdout.String()
	if !strings.Contains(createOutput, "created run: run-") || !strings.Contains(createOutput, "state: created") || !strings.Contains(createOutput, "event_id: evt-000002") {
		t.Fatalf("create output = %q, want human run summary", createOutput)
	}
	runID := onlyRunID(t, repo)

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"run", "list"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run list exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "runs: 1") || !strings.Contains(output, runID) || !strings.Contains(output, "state=created") || !strings.Contains(output, "task_id=runwf-001") {
		t.Fatalf("list output = %q, want human list summary", output)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"run", "show", runID}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run show exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "run_id: "+runID) || !strings.Contains(output, "title: Human run") || !strings.Contains(output, "state: created") {
		t.Fatalf("show output = %q, want human metadata", output)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"run", "activate", runID}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run activate exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "activated run: "+runID) || !strings.Contains(output, "state: active") || !strings.Contains(output, "event_id: evt-000003") {
		t.Fatalf("activate output = %q, want human lifecycle summary", output)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"run", "close", runID}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run close exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "closed run: "+runID) || !strings.Contains(output, "state: closed") || !strings.Contains(output, "event_id: evt-000004") {
		t.Fatalf("close output = %q, want human lifecycle summary", output)
	}
}

func TestRunCLIRejectsUnknownOptionsAndExtraArgs(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	createArgs := runCreateArgs("Arg run", "--json")
	if code := runWithOptions(createArgs, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	tests := []struct {
		name string
		args []string
		code string
	}{
		{name: "create unknown option", args: []string{"run", "create", "--bogus", "x", "--json"}, code: "unknown_option"},
		{name: "create duplicate option", args: append(createArgs[:len(createArgs)-1], "--title", "again", "--json"), code: "duplicate_option"},
		{name: "list unknown option", args: []string{"run", "list", "--bogus", "--json"}, code: "unknown_option"},
		{name: "show unknown option", args: []string{"run", "show", created.RunID, "--bogus", "--json"}, code: "unknown_option"},
		{name: "activate unknown option", args: []string{"run", "activate", created.RunID, "--bogus", "--json"}, code: "unknown_option"},
		{name: "activate extra id", args: []string{"run", "activate", created.RunID, "extra", "--json"}, code: "run_id_required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			assertCLIErrorCode(t, runWithOptions(tt.args, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, tt.code)
		})
	}
}

func TestRunCommandsRefuseEventCoherenceMismatch(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions(runCreateArgs("Blocked", "--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	appendCrashEvent(t, repo, "evt-000003", created.RunID)

	tests := [][]string{
		runCreateArgs("Blocked", "--json"),
		{"run", "list", "--json"},
		{"run", "show", created.RunID, "--json"},
		{"run", "activate", created.RunID, "--json"},
		{"run", "close", created.RunID, "--json"},
		{"run", "abort", created.RunID, "--json"},
	}
	for _, args := range tests {
		stdout.Reset()
		stderr.Reset()
		assertCLIErrorCode(t, runWithOptions(args, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "last_event_id_mismatch")
	}
}

func runCreateArgs(title string, overrides ...string) []string {
	args := []string{
		"run", "create",
		"--title", title,
		"--work-path", "A_development_execution",
		"--work-mode", "standard",
		"--urgency", "normal",
		"--sot-policy", "existing_sot_basis",
		"--execution-mode", "production_write",
		"--commander", "Gongmyeong",
	}
	for i := 0; i < len(overrides); {
		key := overrides[i]
		if key == "--json" {
			args = append(args, key)
			i++
			continue
		}
		if i+1 >= len(overrides) {
			args = append(args, key)
			break
		}
		value := overrides[i+1]
		i += 2
		replaced := false
		for j := 0; j+1 < len(args); j += 2 {
			if args[j] == key {
				args[j+1] = value
				replaced = true
				break
			}
		}
		if !replaced {
			args = append(args, key, value)
		}
	}
	return args
}

func onlyRunID(t *testing.T, repo string) string {
	t.Helper()
	entries, err := os.ReadDir(filepath.Join(repo, ".kkachi", "runs"))
	if err != nil {
		t.Fatalf("read runs: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("run entry count = %d, want 1", len(entries))
	}
	return entries[0].Name()
}

func appendCrashEvent(t *testing.T, repo string, eventID string, runID string) {
	t.Helper()
	line := `{"version":"0.1","event_id":"` + eventID + `","occurred_at":"2026-04-30T03:00:00Z","run_id":"` + runID + `","type":"run.created","actor":"helper","payload":{}}` + "\n"
	file, err := os.OpenFile(filepath.Join(repo, ".kkachi", "events.jsonl"), os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	if _, err := file.WriteString(line); err != nil {
		t.Fatalf("append crash event: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}
}

func TestEventAppendJSONOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()

	exitCode := runWithOptions(
		[]string{"event", "append", "artifact.written", "--run", "run-abc", "--payload", `{"path":"impl-log.md"}`, "--json"},
		&stdout,
		&stderr,
		testBuildInfo(),
		runOptions{workingDir: repo},
	)

	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d\nstderr: %s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertNoHumanDecoration(t, stdout.String())
	var payload eventAppendOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if payload.EventID != "evt-000002" || payload.PreviousID != "evt-000001" || payload.EventsPath != ".kkachi/events.jsonl" {
		t.Fatalf("payload = %#v, want appended event summary", payload)
	}
	statusBytes, err := os.ReadFile(filepath.Join(repo, ".kkachi", "status.json"))
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(string(statusBytes), `"last_event_id": "evt-000002"`) {
		t.Fatalf("status = %s, want advanced last_event_id", string(statusBytes))
	}
}

func TestEventAppendValidatesOptionsAndPayload(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	tests := []struct {
		name string
		args []string
		code string
	}{
		{
			name: "missing run",
			args: []string{"event", "append", "artifact.written", "--payload", `{}`, "--json"},
			code: "run_id_required",
		},
		{
			name: "missing payload",
			args: []string{"event", "append", "artifact.written", "--run", "run-abc", "--json"},
			code: "payload_required",
		},
		{
			name: "invalid payload json",
			args: []string{"event", "append", "artifact.written", "--run", "run-abc", "--payload", `{`, "--json"},
			code: "payload_invalid_json",
		},
		{
			name: "payload array is not object",
			args: []string{"event", "append", "artifact.written", "--run", "run-abc", "--payload", `[]`, "--json"},
			code: "payload_invalid_json",
		},
		{
			name: "payload null is not object",
			args: []string{"event", "append", "artifact.written", "--run", "run-abc", "--payload", `null`, "--json"},
			code: "payload_invalid_json",
		},
		{
			name: "empty event type",
			args: []string{"event", "append", "", "--run", "run-abc", "--payload", `{}`, "--json"},
			code: "event_type_required",
		},
		{
			name: "oversized payload",
			args: []string{"event", "append", "artifact.written", "--run", "run-abc", "--payload", `{"blob":"` + strings.Repeat("x", project.MaxEventPayloadBytes) + `"}`, "--json"},
			code: "payload_too_large",
		},
		{
			name: "control character run id",
			args: []string{"event", "append", "artifact.written", "--run", "run-abc\nsecond-line", "--payload", `{}`, "--json"},
			code: "run_id_invalid",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			assertCLIErrorCode(t, runWithOptions(tt.args, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, tt.code)
		})
	}
}

func TestEventAppendRefusesCoherenceMismatch(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "events.jsonl"), []byte(`{"version":"0.1","event_id":"evt-000002","occurred_at":"2026-04-30T02:00:00Z","run_id":null,"type":"run.created","actor":"helper","payload":{}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write divergent event log: %v", err)
	}
	stdout.Reset()
	stderr.Reset()

	exitCode := runWithOptions(
		[]string{"event", "append", "artifact.written", "--run", "run-abc", "--payload", `{}`, "--json"},
		&stdout,
		&stderr,
		testBuildInfo(),
		runOptions{workingDir: repo},
	)

	if exitCode != ExitSafety {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitSafety)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "last_event_id_mismatch" {
		t.Fatalf("error code = %q, want last_event_id_mismatch", env.Error.Code)
	}
	if env.Error.Expected != "evt-000002" || env.Error.Actual != "evt-000001" {
		t.Fatalf("error coherence fields = %#v, want expected event tail and actual status", env.Error)
	}
}

func assertCLIErrorCode(t *testing.T, exitCode int, stdout bytes.Buffer, stderr bytes.Buffer, wantExitCode int, wantCode string) {
	t.Helper()

	if exitCode != wantExitCode {
		t.Fatalf("exitCode = %d, want %d", exitCode, wantExitCode)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != wantCode {
		t.Fatalf("error code = %q, want %s", env.Error.Code, wantCode)
	}
	if env.Error.Hint == "" || env.Error.Expected == "" || env.Error.Actual == "" {
		t.Fatalf("error = %#v, want structured remediation fields", env.Error)
	}
	assertNoHumanDecoration(t, stderr.String())
}

func testBuildInfo() BuildInfo {
	return BuildInfo{
		Name:      "kkachi-agent-helper",
		Version:   "1.2.3",
		Commit:    "abc123",
		BuildDate: "2026-04-30T00:00:00Z",
	}
}

func decodeErrorEnvelope(t *testing.T, data []byte) errorEnvelope {
	t.Helper()

	var env errorEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		t.Fatalf("stderr is not valid JSON: %v\n%s", err, string(data))
	}
	return env
}

func assertHumanError(t *testing.T, output string, wantMessage string) {
	t.Helper()

	if !strings.Contains(output, "error: ") || !strings.Contains(output, wantMessage) {
		t.Fatalf("stderr = %q, want message %q", output, wantMessage)
	}
	if !strings.Contains(output, "hint: ") {
		t.Fatalf("stderr = %q, want hint", output)
	}
}

func assertNoHumanDecoration(t *testing.T, output string) {
	t.Helper()

	if strings.Contains(output, "error:") || strings.Contains(output, "hint:") {
		t.Fatalf("output = %q, want raw JSON without human decoration", output)
	}
}

func tempGitRepo(t *testing.T) string {
	t.Helper()

	repo := t.TempDir()
	if err := os.MkdirAll(filepath.Join(repo, ".git"), 0o755); err != nil {
		t.Fatalf("mkdir .git: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "nested"), 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	return repo
}

func TestLockRecoverCLIJSONAndConflictShape(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	fresh := project.LockMetadata{Version: project.LockVersion, LockName: project.ProjectWriteLockName, OwnerPID: os.Getpid(), Hostname: cliMustHostname(t), Command: "fresh writer", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	writeCLILock(t, repo, project.ProjectWriteLockName, fresh)
	stdout.Reset()
	stderr.Reset()
	code := runWithOptions([]string{"lock", "recover", "project-write", "--reason", "test", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	assertCLIErrorCode(t, code, stdout, stderr, ExitSafety, "lock_conflict")

	oldNow := time.Date(2026, 4, 30, 3, 4, 5, 0, time.UTC)
	stale := project.LockMetadata{Version: project.LockVersion, LockName: project.ProjectWriteLockName, OwnerPID: 999999, Hostname: "other-host", Command: "stale writer", CreatedAt: oldNow.Add(-31 * time.Minute).Format(time.RFC3339)}
	writeCLILock(t, repo, project.ProjectWriteLockName, stale)
	stdout.Reset()
	stderr.Reset()
	code = runWithOptions([]string{"lock", "recover", "project-write", "--reason", "test stale", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if code != ExitOK {
		t.Fatalf("lock recover exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var payload lockRecoverOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("recover stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if len(payload.Recovered) != 1 || payload.Recovered[0].LockName != project.ProjectWriteLockName {
		t.Fatalf("payload = %#v, want recovered project_write", payload)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "project_write.lock")); !os.IsNotExist(err) {
		t.Fatalf("project_write lock stat = %v, want absent", err)
	}
	if events := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); !strings.Contains(events, `"type":"lock.recovered"`) {
		t.Fatalf("events = %s, want lock.recovered", events)
	}
}

func TestEventAppendCLIFailsUnderFreshProjectWriteLock(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	fresh := project.LockMetadata{Version: project.LockVersion, LockName: project.ProjectWriteLockName, OwnerPID: os.Getpid(), Hostname: cliMustHostname(t), Command: "fresh event writer", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	writeCLILock(t, repo, project.ProjectWriteLockName, fresh)
	stdout.Reset()
	stderr.Reset()
	code := runWithOptions([]string{"event", "append", "artifact.written", "--run", "run-abc", "--payload", `{"path":"impl-log.md"}`, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	assertCLIErrorCode(t, code, stdout, stderr, ExitSafety, "lock_conflict")
	if events := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); strings.Contains(events, "artifact.written") {
		t.Fatalf("events = %s, want no appended artifact event under lock conflict", events)
	}
}

func writeCLILock(t *testing.T, repo string, name string, metadata project.LockMetadata) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "project_write.lock")
	if name == project.ActiveRunLockName {
		path = filepath.Join(repo, ".kkachi", "active_run.lock")
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		t.Fatalf("marshal lock: %v", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}
}

func cliMustHostname(t *testing.T) string {
	t.Helper()
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("hostname: %v", err)
	}
	return hostname
}

func readCLIText(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

func TestArtifactInitListCLI(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	createArgs := runCreateArgs("Artifact run", "--task-id", "runwf-003", "--json")
	if code := runWithOptions(createArgs, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "init", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact init exit = %d stderr=%s", code, stderr.String())
	}
	var initialized artifactInitOutput
	if err := json.Unmarshal(stdout.Bytes(), &initialized); err != nil {
		t.Fatalf("artifact init stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if initialized.RunID != created.RunID || initialized.EventID != "evt-000003" || len(initialized.Created) == 0 || len(initialized.RequiredArtifacts) == 0 {
		t.Fatalf("initialized = %#v, want artifact init payload", initialized)
	}
	if text := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); !strings.Contains(text, `"type":"artifact.written"`) {
		t.Fatalf("events = %s, want artifact.written", text)
	}
	if text := readCLIText(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")); !strings.Contains(text, `"required_artifacts": [`) || !strings.Contains(text, `"diff.patch"`) {
		t.Fatalf("metadata = %s, want required artifacts", text)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "list", created.RunID[:24], "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact list exit = %d stderr=%s", code, stderr.String())
	}
	var listed artifactListOutput
	if err := json.Unmarshal(stdout.Bytes(), &listed); err != nil {
		t.Fatalf("artifact list stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if listed.RunID != created.RunID || len(listed.Artifacts) == 0 || !listed.Artifacts[0].Exists {
		t.Fatalf("listed = %#v, want initialized artifacts", listed)
	}
}

func TestArtifactCLIValidationAndHumanOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions(runCreateArgs("Artifact human"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	runID := onlyRunID(t, repo)

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "init", runID}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact init exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "initialized artifacts for run: "+runID) || !strings.Contains(output, "event_id: evt-000003") || !strings.Contains(output, "required_artifacts:") {
		t.Fatalf("artifact init output = %q, want human summary", output)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "list", runID}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact list exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "artifacts for run: "+runID) || !strings.Contains(output, "intake-classification.md required state=present") {
		t.Fatalf("artifact list output = %q, want human list", output)
	}

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"artifact", "list", runID, "--bogus", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "unknown_option")

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"artifact", "init", "missing", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "run_not_found")
}

func TestArtifactValidateCLIJSONHumanAndFailures(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions(runCreateArgs("Artifact validate", "--task-id", "runwf-004", "--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "init", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact init exit = %d stderr=%s", code, stderr.String())
	}
	beforeEvents := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "validate", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("artifact validate pending exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var failed artifactValidateOutput
	if err := json.Unmarshal(stdout.Bytes(), &failed); err != nil {
		t.Fatalf("decode failed validate: %v\n%s", err, stdout.String())
	}
	if failed.Status != project.ValidationStatusFail || !cliValidationCheckStatus(failed.Checks, "intake_status", project.ValidationStatusFail) {
		t.Fatalf("failed validate = %#v, want intake_status failure", failed)
	}
	if afterEvents := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); afterEvents != beforeEvents {
		t.Fatalf("artifact validate mutated events\nbefore=%s\nafter=%s", beforeEvents, afterEvents)
	}

	writeCLIIntakeClassification(t, repo, created.Metadata, "")
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "validate", created.RunID[:24], "--gate", "intake", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact validate pass exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var passed artifactValidateOutput
	if err := json.Unmarshal(stdout.Bytes(), &passed); err != nil {
		t.Fatalf("decode passed validate: %v\n%s", err, stdout.String())
	}
	if passed.RunID != created.RunID || passed.Gate != project.ArtifactGateIntake || passed.Status != project.ValidationStatusPass {
		t.Fatalf("passed validate = %#v, want pass", passed)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "validate", created.RunID}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact validate human exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "artifact validation for run: "+created.RunID) || !strings.Contains(output, "status: pass") || !strings.Contains(output, "required_artifacts pass") {
		t.Fatalf("human validate output = %q, want pass summary", output)
	}

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"artifact", "validate", created.RunID, "--gate", "final", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "unsupported_gate")

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"artifact", "validate", created.RunID, "--bogus", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "unknown_option")
}

func writeCLIIntakeClassification(t *testing.T, repo string, metadata project.RunMetadata, extra string) {
	t.Helper()
	content := strings.Join([]string{
		"# intake-classification.md",
		"",
		"Status: complete",
		"Work Path: " + metadata.WorkPath,
		"Work Mode: " + metadata.WorkMode,
		"SOT Policy: " + metadata.SOTPolicy,
		"Urgency: " + metadata.Urgency,
		strings.TrimRight(extra, "\n"),
		"",
	}, "\n")
	path := filepath.Join(repo, ".kkachi", "runs", metadata.RunID, "intake-classification.md")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write intake classification: %v", err)
	}
}

func cliValidationCheckStatus(checks []project.ArtifactValidationCheck, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func TestGateCheckCLIJSONHumanAndPlanFailure(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions([]string{"project", "init"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions(runCreateArgs("Gate check", "--task-id", "gates-002", "--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "init", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact init exit = %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "check", created.RunID, "intake", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("gate check pending exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var failed gateCheckOutput
	if err := json.Unmarshal(stdout.Bytes(), &failed); err != nil {
		t.Fatalf("decode failed gate check: %v\n%s", err, stdout.String())
	}
	if failed.Status != project.GateStatusFail || failed.EventID != "evt-000004" || !cliGateCheckStatus(failed.Checks, "intake_status", project.GateStatusFail) || len(failed.MissingEvidence) == 0 {
		t.Fatalf("failed gate = %#v, want intake failure with missing evidence", failed)
	}
	if text := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); !strings.Contains(text, `"type":"gate.failed"`) {
		t.Fatalf("events missing gate.failed: %s", text)
	}
	if text := readCLIText(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json")); !strings.Contains(text, `"gate_state"`) || !strings.Contains(text, `"event_id": "evt-000004"`) {
		t.Fatalf("metadata missing recorded gate state: %s", text)
	}
	if text := readCLIText(t, filepath.Join(repo, ".kkachi", "status.json")); !strings.Contains(text, `"gate_summary"`) || !strings.Contains(text, `"intake"`) || !strings.Contains(text, `"event_id": "evt-000004"`) {
		t.Fatalf("status missing gate summary: %s", text)
	}

	writeCLIIntakeClassification(t, repo, created.Metadata, "")
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "check", created.RunID[:24], "intake", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("gate check pass exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var passed gateCheckOutput
	if err := json.Unmarshal(stdout.Bytes(), &passed); err != nil {
		t.Fatalf("decode passed gate check: %v\n%s", err, stdout.String())
	}
	if passed.RunID != created.RunID || passed.Gate != project.GateIntake || passed.Status != project.GateStatusPass || passed.EventID != "evt-000005" {
		t.Fatalf("passed gate = %#v, want pass evt-000005", passed)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "check", created.RunID, "plan", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("gate check plan exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var planFailed gateCheckOutput
	if err := json.Unmarshal(stdout.Bytes(), &planFailed); err != nil {
		t.Fatalf("decode failed plan gate check: %v\n%s", err, stdout.String())
	}
	if planFailed.Status != project.GateStatusFail || planFailed.EventID != "evt-000006" || !cliGateCheckStatus(planFailed.Checks, "acceptance_criteria", project.GateStatusFail) || !cliGateCheckStatus(planFailed.Checks, "plan_artifact", project.GateStatusFail) || !cliGateCheckStatus(planFailed.Checks, "checklist_artifact", project.GateStatusFail) {
		t.Fatalf("planFailed = %#v, want failed plan gate with missing artifacts", planFailed)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "check", created.RunID, "intake"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("gate check human exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "gate check for run: "+created.RunID) || !strings.Contains(output, "status: pass") || !strings.Contains(output, "event_id: evt-000007") {
		t.Fatalf("human gate output = %q, want pass summary", output)
	}

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"gate", "check", created.RunID, "bogus", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "gate_unknown")
	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"gate", "check", created.RunID, "   ", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "gate_unknown")
	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"gate", "final", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "not_implemented")
}

func cliGateCheckStatus(checks []project.GateCheck, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

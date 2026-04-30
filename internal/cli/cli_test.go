package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

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

func TestKnownCommandGroupIsNotImplemented(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"run", "create"}, &stdout, &stderr, testBuildInfo())

	if exitCode == 0 {
		t.Fatal("exitCode = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	assertHumanError(t, stderr.String(), `command group "run" is not implemented yet`)
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
		[]string{"project", "status"},
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

func TestKnownCommandGroupJSONError(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := Run([]string{"gate", "check", "run-1", "final", "--json"}, &stdout, &stderr, testBuildInfo())

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
	if !strings.Contains(env.Error.Message, "gate") {
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

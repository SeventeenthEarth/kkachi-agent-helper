package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
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

	exitCode := Run([]string{"project", "init"}, &stdout, &stderr, testBuildInfo())

	if exitCode == 0 {
		t.Fatal("exitCode = 0, want non-zero")
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	assertHumanError(t, stderr.String(), `command group "project" is not implemented yet`)
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

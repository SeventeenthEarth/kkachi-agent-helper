package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/SeventeenthEarth/kkachi-agent-helper/internal/project"
)

func projectInitArgs(extra ...string) []string {
	args := []string{
		"project", "init",
		"--project-name", "kkachi-test",
		"--stack", "go",
		"--repo-path", "/tmp/kkachi-test",
		"--commander", "Gongmyeong",
		"--redteam", "Macho",
		"--docs-map-roadmap", "docs/roadmap.md",
		"--docs-map-spec", "docs/specs.md",
		"--docs-map-architecture", "docs/architecture.md",
		"--docs-map-adr-dir", "docs/adr",
		"--docs-map-todo-dir", "docs/todo",
		"--docs-map-spec-dir", "docs/specs",
		"--test-commands", "go test ./...,make test",
		"--backend-policy", "codex",
		"--execution-mode", "production_write",
		"--sot-policy", "existing_sot_basis",
	}
	return append(args, extra...)
}

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

func TestCapabilitiesJSONOutputIsProjectIndependent(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions([]string{"capabilities", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: t.TempDir()})

	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertNoHumanDecoration(t, stdout.String())
	rawJSON := stdout.String()
	for _, want := range []string{`"capabilities_schema_version":"0.1"`, `"project_schema_version":"0.1"`, `"compatibility_flags":`, `"omitted_surfaces":`} {
		if !strings.Contains(rawJSON, want) {
			t.Fatalf("stdout = %q, want raw JSON field %q", rawJSON, want)
		}
	}

	var payload capabilitiesOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if payload.Helper != testBuildInfo() {
		t.Fatalf("helper = %#v, want %#v", payload.Helper, testBuildInfo())
	}
	if payload.CapabilitiesSchemaVersion != capabilitiesSchemaVersion {
		t.Fatalf("capabilities schema version = %q, want %q", payload.CapabilitiesSchemaVersion, capabilitiesSchemaVersion)
	}
	if payload.ProjectSchemaVersion != project.SchemaVersion {
		t.Fatalf("project schema version = %q, want %q", payload.ProjectSchemaVersion, project.SchemaVersion)
	}
	flags := payload.CompatibilityFlags
	if !flags.ProjectInit || !flags.RunLifecycle || !flags.ArtifactInit || !flags.ArtifactList || !flags.ArtifactValidate || !flags.ArtifactMutation || !flags.Gates || !flags.BackendEvidenceRequirements || !flags.DiagnosticsExport || !flags.PhasePlan || !flags.ApprovalRecords || !flags.WorkflowGraphReadonly {
		t.Fatalf("compatibility flags = %#v, want implemented surfaces enabled", flags)
	}
	if flags.InstallCommand {
		t.Fatalf("compatibility flags = %#v, want omitted install surface disabled", flags)
	}
	assertCapabilityCommandGroups(t, payload.CommandGroups)
	if len(payload.DeprecatedSurfaces) != 0 {
		t.Fatalf("deprecated surfaces = %#v, want none", payload.DeprecatedSurfaces)
	}
	if len(payload.OmittedSurfaces) != 1 || payload.OmittedSurfaces[0].Name != "install" || payload.OmittedSurfaces[0].Status != capabilityStatusOmitted || payload.OmittedSurfaces[0].Reason == "" {
		t.Fatalf("omitted surfaces = %#v, want install omitted", payload.OmittedSurfaces)
	}
}

func TestCapabilitiesHumanOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions([]string{"capabilities"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: t.TempDir()})

	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	output := stdout.String()
	for _, want := range []string{"kkachi-agent-helper capabilities", "helper_version: 1.2.3", "project_schema_version: 0.1", "json_contract: use capabilities --json"} {
		if !strings.Contains(output, want) {
			t.Fatalf("stdout = %q, want %q", output, want)
		}
	}
}

func TestCapabilitiesRejectsSubcommands(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions([]string{"capabilities", "extra", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: t.TempDir()})

	if exitCode != ExitUsage {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitUsage)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "unknown_option" {
		t.Fatalf("error code = %q, want unknown_option", env.Error.Code)
	}
}

func TestHelpCommandsExitZeroWithoutProjectState(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{name: "help", args: []string{"help"}, want: []string{"kkachi-agent-helper", "Usage:", "JSON behavior:"}},
		{name: "global help", args: []string{"--help"}, want: []string{"kkachi-agent-helper", "capabilities", "--json"}},
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
		{name: "approval group", args: []string{"approval", "--help"}, want: []string{"kkachi-agent-helper approval", "request <run_id>", "--decision <approved|rejected>"}},
		{name: "graph group", args: []string{"graph", "--help"}, want: []string{"kkachi-agent-helper graph", "validate", "--file <repo-relative-path>"}},
		{name: "help alias", args: []string{"help", "run", "create"}, want: []string{"kkachi-agent-helper run create", "--execution-mode"}},
		{name: "help help", args: []string{"help", "help"}, want: []string{"kkachi-agent-helper help", "[command] [subcommand]", "JSON behavior:"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var stdout bytes.Buffer
			var stderr bytes.Buffer
			exitCode := runWithOptions(tc.args, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: t.TempDir()})
			if exitCode != ExitOK {
				t.Fatalf("exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
			output := stdout.String()
			if output == "" {
				t.Fatal("stdout is empty")
			}
			for _, want := range tc.want {
				if !strings.Contains(output, want) {
					t.Fatalf("stdout = %q, want %q", output, want)
				}
			}
		})
	}
}

func TestImplementedCommandGroupsHaveHelpPages(t *testing.T) {
	for command := range commandGroups {
		if _, ok := helpPages[command]; !ok {
			t.Fatalf("command group %q missing help page", command)
		}
	}
}

func TestHelpJSONOutput(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions([]string{"--json", "run", "create", "--help"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: t.TempDir()})
	if exitCode != ExitOK {
		t.Fatalf("exitCode = %d, want %d stderr=%s", exitCode, ExitOK, stderr.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr = %q, want empty", stderr.String())
	}
	assertNoHumanDecoration(t, stdout.String())
	var payload helpOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, stdout.String())
	}
	if payload.Command != "kkachi-agent-helper run create" || payload.Status != capabilityStatusSupported {
		t.Fatalf("payload = %#v, want run create help", payload)
	}
	if !strings.Contains(payload.JSONBehavior, "structured") {
		t.Fatalf("json_behavior = %q, want structured behavior documentation", payload.JSONBehavior)
	}
}

func TestHelpDoesNotChangeNonHelpUsageErrors(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions([]string{"run", "create", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: tempGitRepo(t)})
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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

func TestProjectInitRequiresBootstrapOptions(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions([]string{"project", "init", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})

	if exitCode != ExitUsage {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitUsage)
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "missing_required_option" || env.Error.Field != "--project-name" {
		t.Fatalf("error = %#v, want missing project-name", env.Error)
	}
}

func TestProjectInitForceReconfiguresCLI(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs("--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	forceArgs := projectInitArgs("--json", "--force")
	for i := range forceArgs {
		if forceArgs[i] == "kkachi-test" {
			forceArgs[i] = "kkachi-reset"
			break
		}
	}
	if code := runWithOptions(forceArgs, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init --force exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var payload projectInitOutput
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("decode force init: %v\n%s", err, stdout.String())
	}
	if !payload.Forced || payload.ReconfiguredEventID != "evt-000002" || payload.ProjectName != "kkachi-reset" {
		t.Fatalf("payload = %#v, want forced reconfigure", payload)
	}
}

func TestProjectInitHumanOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions(
		projectInitArgs(),
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
		projectInitArgs("--json"),
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
	if len(payload.CreatedPaths) != 5 || len(payload.SchemaPaths) != 6 {
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
		projectInitArgs("--json"),
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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

func TestSchemaValidateRequiresSchemaJSONError(t *testing.T) {
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
	if env.Error.Code != "missing_required_option" {
		t.Fatalf("error code = %q, want missing_required_option", env.Error.Code)
	}
	if !strings.Contains(env.Error.Message, "schema") {
		t.Fatalf("message = %q, want command group name", env.Error.Message)
	}
	if env.Error.ExitCode != ExitUsage {
		t.Fatalf("exit code = %d, want %d", env.Error.ExitCode, ExitUsage)
	}
	assertNoHumanDecoration(t, stderr.String())
}

func TestSchemaValidateAndExportCLI(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs("--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"schema", "validate", ".kkachi/status.json", "--schema", "status", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("schema validate status exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var validated schemaValidateOutput
	if err := json.Unmarshal(stdout.Bytes(), &validated); err != nil {
		t.Fatalf("schema validate stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if validated.Schema != "status" || validated.Status != "pass" || validated.FilePath != ".kkachi/status.json" || len(validated.Checks) == 0 {
		t.Fatalf("validated = %#v, want passing status validation", validated)
	}

	statusPath := filepath.Join(repo, ".kkachi", "status.json")
	if err := os.WriteFile(statusPath, []byte(`{"version":"0.1","project_id":"p","active_run_id":null,"active_run_state":null,"last_event_id":"bad","updated_at":"2026-04-30T01:02:03Z","gate_summary":{}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write invalid status: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"schema", "validate", ".kkachi/status.json", "--schema", ".kkachi/schemas/status.schema.json", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("invalid schema validate exit = %d want %d stderr=%s stdout=%s", code, ExitSafety, stderr.String(), stdout.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &validated); err != nil {
		t.Fatalf("invalid schema validate stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if validated.Status != "fail" || !schemaCheckListed(validated.Checks, "last_event_id", "fail") {
		t.Fatalf("validated = %#v, want last_event_id failure", validated)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"schema", "export", "--schema", "status", "--dry-run", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("schema dry-run exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"schema", "export", "--schema", "status", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("schema export under incoherent status exit = %d want safety stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
}

func schemaCheckListed(checks []project.SchemaCheck, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func TestSchemaCLIUsageAndSafetyErrors(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	cases := []struct {
		name     string
		args     []string
		exitCode int
		code     string
	}{
		{name: "unknown schema", args: []string{"schema", "validate", ".kkachi/status.json", "--schema", "unknown", "--json"}, exitCode: ExitUsage, code: "schema_unknown"},
		{name: "duplicate validate schema", args: []string{"schema", "validate", ".kkachi/status.json", "--schema", "status", "--schema", "event", "--json"}, exitCode: ExitUsage, code: "duplicate_option"},
		{name: "missing file", args: []string{"schema", "validate", ".kkachi/missing.json", "--schema", "status", "--json"}, exitCode: ExitSafety, code: "schema_validation_read_failed"},
		{name: "absolute file", args: []string{"schema", "validate", filepath.Join(repo, ".kkachi", "status.json"), "--schema", "status", "--json"}, exitCode: ExitSafety, code: "absolute_path"},
		{name: "export selector conflict", args: []string{"schema", "export", "--all", "--schema", "status", "--json"}, exitCode: ExitUsage, code: "schema_export_selector_conflict"},
		{name: "duplicate export all", args: []string{"schema", "export", "--all", "--all", "--json"}, exitCode: ExitUsage, code: "duplicate_option"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			assertCLIErrorCode(t, runWithOptions(tt.args, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, tt.exitCode, tt.code)
		})
	}
}

func TestSchemaExportCLIWritesAllIdempotentAndRespectsLocks(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	oldConfigSchema := filepath.Join(repo, ".kkachi", "schemas", "config.schema.json")
	if err := os.WriteFile(oldConfigSchema, []byte(`{"$id":"https://kkachi.local/schemas/config.schema.json","version":"0.1"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write old config schema: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"schema", "export", "--all", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("schema export --all exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var exported schemaExportOutput
	if err := json.Unmarshal(stdout.Bytes(), &exported); err != nil {
		t.Fatalf("schema export stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if exported.EventID != "evt-000002" || len(exported.Schemas) != 6 || len(exported.Written) != 1 || exported.Written[0] != ".kkachi/schemas/config.schema.json" || len(exported.Unchanged) != 5 {
		t.Fatalf("exported = %#v, want one refreshed config schema, five unchanged, and evt-000002", exported)
	}
	if !strings.Contains(readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"schema.exported"`) {
		t.Fatalf("events missing schema.exported")
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"schema", "export", "--all", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("idempotent schema export exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	exported = schemaExportOutput{}
	if err := json.Unmarshal(stdout.Bytes(), &exported); err != nil {
		t.Fatalf("idempotent export stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if exported.EventID != "" || len(exported.Written) != 0 || len(exported.Unchanged) != 6 {
		t.Fatalf("idempotent exported = %#v, want no writes and no event", exported)
	}

	fresh := project.LockMetadata{Version: project.LockVersion, LockName: project.ProjectWriteLockName, OwnerPID: os.Getpid(), Hostname: cliMustHostname(t), Command: "fresh schema export", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	writeCLILock(t, repo, project.ProjectWriteLockName, fresh)
	stdout.Reset()
	stderr.Reset()
	code := runWithOptions([]string{"schema", "export", "--schema", "status", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	assertCLIErrorCode(t, code, stdout, stderr, ExitSafety, "lock_conflict")
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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

func assertCapabilityCommandGroups(t *testing.T, groups []capabilityCommandGroup) {
	t.Helper()
	want := []capabilityCommandGroup{
		{Name: "project", Status: capabilityStatusSupported, Subcommands: []string{"init", "status", "doctor"}},
		{Name: "run", Status: capabilityStatusSupported, Subcommands: []string{"create", "activate", "close", "abort", "list", "show"}},
		{Name: "artifact", Status: capabilityStatusSupported, Subcommands: []string{"init", "list", "validate", "write", "append", "set-status"}},
		{Name: "gate", Status: capabilityStatusSupported, Subcommands: []string{"check", "final"}},
		{Name: "event", Status: capabilityStatusSupported, Subcommands: []string{"append"}},
		{Name: "schema", Status: capabilityStatusSupported, Subcommands: []string{"validate", "export", "migrate"}},
		{Name: "lock", Status: capabilityStatusSupported, Subcommands: []string{"recover"}},
		{Name: "diagnostics", Status: capabilityStatusSupported, Subcommands: []string{"export"}},
		{Name: "phase-plan", Status: capabilityStatusSupported, Subcommands: []string{"init", "show", "set", "validate"}},
		{Name: "approval", Status: capabilityStatusSupported, Subcommands: []string{"request", "record", "show"}},
		{Name: "graph", Status: capabilityStatusSupported, Subcommands: []string{"validate", "explain"}},
	}
	if !slices.EqualFunc(groups, want, func(got capabilityCommandGroup, want capabilityCommandGroup) bool {
		return got.Name == want.Name && got.Status == want.Status && slices.Equal(got.Subcommands, want.Subcommands)
	}) {
		t.Fatalf("command groups = %#v, want %#v", groups, want)
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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

func TestArtifactMutationCLIJSONHumanAndFailures(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions(runCreateArgs("Artifact mutate", "--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if err := os.WriteFile(filepath.Join(repo, "plan-source.md"), []byte("# plan\nStatus: pending\n"), 0o600); err != nil {
		t.Fatalf("write source: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "write", created.RunID[:24], "plan.md", "--from", "plan-source.md", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact write exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var written artifactMutationOutput
	if err := json.Unmarshal(stdout.Bytes(), &written); err != nil {
		t.Fatalf("decode write: %v\n%s", err, stdout.String())
	}
	if written.RunID != created.RunID || written.Path != "plan.md" || written.Operation != "write" || written.ArtifactKind != "canonical" || written.SourcePath != "plan-source.md" || written.EventID != "evt-000004" {
		t.Fatalf("written = %#v, want write payload", written)
	}
	if got := readCLIText(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "plan.md")); got != "# plan\nStatus: pending\n" {
		t.Fatalf("plan.md = %q, want source bytes", got)
	}

	if err := os.WriteFile(filepath.Join(repo, "append.md"), []byte("- [x] done\n"), 0o600); err != nil {
		t.Fatalf("write append: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "append", created.RunID, "checklist.md", "--from", "append.md"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact append exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "operation: append") || !strings.Contains(output, "event_id: evt-000005") {
		t.Fatalf("append output = %q, want human mutation summary", output)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "set-status", created.RunID, "checklist.md", "--status", "complete", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact set-status exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var updated artifactMutationOutput
	if err := json.Unmarshal(stdout.Bytes(), &updated); err != nil {
		t.Fatalf("decode set-status: %v\n%s", err, stdout.String())
	}
	if updated.Operation != "set-status" || updated.Status != "complete" || updated.EventID != "evt-000006" {
		t.Fatalf("updated = %#v, want set-status payload", updated)
	}
	if got := readCLIText(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "checklist.md")); !strings.Contains(got, "Status: complete") {
		t.Fatalf("checklist.md = %q, want complete status", got)
	}
	if events := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); !strings.Contains(events, `"operation":"write"`) || !strings.Contains(events, `"operation":"append"`) || !strings.Contains(events, `"operation":"set-status"`) {
		t.Fatalf("events = %s, want mutation operations", events)
	}

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"artifact", "set-status", created.RunID, "selected-cli.json", "--status", "complete", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "artifact_status_not_applicable")
	if got := readCLIText(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "selected-cli.json")); !strings.Contains(got, `"status": "pending"`) {
		t.Fatalf("selected-cli.json = %q, want unchanged pending baseline after rejected set-status", got)
	}

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"artifact", "write", created.RunID, "supplemental/note.md", "--from", "plan-source.md", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "artifact_path_invalid")
	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"artifact", "append", created.RunID, "plan.md", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "from_required")
	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"artifact", "set-status", created.RunID, "plan.md", "--status", "not_applicable", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "artifact_reason_required")
}

func TestArtifactValidateCLIJSONHumanAndFailures(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if failed.Status != project.GateStatusFail || failed.EventID != "evt-000004" || failed.ReportPath == "" || !cliGateCheckStatus(failed.Checks, "intake_status", project.GateStatusFail) || len(failed.MissingEvidence) == 0 {
		t.Fatalf("failed gate = %#v, want intake failure with report path and missing evidence", failed)
	}
	if text := readCLIText(t, filepath.Join(repo, failed.ReportPath)); !strings.Contains(text, `"status": "fail"`) || !strings.Contains(text, `"event_id": "evt-000004"`) {
		t.Fatalf("gate report missing failed result: %s", text)
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
	if passed.RunID != created.RunID || passed.Gate != project.GateIntake || passed.Status != project.GateStatusPass || passed.EventID != "evt-000005" || passed.ReportPath != failed.ReportPath {
		t.Fatalf("passed gate = %#v, want pass evt-000005 and same report path %q", passed, failed.ReportPath)
	}
	if text := readCLIText(t, filepath.Join(repo, passed.ReportPath)); !strings.Contains(text, `"status": "pass"`) || !strings.Contains(text, `"event_id": "evt-000005"`) {
		t.Fatalf("gate report missing passed result: %s", text)
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
	if output := stdout.String(); !strings.Contains(output, "gate check for run: "+created.RunID) || !strings.Contains(output, "status: pass") || !strings.Contains(output, "event_id: evt-000007") || !strings.Contains(output, "report_path: ") {
		t.Fatalf("human gate output = %q, want pass summary with report path", output)
	}

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"gate", "check", created.RunID, "bogus", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "gate_unknown")
	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"gate", "check", created.RunID, "   ", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "gate_unknown")
	stdout.Reset()
	stderr.Reset()
}

func cliGateCheckStatus(checks []project.GateCheck, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func TestGateCheckBackendCLIJSONHumanAndStateUpdates(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions(runCreateArgs("Backend gate", "--execution-mode", "adapter_qa", "--task-id", "gates-003", "--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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
	if code := runWithOptions([]string{"gate", "check", created.RunID, "backend", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("pending backend exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var failed gateCheckOutput
	if err := json.Unmarshal(stdout.Bytes(), &failed); err != nil {
		t.Fatalf("decode failed backend: %v\n%s", err, stdout.String())
	}
	if failed.Status != project.GateStatusFail || failed.EventID != "evt-000004" || !cliGateCheckStatus(failed.Checks, "selected_cli", project.GateStatusFail) || len(failed.MissingEvidence) == 0 {
		t.Fatalf("failed = %#v, want pending backend failure", failed)
	}
	if text := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); !strings.Contains(text, `"type":"gate.failed"`) {
		t.Fatalf("events missing gate.failed: %s", text)
	}

	writeCLIBackendEvidence(t, repo, created.RunID)
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "check", created.RunID[:24], "backend", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("backend pass exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var passed gateCheckOutput
	if err := json.Unmarshal(stdout.Bytes(), &passed); err != nil {
		t.Fatalf("decode passed backend: %v\n%s", err, stdout.String())
	}
	if passed.RunID != created.RunID || passed.Gate != project.GateBackend || passed.Status != project.GateStatusPass || passed.EventID != "evt-000005" || !cliGateCheckStatus(passed.Checks, "bridge_events", project.GateStatusPass) {
		t.Fatalf("passed = %#v, want backend pass evt-000005", passed)
	}
	metadataText := readCLIText(t, filepath.Join(repo, ".kkachi", "runs", created.RunID, "run-metadata.json"))
	if !strings.Contains(metadataText, `"backend"`) || !strings.Contains(metadataText, `"event_id": "evt-000005"`) {
		t.Fatalf("metadata missing backend gate state: %s", metadataText)
	}
	statusText := readCLIText(t, filepath.Join(repo, ".kkachi", "status.json"))
	if !strings.Contains(statusText, `"backend"`) || !strings.Contains(statusText, `"status": "pass"`) {
		t.Fatalf("status missing backend summary: %s", statusText)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "check", created.RunID, "backend"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("backend human exit = %d stderr=%s", code, stderr.String())
	}
	if output := stdout.String(); !strings.Contains(output, "gate: backend") || !strings.Contains(output, "status: pass") || !strings.Contains(output, "selected_cli pass") {
		t.Fatalf("human backend output = %q, want summary and checks", output)
	}
}

func TestRunCreateBackendEvidenceRequiredProductionWrite(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions(runCreateArgs("KAB production", "--backend-evidence", "required", "--task-id", "align-002", "--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s", code, stderr.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v\n%s", err, stdout.String())
	}
	if created.Metadata.ExecutionMode != "production_write" || created.Metadata.BackendEvidence != project.BackendEvidenceRequired {
		t.Fatalf("metadata = %#v, want production_write with required backend evidence", created.Metadata)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "init", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact init exit = %d stderr=%s", code, stderr.String())
	}
	var initialized artifactInitOutput
	if err := json.Unmarshal(stdout.Bytes(), &initialized); err != nil {
		t.Fatalf("decode artifact init: %v\n%s", err, stdout.String())
	}
	for _, artifact := range []string{"selected-cli.json", "capability-check.md", "bridge-session-snapshot.json", "bridge-events.md", "diff.patch", "impl-log.md"} {
		if !slices.Contains(initialized.RequiredArtifacts, artifact) {
			t.Fatalf("required_artifacts = %#v, missing %s", initialized.RequiredArtifacts, artifact)
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "check", created.RunID, "backend", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("pending backend exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var failed gateCheckOutput
	if err := json.Unmarshal(stdout.Bytes(), &failed); err != nil {
		t.Fatalf("decode failed backend: %v\n%s", err, stdout.String())
	}
	if failed.Status != project.GateStatusFail || !cliGateCheckStatus(failed.Checks, "selected_cli", project.GateStatusFail) {
		t.Fatalf("failed = %#v, want missing backend evidence failure", failed)
	}

	writeCLIBackendEvidence(t, repo, created.RunID)
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "check", created.RunID, "backend", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("backend pass exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var passed gateCheckOutput
	if err := json.Unmarshal(stdout.Bytes(), &passed); err != nil {
		t.Fatalf("decode passed backend: %v\n%s", err, stdout.String())
	}
	if passed.Status != project.GateStatusPass || !cliGateCheckStatus(passed.Checks, "bridge_events", project.GateStatusPass) {
		t.Fatalf("passed = %#v, want backend pass", passed)
	}
}

func TestRunCreateRejectsInvalidBackendEvidence(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions(runCreateArgs("Bad backend evidence", "--backend-evidence", "maybe", "--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "run_metadata_invalid")
}

func writeCLIBackendEvidence(t *testing.T, repo string, runID string) {
	t.Helper()
	writeCLIJSONArtifact(t, repo, runID, "selected-cli.json", map[string]any{"version": "0.1", "status": "supported", "backend_type": "codex", "adapter_type": "openai-codex", "source_ledger_ref": "docs/ledger.md#codex", "caveats": []string{}})
	writeCLITextArtifact(t, repo, runID, "capability-check.md", "# capability-check.md\n\nStatus: complete\nBackend Type: codex\nAdapter Type: openai-codex\nCapability: thread resume checked\n")
	writeCLIJSONArtifact(t, repo, runID, "bridge-session-snapshot.json", map[string]any{"session_id": "session-123", "backend_type": "codex", "adapter_type": "openai-codex", "state": "running", "lifecycle_class": "interactive", "open_pendings": 0})
	writeCLITextArtifact(t, repo, runID, "bridge-events.md", "# bridge-events.md\n\nStatus: complete\nEvent: bridge opened a codex session and emitted output\n")
}

func writeCLIJSONArtifact(t *testing.T, repo string, runID string, artifact string, payload any) {
	t.Helper()
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal %s: %v", artifact, err)
	}
	writeCLITextArtifact(t, repo, runID, artifact, string(append(data, '\n')))
}

func writeCLITextArtifact(t *testing.T, repo string, runID string, artifact string, content string) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "runs", runID, filepath.FromSlash(artifact))
	if dir := filepath.Dir(path); dir != path {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", dir, err)
		}
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", artifact, err)
	}
}

func TestGateFinalCLI(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions(runCreateArgs("Final gate", "--json"), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
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

	writeCLIIntakeClassification(t, repo, created.Metadata, "")
	writeCLITextArtifact(t, repo, created.RunID, "sot-basis.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "roadmap-update.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "acceptance-criteria.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "plan.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "checklist.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "diff.patch", "diff --git a/f b/f\n+change\n")
	writeCLITextArtifact(t, repo, created.RunID, "impl-log.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "review.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "redteam/impl-review.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "redteam/test-review.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "redteam/final-gate-review.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "test-log.md", "Status: complete\n")
	writeCLITextArtifact(t, repo, created.RunID, "verification.md", "Status: complete\nVerdict: pass\n")
	writeCLITextArtifact(t, repo, created.RunID, "docs-update.md", "Status: complete\nChanged Docs: README.md\n")

	for _, gate := range []string{project.GateIntake, project.GateSOT, project.GateRoadmap, project.GatePlan, project.GateImplementation, project.GateReview, project.GateVerification, project.GateDocs} {
		stdout.Reset()
		stderr.Reset()
		if code := runWithOptions([]string{"gate", "check", created.RunID, gate, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
			t.Fatalf("gate check %s exit = %d stderr=%s", gate, code, stderr.String())
		}
	}

	// gate final without final-report.md should fail
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "final", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("gate final fail exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var finalFailed gateCheckOutput
	if err := json.Unmarshal(stdout.Bytes(), &finalFailed); err != nil {
		t.Fatalf("decode final failed: %v\n%s", err, stdout.String())
	}
	if finalFailed.Status != project.GateStatusFail || !cliGateCheckStatus(finalFailed.Checks, "final_report", project.GateStatusFail) {
		t.Fatalf("finalFailed = %#v, want final_report failure", finalFailed)
	}

	// Write final-report.md and retry
	writeCLITextArtifact(t, repo, created.RunID, "final-report.md", "Status: complete\n")
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "final", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("gate final pass exit = %d stdout=%s stderr=%s", code, stdout.String(), stderr.String())
	}
	var finalPassed gateCheckOutput
	if err := json.Unmarshal(stdout.Bytes(), &finalPassed); err != nil {
		t.Fatalf("decode final passed: %v\n%s", err, stdout.String())
	}
	if finalPassed.Status != project.GateStatusPass || !cliGateCheckStatus(finalPassed.Checks, "final_report", project.GateStatusPass) {
		t.Fatalf("finalPassed = %#v, want final pass", finalPassed)
	}
}

func TestInstallCommandRemoved(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer

	exitCode := runWithOptions([]string{"install", "templates", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})

	if exitCode != ExitUsage {
		t.Fatalf("exitCode = %d, want %d", exitCode, ExitUsage)
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "unknown_command" {
		t.Fatalf("error code = %q, want unknown_command", env.Error.Code)
	}
}

func TestSchemaMigrateCLIDryRunAndRealRun(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	beforeEvents := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"schema", "migrate", "--from", "0.1", "--to", "0.1", "--dry-run", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("schema migrate dry-run exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var dryRun schemaMigrationOutput
	if err := json.Unmarshal(stdout.Bytes(), &dryRun); err != nil {
		t.Fatalf("dry-run stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if !dryRun.DryRun || dryRun.Status != "pass" || dryRun.EventID != "" || dryRun.BackupPath != "" || len(dryRun.WouldBackup) == 0 || len(dryRun.BackedUp) != 0 {
		t.Fatalf("dryRun = %#v, want read-only migration summary", dryRun)
	}
	if got := readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); got != beforeEvents {
		t.Fatalf("events changed on dry-run\nbefore=%s\nafter=%s", beforeEvents, got)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"schema", "migrate", "--from", "0.1", "--to", "0.1", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("schema migrate exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var migrated schemaMigrationOutput
	if err := json.Unmarshal(stdout.Bytes(), &migrated); err != nil {
		t.Fatalf("migrate stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if migrated.DryRun || migrated.EventID != "evt-000002" || migrated.BackupPath == "" || len(migrated.BackedUp) == 0 || len(migrated.Migrated) != 0 {
		t.Fatalf("migrated = %#v, want no-op backup and event", migrated)
	}
	if !strings.Contains(readCLIText(t, filepath.Join(repo, ".kkachi", "events.jsonl")), `"type":"schema.migrated"`) {
		t.Fatalf("events missing schema.migrated")
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(migrated.BackupPath), ".kkachi", "status.json")); err != nil {
		t.Fatalf("backup status missing: %v", err)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"schema", "migrate", "--from", "0.1", "--to", "0.1"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("schema migrate human exit = %d stderr=%s", code, stderr.String())
	}
	if out := stdout.String(); !strings.Contains(out, "schema migrated: 0.1 -> 0.1") || !strings.Contains(out, "event_id: evt-000003") {
		t.Fatalf("human schema migrate output = %q", out)
	}
}

func TestSchemaMigrateCLIUsageSafetyAndLockErrors(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	cases := []struct {
		name     string
		args     []string
		exitCode int
		code     string
	}{
		{name: "missing from", args: []string{"schema", "migrate", "--to", "0.1", "--json"}, exitCode: ExitUsage, code: "missing_required_option"},
		{name: "missing to", args: []string{"schema", "migrate", "--from", "0.1", "--json"}, exitCode: ExitUsage, code: "missing_required_option"},
		{name: "unknown source", args: []string{"schema", "migrate", "--from", "9.9", "--to", "0.1", "--json"}, exitCode: ExitUsage, code: "schema_migration_unknown_source_version"},
		{name: "unknown target", args: []string{"schema", "migrate", "--from", "0.1", "--to", "0.2", "--json"}, exitCode: ExitUsage, code: "schema_migration_not_registered"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			stdout.Reset()
			stderr.Reset()
			assertCLIErrorCode(t, runWithOptions(tt.args, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, tt.exitCode, tt.code)
		})
	}

	fresh := project.LockMetadata{Version: project.LockVersion, LockName: project.ProjectWriteLockName, OwnerPID: os.Getpid(), Hostname: cliMustHostname(t), Command: "fresh schema migrate", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	writeCLILock(t, repo, project.ProjectWriteLockName, fresh)
	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"schema", "migrate", "--from", "0.1", "--to", "0.1", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitSafety, "lock_conflict")
}

func TestDiagnosticsExportRedactsBundleAndWritesOutput(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	createArgs := []string{"run", "create", "--title", "Diagnostics", "--work-path", "A_development_execution", "--work-mode", "standard", "--urgency", "normal", "--sot-policy", "existing_sot_basis", "--execution-mode", "adapter_qa", "--commander", "Gongmyeong", "--json"}
	if code := runWithOptions(createArgs, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("run create stdout is not JSON: %v\n%s", err, stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"artifact", "init", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("artifact init exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	secret := "sk-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	selectedCLIPath := filepath.Join(repo, ".kkachi", "runs", created.RunID, "selected-cli.json")
	if err := os.WriteFile(selectedCLIPath, []byte(`{"version":"0.1","status":"pending","api_token":"`+secret+`"}`+"\n"), 0o600); err != nil {
		t.Fatalf("write selected-cli secret: %v", err)
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"event", "append", "diagnostic.secret", "--run", created.RunID, "--payload", `{"access_token":"` + secret + `"}`, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("event append exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"gate", "check", created.RunID, "intake", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("gate check exit = %d, want safety stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"diagnostics", "export", "--run", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("diagnostics export exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if strings.Contains(stdout.String(), secret) {
		t.Fatalf("diagnostics bundle leaked secret: %s", stdout.String())
	}
	var bundle diagnosticsExportOutput
	if err := json.Unmarshal(stdout.Bytes(), &bundle); err != nil {
		t.Fatalf("diagnostics stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if bundle.RunID != created.RunID || len(bundle.SchemaVersions) != 6 || len(bundle.GateReports) == 0 || len(bundle.SelectedArtifacts) == 0 {
		t.Fatalf("bundle = %#v, want run, schemas, gate reports, and selected artifacts", bundle)
	}
	foundSelectedCLI := false
	for _, artifact := range bundle.SelectedArtifacts {
		if artifact.Path == ".kkachi/runs/"+created.RunID+"/selected-cli.json" {
			foundSelectedCLI = true
			content, ok := artifact.Content.(map[string]any)
			if !ok || content["api_token"] != project.RedactedPlaceholder {
				t.Fatalf("selected-cli content = %#v, want redacted api_token", artifact.Content)
			}
		}
	}
	if !foundSelectedCLI {
		t.Fatalf("selected-cli artifact missing from diagnostics: %#v", bundle.SelectedArtifacts)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"diagnostics", "export", "--run", created.RunID, "--output", "diagnostics/bundle.json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("diagnostics output export exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stdout.String(), "diagnostics bundle exported: diagnostics/bundle.json") {
		t.Fatalf("human diagnostics output = %q", stdout.String())
	}
	written := readCLIText(t, filepath.Join(repo, "diagnostics", "bundle.json"))
	if strings.Contains(written, secret) || !strings.Contains(written, project.RedactedPlaceholder) {
		t.Fatalf("written diagnostics redaction mismatch: %s", written)
	}
}

func TestDiagnosticsExportUsageAndErrorRedaction(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}

	secret := "sk-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	stdout.Reset()
	stderr.Reset()
	code := runWithOptions([]string{"diagnostics", "export", "--output", "../api_token=" + secret, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo})
	if code != ExitSafety {
		t.Fatalf("exitCode = %d, want safety", code)
	}
	if stdout.Len() != 0 {
		t.Fatalf("stdout = %q, want empty", stdout.String())
	}
	if strings.Contains(stderr.String(), secret) {
		t.Fatalf("diagnostics error leaked secret: %s", stderr.String())
	}
	env := decodeErrorEnvelope(t, stderr.Bytes())
	if env.Error.Code != "path_escape" || !strings.Contains(env.Error.Actual, project.RedactedPlaceholder) {
		t.Fatalf("error = %#v, want redacted path_escape", env.Error)
	}

	stdout.Reset()
	stderr.Reset()
	assertCLIErrorCode(t, runWithOptions([]string{"diagnostics", "export", "--bogus", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}), stdout, stderr, ExitUsage, "unknown_option")
}

func TestPhasePlanCLIInitSetValidateAndDiagnostics(t *testing.T) {
	repo := tempGitRepo(t)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	if code := runWithOptions(projectInitArgs(), &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("project init exit = %d stderr=%s", code, stderr.String())
	}
	stdout.Reset()
	stderr.Reset()
	createArgs := []string{"run", "create", "--title", "Phase plan", "--work-path", "A_development_execution", "--work-mode", "standard", "--urgency", "normal", "--sot-policy", "existing_sot_basis", "--execution-mode", "production_write", "--commander", "Gongmyeong", "--json"}
	if code := runWithOptions(createArgs, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("run create exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var created runCreateOutput
	if err := json.Unmarshal(stdout.Bytes(), &created); err != nil {
		t.Fatalf("run create stdout is not JSON: %v\n%s", err, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"phase-plan", "init", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("phase-plan init exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var initialized phasePlanInitOutput
	if err := json.Unmarshal(stdout.Bytes(), &initialized); err != nil {
		t.Fatalf("phase-plan init stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if initialized.EventID == "" || initialized.PhasePlan.Path != ".kkachi/runs/"+created.RunID+"/phase-plan.yaml" || len(initialized.PhasePlan.Phases) == 0 {
		t.Fatalf("initialized = %#v, want event and phase-plan path", initialized)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"phase-plan", "set", created.RunID, "ask", "--status", "not_applicable", "--reason", "No actionable question.", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("phase-plan set exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var updated phasePlanSetOutput
	if err := json.Unmarshal(stdout.Bytes(), &updated); err != nil {
		t.Fatalf("phase-plan set stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if updated.Phase.ID != "ask" || updated.Phase.Status != project.PhaseStatusNotApplicable || updated.Phase.Reason == "" {
		t.Fatalf("updated = %#v, want ask not_applicable with reason", updated)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"phase-plan", "set", created.RunID, "implement", "--status", "in_progress", "--approval-required", "true", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("phase-plan approval-required set exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &updated); err != nil {
		t.Fatalf("phase-plan approval-required stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if !updated.Phase.ApprovalRequired {
		t.Fatalf("updated = %#v, want approval_required phase", updated)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"approval", "request", created.RunID, "--phase", "implement", "--reason", "High-risk write.", "--evidence", "plan.md#risk", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("approval request exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var approval approvalMutationOutput
	if err := json.Unmarshal(stdout.Bytes(), &approval); err != nil {
		t.Fatalf("approval request stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if approval.Record.Type != project.ApprovalEventRequested || approval.Record.Phase != "implement" || approval.Record.Timestamp == "" {
		t.Fatalf("approval request = %#v, want request record", approval)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"approval", "record", created.RunID, "--phase", "implement", "--decision", "approved", "--by", "master", "--evidence", "messages/123", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("approval record exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if err := json.Unmarshal(stdout.Bytes(), &approval); err != nil {
		t.Fatalf("approval record stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if approval.Record.Type != project.ApprovalEventRecorded || approval.Record.Decision != project.ApprovalDecisionApproved || approval.Record.Approver != "master" {
		t.Fatalf("approval record = %#v, want approved decision", approval)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"approval", "show", created.RunID, "--phase", "implement", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("approval show exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var approvals project.ApprovalShowResult
	if err := json.Unmarshal(stdout.Bytes(), &approvals); err != nil {
		t.Fatalf("approval show stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if len(approvals.Records) != 2 {
		t.Fatalf("approvals = %#v, want request and decision", approvals)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"phase-plan", "validate", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("phase-plan validate exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var validation project.PhasePlanValidationResult
	if err := json.Unmarshal(stdout.Bytes(), &validation); err != nil {
		t.Fatalf("phase-plan validate stdout is not JSON: %v\n%s", err, stdout.String())
	}
	if validation.Status != project.PhasePlanStatusPass {
		t.Fatalf("validation = %#v, want pass", validation)
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"phase-plan", "validate", created.RunID, "--final", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety {
		t.Fatalf("phase-plan final validate exit = %d, want safety stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"phase-plan", "set", created.RunID, "optimize", "--status", "not_applicable", "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitSafety && code != ExitUsage {
		t.Fatalf("phase-plan set missing reason exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	if !strings.Contains(stderr.String(), "phase_reason_required") {
		t.Fatalf("phase-plan set missing reason stderr = %s, want phase_reason_required", stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	if code := runWithOptions([]string{"diagnostics", "export", "--run", created.RunID, "--json"}, &stdout, &stderr, testBuildInfo(), runOptions{workingDir: repo}); code != ExitOK {
		t.Fatalf("diagnostics export exit = %d stderr=%s stdout=%s", code, stderr.String(), stdout.String())
	}
	var bundle diagnosticsExportOutput
	if err := json.Unmarshal(stdout.Bytes(), &bundle); err != nil {
		t.Fatalf("diagnostics stdout is not JSON: %v\n%s", err, stdout.String())
	}
	foundPhasePlan := false
	for _, artifact := range bundle.SelectedArtifacts {
		if artifact.Path == ".kkachi/runs/"+created.RunID+"/phase-plan.yaml" && artifact.Status == "present" {
			foundPhasePlan = true
		}
	}
	if !foundPhasePlan {
		t.Fatalf("diagnostics selected artifacts = %#v, want phase-plan.yaml", bundle.SelectedArtifacts)
	}
	if len(bundle.ApprovalRecords) != 2 {
		t.Fatalf("diagnostics approval records = %#v, want request and decision", bundle.ApprovalRecords)
	}
}

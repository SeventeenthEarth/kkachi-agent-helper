package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestStartGJCRecordsRunLocalStatusAndReusesSession(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-002"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/packet.json")
	packetAbs := writeGJCTestFile(t, repo, runID, "artifacts/gjc/packet.json", []byte(`{"task":"GAJAE-002"}`+"\n"))
	packetAbs = canonicalTestPath(t, packetAbs)
	artifact := writeGJCTestFile(t, repo, runID, "artifacts/plan/gjc-plan.md", []byte("# Candidate plan\n"))
	artifactRef := GJCArtifactRef{Path: packetRelative(runID, "artifacts/plan/gjc-plan.md"), SHA256: sha256String(mustReadBytes(t, artifact))}

	var sessions []string
	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		if invocation.RealUserHome != GJCDefaultRealHome || !envContains(invocation.Env, "HOME="+GJCDefaultRealHome) {
			t.Fatalf("invocation home = %q env=%#v, want default real-user HOME", invocation.RealUserHome, invocation.Env)
		}
		if !envContains(invocation.Env, "GJC_SESSION_ID="+invocation.SessionID) {
			t.Fatalf("env=%#v, want GJC_SESSION_ID", invocation.Env)
		}
		if !slices.Equal(invocation.Args, []string{"ralplan", "--write", "--packet", packetAbs, "--json"}) {
			t.Fatalf("args = %#v, want ralplan packet invocation", invocation.Args)
		}
		sessions = append(sessions, invocation.SessionID)
		receipt := map[string]any{
			"status":                 "ralplan_ready",
			"artifact_refs":          []GJCArtifactRef{artifactRef},
			"current_required_actor": "kas",
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	result, err := StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-002", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("StartGJC(first) error = %v", err)
	}
	if result.Status.SchemaVersion != GJCSchemaVersion || result.Status.Process.Status != "ralplan_ready" || result.Status.RealUserHome != GJCDefaultRealHome {
		t.Fatalf("status = %#v, want successful ralplan candidate status", result.Status)
	}
	if result.Status.StatusPath != packetRelative(runID, "artifacts/gjc/status.json") || result.Status.StatusHash == "" || result.EventID == "" {
		t.Fatalf("status path/hash/event = %#v event=%q, want persisted status evidence", result.Status, result.EventID)
	}

	shown, err := ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ShowGJCStatus() error = %v", err)
	}
	if shown.Status.GJCSessionID != result.Status.GJCSessionID {
		t.Fatalf("shown session = %q, want %q", shown.Status.GJCSessionID, result.Status.GJCSessionID)
	}

	_, err = StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-002", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("StartGJC(second) error = %v", err)
	}
	if len(sessions) != 2 || sessions[0] == "" || sessions[0] != sessions[1] {
		t.Fatalf("sessions = %#v, want stable run-local session reuse", sessions)
	}
}

func TestStartGJCRejectsChecksumMismatchAndRecordsFailureStatus(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-002"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/packet.json")
	writeGJCTestFile(t, repo, runID, "artifacts/gjc/packet.json", []byte(`{"task":"GAJAE-002"}`+"\n"))
	writeGJCTestFile(t, repo, runID, "artifacts/plan/gjc-plan.md", []byte("# Candidate plan\n"))

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		receipt := map[string]any{
			"status":        "ralplan_ready",
			"artifact_refs": []GJCArtifactRef{{Path: packetRelative(runID, "artifacts/plan/gjc-plan.md"), SHA256: "sha256:" + strings.Repeat("0", 64)}},
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	result, err := StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-002", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(4)})
	assertProblemCode(t, err, "gjc_checksum_mismatch")
	if result.Status.Error == nil || result.Status.Error.Code != "gjc_checksum_mismatch" || result.Status.Process.Status != GJCStatusFailed {
		t.Fatalf("result status = %#v, want recorded checksum failure", result.Status)
	}
	if _, err := os.Stat(filepath.Join(repo, filepath.FromSlash(packetRelative(runID, "artifacts/gjc/status.json")))); err != nil {
		t.Fatalf("status evidence missing after failure: %v", err)
	}
}

func TestShowGJCStatusRejectsTamperedStatusHash(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-002"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/packet.json")
	writeGJCTestFile(t, repo, runID, "artifacts/gjc/packet.json", []byte(`{"task":"GAJAE-002"}`+"\n"))
	artifact := writeGJCTestFile(t, repo, runID, "artifacts/plan/gjc-plan.md", []byte("# Candidate plan\n"))
	artifactRef := GJCArtifactRef{Path: packetRelative(runID, "artifacts/plan/gjc-plan.md"), SHA256: sha256String(mustReadBytes(t, artifact))}

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		receipt := map[string]any{
			"status":                 "ralplan_ready",
			"artifact_refs":          []GJCArtifactRef{artifactRef},
			"current_required_actor": "kas",
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	result, err := StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-002", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("StartGJC() error = %v", err)
	}
	statusPath := filepath.Join(repo, filepath.FromSlash(result.Status.StatusPath))
	statusData := mustReadBytes(t, statusPath)
	tampered := strings.Replace(string(statusData), `"current_required_actor": "kas"`, `"current_required_actor": "color"`, 1)
	if tampered == string(statusData) {
		t.Fatalf("test fixture did not find current_required_actor to tamper")
	}
	if err := os.WriteFile(statusPath, []byte(tampered), 0o600); err != nil {
		t.Fatalf("tamper status: %v", err)
	}

	_, err = ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	assertProblemCode(t, err, "gjc_status_hash_mismatch")
}

func TestGJCEnvOverridesInheritedHomeAndSession(t *testing.T) {
	t.Setenv("HOME", "/tmp/stale-home")
	t.Setenv("GJC_SESSION_ID", "stale-session")
	t.Setenv("GAJAE_KEEP", "preserved")

	sessionID := "gjc-run-20260625T184550Z-180c8b6e9385"
	env := gjcEnv(GJCDefaultRealHome, sessionID)

	if countEnvPrefix(env, "HOME=") != 1 || !envContains(env, "HOME="+GJCDefaultRealHome) {
		t.Fatalf("env HOME entries = %#v, want exactly normalized HOME", envValuesWithPrefix(env, "HOME="))
	}
	if countEnvPrefix(env, "GJC_SESSION_ID=") != 1 || !envContains(env, "GJC_SESSION_ID="+sessionID) {
		t.Fatalf("env session entries = %#v, want exactly normalized GJC_SESSION_ID", envValuesWithPrefix(env, "GJC_SESSION_ID="))
	}
	if !envContains(env, "GAJAE_KEEP=preserved") {
		t.Fatalf("env lost unrelated entry: %#v", env)
	}
	if env[len(env)-2] != "HOME="+GJCDefaultRealHome || env[len(env)-1] != "GJC_SESSION_ID="+sessionID {
		t.Fatalf("env tail = %#v, want normalized HOME/session appended last", env[len(env)-2:])
	}
}

func TestStartGJCRejectsCrossRunPacketBeforeRunner(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	first, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun(first) error = %v", err)
	}
	secondOptions := deterministicCreateRunOptions()
	secondOptions.Now = testRunNow(4)
	secondOptions.RandomHex = func(int) (string, error) { return "222222222222", nil }
	second, err := CreateRun(root, secondOptions)
	if err != nil {
		t.Fatalf("CreateRun(second) error = %v", err)
	}
	writeGJCTestFile(t, repo, second.Metadata.RunID, "artifacts/gjc/packet.json", []byte(`{"task":"GAJAE-002"}`+"\n"))

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		t.Fatalf("runner should not be called for cross-run packet")
		return gjcRunnerResult{}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	_, err = StartGJC(root, GJCStartOptions{RunID: first.Metadata.RunID, TaskID: "GAJAE-002", Packet: packetRelative(second.Metadata.RunID, "artifacts/gjc/packet.json"), CommandKind: "start-ralplan", Now: testRunNow(5)})
	assertProblemCode(t, err, "gjc_ref_cross_run")
}

func writeGJCTestFile(t *testing.T, repo string, runID string, relative string, content []byte) string {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(packetRelative(runID, relative)))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir test file: %v", err)
	}
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return path
}

func packetRelative(runID string, relative string) string {
	return filepath.ToSlash(filepath.Join(RunRootPath, runID, relative))
}

func mustReadBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func envContains(env []string, want string) bool {
	for _, item := range env {
		if item == want {
			return true
		}
	}
	return false
}

func countEnvPrefix(env []string, prefix string) int {
	count := 0
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			count++
		}
	}
	return count
}

func envValuesWithPrefix(env []string, prefix string) []string {
	var values []string
	for _, item := range env {
		if strings.HasPrefix(item, prefix) {
			values = append(values, item)
		}
	}
	return values
}

func canonicalTestPath(t *testing.T, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatalf("canonicalize %s: %v", path, err)
	}
	return resolved
}

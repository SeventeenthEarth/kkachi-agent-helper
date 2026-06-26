package project

import (
	"encoding/json"
	"errors"
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

func TestStartGJCPreservesPacketRefAndShowStatusValidatesIt(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-003"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/gjc-ralplan-packet.yaml")
	packetAbs := writeGJCTestFile(t, repo, runID, "artifacts/gjc/gjc-ralplan-packet.yaml", []byte("packet_kind: ralplan\n"))
	packetHash := sha256String(mustReadBytes(t, packetAbs))
	artifact := writeGJCTestFile(t, repo, runID, "artifacts/plan/gjc-plan.md", []byte("# Candidate plan\n"))
	artifactRef := GJCArtifactRef{Path: packetRelative(runID, "artifacts/plan/gjc-plan.md"), SHA256: sha256String(mustReadBytes(t, artifact))}

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		receipt := map[string]any{
			"status":        "ralplan_ready",
			"artifact_refs": []GJCArtifactRef{artifactRef},
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	result, err := StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-003", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("StartGJC() error = %v", err)
	}
	if result.Status.Packet.Path != packetRel || result.Status.Packet.SHA256 != packetHash {
		t.Fatalf("packet_ref = %#v, want path %q hash %q", result.Status.Packet, packetRel, packetHash)
	}

	shown, err := ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	if err != nil {
		t.Fatalf("ShowGJCStatus() error = %v", err)
	}
	if shown.Status.Packet != result.Status.Packet {
		t.Fatalf("shown packet_ref = %#v, want %#v", shown.Status.Packet, result.Status.Packet)
	}
}

func TestShowGJCStatusRejectsPacketRefHashDrift(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-003"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/gjc-ralplan-packet.yaml")
	writeGJCTestFile(t, repo, runID, "artifacts/gjc/gjc-ralplan-packet.yaml", []byte("packet_kind: ralplan\n"))
	artifact := writeGJCTestFile(t, repo, runID, "artifacts/plan/gjc-plan.md", []byte("# Candidate plan\n"))
	artifactRef := GJCArtifactRef{Path: packetRelative(runID, "artifacts/plan/gjc-plan.md"), SHA256: sha256String(mustReadBytes(t, artifact))}

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		receipt := map[string]any{
			"status":        "ralplan_ready",
			"artifact_refs": []GJCArtifactRef{artifactRef},
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	if _, err := StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-003", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(4)}); err != nil {
		t.Fatalf("StartGJC() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, filepath.FromSlash(packetRel)), []byte("packet_kind: ralplan\nchanged: true\n"), 0o600); err != nil {
		t.Fatalf("tamper packet: %v", err)
	}

	_, err = ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	assertProblemCode(t, err, "gjc_checksum_mismatch")
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

func TestStartGJCRejectsMissingPacketBeforeRunner(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-003"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		t.Fatalf("runner should not be called for missing packet")
		return gjcRunnerResult{}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	_, err = StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-003", Packet: packetRelative(runID, "artifacts/gjc/missing-packet.json"), CommandKind: "start-ralplan", Now: testRunNow(5)})
	assertProblemCode(t, err, "gjc_ref_missing")
}

func TestStartGJCRejectsNonRegularPacketBeforeRunner(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-003"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/packet-dir")
	if err := os.MkdirAll(filepath.Join(repo, filepath.FromSlash(packetRel)), 0o755); err != nil {
		t.Fatalf("mkdir packet dir: %v", err)
	}

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		t.Fatalf("runner should not be called for non-regular packet")
		return gjcRunnerResult{}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	_, err = StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-003", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(5)})
	assertProblemCode(t, err, "gjc_ref_invalid")
}

func TestStartGJCRejectsEscapingPacketBeforeRunner(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-003"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		t.Fatalf("runner should not be called for escaping packet")
		return gjcRunnerResult{}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	_, err = StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-003", Packet: "../escape.json", CommandKind: "start-ralplan", Now: testRunNow(5)})
	assertProblemCode(t, err, "path_escape")
}

func TestShowGJCStatusRejectsUnsupportedStatus(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-003"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/gjc-ralplan-packet.yaml")
	writeGJCTestFile(t, repo, runID, "artifacts/gjc/gjc-ralplan-packet.yaml", []byte("packet_kind: ralplan\n"))
	artifact := writeGJCTestFile(t, repo, runID, "artifacts/plan/gjc-plan.md", []byte("# Candidate plan\n"))
	artifactRef := GJCArtifactRef{Path: packetRelative(runID, "artifacts/plan/gjc-plan.md"), SHA256: sha256String(mustReadBytes(t, artifact))}

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		receipt := map[string]any{
			"status":        "ralplan_ready",
			"artifact_refs": []GJCArtifactRef{artifactRef},
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	result, err := StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-003", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("StartGJC() error = %v", err)
	}
	status := result.Status
	status.Process.Status = "accepted"
	hash, err := computeGJCStatusHash(status)
	if err != nil {
		t.Fatalf("compute status hash: %v", err)
	}
	status.StatusHash = hash
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	statusPath := filepath.Join(repo, filepath.FromSlash(status.StatusPath))
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write unsupported status: %v", err)
	}

	_, err = ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	assertProblemCode(t, err, "gjc_status_unsupported")
}

func TestStartGJCRejectsRalplanReadyWithoutPlanArtifact(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-004"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/gjc-ralplan-packet.yaml")
	writeGJCTestFile(t, repo, runID, "artifacts/gjc/gjc-ralplan-packet.yaml", []byte("packet_kind: ralplan\n"))
	nonPlanArtifact := writeGJCTestFile(t, repo, runID, "artifacts/gjc/receipt.json", []byte(`{"stage":"plan"}`+"\n"))
	artifactRef := GJCArtifactRef{Path: packetRelative(runID, "artifacts/gjc/receipt.json"), SHA256: sha256String(mustReadBytes(t, nonPlanArtifact))}

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		receipt := map[string]any{
			"status":        "ralplan_ready",
			"artifact_refs": []GJCArtifactRef{artifactRef},
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	_, err = StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-004", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(4)})
	assertProblemCode(t, err, "gjc_plan_artifact_missing")
}

func TestGJCCallbackRecordsIdempotentEvidenceAndRejectsConflicts(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-004"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/gjc-ralplan-packet.yaml")
	writeGJCTestFile(t, repo, runID, "artifacts/gjc/gjc-ralplan-packet.yaml", []byte("packet_kind: ralplan\n"))
	planArtifact := writeGJCTestFile(t, repo, runID, "artifacts/plan/gjc-plan.md", []byte("# Candidate plan\n"))
	artifactRef := GJCArtifactRef{Path: packetRelative(runID, "artifacts/plan/gjc-plan.md"), SHA256: sha256String(mustReadBytes(t, planArtifact))}

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		receipt := map[string]any{
			"status":        "ralplan_ready",
			"artifact_refs": []GJCArtifactRef{artifactRef},
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	start, err := StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-004", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("StartGJC() error = %v", err)
	}

	callback := GJCCallbackOptions{
		RunID:            runID,
		TaskID:           "GAJAE-004",
		Status:           "callback_delivered",
		Result:           "delivered",
		IdempotencyKey:   "gajae004-plan-ready",
		SourceStatusHash: start.Status.StatusHash,
		NotificationRef:  "no-wake-claim",
		Now:              testRunNow(5),
	}
	first, err := RecordGJCCallback(root, callback)
	if err != nil {
		t.Fatalf("RecordGJCCallback(first) error = %v", err)
	}
	if first.Status.Callback == nil || first.Status.Callback.IdempotencyKey != callback.IdempotencyKey || first.Status.Callback.NotificationRef != "no-wake-claim" || first.Status.Callback.SameThreadWakeClaim {
		t.Fatalf("callback evidence = %#v, want idempotency, no-wake-claim metadata, and no same-thread wake claim", first.Status.Callback)
	}

	notificationCallback := callback
	notificationCallback.IdempotencyKey = "gajae004-plan-ready-notification"
	notificationCallback.SourceStatusHash = first.Status.StatusHash
	notificationCallback.NotificationRef = "discord:origin-thread"
	notified, err := RecordGJCCallback(root, notificationCallback)
	if err != nil {
		t.Fatalf("RecordGJCCallback(notification) error = %v", err)
	}
	if notified.Status.Callback == nil || notified.Status.Callback.NotificationRef != "discord:origin-thread" || notified.Status.Callback.SameThreadWakeClaim {
		t.Fatalf("notification callback evidence = %#v, want metadata preserved without wake claim", notified.Status.Callback)
	}

	second, err := RecordGJCCallback(root, notificationCallback)
	if err != nil {
		t.Fatalf("RecordGJCCallback(second) error = %v", err)
	}
	if second.Status.Callback == nil || second.Status.Callback.LastCallbackStatus != "delivered" {
		t.Fatalf("second callback evidence = %#v, want idempotent delivered evidence", second.Status.Callback)
	}

	conflict := notificationCallback
	conflict.SourceStatusHash = "sha256:" + strings.Repeat("1", 64)
	_, err = RecordGJCCallback(root, conflict)
	assertProblemCode(t, err, "gjc_callback_idempotency_conflict")
}

func TestGJCPlanLockBindsAcceptedHashAndRejectsDrift(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.TaskID = "GAJAE-004"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	packetRel := packetRelative(runID, "artifacts/gjc/gjc-ralplan-packet.yaml")
	writeGJCTestFile(t, repo, runID, "artifacts/gjc/gjc-ralplan-packet.yaml", []byte("packet_kind: ralplan\n"))
	planArtifact := writeGJCTestFile(t, repo, runID, "artifacts/plan/gjc-plan.md", []byte("# Candidate plan\n"))
	artifactRef := GJCArtifactRef{Path: packetRelative(runID, "artifacts/plan/gjc-plan.md"), SHA256: sha256String(mustReadBytes(t, planArtifact))}

	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		receipt := map[string]any{
			"status":        "ralplan_ready",
			"artifact_refs": []GJCArtifactRef{artifactRef},
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()

	start, err := StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-004", Packet: packetRel, CommandKind: "start-ralplan", Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("StartGJC() error = %v", err)
	}
	locked, err := LockGJCPlan(root, GJCPlanLockOptions{RunID: runID, AcceptedPlanHash: start.Status.Plan.ArtifactHash, ApprovalRef: "blue-plan-vet:t_48ebdfef", Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("LockGJCPlan() error = %v", err)
	}
	if locked.Status.Plan.LockStatus != "locked" || locked.Status.Plan.AcceptedPlanHash != start.Status.Plan.ArtifactHash {
		t.Fatalf("plan evidence = %#v, want locked accepted hash", locked.Status.Plan)
	}

	if err := os.WriteFile(planArtifact, []byte("# Candidate plan\nchanged\n"), 0o600); err != nil {
		t.Fatalf("tamper plan artifact: %v", err)
	}
	_, err = ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	assertProblemCode(t, err, "gjc_plan_lock_conflict")
	var problem *Problem
	if !errors.As(err, &problem) || problem.Path != packetRelative(runID, "artifacts/gjc/plan-conflict.json") {
		t.Fatalf("problem = %#v, want plan-conflict report path", err)
	}
	conflictPath := filepath.Join(repo, filepath.FromSlash(packetRelative(runID, "artifacts/gjc/plan-conflict.json")))
	var report map[string]string
	if err := json.Unmarshal(mustReadBytes(t, conflictPath), &report); err != nil {
		t.Fatalf("decode conflict report: %v", err)
	}
	if report["accepted_plan_hash"] != start.Status.Plan.ArtifactHash || report["current_artifact_hash"] == "" || report["status"] != "plan_conflict_reported" {
		t.Fatalf("conflict report = %#v, want accepted/current hashes", report)
	}
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

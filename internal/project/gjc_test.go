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
	if first.Status.Callback == nil || first.Status.Callback.IdempotencyKey != callback.IdempotencyKey || first.Status.Callback.NotificationRef != "no-wake-claim" || first.Status.Callback.NotificationStatus != "no_wake_claim" || first.Status.Callback.WakeEvidenceStatus != "missing_watcher_evidence" || first.Status.Callback.SameThreadWakeClaim {
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
	if notified.Status.Callback == nil || notified.Status.Callback.NotificationRef != "discord:origin-thread" || notified.Status.Callback.NotificationStatus != "metadata_recorded_no_wake_claim" || notified.Status.Callback.WakeEvidenceStatus != "missing_watcher_evidence" || notified.Status.Callback.SameThreadWakeClaim {
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

func TestGJCCallbackRejectsUnsupportedWakeEvidenceClaims(t *testing.T) {
	cases := []struct {
		name     string
		mutate   func(*GJCCallback)
		wantCode string
	}{
		{
			name: "unsupported notification status",
			mutate: func(callback *GJCCallback) {
				callback.NotificationStatus = "wake_ready"
			},
			wantCode: "gjc_callback_notification_status_unsupported",
		},
		{
			name: "unsupported wake evidence status",
			mutate: func(callback *GJCCallback) {
				callback.WakeEvidenceStatus = "same_thread_wake_verified"
			},
			wantCode: "gjc_callback_wake_evidence_unsupported",
		},
		{
			name: "same thread wake claim",
			mutate: func(callback *GJCCallback) {
				callback.SameThreadWakeClaim = true
			},
			wantCode: "gjc_callback_wake_claim_unsupported",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
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
			result, err := RecordGJCCallback(root, GJCCallbackOptions{
				RunID:            runID,
				TaskID:           "GAJAE-004",
				IdempotencyKey:   "gajae004-plan-ready",
				SourceStatusHash: start.Status.StatusHash,
				NotificationRef:  "discord:origin-thread",
				Now:              testRunNow(5),
			})
			if err != nil {
				t.Fatalf("RecordGJCCallback() error = %v", err)
			}
			status := result.Status
			if status.Callback == nil {
				t.Fatal("callback evidence missing from status")
			}
			callback := *status.Callback
			tc.mutate(&callback)
			status.Callback = &callback
			rewriteGJCStatusForTest(t, root, status)

			_, err = ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
			assertProblemCode(t, err, tc.wantCode)
		})
	}
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

func TestAttachGJCKATEvidenceRecordsRunLocalEvidenceWithoutAcceptance(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	start := startGJCUltragoalReadyForTest(t, root, repo, runID)
	kat := writeGJCKATEvidenceForTest(t, repo, runID, map[string]any{
		"schema_version":      "kat.status.v1",
		"run_id":              runID,
		"status":              "failed",
		"extractor_status":    "degraded",
		"command_exit_code":   1,
		"self_approval":       false,
		"final_accepted":      false,
		"review_approved":     false,
		"waiver_approved":     false,
		"candidate_evidence":  true,
		"source_status_hash":  start.Status.StatusHash,
		"current_authority":   "KAS/Blue/color/MAR/final",
		"completion_boundary": "not_final_acceptance",
	})

	result, err := AttachGJCKATEvidence(root, GJCKATAttachOptions{
		RunID:            runID,
		StatusPath:       kat.status.Path,
		StatusHash:       kat.status.SHA256,
		SummaryPath:      kat.summary.Path,
		SummaryHash:      kat.summary.SHA256,
		SummaryMDPath:    kat.summaryMD.Path,
		SummaryMDHash:    kat.summaryMD.SHA256,
		RawLogPath:       kat.rawLog.Path,
		RawLogHash:       kat.rawLog.SHA256,
		AttachmentStatus: "kat_evidence_ready",
		Now:              testRunNow(8),
	})
	if err != nil {
		t.Fatalf("AttachGJCKATEvidence() error = %v", err)
	}
	if result.Status.KAT == nil {
		t.Fatalf("KAT evidence missing from status")
	}
	if result.Status.Process.Status != "ultragoal_ready" {
		t.Fatalf("process status = %q, want candidate ultragoal_ready preserved", result.Status.Process.Status)
	}
	if result.Status.KAT.RunID != runID || result.Status.KAT.StatusRef != kat.status || result.Status.KAT.RawLogHash != kat.rawLog.SHA256 {
		t.Fatalf("KAT evidence = %#v, want run-local refs and hashes", result.Status.KAT)
	}
	if result.Status.KAT.ExtractorStatus != "degraded" || result.Status.KAT.CommandExitCode != 1 || result.Status.KAT.AttachmentStatus != "kat_evidence_ready" {
		t.Fatalf("KAT status fields = %#v, want factual extractor/exit/attachment evidence", result.Status.KAT)
	}
	if result.Status.KAT.SourceStatusHash != start.Status.StatusHash {
		t.Fatalf("source status hash = %q, want %q", result.Status.KAT.SourceStatusHash, start.Status.StatusHash)
	}
	if result.Status.CurrentRequiredActor != GJCActorColor {
		t.Fatalf("current actor = %q, want color review routing without acceptance", result.Status.CurrentRequiredActor)
	}
}

func TestAttachGJCKATEvidenceFailsClosed(t *testing.T) {
	tests := []struct {
		name       string
		mutate     func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any)
		wantCode   string
		wantNoFile bool
	}{
		{
			name: "missing raw log hash",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				opts.RawLogHash = ""
			},
			wantCode: "gjc_checksum_malformed",
		},
		{
			name: "status run id mismatch",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				status["run_id"] = "run-20260626T134058Z-deadbeef0000"
				rewriteGJCKATStatusForTest(t, repo, opts, status)
			},
			wantCode: "gjc_kat_run_id_mismatch",
		},
		{
			name: "unsupported final status",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				status["status"] = "final_accepted"
				rewriteGJCKATStatusForTest(t, repo, opts, status)
			},
			wantCode: "gjc_kat_status_unsupported",
		},
		{
			name: "self approval claim",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				status["self_approval"] = true
				rewriteGJCKATStatusForTest(t, repo, opts, status)
			},
			wantCode: "gjc_kat_authority_claim",
		},
		{
			name: "mar accepted claim",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				status["mar_accepted"] = true
				rewriteGJCKATStatusForTest(t, repo, opts, status)
			},
			wantCode: "gjc_kat_authority_claim",
		},
		{
			name: "mar approved claim",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				status["mar_approved"] = true
				rewriteGJCKATStatusForTest(t, repo, opts, status)
			},
			wantCode: "gjc_kat_authority_claim",
		},
		{
			name: "malformed json",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				path := filepath.Join(repo, filepath.FromSlash(opts.StatusPath))
				if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
					t.Fatalf("write malformed KAT status: %v", err)
				}
				opts.StatusHash = sha256String(mustReadBytes(t, path))
			},
			wantCode: "gjc_kat_status_invalid_json",
		},
		{
			name: "missing command exit code",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				delete(status, "command_exit_code")
				rewriteGJCKATStatusForTest(t, repo, opts, status)
			},
			wantCode: "gjc_kat_status_invalid",
		},
		{
			name: "missing source status hash",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				delete(status, "source_status_hash")
				rewriteGJCKATStatusForTest(t, repo, opts, status)
			},
			wantCode: "gjc_kat_status_invalid",
		},
		{
			name: "malformed source status hash",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				status["source_status_hash"] = "sha256:not-a-real-hash"
				rewriteGJCKATStatusForTest(t, repo, opts, status)
			},
			wantCode: "gjc_checksum_malformed",
		},
		{
			name: "mismatched source status hash",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				status["source_status_hash"] = "sha256:" + strings.Repeat("1", 64)
				rewriteGJCKATStatusForTest(t, repo, opts, status)
			},
			wantCode: "gjc_kat_source_status_hash_mismatch",
		},
		{
			name: "cross run summary",
			mutate: func(t *testing.T, repo string, runID string, opts *GJCKATAttachOptions, status map[string]any) {
				otherRun := "run-20260626T134058Z-deadbeef0000"
				path := writeGJCTestFile(t, repo, otherRun, "artifacts/test/summary.json", []byte(`{"other":true}`+"\n"))
				opts.SummaryPath = packetRelative(otherRun, "artifacts/test/summary.json")
				opts.SummaryHash = sha256String(mustReadBytes(t, path))
			},
			wantCode: "gjc_ref_cross_run",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			created, err := CreateRun(root, deterministicCreateRunOptions())
			if err != nil {
				t.Fatalf("CreateRun() error = %v", err)
			}
			runID := created.Metadata.RunID
			start := startGJCUltragoalReadyForTest(t, root, repo, runID)
			status := map[string]any{
				"schema_version":     "kat.status.v1",
				"run_id":             runID,
				"status":             "passed",
				"extractor_status":   "precise",
				"command_exit_code":  0,
				"self_approval":      false,
				"final_accepted":     false,
				"review_approved":    false,
				"waiver_approved":    false,
				"candidate_evidence": true,
				"source_status_hash": start.Status.StatusHash,
			}
			kat := writeGJCKATEvidenceForTest(t, repo, runID, status)
			opts := GJCKATAttachOptions{
				RunID:            runID,
				StatusPath:       kat.status.Path,
				StatusHash:       kat.status.SHA256,
				SummaryPath:      kat.summary.Path,
				SummaryHash:      kat.summary.SHA256,
				SummaryMDPath:    kat.summaryMD.Path,
				SummaryMDHash:    kat.summaryMD.SHA256,
				RawLogPath:       kat.rawLog.Path,
				RawLogHash:       kat.rawLog.SHA256,
				AttachmentStatus: "kat_evidence_ready",
				Now:              testRunNow(9),
			}
			tc.mutate(t, repo, runID, &opts, status)

			_, err = AttachGJCKATEvidence(root, opts)
			assertProblemCode(t, err, tc.wantCode)
		})
	}
}

func TestShowGJCStatusRevalidatesPersistedKATStatusRefContent(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	start := startGJCUltragoalReadyForTest(t, root, repo, runID)
	statusEvidence := map[string]any{
		"schema_version":     "kat.status.v1",
		"run_id":             runID,
		"status":             "passed",
		"extractor_status":   "precise",
		"command_exit_code":  0,
		"self_approval":      false,
		"final_accepted":     false,
		"review_approved":    false,
		"waiver_approved":    false,
		"candidate_evidence": true,
		"source_status_hash": start.Status.StatusHash,
	}
	kat := writeGJCKATEvidenceForTest(t, repo, runID, statusEvidence)
	attached, err := AttachGJCKATEvidence(root, GJCKATAttachOptions{
		RunID:            runID,
		StatusPath:       kat.status.Path,
		StatusHash:       kat.status.SHA256,
		SummaryPath:      kat.summary.Path,
		SummaryHash:      kat.summary.SHA256,
		SummaryMDPath:    kat.summaryMD.Path,
		SummaryMDHash:    kat.summaryMD.SHA256,
		RawLogPath:       kat.rawLog.Path,
		RawLogHash:       kat.rawLog.SHA256,
		AttachmentStatus: "kat_evidence_ready",
		Now:              testRunNow(8),
	})
	if err != nil {
		t.Fatalf("AttachGJCKATEvidence() error = %v", err)
	}

	statusEvidence["mar_accepted"] = true
	katStatusHash := rewriteGJCKATStatusContentForTest(t, repo, attached.Status.KAT.StatusRef.Path, statusEvidence)
	persisted := attached.Status
	persisted.KAT.StatusRef.SHA256 = katStatusHash
	persisted.KAT.StatusHash = katStatusHash
	persistedHash, err := computeGJCStatusHash(persisted)
	if err != nil {
		t.Fatalf("compute persisted status hash: %v", err)
	}
	persisted.StatusHash = persistedHash
	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		t.Fatalf("marshal persisted status: %v", err)
	}
	statusPath := filepath.Join(repo, filepath.FromSlash(persisted.StatusPath))
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write persisted status: %v", err)
	}

	_, err = ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	assertProblemCode(t, err, "gjc_kat_authority_claim")
}

func TestShowGJCStatusRejectsPersistedKATSourceStatusHashDrift(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	start := startGJCUltragoalReadyForTest(t, root, repo, runID)
	statusEvidence := map[string]any{
		"schema_version":     "kat.status.v1",
		"run_id":             runID,
		"status":             "passed",
		"extractor_status":   "precise",
		"command_exit_code":  0,
		"self_approval":      false,
		"final_accepted":     false,
		"review_approved":    false,
		"waiver_approved":    false,
		"candidate_evidence": true,
		"source_status_hash": start.Status.StatusHash,
	}
	kat := writeGJCKATEvidenceForTest(t, repo, runID, statusEvidence)
	attached, err := AttachGJCKATEvidence(root, GJCKATAttachOptions{
		RunID:            runID,
		StatusPath:       kat.status.Path,
		StatusHash:       kat.status.SHA256,
		SummaryPath:      kat.summary.Path,
		SummaryHash:      kat.summary.SHA256,
		SummaryMDPath:    kat.summaryMD.Path,
		SummaryMDHash:    kat.summaryMD.SHA256,
		RawLogPath:       kat.rawLog.Path,
		RawLogHash:       kat.rawLog.SHA256,
		AttachmentStatus: "kat_evidence_ready",
		Now:              testRunNow(8),
	})
	if err != nil {
		t.Fatalf("AttachGJCKATEvidence() error = %v", err)
	}

	statusEvidence["source_status_hash"] = "sha256:" + strings.Repeat("2", 64)
	katStatusHash := rewriteGJCKATStatusContentForTest(t, repo, attached.Status.KAT.StatusRef.Path, statusEvidence)
	persisted := attached.Status
	persisted.KAT.StatusRef.SHA256 = katStatusHash
	persisted.KAT.StatusHash = katStatusHash
	persistedHash, err := computeGJCStatusHash(persisted)
	if err != nil {
		t.Fatalf("compute persisted status hash: %v", err)
	}
	persisted.StatusHash = persistedHash
	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		t.Fatalf("marshal persisted status: %v", err)
	}
	statusPath := filepath.Join(repo, filepath.FromSlash(persisted.StatusPath))
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write persisted status: %v", err)
	}

	_, err = ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	assertProblemCode(t, err, "gjc_kat_source_status_hash_mismatch")
}

func TestShowGJCStatusRejectsPersistedKATSourceStatusHashRebind(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	start := startGJCUltragoalReadyForTest(t, root, repo, runID)
	statusEvidence := map[string]any{
		"schema_version":     "kat.status.v1",
		"run_id":             runID,
		"status":             "passed",
		"extractor_status":   "precise",
		"command_exit_code":  0,
		"self_approval":      false,
		"final_accepted":     false,
		"review_approved":    false,
		"waiver_approved":    false,
		"candidate_evidence": true,
		"source_status_hash": start.Status.StatusHash,
	}
	kat := writeGJCKATEvidenceForTest(t, repo, runID, statusEvidence)
	attached, err := AttachGJCKATEvidence(root, GJCKATAttachOptions{
		RunID:            runID,
		StatusPath:       kat.status.Path,
		StatusHash:       kat.status.SHA256,
		SummaryPath:      kat.summary.Path,
		SummaryHash:      kat.summary.SHA256,
		SummaryMDPath:    kat.summaryMD.Path,
		SummaryMDHash:    kat.summaryMD.SHA256,
		RawLogPath:       kat.rawLog.Path,
		RawLogHash:       kat.rawLog.SHA256,
		AttachmentStatus: "kat_evidence_ready",
		Now:              testRunNow(8),
	})
	if err != nil {
		t.Fatalf("AttachGJCKATEvidence() error = %v", err)
	}

	staleHash := "sha256:" + strings.Repeat("2", 64)
	statusEvidence["source_status_hash"] = staleHash
	katStatusHash := rewriteGJCKATStatusContentForTest(t, repo, attached.Status.KAT.StatusRef.Path, statusEvidence)
	persisted := attached.Status
	persisted.KAT.StatusRef.SHA256 = katStatusHash
	persisted.KAT.StatusHash = katStatusHash
	persisted.KAT.SourceStatusHash = staleHash
	persistedHash, err := computeGJCStatusHash(persisted)
	if err != nil {
		t.Fatalf("compute persisted status hash: %v", err)
	}
	persisted.StatusHash = persistedHash
	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		t.Fatalf("marshal persisted status: %v", err)
	}
	statusPath := filepath.Join(repo, filepath.FromSlash(persisted.StatusPath))
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write persisted status: %v", err)
	}

	_, err = ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	assertProblemCode(t, err, "gjc_kat_source_status_ref_mismatch")
}

func TestShowGJCStatusRejectsPersistedKATSourceStatusRefPathRebind(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runID := created.Metadata.RunID
	start := startGJCUltragoalReadyForTest(t, root, repo, runID)
	statusEvidence := map[string]any{
		"schema_version":     "kat.status.v1",
		"run_id":             runID,
		"status":             "passed",
		"extractor_status":   "precise",
		"command_exit_code":  0,
		"self_approval":      false,
		"final_accepted":     false,
		"review_approved":    false,
		"waiver_approved":    false,
		"candidate_evidence": true,
		"source_status_hash": start.Status.StatusHash,
	}
	kat := writeGJCKATEvidenceForTest(t, repo, runID, statusEvidence)
	attached, err := AttachGJCKATEvidence(root, GJCKATAttachOptions{
		RunID:            runID,
		StatusPath:       kat.status.Path,
		StatusHash:       kat.status.SHA256,
		SummaryPath:      kat.summary.Path,
		SummaryHash:      kat.summary.SHA256,
		SummaryMDPath:    kat.summaryMD.Path,
		SummaryMDHash:    kat.summaryMD.SHA256,
		RawLogPath:       kat.rawLog.Path,
		RawLogHash:       kat.rawLog.SHA256,
		AttachmentStatus: "kat_evidence_ready",
		Now:              testRunNow(8),
	})
	if err != nil {
		t.Fatalf("AttachGJCKATEvidence() error = %v", err)
	}

	fakeSource := start.Status
	fakeSource.CurrentWaitReason = stringPtr("fake but self-consistent pre-attachment source")
	fakeSourceHash, err := computeGJCStatusHash(fakeSource)
	if err != nil {
		t.Fatalf("compute fake source hash: %v", err)
	}
	fakeSource.StatusHash = fakeSourceHash
	fakeData, err := json.MarshalIndent(fakeSource, "", "  ")
	if err != nil {
		t.Fatalf("marshal fake source: %v", err)
	}
	fakeRel := packetRelative(runID, "artifacts/gjc/fake-kat-source-status.json")
	fakePath := writeGJCTestFile(t, repo, runID, "artifacts/gjc/fake-kat-source-status.json", append(fakeData, '\n'))
	fakeRef := GJCArtifactRef{Path: fakeRel, SHA256: sha256String(mustReadBytes(t, fakePath))}

	statusEvidence["source_status_hash"] = fakeSourceHash
	katStatusHash := rewriteGJCKATStatusContentForTest(t, repo, attached.Status.KAT.StatusRef.Path, statusEvidence)
	persisted := attached.Status
	persisted.KAT.StatusRef.SHA256 = katStatusHash
	persisted.KAT.StatusHash = katStatusHash
	persisted.KAT.SourceStatusHash = fakeSourceHash
	persisted.KAT.SourceStatusRef = fakeRef
	persistedHash, err := computeGJCStatusHash(persisted)
	if err != nil {
		t.Fatalf("compute persisted status hash: %v", err)
	}
	persisted.StatusHash = persistedHash
	data, err := json.MarshalIndent(persisted, "", "  ")
	if err != nil {
		t.Fatalf("marshal persisted status: %v", err)
	}
	statusPath := filepath.Join(repo, filepath.FromSlash(persisted.StatusPath))
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write persisted status: %v", err)
	}

	_, err = ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	assertProblemCode(t, err, "gjc_kat_source_status_ref_mismatch")
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

func startGJCUltragoalReadyForTest(t *testing.T, root Root, repo string, runID string) GJCStartResult {
	t.Helper()
	packetRel := packetRelative(runID, "artifacts/gjc/gjc-ultragoal-packet.yaml")
	writeGJCTestFile(t, repo, runID, "artifacts/gjc/gjc-ultragoal-packet.yaml", []byte("packet_kind: ultragoal\n"))
	brief := writeGJCTestFile(t, repo, runID, "artifacts/gjc/brief.md", []byte("# Brief\n"))
	goals := writeGJCTestFile(t, repo, runID, "artifacts/gjc/goals.json", []byte(`{"goals":[]}`+"\n"))
	ledger := writeGJCTestFile(t, repo, runID, "artifacts/gjc/ledger.jsonl", []byte(`{"event":"created"}`+"\n"))
	artifactRefs := []GJCArtifactRef{
		{Path: packetRelative(runID, "artifacts/gjc/brief.md"), SHA256: sha256String(mustReadBytes(t, brief))},
		{Path: packetRelative(runID, "artifacts/gjc/goals.json"), SHA256: sha256String(mustReadBytes(t, goals))},
		{Path: packetRelative(runID, "artifacts/gjc/ledger.jsonl"), SHA256: sha256String(mustReadBytes(t, ledger))},
	}
	oldRunner := gjcRunCommand
	gjcRunCommand = func(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
		receipt := map[string]any{
			"status":                 "ultragoal_ready",
			"artifact_refs":          artifactRefs,
			"current_required_actor": "kat",
		}
		data, _ := json.Marshal(receipt)
		return gjcRunnerResult{PID: 1234, ExitCode: 0, Stdout: data}, nil
	}
	defer func() { gjcRunCommand = oldRunner }()
	result, err := StartGJC(root, GJCStartOptions{RunID: runID, TaskID: "GAJAE-005", Packet: packetRel, CommandKind: "start-ultragoal", Now: testRunNow(7)})
	if err != nil {
		t.Fatalf("StartGJC(ultragoal) error = %v", err)
	}
	return result
}

type gjcKATFixture struct {
	status    GJCArtifactRef
	summary   GJCArtifactRef
	summaryMD GJCArtifactRef
	rawLog    GJCArtifactRef
}

func writeGJCKATEvidenceForTest(t *testing.T, repo string, runID string, status map[string]any) gjcKATFixture {
	t.Helper()
	statusData, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal KAT status: %v", err)
	}
	statusData = append(statusData, '\n')
	statusPath := writeGJCTestFile(t, repo, runID, "artifacts/test/kat.status.json", statusData)
	summaryPath := writeGJCTestFile(t, repo, runID, "artifacts/test/kat.summary.json", []byte(`{"summary":"fixture"}`+"\n"))
	summaryMDPath := writeGJCTestFile(t, repo, runID, "artifacts/test/kat.summary.md", []byte("# KAT summary\n"))
	rawLogPath := writeGJCTestFile(t, repo, runID, "artifacts/test/kat.raw.log", []byte("kat fixture log\n"))
	return gjcKATFixture{
		status:    GJCArtifactRef{Path: packetRelative(runID, "artifacts/test/kat.status.json"), SHA256: sha256String(mustReadBytes(t, statusPath))},
		summary:   GJCArtifactRef{Path: packetRelative(runID, "artifacts/test/kat.summary.json"), SHA256: sha256String(mustReadBytes(t, summaryPath))},
		summaryMD: GJCArtifactRef{Path: packetRelative(runID, "artifacts/test/kat.summary.md"), SHA256: sha256String(mustReadBytes(t, summaryMDPath))},
		rawLog:    GJCArtifactRef{Path: packetRelative(runID, "artifacts/test/kat.raw.log"), SHA256: sha256String(mustReadBytes(t, rawLogPath))},
	}
}

func rewriteGJCStatusForTest(t *testing.T, root Root, status GJCStatus) {
	t.Helper()
	path, err := gjcStatusPath(root, status.RunID)
	if err != nil {
		t.Fatalf("resolve GJC status path: %v", err)
	}
	statusHash, err := computeGJCStatusHash(status)
	if err != nil {
		t.Fatalf("compute GJC status hash: %v", err)
	}
	status.StatusHash = statusHash
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal GJC status: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path.Absolute, data, 0o600); err != nil {
		t.Fatalf("rewrite GJC status: %v", err)
	}
}

func rewriteGJCKATStatusForTest(t *testing.T, repo string, opts *GJCKATAttachOptions, status map[string]any) {
	t.Helper()
	opts.StatusHash = rewriteGJCKATStatusContentForTest(t, repo, opts.StatusPath, status)
}

func rewriteGJCKATStatusContentForTest(t *testing.T, repo string, relative string, status map[string]any) string {
	t.Helper()
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal KAT status: %v", err)
	}
	data = append(data, '\n')
	path := filepath.Join(repo, filepath.FromSlash(relative))
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("rewrite KAT status: %v", err)
	}
	return sha256String(data)
}

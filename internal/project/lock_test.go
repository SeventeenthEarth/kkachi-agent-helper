package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestLockAcquireConflictAndValidRelease(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)

	first, err := acquireLock(root, ProjectWriteLockName, "test writer", "", time.Now().UTC())
	if err != nil {
		t.Fatalf("acquire first lock: %v", err)
	}
	_, err = acquireLock(root, ProjectWriteLockName, "test contender", "", time.Now().UTC())
	assertProblemCode(t, err, "lock_conflict")
	if err := first.release(); err != nil {
		t.Fatalf("release first lock: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "project_write.lock")); !os.IsNotExist(err) {
		t.Fatalf("lock stat after release = %v, want absent", err)
	}
}

func TestLockAcquireReportsStaleRecoveryRequired(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	writeLockMetadata(t, repo, ProjectWriteLockName, LockMetadata{
		Version:   LockVersion,
		LockName:  ProjectWriteLockName,
		OwnerPID:  999999,
		Hostname:  mustHostname(t),
		Command:   "stale writer",
		CreatedAt: time.Now().UTC().Format(time.RFC3339),
	})

	_, err := acquireLock(root, ProjectWriteLockName, "contender", "", time.Now().UTC())
	assertProblemCode(t, err, "lock_stale_recovery_required")
}

func TestMutatorsFailClosedWhenFreshLocksExist(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	beforeStatus := readText(t, filepath.Join(repo, ".kkachi", "status.json"))
	beforeEvents := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))
	lock, err := acquireLock(root, ProjectWriteLockName, "external writer", "", time.Now().UTC())
	if err != nil {
		t.Fatalf("acquire external lock: %v", err)
	}
	defer lock.release()

	_, err = CreateRun(root, deterministicCreateRunOptions())
	assertProblemCode(t, err, "lock_conflict")
	if got := readText(t, filepath.Join(repo, ".kkachi", "status.json")); got != beforeStatus {
		t.Fatalf("status changed after lock conflict")
	}
	if got := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); got != beforeEvents {
		t.Fatalf("events changed after lock conflict")
	}
}

func TestRunLifecycleFreshActiveLockBlocksWithoutMutation(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	beforeStatus := readText(t, filepath.Join(repo, ".kkachi", "status.json"))
	beforeEvents := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))
	beforeMetadata := readText(t, filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "run-metadata.json"))
	lock, err := acquireLock(root, ActiveRunLockName, "external lifecycle", created.Metadata.RunID, time.Now().UTC())
	if err != nil {
		t.Fatalf("acquire active lock: %v", err)
	}
	defer lock.release()

	_, err = ActivateRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	assertProblemCode(t, err, "lock_conflict")
	if got := readText(t, filepath.Join(repo, ".kkachi", "status.json")); got != beforeStatus {
		t.Fatalf("status changed after active lock conflict")
	}
	if got := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); got != beforeEvents {
		t.Fatalf("events changed after active lock conflict")
	}
	if got := readText(t, filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "run-metadata.json")); got != beforeMetadata {
		t.Fatalf("metadata changed after active lock conflict")
	}
}

func TestRecoverLocksAppendsEventBeforeRemoval(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	recoveryNow := time.Date(2026, 4, 30, 3, 4, 5, 0, time.UTC)
	metadata := LockMetadata{Version: LockVersion, LockName: ProjectWriteLockName, OwnerPID: 999999, Hostname: "other-host", Command: "old writer", CreatedAt: recoveryNow.Add(-31 * time.Minute).Format(time.RFC3339)}
	writeLockMetadata(t, repo, ProjectWriteLockName, metadata)

	result, err := RecoverLocks(root, LockRecoveryOptions{Target: "project-write", Reason: "test stale recovery", Now: func() time.Time { return recoveryNow }})
	if err != nil {
		t.Fatalf("RecoverLocks() error = %v", err)
	}
	if len(result.Recovered) != 1 || result.Recovered[0].LockName != ProjectWriteLockName {
		t.Fatalf("recovered = %#v, want project_write", result.Recovered)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "project_write.lock")); !os.IsNotExist(err) {
		t.Fatalf("lock stat after recovery = %v, want absent", err)
	}
	lines := eventLines(t, repo)
	if !strings.Contains(lines[len(lines)-1], `"type":"lock.recovered"`) || !strings.Contains(lines[len(lines)-1], `"lock_name":"project_write"`) {
		t.Fatalf("recovery event = %s", lines[len(lines)-1])
	}
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["last_event_id"] != "evt-000002" {
		t.Fatalf("status last_event_id = %q, want recovery event id", status["last_event_id"])
	}
}

func TestRecoverLocksAllRecordsRunIDAndRemovesBothLocks(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	recoveryNow := time.Date(2026, 4, 30, 3, 4, 5, 0, time.UTC)
	writeLockMetadata(t, repo, ActiveRunLockName, LockMetadata{Version: LockVersion, LockName: ActiveRunLockName, RunID: "run-abc", OwnerPID: 999999, Hostname: "other-host", Command: "old active lifecycle", CreatedAt: recoveryNow.Add(-31 * time.Minute).Format(time.RFC3339)})
	writeLockMetadata(t, repo, ProjectWriteLockName, LockMetadata{Version: LockVersion, LockName: ProjectWriteLockName, OwnerPID: 999999, Hostname: "other-host", Command: "old project writer", CreatedAt: recoveryNow.Add(-31 * time.Minute).Format(time.RFC3339)})

	result, err := RecoverLocks(root, LockRecoveryOptions{Target: "all", Reason: "recover both stale locks", RunID: "run-abc", Now: func() time.Time { return recoveryNow }})
	if err != nil {
		t.Fatalf("RecoverLocks(all) error = %v", err)
	}
	if len(result.Recovered) != 2 || result.Recovered[0].LockName != ActiveRunLockName || result.Recovered[1].LockName != ProjectWriteLockName {
		t.Fatalf("recovered = %#v, want active_run then project_write", result.Recovered)
	}
	for _, relative := range []string{".kkachi/active_run.lock", ".kkachi/project_write.lock"} {
		if _, err := os.Stat(filepath.Join(repo, relative)); !os.IsNotExist(err) {
			t.Fatalf("%s stat = %v, want absent", relative, err)
		}
	}
	lines := eventLines(t, repo)
	if len(lines) != 3 {
		t.Fatalf("event lines = %d, want init plus two recovery events", len(lines))
	}
	for _, line := range lines[1:] {
		if !strings.Contains(line, `"type":"lock.recovered"`) || !strings.Contains(line, `"run_id":"run-abc"`) {
			t.Fatalf("recovery event = %s, want run-scoped lock.recovered", line)
		}
	}
}

func TestRecoverLocksActiveRunSerializesThroughProjectWriteLock(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	recoveryNow := time.Date(2026, 4, 30, 3, 4, 5, 0, time.UTC)
	writeLockMetadata(t, repo, ActiveRunLockName, LockMetadata{Version: LockVersion, LockName: ActiveRunLockName, RunID: "run-abc", OwnerPID: 999999, Hostname: "other-host", Command: "old active lifecycle", CreatedAt: recoveryNow.Add(-31 * time.Minute).Format(time.RFC3339)})

	result, err := RecoverLocks(root, LockRecoveryOptions{Target: "active-run", Reason: "recover active lock", RunID: "run-abc", Now: func() time.Time { return recoveryNow }})
	if err != nil {
		t.Fatalf("RecoverLocks(active-run) error = %v", err)
	}
	if len(result.Recovered) != 1 || result.Recovered[0].LockName != ActiveRunLockName {
		t.Fatalf("recovered = %#v, want active_run", result.Recovered)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "active_run.lock")); !os.IsNotExist(err) {
		t.Fatalf("active_run lock stat = %v, want absent", err)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "project_write.lock")); !os.IsNotExist(err) {
		t.Fatalf("project_write lock stat = %v, want temporary serialization lock released", err)
	}
	lines := eventLines(t, repo)
	if !strings.Contains(lines[len(lines)-1], `"type":"lock.recovered"`) || !strings.Contains(lines[len(lines)-1], `"lock_name":"active_run"`) {
		t.Fatalf("recovery event = %s, want active_run lock.recovered", lines[len(lines)-1])
	}
}

func TestRecoverLocksMissingTargetFailsClosed(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)

	_, err := RecoverLocks(root, LockRecoveryOptions{Target: "project-write", Reason: "nothing to recover"})
	assertProblemCode(t, err, "lock_not_found")
}

func TestRecoverActiveRunBlockedByFreshProjectWriteLock(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	recoveryNow := time.Date(2026, 4, 30, 3, 4, 5, 0, time.UTC)
	writeLockMetadata(t, repo, ActiveRunLockName, LockMetadata{Version: LockVersion, LockName: ActiveRunLockName, OwnerPID: 999999, Hostname: "other-host", Command: "old active lifecycle", CreatedAt: recoveryNow.Add(-31 * time.Minute).Format(time.RFC3339)})
	writeLockMetadata(t, repo, ProjectWriteLockName, LockMetadata{Version: LockVersion, LockName: ProjectWriteLockName, OwnerPID: os.Getpid(), Hostname: mustHostname(t), Command: "fresh writer", CreatedAt: time.Now().UTC().Format(time.RFC3339)})

	_, err := RecoverLocks(root, LockRecoveryOptions{Target: "active-run", Reason: "blocked by active writer", Now: func() time.Time { return recoveryNow }})
	assertProblemCode(t, err, "lock_conflict")
	if _, err := os.Stat(filepath.Join(repo, ".kkachi", "active_run.lock")); err != nil {
		t.Fatalf("active_run lock stat = %v, want preserved", err)
	}
	if len(eventLines(t, repo)) != 1 {
		t.Fatalf("recovery mutated events despite fresh project_write lock")
	}
}

func TestRecoverLocksRefusesMalformedAndFreshLocks(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "active_run.lock"), []byte("not-json\n"), 0o600); err != nil {
		t.Fatalf("write malformed lock: %v", err)
	}
	_, err := RecoverLocks(root, LockRecoveryOptions{Target: "active-run", Reason: "test"})
	assertProblemCode(t, err, "lock_metadata_invalid")

	if err := os.Remove(filepath.Join(repo, ".kkachi", "active_run.lock")); err != nil {
		t.Fatalf("remove malformed lock: %v", err)
	}
	writeLockMetadata(t, repo, ActiveRunLockName, LockMetadata{Version: LockVersion, LockName: ActiveRunLockName, OwnerPID: os.Getpid(), Hostname: mustHostname(t), Command: "fresh writer", CreatedAt: time.Now().UTC().Format(time.RFC3339)})
	_, err = RecoverLocks(root, LockRecoveryOptions{Target: "active-run", Reason: "test"})
	assertProblemCode(t, err, "lock_conflict")
}

func TestProcessSignalPermissionMeansAlive(t *testing.T) {
	if !processSignalMeansAlive(syscall.EPERM) {
		t.Fatalf("EPERM from signal 0 should mean the process exists")
	}
	if processSignalMeansAlive(syscall.ESRCH) {
		t.Fatalf("ESRCH from signal 0 should mean the process does not exist")
	}
}

func writeLockMetadata(t *testing.T, repo string, name string, metadata LockMetadata) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "project_write.lock")
	if name == ActiveRunLockName {
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

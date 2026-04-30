package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestInspectProjectStatusInitializedProject(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}

	status, err := InspectProjectStatus(root)
	if err != nil {
		t.Fatalf("InspectProjectStatus() error = %v", err)
	}
	if status.Health != HealthOK || status.ProjectID == "" || status.ProjectName == "" || status.LastEventID != "evt-000001" || status.EventTailID != "evt-000001" || status.EventCount != 1 || len(status.Issues) != 0 {
		t.Fatalf("status = %#v, want healthy initialized project", status)
	}
}

func TestDoctorInitializedProjectPasses(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	if report.Health != HealthOK || report.Summary.Failed != 0 || report.Summary.Warnings != 0 || report.Summary.Passed == 0 {
		t.Fatalf("report = %#v, want all pass", report)
	}
}

func TestDoctorReportsMissingManagedFiles(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	if err := os.Remove(filepath.Join(repo, ".kkachi", "config.yaml")); err != nil {
		t.Fatalf("remove config: %v", err)
	}

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	assertDiagnostic(t, report.Checks, "config", CheckFail, ".kkachi/config.yaml")
	if report.Health != HealthFail {
		t.Fatalf("health = %q, want fail", report.Health)
	}
}

func TestDoctorReportsInvalidStatusAndEventLog(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "status.json"), []byte("{\n"), 0o600); err != nil {
		t.Fatalf("write invalid status: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "events.jsonl"), []byte("\n"), 0o600); err != nil {
		t.Fatalf("write invalid events: %v", err)
	}

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	assertDiagnostic(t, report.Checks, "status", CheckFail, ".kkachi/status.json")
	assertDiagnostic(t, report.Checks, "events", CheckFail, ".kkachi/events.jsonl")
}

func TestDoctorAndStatusRequireRoot(t *testing.T) {
	if _, err := Doctor(Root{}); err == nil {
		t.Fatal("Doctor() error = nil, want repo_root_required")
	} else {
		assertProblemCode(t, err, "repo_root_required")
	}

	if _, err := InspectProjectStatus(Root{}); err == nil {
		t.Fatal("InspectProjectStatus() error = nil, want repo_root_required")
	} else {
		assertProblemCode(t, err, "repo_root_required")
	}
}

func TestDoctorReportsNonSequentialEventsAndCoherenceMismatch(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	writeEventLog(t, repo, `{"version":"0.1","event_id":"evt-000001","occurred_at":"2026-04-30T01:00:00Z","run_id":null,"type":"project.initialized","actor":"helper","payload":{}}`+"\n"+`{"version":"0.1","event_id":"evt-000003","occurred_at":"2026-04-30T03:00:00Z","run_id":null,"type":"run.created","actor":"helper","payload":{}}`+"\n")

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	check := assertDiagnostic(t, report.Checks, "events", CheckFail, ".kkachi/events.jsonl")
	if check.Expected != "evt-000002" || check.Actual != "evt-000003" {
		t.Fatalf("events check = %#v, want sequential id evidence", check)
	}
}

func TestDoctorReportsOversizedEventLine(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	writeEventLog(t, repo, strings.Repeat("x", MaxEventLineBytes+1)+"\n")

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	check := assertDiagnostic(t, report.Checks, "events", CheckFail, ".kkachi/events.jsonl")
	if check.Field != "events" || !strings.Contains(check.Actual, "exceeds") {
		t.Fatalf("events check = %#v, want oversized line evidence", check)
	}
}

func TestDoctorReportsTailStatusMismatch(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	writeEventLog(t, repo, `{"version":"0.1","event_id":"evt-000001","occurred_at":"2026-04-30T01:00:00Z","run_id":null,"type":"project.initialized","actor":"helper","payload":{}}`+"\n"+`{"version":"0.1","event_id":"evt-000002","occurred_at":"2026-04-30T02:00:00Z","run_id":null,"type":"run.created","actor":"helper","payload":{}}`+"\n")

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	check := assertDiagnostic(t, report.Checks, "coherence", CheckFail, ".kkachi/status.json")
	if check.Expected != "evt-000002" || check.Actual != "evt-000001" {
		t.Fatalf("coherence check = %#v, want tail/status evidence", check)
	}
}

func TestInspectProjectStatusReportsStatusFieldFailures(t *testing.T) {
	tests := []struct {
		name      string
		mutate    func(map[string]any)
		wantField string
	}{
		{
			name: "missing last_event_id",
			mutate: func(status map[string]any) {
				delete(status, "last_event_id")
			},
			wantField: "last_event_id",
		},
		{
			name: "invalid last_event_id",
			mutate: func(status map[string]any) {
				status["last_event_id"] = "bogus"
			},
			wantField: "last_event_id",
		},
		{
			name: "invalid gate_summary",
			mutate: func(status map[string]any) {
				status["gate_summary"] = "closed"
			},
			wantField: "gate_summary",
		},
		{
			name: "invalid updated_at",
			mutate: func(status map[string]any) {
				status["updated_at"] = "not-a-time"
			},
			wantField: "updated_at",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, err := DiscoverRoot(repo)
			if err != nil {
				t.Fatalf("DiscoverRoot() error = %v", err)
			}
			mutateStatusFile(t, repo, tt.mutate)

			status, err := InspectProjectStatus(root)
			if err != nil {
				t.Fatalf("InspectProjectStatus() error = %v", err)
			}
			if status.Health != HealthFail {
				t.Fatalf("health = %q, want fail", status.Health)
			}
			found := false
			for _, issue := range status.Issues {
				if issue.Name == "status" && issue.Status == CheckFail && issue.Field == tt.wantField {
					found = true
				}
			}
			if !found {
				t.Fatalf("issues = %#v, want status failure for %s", status.Issues, tt.wantField)
			}
		})
	}
}

func TestDoctorReportsSchemaMissingVersionRequirement(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	schemaPath := filepath.Join(repo, ".kkachi", "schemas", "status.schema.json")
	if err := os.WriteFile(schemaPath, []byte(`{"type":"object","required":[]}`+"\n"), 0o600); err != nil {
		t.Fatalf("write schema: %v", err)
	}

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	check := assertDiagnostic(t, report.Checks, "schemas", CheckFail, ".kkachi/schemas/status.schema.json")
	if !strings.Contains(check.Message, "version") {
		t.Fatalf("schema check = %#v, want version requirement message", check)
	}
}

func TestDoctorReportsSchemaMissingAndInvalidJSON(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(string)
	}{
		{
			name: "missing",
			mutate: func(schemaPath string) {
				if err := os.Remove(schemaPath); err != nil {
					panic(err)
				}
			},
		},
		{
			name: "invalid json",
			mutate: func(schemaPath string) {
				if err := os.WriteFile(schemaPath, []byte("{\n"), 0o600); err != nil {
					panic(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, err := DiscoverRoot(repo)
			if err != nil {
				t.Fatalf("DiscoverRoot() error = %v", err)
			}
			schemaPath := filepath.Join(repo, ".kkachi", "schemas", "status.schema.json")
			func() {
				defer func() {
					if recovered := recover(); recovered != nil {
						t.Fatalf("mutate schema: %v", recovered)
					}
				}()
				tt.mutate(schemaPath)
			}()

			report, err := Doctor(root)
			if err != nil {
				t.Fatalf("Doctor() error = %v", err)
			}
			assertDiagnostic(t, report.Checks, "schemas", CheckFail, ".kkachi/schemas/status.schema.json")
			if report.Health != HealthFail {
				t.Fatalf("health = %q, want fail", report.Health)
			}
		})
	}
}

func TestDoctorReportsPresentLockAsWarning(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	metadata := LockMetadata{Version: LockVersion, LockName: ActiveRunLockName, OwnerPID: os.Getpid(), Hostname: mustHostname(t), Command: "test", CreatedAt: time.Now().UTC().Format(time.RFC3339)}
	data, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "active_run.lock"), append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write lock: %v", err)
	}

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	assertDiagnostic(t, report.Checks, "locks", CheckWarn, ".kkachi/active_run.lock")
	if report.Health != HealthWarning {
		t.Fatalf("health = %q, want warning", report.Health)
	}
}

func TestDoctorReportsUnreadableLockAsFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read 000 lock files")
	}
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	lockPath := filepath.Join(repo, ".kkachi", "project_write.lock")
	if err := os.WriteFile(lockPath, []byte("locked\n"), 0o000); err != nil {
		t.Fatalf("write lock: %v", err)
	}
	if err := os.Chmod(lockPath, 0o000); err != nil {
		t.Fatalf("chmod lock: %v", err)
	}
	defer os.Chmod(lockPath, 0o600)

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	assertDiagnostic(t, report.Checks, "locks", CheckFail, ".kkachi/project_write.lock")
	if report.Health != HealthFail {
		t.Fatalf("health = %q, want fail", report.Health)
	}
}

func TestDoctorReportsLockDirectoryAsFailure(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	lockPath := filepath.Join(repo, ".kkachi", "project_write.lock")
	if err := os.Mkdir(lockPath, 0o700); err != nil {
		t.Fatalf("mkdir lock path: %v", err)
	}

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	check := assertDiagnostic(t, report.Checks, "locks", CheckFail, ".kkachi/project_write.lock")
	if !strings.Contains(check.Actual, "d") {
		t.Fatalf("lock check = %#v, want directory mode evidence", check)
	}
}

func TestDoctorReportsUnsafeLockPath(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	outside := t.TempDir()
	if err := os.RemoveAll(filepath.Join(repo, ".kkachi")); err != nil {
		t.Fatalf("remove kkachi: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(repo, ".kkachi")); err != nil {
		t.Fatalf("symlink kkachi: %v", err)
	}

	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() error = %v", err)
	}
	assertDiagnostic(t, report.Checks, "locks", CheckFail, ".kkachi/active_run.lock")
	if report.Health != HealthFail {
		t.Fatalf("health = %q, want fail", report.Health)
	}
}

func mutateStatusFile(t *testing.T, repo string, mutate func(map[string]any)) {
	t.Helper()
	statusPath := filepath.Join(repo, ".kkachi", "status.json")
	var status map[string]any
	readJSONFile(t, statusPath, &status)
	mutate(status)
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}
}

func assertDiagnostic(t *testing.T, checks []DiagnosticCheck, name string, status string, path string) DiagnosticCheck {
	t.Helper()
	for _, check := range checks {
		if check.Name == name && check.Status == status && check.Path == path {
			return check
		}
	}
	t.Fatalf("checks = %#v, want %s %s at %s", checks, name, status, path)
	return DiagnosticCheck{}
}

func mustHostname(t *testing.T) string {
	t.Helper()
	hostname, err := os.Hostname()
	if err != nil {
		t.Fatalf("hostname: %v", err)
	}
	return hostname
}

func TestDoctorReportsMalformedAndStaleLockDiagnostics(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "active_run.lock"), []byte("not-json\n"), 0o600); err != nil {
		t.Fatalf("write malformed lock: %v", err)
	}
	report, err := Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() malformed error = %v", err)
	}
	check := assertDiagnostic(t, report.Checks, "locks", CheckFail, ".kkachi/active_run.lock")
	if check.Field != "json" {
		t.Fatalf("malformed lock check = %#v, want json field", check)
	}

	if err := os.Remove(filepath.Join(repo, ".kkachi", "active_run.lock")); err != nil {
		t.Fatalf("remove malformed lock: %v", err)
	}
	metadata := LockMetadata{Version: LockVersion, LockName: ActiveRunLockName, OwnerPID: 999999, Hostname: "other-host", Command: "stale writer", CreatedAt: time.Now().UTC().Add(-31 * time.Minute).Format(time.RFC3339)}
	data, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "active_run.lock"), append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write stale lock: %v", err)
	}
	report, err = Doctor(root)
	if err != nil {
		t.Fatalf("Doctor() stale error = %v", err)
	}
	check = assertDiagnostic(t, report.Checks, "locks", CheckWarn, ".kkachi/active_run.lock")
	if !strings.Contains(check.Message, "stale") || !strings.Contains(check.Hint, "lock recover") {
		t.Fatalf("stale lock check = %#v, want recovery hint", check)
	}
}

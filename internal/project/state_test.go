package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestAppendEventAdvancesLogAndStatusCoherently(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}

	result, err := AppendEvent(root, deterministicAppendOptions("artifact.written"))
	if err != nil {
		t.Fatalf("AppendEvent() error = %v", err)
	}
	if result.EventID != "evt-000002" || result.PreviousID != "evt-000001" {
		t.Fatalf("result ids = %q/%q, want evt-000002/evt-000001", result.EventID, result.PreviousID)
	}

	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["last_event_id"] != "evt-000002" {
		t.Fatalf("status last_event_id = %q, want evt-000002", status["last_event_id"])
	}
	if status["updated_at"] != "2026-04-30T02:03:04Z" {
		t.Fatalf("status updated_at = %q, want append timestamp", status["updated_at"])
	}

	lines := eventLines(t, repo)
	if len(lines) != 2 {
		t.Fatalf("event line count = %d, want 2", len(lines))
	}
	var event Event
	if err := json.Unmarshal([]byte(lines[1]), &event); err != nil {
		t.Fatalf("second event is not JSON: %v\n%s", err, lines[1])
	}
	if event.EventID != "evt-000002" || event.Type != "artifact.written" || event.Actor != "helper" || event.RunID == nil || *event.RunID != "run-abc" {
		t.Fatalf("event = %#v, want appended helper event for run", event)
	}
	if event.Payload["path"] != "impl-log.md" {
		t.Fatalf("payload = %#v, want path", event.Payload)
	}
}

func TestAppendEventRefusesCrashGapBetweenEventLogAndStatus(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	eventsPath := filepath.Join(repo, ".kkachi", "events.jsonl")
	crashLine := `{"version":"0.1","event_id":"evt-000002","occurred_at":"2026-04-30T02:00:00Z","run_id":"run-abc","type":"run.created","actor":"helper","payload":{}}` + "\n"
	file, err := os.OpenFile(eventsPath, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open event log: %v", err)
	}
	if _, err := file.WriteString(crashLine); err != nil {
		t.Fatalf("append crash line: %v", err)
	}
	if err := file.Close(); err != nil {
		t.Fatalf("close event log: %v", err)
	}

	_, err = AppendEvent(root, deterministicAppendOptions("artifact.written"))
	assertProblemCode(t, err, "last_event_id_mismatch")

	lines := eventLines(t, repo)
	if len(lines) != 2 {
		t.Fatalf("event line count after refused append = %d, want original crash gap only", len(lines))
	}
	var status map[string]any
	readJSONFile(t, filepath.Join(repo, ".kkachi", "status.json"), &status)
	if status["last_event_id"] != "evt-000001" {
		t.Fatalf("status last_event_id = %q, want unchanged evt-000001", status["last_event_id"])
	}
}

func TestAppendEventRejectsInvalidEventLogWithoutMutation(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	eventsPath := filepath.Join(repo, ".kkachi", "events.jsonl")
	before := readText(t, eventsPath)
	if err := os.WriteFile(eventsPath, []byte(before+"not-json\n"), 0o600); err != nil {
		t.Fatalf("corrupt event log: %v", err)
	}

	_, err = AppendEvent(root, deterministicAppendOptions("artifact.written"))
	assertProblemCode(t, err, "event_log_invalid")

	if got := readText(t, eventsPath); got != before+"not-json\n" {
		t.Fatalf("event log changed after invalid append refusal\n got: %q\nwant: %q", got, before+"not-json\n")
	}
}

func TestAppendEventRejectsControlCharacterRunID(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	options := deterministicAppendOptions("artifact.written")
	options.RunID = "run-abc\nsecond-line"

	_, err = AppendEvent(root, options)
	assertProblemCode(t, err, "run_id_invalid")
}

func TestAppendEventRejectsOversizedEventLineWithoutMutation(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	before := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))
	options := deterministicAppendOptions("artifact.written")
	options.Payload = map[string]any{"blob": strings.Repeat("x", MaxEventLineBytes)}

	_, err = AppendEvent(root, options)
	assertProblemCode(t, err, "event_line_too_large")
	if got := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); got != before {
		t.Fatalf("event log changed after oversized event refusal")
	}
}

func TestAppendEventRejectsOversizedExistingEventLine(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	writeEventLog(t, repo, strings.Repeat("x", MaxEventLineBytes+1)+"\n")

	_, err = AppendEvent(root, deterministicAppendOptions("artifact.written"))
	assertProblemCode(t, err, "event_line_too_large")
}

func TestAppendEventRejectsMalformedTailEventID(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	setStatusLastEventID(t, repo, "bogus")
	writeEventLog(t, repo, `{"version":"0.1","event_id":"bogus","occurred_at":"2026-04-30T02:00:00Z","run_id":null,"type":"run.created","actor":"helper","payload":{}}`+"\n")

	_, err = AppendEvent(root, deterministicAppendOptions("artifact.written"))
	assertProblemCode(t, err, "event_id_invalid")
}

func TestAppendEventRejectsExhaustedEventID(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	setStatusLastEventID(t, repo, "evt-999999")
	writeEventLog(t, repo, `{"version":"0.1","event_id":"evt-999999","occurred_at":"2026-04-30T02:00:00Z","run_id":null,"type":"run.created","actor":"helper","payload":{}}`+"\n")

	_, err = AppendEvent(root, deterministicAppendOptions("artifact.written"))
	assertProblemCode(t, err, "event_id_exhausted")
}

func TestAppendEventRejectsEmptyEventLog(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	writeEventLog(t, repo, "")

	_, err = AppendEvent(root, deterministicAppendOptions("artifact.written"))
	assertProblemCode(t, err, "event_log_empty")
}

func TestAtomicNewFileWriteRefusesExistingFileAndIgnoresStaleTemp(t *testing.T) {
	repo := t.TempDir()
	path := SafePath{Relative: ".kkachi/status.json", Absolute: filepath.Join(repo, ".kkachi", "status.json")}
	mustMkdir(t, filepath.Dir(path.Absolute))
	staleTemp := filepath.Join(filepath.Dir(path.Absolute), ".status.json.tmp-stale")
	if err := os.WriteFile(staleTemp, []byte("partial\n"), 0o600); err != nil {
		t.Fatalf("write stale temp: %v", err)
	}

	if err := writeNewFileAtomically(path, []byte("created\n")); err != nil {
		t.Fatalf("writeNewFileAtomically() error = %v", err)
	}
	if got := readText(t, path.Absolute); got != "created\n" {
		t.Fatalf("created file = %q, want created", got)
	}
	if got := readText(t, staleTemp); got != "partial\n" {
		t.Fatalf("stale temp = %q, want untouched", got)
	}

	err := writeNewFileAtomically(path, []byte("overwrite\n"))
	assertProblemCode(t, err, "helper_state_exists")
	if got := readText(t, path.Absolute); got != "created\n" {
		t.Fatalf("existing file changed after refused new write = %q", got)
	}
}

func TestAtomicNewFileWriteRemovesSuccessfulTempFile(t *testing.T) {
	repo := t.TempDir()
	path := SafePath{Relative: ".kkachi/status.json", Absolute: filepath.Join(repo, ".kkachi", "status.json")}
	mustMkdir(t, filepath.Dir(path.Absolute))

	if err := writeNewFileAtomically(path, []byte("created\n")); err != nil {
		t.Fatalf("writeNewFileAtomically() error = %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path.Absolute), ".status.json.tmp-*"))
	if err != nil {
		t.Fatalf("glob temp files: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary files remained after successful atomic write: %v", matches)
	}
}

func TestAtomicStateReplacementPreservesFileWhenStaleTempExists(t *testing.T) {
	repo := t.TempDir()
	path := SafePath{Relative: ".kkachi/status.json", Absolute: filepath.Join(repo, ".kkachi", "status.json")}
	mustMkdir(t, filepath.Dir(path.Absolute))
	if err := os.WriteFile(path.Absolute, []byte("old\n"), 0o600); err != nil {
		t.Fatalf("write old file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(path.Absolute), ".status.json.tmp-stale"), []byte("partial\n"), 0o600); err != nil {
		t.Fatalf("write stale temp: %v", err)
	}

	if err := writeExistingFileAtomically(path, []byte("new\n")); err != nil {
		t.Fatalf("writeExistingFileAtomically() error = %v", err)
	}
	if got := readText(t, path.Absolute); got != "new\n" {
		t.Fatalf("state content = %q, want new", got)
	}
	if got := readText(t, filepath.Join(filepath.Dir(path.Absolute), ".status.json.tmp-stale")); got != "partial\n" {
		t.Fatalf("stale temp = %q, want untouched", got)
	}
}

func initializedRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	if _, err := InitProject(root, deterministicInitOptions()); err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}
	return repo
}

func setStatusLastEventID(t *testing.T, repo string, eventID string) {
	t.Helper()
	statusPath := filepath.Join(repo, ".kkachi", "status.json")
	var status map[string]any
	readJSONFile(t, statusPath, &status)
	status["last_event_id"] = eventID
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		t.Fatalf("marshal status: %v", err)
	}
	if err := os.WriteFile(statusPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write status: %v", err)
	}
}

func writeEventLog(t *testing.T, repo string, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(repo, ".kkachi", "events.jsonl"), []byte(content), 0o600); err != nil {
		t.Fatalf("write event log: %v", err)
	}
}

func deterministicAppendOptions(eventType string) AppendEventOptions {
	return AppendEventOptions{
		Type:  eventType,
		RunID: "run-abc",
		Payload: map[string]any{
			"path": "impl-log.md",
		},
		Now: func() time.Time {
			return time.Date(2026, 4, 30, 2, 3, 4, 0, time.UTC)
		},
	}
}

func eventLines(t *testing.T, repo string) []string {
	t.Helper()
	return strings.Split(strings.TrimSpace(readText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))), "\n")
}

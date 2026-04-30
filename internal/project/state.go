package project

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	EventVersion         = "0.1"
	EventActorHelper     = "helper"
	MaxEventLineBytes    = 1024 * 1024
	MaxEventPayloadBytes = 256 * 1024
	MaxEventRunIDBytes   = 256
)

var eventIDPattern = regexp.MustCompile(`^evt-(\d{6})$`)

// Event is one append-only entry in .kkachi/events.jsonl.
type Event struct {
	Version    string         `json:"version"`
	EventID    string         `json:"event_id"`
	OccurredAt string         `json:"occurred_at"`
	RunID      *string        `json:"run_id"`
	Type       string         `json:"type"`
	Actor      string         `json:"actor"`
	Payload    map[string]any `json:"payload"`
}

// AppendEventOptions controls append-only event recording. Tests may inject a
// deterministic clock while production uses the helper clock.
type AppendEventOptions struct {
	Type    string
	RunID   string
	Actor   string
	Payload map[string]any
	Now     func() time.Time
}

// AppendEventResult summarizes a successfully appended event.
type AppendEventResult struct {
	EventID    string
	PreviousID string
	StatusPath string
	EventsPath string
	OccurredAt string
}

// AppendEvent appends one event line and atomically advances status.last_event_id.
// It refuses to mutate state when status.json and events.jsonl disagree.
// Callers must provide a single-writer lane; cross-process locking is deferred
// to runwf-002.
func AppendEvent(root Root, options AppendEventOptions) (AppendEventResult, error) {
	return appendEventWithStatusMutation(root, options, nil)
}

func appendEventWithStatusMutation(root Root, options AppendEventOptions, mutateStatus func(map[string]any, string) error) (AppendEventResult, error) {
	if strings.TrimSpace(root.Path) == "" {
		return AppendEventResult{}, problem("repo_root_required", "repository root is required", "Discover the repository root before appending an event.")
	}
	if strings.TrimSpace(options.Type) == "" {
		return AppendEventResult{}, &Problem{
			Code:     "event_type_required",
			Message:  "event type is required",
			Hint:     "Pass a non-empty event type such as artifact.written.",
			Field:    "type",
			Expected: "non-empty event type",
			Actual:   "empty",
		}
	}
	if options.Payload == nil {
		options.Payload = map[string]any{}
	}
	if options.Actor == "" {
		options.Actor = EventActorHelper
	}
	runID, err := normalizeEventRunID(options.RunID)
	if err != nil {
		return AppendEventResult{}, err
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}

	statusPath, err := ResolveRelativePath(root, StatusPath)
	if err != nil {
		return AppendEventResult{}, err
	}
	eventsPath, err := ResolveRelativePath(root, EventsPath)
	if err != nil {
		return AppendEventResult{}, err
	}

	status, err := readStatus(statusPath)
	if err != nil {
		return AppendEventResult{}, err
	}
	statusLast, err := statusLastEventID(status, statusPath.Relative)
	if err != nil {
		return AppendEventResult{}, err
	}
	logLast, err := scanLastEventID(eventsPath)
	if err != nil {
		return AppendEventResult{}, err
	}
	if statusLast != logLast {
		return AppendEventResult{}, &Problem{
			Code:     "last_event_id_mismatch",
			Message:  "status last_event_id does not match the event log tail",
			Hint:     "Do not append new events until helper state is inspected by project doctor or restored from a coherent backup.",
			Path:     statusPath.Relative,
			Field:    "last_event_id",
			Expected: logLast,
			Actual:   statusLast,
		}
	}

	nextID, err := nextEventID(logLast, eventsPath.Relative)
	if err != nil {
		return AppendEventResult{}, err
	}
	occurredAt := options.Now().UTC().Format(time.RFC3339)
	event := Event{
		Version:    EventVersion,
		EventID:    nextID,
		OccurredAt: occurredAt,
		RunID:      runID,
		Type:       strings.TrimSpace(options.Type),
		Actor:      options.Actor,
		Payload:    options.Payload,
	}

	line, err := json.Marshal(event)
	if err != nil {
		return AppendEventResult{}, &Problem{
			Code:     "event_payload_invalid",
			Message:  "event payload cannot be encoded as JSON",
			Hint:     "Use a JSON object payload with values supported by JSON.",
			Field:    "payload",
			Expected: "JSON-encodable object",
			Actual:   err.Error(),
		}
	}
	line = append(line, '\n')
	if len(line) > MaxEventLineBytes {
		return AppendEventResult{}, eventLineTooLargeProblem(eventsPath.Relative, len(line))
	}
	if err := appendEventLine(eventsPath, line); err != nil {
		return AppendEventResult{}, err
	}

	if mutateStatus != nil {
		if err := mutateStatus(status, occurredAt); err != nil {
			return AppendEventResult{}, err
		}
	}
	status["last_event_id"] = nextID
	status["updated_at"] = occurredAt
	updatedStatus, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return AppendEventResult{}, &Problem{
			Code:     "status_encode_failed",
			Message:  "cannot encode status after event append",
			Hint:     "Inspect status.json for unsupported values and restore from backup if needed.",
			Path:     statusPath.Relative,
			Field:    "status",
			Expected: "JSON object",
			Actual:   err.Error(),
		}
	}
	updatedStatus = append(updatedStatus, '\n')
	if err := writeExistingFileAtomically(statusPath, updatedStatus); err != nil {
		return AppendEventResult{}, err
	}

	return AppendEventResult{
		EventID:    nextID,
		PreviousID: statusLast,
		StatusPath: statusPath.Relative,
		EventsPath: eventsPath.Relative,
		OccurredAt: occurredAt,
	}, nil
}

func normalizeEventRunID(value string) (*string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	if len(value) > MaxEventRunIDBytes {
		return nil, &Problem{
			Code:     "run_id_too_large",
			Message:  "event run id is too large",
			Hint:     "Use a concise run id; the full run id policy is defined by later run workflow tasks.",
			Field:    "run_id",
			Expected: fmt.Sprintf("at most %d bytes", MaxEventRunIDBytes),
			Actual:   fmt.Sprintf("%d bytes", len(value)),
		}
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return nil, &Problem{
				Code:     "run_id_invalid",
				Message:  "event run id contains control characters",
				Hint:     "Use a printable run id without newlines or terminal control characters.",
				Field:    "run_id",
				Expected: "printable run id",
				Actual:   "contains control character",
			}
		}
	}
	return &value, nil
}

func eventLineTooLargeProblem(relative string, bytes int) *Problem {
	return &Problem{
		Code:     "event_line_too_large",
		Message:  "event line exceeds the maximum supported size",
		Hint:     "Store large evidence in artifacts and keep event payloads compact.",
		Path:     relative,
		Field:    "event",
		Expected: fmt.Sprintf("at most %d bytes", MaxEventLineBytes),
		Actual:   fmt.Sprintf("%d bytes", bytes),
	}
}

func readStatus(path SafePath) (map[string]any, error) {
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return nil, &Problem{
			Code:     "status_read_failed",
			Message:  "cannot read project status",
			Hint:     "Run project init first or restore .kkachi/status.json from backup.",
			Path:     path.Relative,
			Field:    "path",
			Expected: "readable JSON status file",
			Actual:   err.Error(),
		}
	}
	var status map[string]any
	if err := json.Unmarshal(data, &status); err != nil {
		return nil, &Problem{
			Code:     "status_invalid_json",
			Message:  "project status is not valid JSON",
			Hint:     "Restore .kkachi/status.json from a coherent backup before appending events.",
			Path:     path.Relative,
			Field:    "json",
			Expected: "JSON object status file",
			Actual:   err.Error(),
		}
	}
	if status == nil {
		return nil, &Problem{
			Code:     "status_invalid_json",
			Message:  "project status must be a JSON object",
			Hint:     "Restore .kkachi/status.json from a coherent backup before appending events.",
			Path:     path.Relative,
			Field:    "json",
			Expected: "JSON object status file",
			Actual:   "null",
		}
	}
	return status, nil
}

func statusLastEventID(status map[string]any, relative string) (string, error) {
	value, ok := status["last_event_id"].(string)
	if !ok || strings.TrimSpace(value) == "" {
		return "", &Problem{
			Code:     "status_last_event_id_invalid",
			Message:  "project status is missing a valid last_event_id",
			Hint:     "Restore status.json from a coherent backup before appending events.",
			Path:     relative,
			Field:    "last_event_id",
			Expected: "non-empty event id string",
			Actual:   fmt.Sprintf("%v", status["last_event_id"]),
		}
	}
	return value, nil
}

func scanLastEventID(path SafePath) (string, error) {
	file, err := os.Open(path.Absolute)
	if err != nil {
		return "", &Problem{
			Code:     "event_log_read_failed",
			Message:  "cannot read event log",
			Hint:     "Run project init first or restore .kkachi/events.jsonl from backup.",
			Path:     path.Relative,
			Field:    "path",
			Expected: "readable JSONL event log",
			Actual:   err.Error(),
		}
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), MaxEventLineBytes)
	lineNumber := 0
	lastID := ""
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return "", eventLogProblem(path.Relative, lineNumber, "non-empty JSON object line", "empty line")
		}
		var entry struct {
			EventID string `json:"event_id"`
		}
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return "", eventLogProblem(path.Relative, lineNumber, "valid JSON object line", err.Error())
		}
		if entry.EventID == "" {
			return "", eventLogProblem(path.Relative, lineNumber, "event_id string", "missing")
		}
		lastID = entry.EventID
	}
	if err := scanner.Err(); err != nil {
		if strings.Contains(err.Error(), "token too long") {
			return "", eventLineTooLargeProblem(path.Relative, MaxEventLineBytes+1)
		}
		return "", &Problem{
			Code:     "event_log_read_failed",
			Message:  "cannot scan event log",
			Hint:     "Check event log permissions and restore from backup if the file is damaged.",
			Path:     path.Relative,
			Field:    "events",
			Expected: "readable JSONL event log",
			Actual:   err.Error(),
		}
	}
	if lineNumber == 0 {
		return "", &Problem{
			Code:     "event_log_empty",
			Message:  "event log is empty",
			Hint:     "Restore .kkachi/events.jsonl from a coherent backup before appending events.",
			Path:     path.Relative,
			Field:    "events",
			Expected: "at least one event line",
			Actual:   "empty",
		}
	}
	return lastID, nil
}

func eventLogProblem(relative string, line int, expected string, actual string) *Problem {
	return &Problem{
		Code:     "event_log_invalid",
		Message:  "event log contains an invalid line",
		Hint:     "Restore .kkachi/events.jsonl from a coherent backup before appending events.",
		Path:     relative,
		Field:    fmt.Sprintf("line_%d", line),
		Expected: expected,
		Actual:   actual,
	}
}

func nextEventID(previous string, relative string) (string, error) {
	matches := eventIDPattern.FindStringSubmatch(previous)
	if matches == nil {
		return "", &Problem{
			Code:     "event_id_invalid",
			Message:  "cannot allocate next event id from malformed tail event id",
			Hint:     "Restore coherent helper state or wait for a future migration/recovery command.",
			Path:     relative,
			Field:    "event_id",
			Expected: "evt- followed by six digits",
			Actual:   previous,
		}
	}
	number, err := strconv.Atoi(matches[1])
	if err != nil || number >= 999999 {
		return "", &Problem{
			Code:     "event_id_exhausted",
			Message:  "cannot allocate next event id",
			Hint:     "Start a migrated event log with a wider id policy when that migration exists.",
			Path:     relative,
			Field:    "event_id",
			Expected: "event id below evt-999999",
			Actual:   previous,
		}
	}
	return fmt.Sprintf("evt-%06d", number+1), nil
}

func appendEventLine(path SafePath, line []byte) error {
	file, err := os.OpenFile(path.Absolute, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return &Problem{
			Code:     "event_log_append_failed",
			Message:  "cannot append to event log",
			Hint:     "Check .kkachi/events.jsonl permissions and retry after preserving the current file.",
			Path:     path.Relative,
			Field:    "path",
			Expected: "appendable event log",
			Actual:   err.Error(),
		}
	}
	closed := false
	defer func() {
		if !closed {
			_ = file.Close()
		}
	}()
	if _, err := file.Write(line); err != nil {
		return &Problem{
			Code:     "event_log_append_failed",
			Message:  "cannot write event log line",
			Hint:     "Check repository storage health and preserve .kkachi/events.jsonl for diagnosis.",
			Path:     path.Relative,
			Field:    "events",
			Expected: "durable event line write",
			Actual:   err.Error(),
		}
	}
	if err := file.Sync(); err != nil {
		return &Problem{
			Code:     "event_log_sync_failed",
			Message:  "cannot sync event log after append",
			Hint:     "Check repository storage health before retrying.",
			Path:     path.Relative,
			Field:    "events",
			Expected: "synced event log",
			Actual:   err.Error(),
		}
	}
	if err := file.Close(); err != nil {
		closed = true
		return &Problem{
			Code:     "event_log_close_failed",
			Message:  "cannot close event log after append",
			Hint:     "Check repository storage health before retrying.",
			Path:     path.Relative,
			Field:    "events",
			Expected: "closed event log",
			Actual:   err.Error(),
		}
	}
	closed = true
	return nil
}

func writeNewFileAtomically(path SafePath, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path.Absolute), 0o755); err != nil {
		return helperStateDirectoryProblem(path, err)
	}
	if err := writeTempThenLink(path.Absolute, content, 0o600); err != nil {
		if errors.Is(err, os.ErrExist) {
			return helperStateExistsProblem(path.Relative, nil)
		}
		return &Problem{
			Code:     "helper_state_write_failed",
			Message:  "cannot create helper state file atomically",
			Hint:     "Check repository permissions and preserve stderr for diagnosis.",
			Path:     path.Relative,
			Field:    "path",
			Expected: "atomic new file write",
			Actual:   err.Error(),
		}
	}
	return nil
}

func writeExistingFileAtomically(path SafePath, content []byte) error {
	if err := os.MkdirAll(filepath.Dir(path.Absolute), 0o755); err != nil {
		return helperStateDirectoryProblem(path, err)
	}
	tmp, err := writeSyncedTemp(filepath.Dir(path.Absolute), filepath.Base(path.Absolute), content, 0o600)
	if err != nil {
		return &Problem{
			Code:     "helper_state_write_failed",
			Message:  "cannot write temporary state file",
			Hint:     "Check repository permissions and available disk space.",
			Path:     path.Relative,
			Field:    "path",
			Expected: "writable temporary file",
			Actual:   err.Error(),
		}
	}
	defer os.Remove(tmp)
	if err := os.Rename(tmp, path.Absolute); err != nil {
		return &Problem{
			Code:     "helper_state_replace_failed",
			Message:  "cannot atomically replace helper state file",
			Hint:     "Check repository permissions and preserve the temporary file if present.",
			Path:     path.Relative,
			Field:    "path",
			Expected: "atomic rename over state file",
			Actual:   err.Error(),
		}
	}
	if err := syncDirectory(filepath.Dir(path.Absolute)); err != nil {
		return &Problem{
			Code:     "helper_state_sync_failed",
			Message:  "cannot sync helper state directory",
			Hint:     "Check repository storage health before retrying.",
			Path:     filepath.ToSlash(filepath.Dir(path.Relative)),
			Field:    "path",
			Expected: "synced state directory",
			Actual:   err.Error(),
		}
	}
	return nil
}

func writeTempThenLink(final string, content []byte, perm os.FileMode) error {
	tmp, err := writeSyncedTemp(filepath.Dir(final), filepath.Base(final), content, perm)
	if err != nil {
		return err
	}
	defer os.Remove(tmp)
	if err := os.Link(tmp, final); err != nil {
		return err
	}
	return syncDirectory(filepath.Dir(final))
}

func writeSyncedTemp(dir string, base string, content []byte, perm os.FileMode) (string, error) {
	file, err := os.CreateTemp(dir, "."+base+".tmp-")
	if err != nil {
		return "", err
	}
	tmp := file.Name()
	closed := false
	cleanup := func() {
		if !closed {
			_ = file.Close()
		}
		_ = os.Remove(tmp)
	}
	if err := file.Chmod(perm); err != nil {
		cleanup()
		return "", err
	}
	if _, err := file.Write(content); err != nil {
		cleanup()
		return "", err
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return "", err
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", err
	}
	closed = true
	return tmp, nil
}

func syncDirectory(dir string) error {
	file, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Sync(); err != nil {
		return err
	}
	return nil
}

func helperStateDirectoryProblem(path SafePath, err error) *Problem {
	return &Problem{
		Code:     "helper_state_directory_failed",
		Message:  "cannot create helper state directory",
		Hint:     "Check permissions and ensure helper-managed parent paths are directories.",
		Path:     filepath.ToSlash(filepath.Dir(path.Relative)),
		Field:    "path",
		Expected: "writable directory path",
		Actual:   err.Error(),
	}
}

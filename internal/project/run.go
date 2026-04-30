package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	RunMetadataVersion = "0.1"
	RunRootPath        = ".kkachi/runs"

	RunStateCreated = "created"
	RunStateActive  = "active"
	RunStateClosed  = "closed"
	RunStateAborted = "aborted"
)

var (
	runIDPattern       = regexp.MustCompile(`^run-\d{8}T\d{6}Z-[0-9a-f]{12}$`)
	runIDSuffixPattern = regexp.MustCompile(`^[0-9a-f]{12}$`)

	writeRunMetadataNewDuringEvent      = writeRunMetadataNew
	writeRunMetadataExistingDuringEvent = writeRunMetadataExisting
)

type RunMetadata struct {
	Version           string         `json:"version"`
	RunID             string         `json:"run_id"`
	TaskID            *string        `json:"task_id"`
	Title             string         `json:"title"`
	WorkPath          string         `json:"work_path"`
	WorkMode          string         `json:"work_mode"`
	Urgency           string         `json:"urgency"`
	SOTPolicy         string         `json:"sot_policy"`
	ExecutionMode     string         `json:"execution_mode"`
	Commander         string         `json:"commander"`
	Redteam           *string        `json:"redteam"`
	CreatedAt         string         `json:"created_at"`
	State             string         `json:"state"`
	RequiredArtifacts []string       `json:"required_artifacts"`
	GateState         map[string]any `json:"gate_state"`
}

type RunSummary struct {
	RunID     string  `json:"run_id"`
	Title     string  `json:"title"`
	TaskID    *string `json:"task_id"`
	State     string  `json:"state"`
	CreatedAt string  `json:"created_at"`
}

type CreateRunOptions struct {
	TaskID        string
	Title         string
	WorkPath      string
	WorkMode      string
	Urgency       string
	SOTPolicy     string
	ExecutionMode string
	Commander     string
	Redteam       string
	Now           func() time.Time
	RandomHex     func(int) (string, error)
}

type RunLifecycleOptions struct {
	RunID string
	Now   func() time.Time
}

type CreateRunResult struct {
	Metadata  RunMetadata
	EventID   string
	RunPath   string
	CreatedAt string
}

type RunLifecycleResult struct {
	Metadata RunMetadata
	EventID  string
}

func GenerateRunID(now time.Time, randomSource func(int) (string, error)) (string, error) {
	if randomSource == nil {
		randomSource = randomHex
	}
	suffix, err := randomSource(6)
	if err != nil {
		return "", &Problem{Code: "run_id_generation_failed", Message: "cannot generate run id", Hint: "Retry run creation and preserve stderr if the problem repeats.", Field: "run_id", Expected: "crypto-random 12 hex suffix", Actual: err.Error()}
	}
	suffix = strings.ToLower(strings.TrimSpace(suffix))
	if !runIDSuffixPattern.MatchString(suffix) {
		return "", &Problem{Code: "run_id_generation_failed", Message: "generated run id suffix is invalid", Hint: "Use the default helper entropy source or provide a 12-hex test suffix.", Field: "run_id", Expected: "12 lowercase hex characters", Actual: suffix}
	}
	return fmt.Sprintf("run-%s-%s", now.UTC().Format("20060102T150405Z"), suffix), nil
}

func CreateRun(root Root, options CreateRunOptions) (CreateRunResult, error) {
	if strings.TrimSpace(root.Path) == "" {
		return CreateRunResult{}, problem("repo_root_required", "repository root is required", "Discover the repository root before creating a run.")
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if options.RandomHex == nil {
		options.RandomHex = randomHex
	}
	createdAt := options.Now().UTC().Format(time.RFC3339)
	if err := validateCreateRunOptions(options); err != nil {
		return CreateRunResult{}, err
	}
	if err := preflightEventCoherence(root); err != nil {
		return CreateRunResult{}, err
	}

	var runID string
	var runPath SafePath
	for attempt := 0; attempt < 16; attempt++ {
		candidate, err := GenerateRunID(options.Now().UTC(), options.RandomHex)
		if err != nil {
			return CreateRunResult{}, err
		}
		path, err := runMetadataPath(root, candidate)
		if err != nil {
			return CreateRunResult{}, err
		}
		if _, err := os.Lstat(path.Absolute); os.IsNotExist(err) {
			runID = candidate
			runPath = path
			break
		} else if err != nil {
			return CreateRunResult{}, &Problem{Code: "path_inspection_failed", Message: "cannot inspect run metadata path", Hint: "Check .kkachi/runs permissions before creating another run.", Path: path.Relative, Field: "path", Expected: "inspectable run metadata path", Actual: err.Error()}
		}
	}
	if runID == "" {
		return CreateRunResult{}, &Problem{Code: "run_id_collision", Message: "cannot allocate a unique run id", Hint: "Retry run creation; repeated collisions indicate a broken entropy source.", Field: "run_id", Expected: "unique helper-generated run id", Actual: "16 collisions"}
	}

	metadata := RunMetadata{
		Version:           RunMetadataVersion,
		RunID:             runID,
		TaskID:            optionalTrimmedString(options.TaskID),
		Title:             strings.TrimSpace(options.Title),
		WorkPath:          strings.TrimSpace(options.WorkPath),
		WorkMode:          strings.TrimSpace(options.WorkMode),
		Urgency:           strings.TrimSpace(options.Urgency),
		SOTPolicy:         strings.TrimSpace(options.SOTPolicy),
		ExecutionMode:     strings.TrimSpace(options.ExecutionMode),
		Commander:         strings.TrimSpace(options.Commander),
		Redteam:           optionalTrimmedString(options.Redteam),
		CreatedAt:         createdAt,
		State:             RunStateCreated,
		RequiredArtifacts: []string{},
		GateState:         map[string]any{},
	}
	if err := validateRunMetadata(metadata, runPath.Relative); err != nil {
		return CreateRunResult{}, err
	}
	appendResult, err := appendRunLifecycleEvent(root, AppendEventOptions{Type: "run.created", RunID: runID, Payload: runEventPayload(metadata), Now: options.Now}, nil, nil, func() error {
		return writeRunMetadataNewDuringEvent(runPath, metadata)
	})
	if err != nil {
		return CreateRunResult{}, err
	}
	return CreateRunResult{Metadata: metadata, EventID: appendResult.EventID, RunPath: filepath.ToSlash(filepath.Dir(runPath.Relative)), CreatedAt: createdAt}, nil
}

func ActivateRun(root Root, options RunLifecycleOptions) (RunLifecycleResult, error) {
	return transitionRun(root, options, RunStateActive, "run.activated")
}

func CloseRun(root Root, options RunLifecycleOptions) (RunLifecycleResult, error) {
	return transitionRun(root, options, RunStateClosed, "run.closed")
}

func AbortRun(root Root, options RunLifecycleOptions) (RunLifecycleResult, error) {
	return transitionRun(root, options, RunStateAborted, "run.aborted")
}

func ListRuns(root Root) ([]RunSummary, error) {
	if err := preflightEventCoherence(root); err != nil {
		return nil, err
	}
	ids, err := listRunIDs(root)
	if err != nil {
		return nil, err
	}
	summaries := make([]RunSummary, 0, len(ids))
	for _, id := range ids {
		metadata, _, err := ReadRunMetadata(root, id)
		if err != nil {
			return nil, err
		}
		summaries = append(summaries, RunSummary{RunID: metadata.RunID, Title: metadata.Title, TaskID: metadata.TaskID, State: metadata.State, CreatedAt: metadata.CreatedAt})
	}
	sort.Slice(summaries, func(i, j int) bool {
		if summaries[i].CreatedAt == summaries[j].CreatedAt {
			return summaries[i].RunID < summaries[j].RunID
		}
		return summaries[i].CreatedAt < summaries[j].CreatedAt
	})
	return summaries, nil
}

func ShowRun(root Root, query string) (RunMetadata, error) {
	if err := preflightEventCoherence(root); err != nil {
		return RunMetadata{}, err
	}
	metadata, _, err := ReadRunMetadata(root, query)
	return metadata, err
}

func ReadRunMetadata(root Root, query string) (RunMetadata, SafePath, error) {
	runID, err := ResolveRunID(root, query)
	if err != nil {
		return RunMetadata{}, SafePath{}, err
	}
	path, err := runMetadataPath(root, runID)
	if err != nil {
		return RunMetadata{}, SafePath{}, err
	}
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return RunMetadata{}, SafePath{}, &Problem{Code: "run_metadata_read_failed", Message: "cannot read run metadata", Hint: "Restore the run metadata file or remove the broken run directory after preserving diagnostics.", Path: path.Relative, Field: "path", Expected: "readable run-metadata.json", Actual: err.Error()}
	}
	var metadata RunMetadata
	if err := json.Unmarshal(data, &metadata); err != nil {
		return RunMetadata{}, SafePath{}, &Problem{Code: "run_metadata_invalid_json", Message: "run metadata is not valid JSON", Hint: "Restore run-metadata.json from a coherent backup before mutating the run.", Path: path.Relative, Field: "json", Expected: "JSON object run metadata", Actual: err.Error()}
	}
	if err := validateRunMetadata(metadata, path.Relative); err != nil {
		return RunMetadata{}, SafePath{}, err
	}
	if metadata.RunID != runID {
		return RunMetadata{}, SafePath{}, &Problem{Code: "run_metadata_invalid", Message: "run metadata id does not match its path", Hint: "Restore the run directory from a coherent backup before mutating helper state.", Path: path.Relative, Field: "run_id", Expected: runID, Actual: metadata.RunID}
	}
	return metadata, path, nil
}

func ResolveRunID(root Root, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", &Problem{Code: "run_id_required", Message: "run id is required", Hint: "Pass a full run id or a unique run id prefix.", Field: "run_id", Expected: "non-empty run id or prefix", Actual: "empty"}
	}
	if strings.ContainsAny(query, `/\`) || strings.Contains(query, "..") {
		return "", &Problem{Code: "run_id_invalid", Message: "run id lookup contains unsafe path characters", Hint: "Use a full helper-generated run id or a unique prefix, not a path.", Field: "run_id", Expected: "run id or prefix without path separators", Actual: query}
	}
	ids, err := listRunIDs(root)
	if err != nil {
		return "", err
	}
	if runIDPattern.MatchString(query) {
		for _, id := range ids {
			if id == query {
				return id, nil
			}
		}
		return "", &Problem{Code: "run_not_found", Message: "run id was not found", Hint: "Use run list to inspect available runs.", Field: "run_id", Expected: "existing run id", Actual: query}
	}
	var matches []string
	for _, id := range ids {
		if strings.HasPrefix(id, query) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return "", &Problem{Code: "run_not_found", Message: "run id prefix matched no runs", Hint: "Use run list to inspect available runs, then pass the full id or a longer unique prefix.", Field: "run_id", Expected: "existing run id prefix", Actual: query}
	default:
		return "", &Problem{Code: "run_id_ambiguous", Message: "run id prefix matched multiple runs", Hint: "Pass the full run id or a longer unique prefix.", Field: "run_id", Expected: "unique run id prefix", Actual: query}
	}
}

func transitionRun(root Root, options RunLifecycleOptions, targetState string, eventType string) (RunLifecycleResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	metadata, path, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return RunLifecycleResult{}, err
	}
	if err := validateTransition(metadata, targetState, path.Relative); err != nil {
		return RunLifecycleResult{}, err
	}
	if err := preflightEventCoherence(root); err != nil {
		return RunLifecycleResult{}, err
	}
	status, statusPath, err := readStatusForRun(root)
	if err != nil {
		return RunLifecycleResult{}, err
	}
	activeID, err := optionalString(status, "active_run_id")
	if err != nil {
		return RunLifecycleResult{}, &Problem{Code: "status_active_run_invalid", Message: "project status active_run_id must be null or a string", Hint: "Restore status.json from a coherent backup before mutating runs.", Path: statusPath.Relative, Field: "active_run_id", Expected: "null or string", Actual: fmt.Sprintf("%v", status["active_run_id"])}
	}
	statusActiveState, err := optionalString(status, "active_run_state")
	if err != nil {
		return RunLifecycleResult{}, &Problem{Code: "status_active_run_invalid", Message: "project status active_run_state must be null or a string", Hint: "Restore status.json from a coherent backup before mutating runs.", Path: statusPath.Relative, Field: "active_run_state", Expected: "null or string", Actual: fmt.Sprintf("%v", status["active_run_state"])}
	}
	if targetState == RunStateActive && activeID != nil && *activeID != metadata.RunID {
		return RunLifecycleResult{}, &Problem{Code: "active_run_conflict", Message: "another run is already active", Hint: "Close or abort the active run before activating a different run.", Path: statusPath.Relative, Field: "active_run_id", Expected: "null or same run id", Actual: *activeID}
	}

	oldState := metadata.State
	metadata.State = targetState

	var activeRunID *string
	var activeRunState *string
	if targetState == RunStateActive {
		activeRunID = &metadata.RunID
		activeRunState = &metadata.State
	} else if activeID != nil && *activeID == metadata.RunID {
		activeRunID = nil
		activeRunState = nil
	} else {
		// Preserve unrelated inactive status fields, normally null.
		activeRunID = activeID
		activeRunState = statusActiveState
	}

	appendResult, err := appendRunLifecycleEvent(root, AppendEventOptions{Type: eventType, RunID: metadata.RunID, Payload: map[string]any{"run_id": metadata.RunID, "previous_state": oldState, "state": targetState}, Now: options.Now}, activeRunID, activeRunState, func() error {
		return writeRunMetadataExistingDuringEvent(path, metadata)
	})
	if err != nil {
		return RunLifecycleResult{}, err
	}
	return RunLifecycleResult{Metadata: metadata, EventID: appendResult.EventID}, nil
}

func appendRunLifecycleEvent(root Root, options AppendEventOptions, activeRunID *string, activeRunState *string, beforeStatusUpdate func() error) (AppendEventResult, error) {
	return appendEventWithStatusMutation(root, options, func(status map[string]any, occurredAt string) error {
		if beforeStatusUpdate != nil {
			if err := beforeStatusUpdate(); err != nil {
				return err
			}
		}
		status["active_run_id"] = activeRunID
		status["active_run_state"] = activeRunState
		return nil
	})
}

func preflightEventCoherence(root Root) error {
	statusPath, err := ResolveRelativePath(root, StatusPath)
	if err != nil {
		return err
	}
	eventsPath, err := ResolveRelativePath(root, EventsPath)
	if err != nil {
		return err
	}
	status, err := readStatus(statusPath)
	if err != nil {
		return err
	}
	statusLast, err := statusLastEventID(status, statusPath.Relative)
	if err != nil {
		return err
	}
	logLast, err := scanLastEventID(eventsPath)
	if err != nil {
		return err
	}
	if statusLast != logLast {
		return &Problem{Code: "last_event_id_mismatch", Message: "status last_event_id does not match the event log tail", Hint: "Do not create or mutate runs until helper state is inspected by project doctor or restored from a coherent backup.", Path: statusPath.Relative, Field: "last_event_id", Expected: logLast, Actual: statusLast}
	}
	return nil
}

func runEventPayload(metadata RunMetadata) map[string]any {
	payload := map[string]any{"run_id": metadata.RunID, "title": metadata.Title, "state": metadata.State, "work_path": metadata.WorkPath, "work_mode": metadata.WorkMode, "urgency": metadata.Urgency, "sot_policy": metadata.SOTPolicy, "execution_mode": metadata.ExecutionMode, "commander": metadata.Commander}
	if metadata.TaskID != nil {
		payload["task_id"] = *metadata.TaskID
	}
	if metadata.Redteam != nil {
		payload["redteam"] = *metadata.Redteam
	}
	return payload
}

func validateCreateRunOptions(options CreateRunOptions) error {
	required := []struct{ field, value string }{{"title", options.Title}, {"work_path", options.WorkPath}, {"work_mode", options.WorkMode}, {"urgency", options.Urgency}, {"sot_policy", options.SOTPolicy}, {"execution_mode", options.ExecutionMode}, {"commander", options.Commander}}
	for _, item := range required {
		if strings.TrimSpace(item.value) == "" {
			return &Problem{Code: "run_field_required", Message: "run create is missing a required field", Hint: "Pass all required run create flags: --title, --work-path, --work-mode, --urgency, --sot-policy, --execution-mode, --commander.", Field: item.field, Expected: "non-empty value", Actual: "empty"}
		}
	}
	return nil
}

func validateRunMetadata(metadata RunMetadata, relative string) error {
	checks := []struct{ field, value string }{{"version", metadata.Version}, {"run_id", metadata.RunID}, {"title", metadata.Title}, {"work_path", metadata.WorkPath}, {"work_mode", metadata.WorkMode}, {"urgency", metadata.Urgency}, {"sot_policy", metadata.SOTPolicy}, {"execution_mode", metadata.ExecutionMode}, {"commander", metadata.Commander}, {"created_at", metadata.CreatedAt}, {"state", metadata.State}}
	for _, c := range checks {
		if strings.TrimSpace(c.value) == "" {
			return &Problem{Code: "run_metadata_invalid", Message: "run metadata is missing a required field", Hint: "Restore run-metadata.json from a coherent backup before mutating the run.", Path: relative, Field: c.field, Expected: "non-empty value", Actual: "empty"}
		}
	}
	if metadata.Version != RunMetadataVersion {
		return invalidRunField(relative, "version", RunMetadataVersion, metadata.Version)
	}
	if !runIDPattern.MatchString(metadata.RunID) {
		return invalidRunField(relative, "run_id", "run-YYYYMMDDTHHMMSSZ-<12hex>", metadata.RunID)
	}
	if _, err := time.Parse(time.RFC3339, metadata.CreatedAt); err != nil {
		return invalidRunField(relative, "created_at", "RFC3339 timestamp string", metadata.CreatedAt)
	}
	if !allowed(metadata.WorkPath, "A_development_execution", "B_discovery_shaping") {
		return invalidRunField(relative, "work_path", "A_development_execution or B_discovery_shaping", metadata.WorkPath)
	}
	if !allowed(metadata.WorkMode, "standard", "light") {
		return invalidRunField(relative, "work_mode", "standard or light", metadata.WorkMode)
	}
	if !allowed(metadata.Urgency, "normal", "urgent", "critical") {
		return invalidRunField(relative, "urgency", "normal, urgent, or critical", metadata.Urgency)
	}
	if !allowed(metadata.SOTPolicy, "existing_sot_basis", "minimal_sot_before_code", "full_sot_before_code") {
		return invalidRunField(relative, "sot_policy", "existing_sot_basis, minimal_sot_before_code, or full_sot_before_code", metadata.SOTPolicy)
	}
	if !allowed(metadata.ExecutionMode, "production_write", "adapter_qa", "readiness_hardening", "research", "verification", "docs_only") {
		return invalidRunField(relative, "execution_mode", "production_write, adapter_qa, readiness_hardening, research, verification, or docs_only", metadata.ExecutionMode)
	}
	if !allowed(metadata.State, RunStateCreated, RunStateActive, RunStateClosed, RunStateAborted) {
		return invalidRunField(relative, "state", "created, active, closed, or aborted", metadata.State)
	}
	if metadata.RequiredArtifacts == nil {
		return invalidRunField(relative, "required_artifacts", "array", "null")
	}
	if metadata.GateState == nil {
		return invalidRunField(relative, "gate_state", "object", "null")
	}
	return nil
}

func invalidRunField(relative, field, expected, actual string) *Problem {
	return &Problem{Code: "run_metadata_invalid", Message: "run metadata contains an invalid field", Hint: "Restore run-metadata.json from a coherent backup before mutating the run.", Path: relative, Field: field, Expected: expected, Actual: actual}
}

func validateTransition(metadata RunMetadata, target string, relative string) error {
	switch target {
	case RunStateActive:
		if metadata.State != RunStateCreated {
			return transitionProblem(relative, metadata.RunID, metadata.State, "created")
		}
	case RunStateClosed, RunStateAborted:
		if metadata.State != RunStateCreated && metadata.State != RunStateActive {
			return transitionProblem(relative, metadata.RunID, metadata.State, "created or active")
		}
	}
	return nil
}

func transitionProblem(relative, runID, actual, expected string) *Problem {
	return &Problem{Code: "run_transition_invalid", Message: "run lifecycle transition is not allowed", Hint: "Only created runs can activate; only created or active runs can close or abort.", Path: relative, Field: "state", Expected: expected, Actual: fmt.Sprintf("%s (%s)", actual, runID)}
}

func readStatusForRun(root Root) (map[string]any, SafePath, error) {
	path, err := ResolveRelativePath(root, StatusPath)
	if err != nil {
		return nil, SafePath{}, err
	}
	status, err := readStatus(path)
	return status, path, err
}

func writeRunMetadataNew(path SafePath, metadata RunMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return writeNewFileAtomically(path, append(data, '\n'))
}

func writeRunMetadataExisting(path SafePath, metadata RunMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return writeExistingFileAtomically(path, append(data, '\n'))
}

func runMetadataPath(root Root, runID string) (SafePath, error) {
	if !runIDPattern.MatchString(runID) {
		return SafePath{}, invalidRunField("", "run_id", "run-YYYYMMDDTHHMMSSZ-<12hex>", runID)
	}
	return ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "run-metadata.json")))
}

func listRunIDs(root Root) ([]string, error) {
	path, err := ResolveRelativePath(root, RunRootPath)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(path.Absolute)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, &Problem{Code: "run_root_read_failed", Message: "cannot read run root", Hint: "Check .kkachi/runs permissions before inspecting runs.", Path: path.Relative, Field: "path", Expected: "readable run root directory", Actual: err.Error()}
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() && runIDPattern.MatchString(entry.Name()) {
			ids = append(ids, entry.Name())
		}
	}
	sort.Strings(ids)
	return ids, nil
}

func optionalTrimmedString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func allowed(value string, allowedValues ...string) bool {
	for _, allowedValue := range allowedValues {
		if value == allowedValue {
			return true
		}
	}
	return false
}

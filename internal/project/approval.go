package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

const (
	ApprovalEventRequested = "approval.requested"
	ApprovalEventRecorded  = "approval.recorded"

	ApprovalDecisionApproved = "approved"
	ApprovalDecisionRejected = "rejected"
)

type ApprovalRequestOptions struct {
	RunID    string
	Phase    string
	Reason   string
	Evidence string
	Now      func() time.Time
}

type ApprovalRecordOptions struct {
	RunID    string
	Phase    string
	Decision string
	Approver string
	Evidence string
	Reason   string
	Now      func() time.Time
}

type ApprovalShowOptions struct {
	RunID string
	Phase string
}

type ApprovalMutationResult struct {
	Record  ApprovalRecord `json:"record"`
	EventID string         `json:"event_id"`
}

type ApprovalShowResult struct {
	RunID   string           `json:"run_id"`
	Phase   string           `json:"phase,omitempty"`
	Records []ApprovalRecord `json:"records"`
}

type ApprovalRecord struct {
	EventID    string `json:"event_id"`
	OccurredAt string `json:"occurred_at"`
	Type       string `json:"type"`
	RunID      string `json:"run_id"`
	Phase      string `json:"phase"`
	Reason     string `json:"reason,omitempty"`
	Decision   string `json:"decision,omitempty"`
	Approver   string `json:"approver,omitempty"`
	Timestamp  string `json:"timestamp"`
	Evidence   string `json:"evidence,omitempty"`
}

func RequestApproval(root Root, options ApprovalRequestOptions) (ApprovalMutationResult, error) {
	var result ApprovalMutationResult
	err := withProjectWriteLock(root, "approval request", options.RunID, func() error {
		var err error
		result, err = requestApprovalUnlocked(root, options)
		return err
	})
	return result, err
}

func requestApprovalUnlocked(root Root, options ApprovalRequestOptions) (ApprovalMutationResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	metadata, metadataPath, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return ApprovalMutationResult{}, err
	}
	if metadata.State == RunStateClosed || metadata.State == RunStateAborted {
		return ApprovalMutationResult{}, &Problem{Code: "run_approval_invalid_state", Message: "cannot record approvals for a finished run", Hint: "Create a new run or inspect existing approval records without mutating them.", Path: metadataPath.Relative, Field: "state", Expected: "created or active", Actual: metadata.State}
	}
	if err := preflightEventCoherence(root); err != nil {
		return ApprovalMutationResult{}, err
	}
	timestamp := options.Now().UTC().Format(time.RFC3339)
	record := ApprovalRecord{Type: ApprovalEventRequested, RunID: metadata.RunID, Phase: strings.TrimSpace(options.Phase), Reason: strings.TrimSpace(options.Reason), Timestamp: timestamp, Evidence: strings.TrimSpace(options.Evidence)}
	if err := validateApprovalRequestRecord(record); err != nil {
		return ApprovalMutationResult{}, err
	}
	payload := approvalPayload(record)
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: ApprovalEventRequested, RunID: metadata.RunID, Payload: payload, Now: func() time.Time { parsed, _ := time.Parse(time.RFC3339, timestamp); return parsed }}, nil)
	if err != nil {
		return ApprovalMutationResult{}, err
	}
	record.EventID = appendResult.EventID
	record.OccurredAt = appendResult.OccurredAt
	return ApprovalMutationResult{Record: record, EventID: appendResult.EventID}, nil
}

func RecordApproval(root Root, options ApprovalRecordOptions) (ApprovalMutationResult, error) {
	var result ApprovalMutationResult
	err := withProjectWriteLock(root, "approval record", options.RunID, func() error {
		var err error
		result, err = recordApprovalUnlocked(root, options)
		return err
	})
	return result, err
}

func recordApprovalUnlocked(root Root, options ApprovalRecordOptions) (ApprovalMutationResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	metadata, metadataPath, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return ApprovalMutationResult{}, err
	}
	if metadata.State == RunStateClosed || metadata.State == RunStateAborted {
		return ApprovalMutationResult{}, &Problem{Code: "run_approval_invalid_state", Message: "cannot record approvals for a finished run", Hint: "Create a new run or inspect existing approval records without mutating them.", Path: metadataPath.Relative, Field: "state", Expected: "created or active", Actual: metadata.State}
	}
	if err := preflightEventCoherence(root); err != nil {
		return ApprovalMutationResult{}, err
	}
	timestamp := options.Now().UTC().Format(time.RFC3339)
	record := ApprovalRecord{Type: ApprovalEventRecorded, RunID: metadata.RunID, Phase: strings.TrimSpace(options.Phase), Reason: strings.TrimSpace(options.Reason), Decision: strings.TrimSpace(options.Decision), Approver: strings.TrimSpace(options.Approver), Timestamp: timestamp, Evidence: strings.TrimSpace(options.Evidence)}
	if err := validateApprovalDecisionRecord(record); err != nil {
		return ApprovalMutationResult{}, err
	}
	payload := approvalPayload(record)
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: ApprovalEventRecorded, RunID: metadata.RunID, Payload: payload, Now: func() time.Time { parsed, _ := time.Parse(time.RFC3339, timestamp); return parsed }}, nil)
	if err != nil {
		return ApprovalMutationResult{}, err
	}
	record.EventID = appendResult.EventID
	record.OccurredAt = appendResult.OccurredAt
	return ApprovalMutationResult{Record: record, EventID: appendResult.EventID}, nil
}

func ShowApprovals(root Root, options ApprovalShowOptions) (ApprovalShowResult, error) {
	metadata, _, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return ApprovalShowResult{}, err
	}
	if err := preflightEventCoherence(root); err != nil {
		return ApprovalShowResult{}, err
	}
	records, err := ApprovalRecords(root, metadata.RunID)
	if err != nil {
		return ApprovalShowResult{}, err
	}
	phase := strings.TrimSpace(options.Phase)
	if phase != "" {
		if err := validateApprovalPhase(phase); err != nil {
			return ApprovalShowResult{}, err
		}
		filtered := []ApprovalRecord{}
		for _, record := range records {
			if record.Phase == phase {
				filtered = append(filtered, record)
			}
		}
		records = filtered
	}
	return ApprovalShowResult{RunID: metadata.RunID, Phase: phase, Records: records}, nil
}

func ApprovalRecords(root Root, runID string) ([]ApprovalRecord, error) {
	path, err := ResolveRelativePath(root, EventsPath)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(path.Absolute)
	if err != nil {
		return nil, &Problem{Code: "event_log_read_failed", Message: "cannot read event log", Hint: "Run project init first or restore .kkachi/events.jsonl from backup.", Path: path.Relative, Field: "path", Expected: "readable JSONL event log", Actual: err.Error()}
	}
	defer file.Close()

	records := []ApprovalRecord{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 64*1024), MaxEventLineBytes)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return nil, eventLogProblem(path.Relative, lineNumber, "non-empty JSON object line", "empty line")
		}
		var event Event
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			return nil, eventLogProblem(path.Relative, lineNumber, "valid JSON object line", err.Error())
		}
		if event.RunID == nil || *event.RunID != runID || !isApprovalEventType(event.Type) {
			continue
		}
		record, err := approvalRecordFromEvent(event)
		if err != nil {
			return nil, &Problem{Code: "approval_event_invalid", Message: "approval event payload is invalid", Hint: "Record approvals with approval request or approval record so the strict schema is preserved.", Path: path.Relative, Field: event.EventID, Expected: "valid approval event payload", Actual: err.Error()}
		}
		records = append(records, record)
	}
	if err := scanner.Err(); err != nil {
		if strings.Contains(err.Error(), "token too long") {
			return nil, eventLineTooLargeProblem(path.Relative, MaxEventLineBytes+1)
		}
		return nil, &Problem{Code: "event_log_read_failed", Message: "cannot scan event log", Hint: "Check event log permissions and restore from backup if the file is damaged.", Path: path.Relative, Field: "events", Expected: "readable JSONL event log", Actual: err.Error()}
	}
	return records, nil
}

func approvalRecordFromEvent(event Event) (ApprovalRecord, error) {
	record := ApprovalRecord{EventID: event.EventID, OccurredAt: event.OccurredAt, Type: event.Type}
	if event.RunID != nil {
		record.RunID = *event.RunID
	}
	if phase, ok := event.Payload["phase"].(string); ok {
		record.Phase = strings.TrimSpace(phase)
	}
	if reason, ok := event.Payload["reason"].(string); ok {
		record.Reason = strings.TrimSpace(reason)
	}
	if decision, ok := event.Payload["decision"].(string); ok {
		record.Decision = strings.TrimSpace(decision)
	}
	if approver, ok := event.Payload["approver"].(string); ok {
		record.Approver = strings.TrimSpace(approver)
	}
	if timestamp, ok := event.Payload["timestamp"].(string); ok {
		record.Timestamp = strings.TrimSpace(timestamp)
	}
	if evidence, ok := event.Payload["evidence"].(string); ok {
		record.Evidence = strings.TrimSpace(evidence)
	}
	if event.Type == ApprovalEventRequested {
		return record, validateApprovalRequestRecord(record)
	}
	if event.Type == ApprovalEventRecorded {
		return record, validateApprovalDecisionRecord(record)
	}
	return record, fmt.Errorf("unsupported approval event type %q", event.Type)
}

func ApprovalLatestDecisions(root Root, runID string) (map[string]ApprovalRecord, error) {
	records, err := ApprovalRecords(root, runID)
	if err != nil {
		return nil, err
	}
	latest := map[string]ApprovalRecord{}
	for _, record := range records {
		if record.Type != ApprovalEventRecorded {
			continue
		}
		latest[record.Phase] = record
	}
	return latest, nil
}

func isApprovalEventType(eventType string) bool {
	return eventType == ApprovalEventRequested || eventType == ApprovalEventRecorded
}

func approvalPayload(record ApprovalRecord) map[string]any {
	payload := map[string]any{"phase": record.Phase, "timestamp": record.Timestamp}
	if record.Reason != "" {
		payload["reason"] = record.Reason
	}
	if record.Decision != "" {
		payload["decision"] = record.Decision
	}
	if record.Approver != "" {
		payload["approver"] = record.Approver
	}
	if record.Evidence != "" {
		payload["evidence"] = record.Evidence
	}
	return payload
}

func validateApprovalRequestRecord(record ApprovalRecord) error {
	if err := validateApprovalCommon(record); err != nil {
		return err
	}
	if strings.TrimSpace(record.Reason) == "" {
		return &Problem{Code: "approval_reason_required", Message: "approval request requires a reason", Hint: "Record KHS's explicit approval reason; KAH does not infer whether approval is required.", Field: "reason", Expected: "non-empty reason", Actual: "missing"}
	}
	return nil
}

func validateApprovalDecisionRecord(record ApprovalRecord) error {
	if err := validateApprovalCommon(record); err != nil {
		return err
	}
	if record.Decision != ApprovalDecisionApproved && record.Decision != ApprovalDecisionRejected {
		actual := record.Decision
		if actual == "" {
			actual = "missing"
		}
		return &Problem{Code: "approval_decision_invalid", Message: "approval decision is not supported", Hint: "Use approved or rejected.", Field: "decision", Expected: "approved or rejected", Actual: actual}
	}
	if strings.TrimSpace(record.Approver) == "" {
		return &Problem{Code: "approval_approver_required", Message: "approval decision requires an approver", Hint: "Pass --by with the approving principal, such as master.", Field: "approver", Expected: "non-empty approver", Actual: "missing"}
	}
	if strings.TrimSpace(record.Evidence) == "" {
		return &Problem{Code: "approval_evidence_required", Message: "approval decision requires evidence", Hint: "Pass --evidence with an artifact path or message reference.", Field: "evidence", Expected: "non-empty evidence reference", Actual: "missing"}
	}
	return nil
}

func validateApprovalCommon(record ApprovalRecord) error {
	if err := validateApprovalPhase(record.Phase); err != nil {
		return err
	}
	if strings.TrimSpace(record.Timestamp) == "" {
		return &Problem{Code: "approval_timestamp_required", Message: "approval timestamp is required", Hint: "Use approval commands so KAH records a helper timestamp.", Field: "timestamp", Expected: "RFC3339 timestamp", Actual: "missing"}
	}
	if _, err := time.Parse(time.RFC3339, record.Timestamp); err != nil {
		return &Problem{Code: "approval_timestamp_invalid", Message: "approval timestamp is invalid", Hint: "Use approval commands so KAH records an RFC3339 timestamp.", Field: "timestamp", Expected: "RFC3339 timestamp", Actual: err.Error()}
	}
	return nil
}

func validateApprovalPhase(phase string) error {
	phase = strings.TrimSpace(phase)
	if phase == "" {
		return &Problem{Code: "approval_phase_required", Message: "approval phase is required", Hint: "Pass --phase with the KHS-declared phase id.", Field: "phase", Expected: "non-empty phase id", Actual: "missing"}
	}
	for _, r := range phase {
		if r < 0x20 || r == 0x7f {
			return &Problem{Code: "approval_phase_invalid", Message: "approval phase contains control characters", Hint: "Use a printable KHS phase id.", Field: "phase", Expected: "printable phase id", Actual: "contains control character"}
		}
	}
	return nil
}

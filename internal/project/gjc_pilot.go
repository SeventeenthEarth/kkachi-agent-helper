package project

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	gjcPlanLockUnlocked         = "unlocked"
	gjcPlanLockLocked           = "locked"
	gjcCallbackStatusDelivered  = "callback_delivered"
	gjcCallbackResultPending    = "pending"
	gjcCallbackResultDelivered  = "delivered"
	gjcCallbackResultFailed     = "failed"
	gjcNoWakeClaimNotification  = "no-wake-claim"
	gjcPlanConflictReportName   = "plan-conflict.json"
	gjcCallbackConflictHint     = "Use a new idempotency key for a different callback source hash or repair the existing callback evidence."
	gjcPlanLockAcceptancePolicy = "KAS/Blue/color approval evidence is required; KAH records the supplied hash mechanically only."
)

type GJCCallbackOptions struct {
	RunID            string
	TaskID           string
	Status           string
	Result           string
	IdempotencyKey   string
	SourceStatusHash string
	NotificationRef  string
	Now              func() time.Time
}

type GJCPlanLockOptions struct {
	RunID            string
	AcceptedPlanHash string
	ApprovalRef      string
	Now              func() time.Time
}

func RecordGJCCallback(root Root, options GJCCallbackOptions) (GJCStatusResult, error) {
	var result GJCStatusResult
	err := withProjectWriteLock(root, "gjc callback-kanban", options.RunID, func() error {
		var err error
		result, err = recordGJCCallbackUnlocked(root, options)
		return err
	})
	return result, err
}

func LockGJCPlan(root Root, options GJCPlanLockOptions) (GJCStatusResult, error) {
	var result GJCStatusResult
	err := withProjectWriteLock(root, "gjc lock-plan", options.RunID, func() error {
		var err error
		result, err = lockGJCPlanUnlocked(root, options)
		return err
	})
	return result, err
}

func recordGJCCallbackUnlocked(root Root, options GJCCallbackOptions) (GJCStatusResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	status, err := readValidatedGJCStatus(root, options.RunID)
	if err != nil {
		return GJCStatusResult{}, err
	}
	callback, err := parseGJCCallbackOptions(status, options)
	if err != nil {
		return GJCStatusResult{}, err
	}
	if status.Callback != nil {
		if status.Callback.IdempotencyKey == callback.IdempotencyKey && status.Callback.SourceStatusHash == callback.SourceStatusHash && status.Callback.TaskID == callback.TaskID {
			return GJCStatusResult{Status: status}, nil
		}
		if status.Callback.IdempotencyKey == callback.IdempotencyKey {
			return GJCStatusResult{}, &Problem{Code: "gjc_callback_idempotency_conflict", Message: "GJC callback idempotency key conflicts with existing evidence", Hint: gjcCallbackConflictHint, Field: "idempotency_key", Expected: status.Callback.SourceStatusHash, Actual: callback.SourceStatusHash}
		}
	}
	if callback.SourceStatusHash != status.StatusHash {
		return GJCStatusResult{}, &Problem{Code: "gjc_callback_source_hash_mismatch", Message: "GJC callback source status hash does not match current status evidence", Hint: "Use the current status_hash or replay the existing idempotent callback.", Field: "source_status_hash", Expected: status.StatusHash, Actual: callback.SourceStatusHash}
	}
	status.Callback = &callback
	status.UpdatedAt = options.Now().UTC().Format(time.RFC3339)
	updated, _, err := writeGJCStatusMutation(root, status, gjcEventCallback)
	if err != nil {
		return GJCStatusResult{}, err
	}
	return GJCStatusResult{Status: updated}, nil
}

func lockGJCPlanUnlocked(root Root, options GJCPlanLockOptions) (GJCStatusResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	status, err := readValidatedGJCStatus(root, options.RunID)
	if err != nil {
		return GJCStatusResult{}, err
	}
	acceptedHash := strings.TrimSpace(options.AcceptedPlanHash)
	if !gjcChecksumPattern.MatchString(acceptedHash) {
		return GJCStatusResult{}, &Problem{Code: "gjc_plan_hash_missing", Message: "GJC plan lock requires an accepted plan hash", Hint: gjcPlanLockAcceptancePolicy, Field: "accepted_plan_hash", Expected: "sha256:<64hex>", Actual: acceptedHash}
	}
	if status.Plan.Artifact == "" || status.Plan.ArtifactHash == "" {
		return GJCStatusResult{}, &Problem{Code: "gjc_plan_artifact_missing", Message: "GJC plan lock requires a plan artifact hash", Hint: "Run start-ralplan to produce ralplan_ready plan artifact evidence before locking.", Field: "plan.artifact_hash", Expected: "sha256:<64hex>", Actual: "missing"}
	}
	currentHash, err := currentGJCPlanHash(root, status.RunID, status.Plan.Artifact)
	if err != nil {
		return GJCStatusResult{}, err
	}
	if currentHash != acceptedHash {
		return GJCStatusResult{}, &Problem{Code: "gjc_plan_hash_mismatch", Message: "accepted plan hash does not match current plan artifact", Hint: "Return to KAS plan review before recording a lock.", Path: status.Plan.Artifact, Field: "accepted_plan_hash", Expected: currentHash, Actual: acceptedHash}
	}
	approvalRef := strings.TrimSpace(options.ApprovalRef)
	if approvalRef == "" {
		return GJCStatusResult{}, &Problem{Code: "gjc_plan_lock_approval_missing", Message: "GJC plan lock requires KAS/Blue/color approval evidence", Hint: gjcPlanLockAcceptancePolicy, Field: "approval_ref", Expected: "non-empty approval evidence ref", Actual: "missing"}
	}
	status.Plan.LockStatus = gjcPlanLockLocked
	status.Plan.AcceptedPlanHash = acceptedHash
	status.Plan.ApprovalRef = approvalRef
	status.UpdatedAt = options.Now().UTC().Format(time.RFC3339)
	updated, _, err := writeGJCStatusMutation(root, status, gjcEventPlanLocked)
	if err != nil {
		return GJCStatusResult{}, err
	}
	return GJCStatusResult{Status: updated}, nil
}

func parseGJCCallbackOptions(status GJCStatus, options GJCCallbackOptions) (GJCCallback, error) {
	taskID := strings.TrimSpace(options.TaskID)
	if taskID == "" {
		return GJCCallback{}, &Problem{Code: "gjc_task_required", Message: "GJC callback requires a task id", Hint: "Pass --task with the Kkachi task id supplied by KAS.", Field: "task_id", Expected: "non-empty task id", Actual: "missing"}
	}
	callbackStatus := strings.TrimSpace(options.Status)
	if callbackStatus == "" {
		callbackStatus = gjcCallbackStatusDelivered
	}
	if callbackStatus != gjcCallbackStatusDelivered {
		return GJCCallback{}, &Problem{Code: "gjc_callback_status_unsupported", Message: "GJC callback status is unsupported", Hint: "Callbacks may record callback_delivered evidence only; they must not approve plans or final state.", Field: "status", Expected: gjcCallbackStatusDelivered, Actual: callbackStatus}
	}
	result := strings.TrimSpace(options.Result)
	if result == "" {
		result = gjcCallbackResultDelivered
	}
	if !validGJCCallbackResult(result) {
		return GJCCallback{}, &Problem{Code: "gjc_callback_result_unsupported", Message: "GJC callback result is unsupported", Hint: "Use pending, delivered, or failed as factual callback evidence.", Field: "result", Expected: "pending, delivered, or failed", Actual: result}
	}
	idempotencyKey := strings.TrimSpace(options.IdempotencyKey)
	if idempotencyKey == "" {
		return GJCCallback{}, &Problem{Code: "gjc_callback_idempotency_missing", Message: "GJC callback idempotency key is missing", Hint: "Callbacks must be replay-safe and conflict-detectable.", Field: "idempotency_key", Expected: "non-empty idempotency key", Actual: "missing"}
	}
	sourceHash := strings.TrimSpace(options.SourceStatusHash)
	if !gjcChecksumPattern.MatchString(sourceHash) {
		return GJCCallback{}, &Problem{Code: "gjc_checksum_malformed", Message: "GJC callback source status hash is malformed", Hint: "Use the status_hash from the status evidence that triggered the callback.", Field: "source_status_hash", Expected: "sha256:<64hex>", Actual: sourceHash}
	}
	notificationRef := strings.TrimSpace(options.NotificationRef)
	if notificationRef == "" {
		notificationRef = gjcNoWakeClaimNotification
	}
	return GJCCallback{
		TaskID:              taskID,
		Status:              callbackStatus,
		IdempotencyKey:      idempotencyKey,
		SourceStatusHash:    sourceHash,
		LastCallbackStatus:  result,
		NotificationRef:     notificationRef,
		LastNotifiedHash:    sourceHash,
		SameThreadWakeClaim: false,
		UpdatedAt:           options.Now().UTC().Format(time.RFC3339),
	}, nil
}

func validGJCCallbackResult(result string) bool {
	switch result {
	case gjcCallbackResultPending, gjcCallbackResultDelivered, gjcCallbackResultFailed:
		return true
	default:
		return false
	}
}

func writeGJCReceipt(root Root, runID string, stdout []byte) (GJCArtifactRef, error) {
	path, err := gjcReceiptPath(root, runID)
	if err != nil {
		return GJCArtifactRef{}, err
	}
	content := append([]byte(strings.TrimSpace(string(stdout))), '\n')
	if _, err := os.Stat(path.Absolute); os.IsNotExist(err) {
		if err := writeNewFileAtomically(path, content); err != nil {
			return GJCArtifactRef{}, err
		}
		return GJCArtifactRef{Path: path.Relative, SHA256: sha256String(content)}, nil
	}
	if err != nil {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ref_inspection_failed", Message: "cannot inspect GJC receipt evidence", Hint: "Check run-local evidence permissions before retrying.", Path: path.Relative, Field: "receipt_ref", Expected: "inspectable receipt file", Actual: err.Error()}
	}
	if err := writeExistingFileAtomically(path, content); err != nil {
		return GJCArtifactRef{}, err
	}
	return GJCArtifactRef{Path: path.Relative, SHA256: sha256String(content)}, nil
}

func readValidatedGJCStatus(root Root, runID string) (GJCStatus, error) {
	result, err := ShowGJCStatus(root, GJCStatusOptions{RunID: runID})
	if err != nil {
		return GJCStatus{}, err
	}
	return result.Status, nil
}

func writeGJCStatusMutation(root Root, status GJCStatus, eventType string) (GJCStatus, string, error) {
	statusPath, err := gjcStatusPath(root, status.RunID)
	if err != nil {
		return GJCStatus{}, "", err
	}
	status.StatusPath = statusPath.Relative
	statusHash, err := computeGJCStatusHash(status)
	if err != nil {
		return GJCStatus{}, "", err
	}
	status.StatusHash = statusHash
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return GJCStatus{}, "", &Problem{Code: "gjc_status_encode_failed", Message: "cannot encode GJC status ledger", Hint: "Report the unsupported status payload to KAH maintainers.", Field: "status", Expected: "JSON-encodable status", Actual: err.Error()}
	}
	data = append(data, '\n')
	appendResult, err := appendEventWithPreparedStatusMutation(root, AppendEventOptions{Type: eventType, RunID: status.RunID, Payload: gjcEventPayload(status), Now: timestampFunc(status.UpdatedAt)}, func(_ map[string]any, _ string, _ string) (preparedEventStatusMutation, error) {
		return preparedEventStatusMutation{Payload: gjcEventPayload(status), BeforeAppend: func() error { return writeExistingFileAtomically(statusPath, data) }}, nil
	})
	if err != nil {
		return GJCStatus{}, "", err
	}
	return status, appendResult.EventID, nil
}

func applyGJCPlanEvidence(root Root, runID string, commandKind string, status *GJCStatus) error {
	if commandKind != gjcCommandRalplan || status.Process.Status != "ralplan_ready" {
		return nil
	}
	for _, ref := range status.Artifacts {
		if strings.Contains(ref.Path, "/artifacts/plan/") {
			status.Plan = GJCPlanEvidence{Artifact: ref.Path, ArtifactHash: ref.SHA256, LockStatus: gjcPlanLockUnlocked}
			return nil
		}
	}
	return &Problem{Code: "gjc_plan_artifact_missing", Message: "ralplan_ready requires a run-local plan artifact hash", Hint: "Emit at least one artifact_ref under .kkachi/runs/<run_id>/artifacts/plan/ with a matching sha256.", Field: "artifact_refs", Expected: filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "plan")) + "/...", Actual: "missing"}
}

func validateGJCPlanEvidence(root Root, runID string, status GJCStatus) error {
	if status.CommandKind == gjcCommandRalplan && status.Process.Status == "ralplan_ready" {
		if status.Plan.Artifact == "" || status.Plan.ArtifactHash == "" {
			return &Problem{Code: "gjc_plan_artifact_missing", Message: "ralplan_ready status lacks plan artifact hash", Hint: "Regenerate status from a GJC receipt with a run-local plan artifact_ref.", Field: "plan.artifact_hash", Expected: "sha256:<64hex>", Actual: "missing"}
		}
		currentHash, err := currentGJCPlanHash(root, runID, status.Plan.Artifact)
		if err != nil {
			return err
		}
		if status.Plan.LockStatus == gjcPlanLockLocked && status.Plan.AcceptedPlanHash != currentHash {
			reportPath, reportErr := writeGJCPlanConflictReport(root, status, currentHash)
			if reportErr != nil {
				return reportErr
			}
			return &Problem{Code: "gjc_plan_lock_conflict", Message: "locked GJC plan hash no longer matches the plan artifact", Hint: "Return to KAS with plan-conflict evidence before continuing.", Path: reportPath, Field: "plan.accepted_plan_hash", Expected: currentHash, Actual: status.Plan.AcceptedPlanHash}
		}
		if status.Plan.ArtifactHash != currentHash {
			return &Problem{Code: "gjc_checksum_mismatch", Message: "GJC plan artifact hash does not match file content", Hint: "Regenerate the GJC receipt after plan artifact content changes.", Path: status.Plan.Artifact, Field: "plan.artifact_hash", Expected: currentHash, Actual: status.Plan.ArtifactHash}
		}
	}
	return nil
}

func currentGJCPlanHash(root Root, runID string, artifact string) (string, error) {
	path, err := validateGJCRunRef(root, runID, artifact, "plan.artifact")
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return "", &Problem{Code: "gjc_artifact_read_failed", Message: "cannot read GJC plan artifact", Hint: "Ensure the accepted plan artifact remains readable before consuming status.", Path: path.Relative, Field: "plan.artifact", Expected: "readable plan artifact", Actual: err.Error()}
	}
	return sha256String(data), nil
}

func validateGJCCallback(callback *GJCCallback) error {
	if callback == nil {
		return nil
	}
	if strings.TrimSpace(callback.TaskID) == "" {
		return &Problem{Code: "gjc_task_required", Message: "GJC callback task id is missing", Hint: "Regenerate callback evidence with the Kkachi task id.", Field: "callback.task_id", Expected: "non-empty task id", Actual: "missing"}
	}
	if callback.Status != gjcCallbackStatusDelivered {
		return &Problem{Code: "gjc_callback_status_unsupported", Message: "GJC callback status is unsupported", Hint: "Callbacks may record callback_delivered evidence only.", Field: "callback.status", Expected: gjcCallbackStatusDelivered, Actual: callback.Status}
	}
	if strings.TrimSpace(callback.IdempotencyKey) == "" {
		return &Problem{Code: "gjc_callback_idempotency_missing", Message: "GJC callback idempotency key is missing", Hint: "Regenerate callback evidence with an idempotency key.", Field: "callback.idempotency_key", Expected: "non-empty idempotency key", Actual: "missing"}
	}
	if !gjcChecksumPattern.MatchString(callback.SourceStatusHash) {
		return &Problem{Code: "gjc_checksum_malformed", Message: "GJC callback source status hash is malformed", Hint: "Regenerate callback evidence with the source status_hash.", Field: "callback.source_status_hash", Expected: "sha256:<64hex>", Actual: callback.SourceStatusHash}
	}
	if !validGJCCallbackResult(callback.LastCallbackStatus) {
		return &Problem{Code: "gjc_callback_result_unsupported", Message: "GJC callback result is unsupported", Hint: "Use pending, delivered, or failed.", Field: "callback.last_callback_status", Expected: "pending, delivered, or failed", Actual: callback.LastCallbackStatus}
	}
	return nil
}

func gjcConflictReportPath(runID string) string {
	return filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "gjc", gjcPlanConflictReportName))
}

func writeGJCPlanConflictReport(root Root, status GJCStatus, currentHash string) (string, error) {
	reportPath, err := ResolveRelativePath(root, gjcConflictReportPath(status.RunID))
	if err != nil {
		return "", err
	}
	data, err := encodeGJCConflictReport(status, currentHash)
	if err != nil {
		return "", &Problem{Code: "gjc_plan_conflict_report_encode_failed", Message: "cannot encode GJC plan-conflict report", Hint: "Report the unsupported status payload to KAH maintainers.", Field: "plan.conflict_report", Expected: "JSON-encodable conflict report", Actual: err.Error()}
	}
	if _, err := os.Stat(reportPath.Absolute); os.IsNotExist(err) {
		if err := writeNewFileAtomically(reportPath, data); err != nil {
			return "", err
		}
		return reportPath.Relative, nil
	} else if err != nil {
		return "", &Problem{Code: "gjc_ref_inspection_failed", Message: "cannot inspect GJC plan-conflict report evidence", Hint: "Check run-local evidence permissions before retrying.", Path: reportPath.Relative, Field: "plan.conflict_report_path", Expected: "inspectable conflict report file", Actual: err.Error()}
	}
	if err := writeExistingFileAtomically(reportPath, data); err != nil {
		return "", err
	}
	return reportPath.Relative, nil
}

func encodeGJCConflictReport(status GJCStatus, expectedHash string) ([]byte, error) {
	report := map[string]string{
		"schema_version":        GJCSchemaVersion,
		"run_id":                status.RunID,
		"task_id":               status.TaskID,
		"artifact":              status.Plan.Artifact,
		"accepted_plan_hash":    status.Plan.AcceptedPlanHash,
		"current_artifact_hash": expectedHash,
		"status":                "plan_conflict_reported",
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode GJC conflict report: %w", err)
	}
	return append(data, '\n'), nil
}

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
	gjcKATAttachmentReady       = "kat_evidence_ready"
	gjcKATAttachmentFailed      = "kat_evidence_failed"
	gjcNoWakeClaimNotification  = "no-wake-claim"
	gjcNotificationNoWakeClaim  = "no_wake_claim"
	gjcNotificationMetadataOnly = "metadata_recorded_no_wake_claim"
	gjcWakeEvidenceMissing      = "missing_watcher_evidence"
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

type GJCKATAttachOptions struct {
	RunID            string
	StatusPath       string
	StatusHash       string
	SummaryPath      string
	SummaryHash      string
	SummaryMDPath    string
	SummaryMDHash    string
	RawLogPath       string
	RawLogHash       string
	AttachmentStatus string
	Now              func() time.Time
}

type gjcKATStatusEvidence struct {
	SchemaVersion     string `json:"schema_version"`
	RunID             string `json:"run_id"`
	Status            string `json:"status"`
	ExtractorStatus   string `json:"extractor_status"`
	CommandExitCode   *int   `json:"command_exit_code"`
	ExitCode          *int   `json:"exit_code"`
	SourceStatusHash  string `json:"source_status_hash"`
	SelfApproval      bool   `json:"self_approval"`
	FinalAccepted     bool   `json:"final_accepted"`
	ReviewApproved    bool   `json:"review_approved"`
	WaiverApproved    bool   `json:"waiver_approved"`
	CandidateEvidence bool   `json:"candidate_evidence"`
	CommandID         string `json:"command_id"`
	Lane              string `json:"lane"`
	SummaryPath       string `json:"summary_path"`
	SummarySHA256     string `json:"summary_sha256"`
	RawLogPath        string `json:"raw_log_path"`
	RawLogSHA256      string `json:"raw_log_sha256"`
	KATStatusHash     string `json:"status_hash"`
	UpdatedAt         string `json:"updated_at"`
}

var gjcKATStatusAllowedFields = map[string]bool{
	"schema_version":      true,
	"run_id":              true,
	"status":              true,
	"extractor_status":    true,
	"command_exit_code":   true,
	"exit_code":           true,
	"self_approval":       true,
	"final_accepted":      true,
	"review_approved":     true,
	"waiver_approved":     true,
	"candidate_evidence":  true,
	"source_status_hash":  true,
	"current_authority":   true,
	"completion_boundary": true,
	"command_id":          true,
	"lane":                true,
	"summary_path":        true,
	"summary_sha256":      true,
	"raw_log_path":        true,
	"raw_log_sha256":      true,
	"failure_signatures":  true,
	"warning_signatures":  true,
	"updated_at":          true,
	"status_hash":         true,
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

func AttachGJCKATEvidence(root Root, options GJCKATAttachOptions) (GJCStatusResult, error) {
	var result GJCStatusResult
	err := withProjectWriteLock(root, "gjc attach-kat-evidence", options.RunID, func() error {
		var err error
		result, err = attachGJCKATEvidenceUnlocked(root, options)
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

func attachGJCKATEvidenceUnlocked(root Root, options GJCKATAttachOptions) (GJCStatusResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	status, err := readValidatedGJCStatus(root, options.RunID)
	if err != nil {
		return GJCStatusResult{}, err
	}
	if status.CommandKind != gjcCommandUltragoal || status.Process.Status != "ultragoal_ready" {
		return GJCStatusResult{}, &Problem{Code: "gjc_kat_attachment_invalid_state", Message: "KAT evidence attachment requires ultragoal_ready candidate status", Hint: "Attach KAT evidence only after start-ultragoal records candidate implementation evidence.", Field: "process.status", Expected: "ultragoal_ready", Actual: status.Process.Status}
	}
	sourceStatusRef, err := writeGJCKATSourceStatusSnapshot(root, status)
	if err != nil {
		return GJCStatusResult{}, err
	}
	kat, err := parseGJCKATAttachOptions(root, status.RunID, status.StatusHash, options)
	if err != nil {
		return GJCStatusResult{}, err
	}
	kat.SourceStatusRef = sourceStatusRef
	status.KAT = &kat
	status.CurrentRequiredActor = GJCActorColor
	status.CurrentWaitReason = nil
	status.UpdatedAt = options.Now().UTC().Format(time.RFC3339)
	updated, _, err := writeGJCStatusMutation(root, status, gjcEventKATAttached)
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

func parseGJCKATAttachOptions(root Root, runID string, sourceStatusHash string, options GJCKATAttachOptions) (GJCKATEvidence, error) {
	statusRef, statusData, err := validateGJCKATOptionRef(root, runID, options.StatusPath, options.StatusHash, "kat.status_ref")
	if err != nil {
		return GJCKATEvidence{}, err
	}
	katStatus, err := parseGJCKATStatus(statusData)
	if err != nil {
		return GJCKATEvidence{}, err
	}
	if strings.TrimSpace(katStatus.RunID) != "" && katStatus.RunID != runID {
		return GJCKATEvidence{}, &Problem{Code: "gjc_kat_run_id_mismatch", Message: "KAT status run id does not match selected run", Hint: "Use KAT evidence emitted with kkachi-agent-tester --run-id <run_id> run.", Path: statusRef.Path, Field: "kat.status.run_id", Expected: runID, Actual: katStatus.RunID}
	}
	if err := validateGJCKATStatusShape(katStatus, statusRef.Path); err != nil {
		return GJCKATEvidence{}, err
	}
	options, err = normalizeGJCKATAttachOptions(root, runID, options, katStatus)
	if err != nil {
		return GJCKATEvidence{}, err
	}
	summaryRef, _, err := validateGJCKATOptionRef(root, runID, options.SummaryPath, options.SummaryHash, "kat.summary_ref")
	if err != nil {
		return GJCKATEvidence{}, err
	}
	summaryMDRef, _, err := validateGJCKATOptionRef(root, runID, options.SummaryMDPath, options.SummaryMDHash, "kat.summary_md_ref")
	if err != nil {
		return GJCKATEvidence{}, err
	}
	rawLogRef, _, err := validateGJCKATOptionRef(root, runID, options.RawLogPath, options.RawLogHash, "kat.raw_log_ref")
	if err != nil {
		return GJCKATEvidence{}, err
	}
	if isGJCKATV010Status(katStatus) && strings.TrimSpace(katStatus.SourceStatusHash) == "" {
		katStatus.SourceStatusHash = sourceStatusHash
	}
	if err := validateGJCKATSourceStatusHash(katStatus, sourceStatusHash, statusRef.Path); err != nil {
		return GJCKATEvidence{}, err
	}
	attachmentStatus := strings.TrimSpace(options.AttachmentStatus)
	if attachmentStatus == "" {
		attachmentStatus = gjcKATAttachmentReady
	}
	if !validGJCKATAttachmentStatus(attachmentStatus) {
		return GJCKATEvidence{}, &Problem{Code: "gjc_kat_attachment_status_unsupported", Message: "KAT attachment status is unsupported", Hint: "Use factual KAT evidence attachment states only; KAH cannot approve review, MAR, waiver, or final completion.", Field: "kat.attachment_status", Expected: "kat_evidence_ready or kat_evidence_failed", Actual: attachmentStatus}
	}
	return GJCKATEvidence{
		RunID:            runID,
		StatusRef:        statusRef,
		SummaryRef:       summaryRef,
		SummaryMDRef:     summaryMDRef,
		RawLogRef:        rawLogRef,
		StatusHash:       statusRef.SHA256,
		RawLogHash:       rawLogRef.SHA256,
		SourceStatusHash: katStatus.SourceStatusHash,
		ExtractorStatus:  katStatus.ExtractorStatus,
		CommandExitCode:  *katStatus.CommandExitCode,
		AttachmentStatus: attachmentStatus,
		UpdatedAt:        options.Now().UTC().Format(time.RFC3339),
	}, nil
}

func writeGJCKATSourceStatusSnapshot(root Root, status GJCStatus) (GJCArtifactRef, error) {
	path, err := gjcKATSourceStatusPath(root, status.RunID)
	if err != nil {
		return GJCArtifactRef{}, err
	}
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return GJCArtifactRef{}, &Problem{Code: "gjc_status_encode_failed", Message: "cannot encode KAT source status evidence", Hint: "Report the unsupported status payload to KAH maintainers.", Field: "kat.source_status_ref", Expected: "JSON-encodable pre-attachment GJC status", Actual: err.Error()}
	}
	data = append(data, '\n')
	if _, err := os.Stat(path.Absolute); os.IsNotExist(err) {
		if err := writeNewFileAtomically(path, data); err != nil {
			return GJCArtifactRef{}, err
		}
	} else if err != nil {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ref_inspection_failed", Message: "cannot inspect KAT source status evidence", Hint: "Check run-local evidence permissions before retrying.", Path: path.Relative, Field: "kat.source_status_ref", Expected: "inspectable source status evidence", Actual: err.Error()}
	} else if err := writeExistingFileAtomically(path, data); err != nil {
		return GJCArtifactRef{}, err
	}
	return GJCArtifactRef{Path: path.Relative, SHA256: sha256String(data)}, nil
}

func gjcKATSourceStatusPath(root Root, runID string) (SafePath, error) {
	return ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "gjc", "kat-source-status.json")))
}

func validateGJCKATOptionRef(root Root, runID string, pathValue string, hashValue string, field string) (GJCArtifactRef, []byte, error) {
	ref := GJCArtifactRef{Path: strings.TrimSpace(pathValue), SHA256: strings.TrimSpace(hashValue)}
	if !gjcChecksumPattern.MatchString(ref.SHA256) {
		return GJCArtifactRef{}, nil, &Problem{Code: "gjc_checksum_malformed", Message: "KAT evidence checksum is malformed", Hint: "Use sha256:<64 lowercase hex characters> for every KAT evidence ref.", Field: field + ".sha256", Expected: "sha256:<64hex>", Actual: ref.SHA256}
	}
	path, data, err := readGJCKATRegularRunRef(root, runID, ref.Path, field+".path")
	if err != nil {
		return GJCArtifactRef{}, nil, err
	}
	actual := sha256String(data)
	if actual != ref.SHA256 {
		return GJCArtifactRef{}, nil, &Problem{Code: "gjc_checksum_mismatch", Message: "KAT evidence checksum does not match file content", Hint: "Regenerate or reattach KAT evidence after content changes.", Path: path.Relative, Field: field + ".sha256", Expected: actual, Actual: ref.SHA256}
	}
	return GJCArtifactRef{Path: path.Relative, SHA256: ref.SHA256}, data, nil
}

func readGJCKATRegularRunRef(root Root, runID string, pathValue string, field string) (SafePath, []byte, error) {
	path, err := ResolveRelativePath(root, pathValue)
	if err != nil {
		return SafePath{}, nil, err
	}
	prefix := filepath.ToSlash(filepath.Join(RunRootPath, runID)) + "/"
	if path.Relative != strings.TrimSuffix(prefix, "/") && !strings.HasPrefix(path.Relative, prefix) {
		return SafePath{}, nil, &Problem{Code: "gjc_ref_cross_run", Message: "GJC reference must stay within the selected run root", Hint: "Use a repository-relative path under .kkachi/runs/<run_id>/.", Path: path.Relative, Field: field, Expected: prefix + "...", Actual: path.Relative}
	}
	info, err := os.Lstat(path.Absolute)
	if err != nil {
		return SafePath{}, nil, &Problem{Code: "gjc_ref_inspection_failed", Message: "cannot inspect KAT evidence ref", Hint: "Ensure KAT refs point to readable run-local regular files.", Path: path.Relative, Field: field, Expected: "inspectable KAT evidence file", Actual: err.Error()}
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return SafePath{}, nil, &Problem{Code: "gjc_ref_not_regular", Message: "KAT evidence ref is not a regular file", Hint: "Use direct run-local KAT artifact files; symlinks, directories, devices, and other non-regular refs are rejected.", Path: path.Relative, Field: field, Expected: "run-local regular file", Actual: info.Mode().String()}
	}
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return SafePath{}, nil, &Problem{Code: "gjc_artifact_read_failed", Message: "cannot read KAT evidence ref", Hint: "Ensure KAT refs point to readable run-local evidence files.", Path: path.Relative, Field: field, Expected: "readable KAT evidence file", Actual: err.Error()}
	}
	return path, data, nil
}

func normalizeGJCKATAttachOptions(root Root, runID string, options GJCKATAttachOptions, status gjcKATStatusEvidence) (GJCKATAttachOptions, error) {
	if !isGJCKATV010Status(status) {
		return options, nil
	}
	if strings.TrimSpace(options.SummaryPath) == "" {
		options.SummaryPath = status.SummaryPath
	}
	if strings.TrimSpace(options.SummaryHash) == "" {
		options.SummaryHash = status.SummarySHA256
	}
	if strings.TrimSpace(options.RawLogPath) == "" {
		options.RawLogPath = status.RawLogPath
	}
	if strings.TrimSpace(options.RawLogHash) == "" {
		options.RawLogHash = status.RawLogSHA256
	}
	if strings.TrimSpace(options.SummaryMDPath) == "" {
		options.SummaryMDPath = defaultGJCKATSummaryMDPath(options.SummaryPath)
	}
	if strings.TrimSpace(options.SummaryMDHash) == "" && strings.TrimSpace(options.SummaryMDPath) != "" {
		_, data, err := readGJCKATRegularRunRef(root, runID, options.SummaryMDPath, "kat.summary_md_ref.path")
		if err != nil {
			return options, err
		}
		options.SummaryMDHash = sha256String(data)
	}
	return options, nil
}

func defaultGJCKATSummaryMDPath(summaryPath string) string {
	summaryPath = strings.TrimSpace(summaryPath)
	if summaryPath == "" {
		return ""
	}
	if strings.HasSuffix(summaryPath, ".json") {
		return strings.TrimSuffix(summaryPath, ".json") + ".md"
	}
	return summaryPath + ".md"
}

func parseGJCKATStatus(data []byte) (gjcKATStatusEvidence, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return gjcKATStatusEvidence{}, &Problem{Code: "gjc_kat_status_invalid_json", Message: "KAT status evidence is not valid JSON", Hint: "Attach the KAT status JSON emitted under the selected run id.", Field: "kat.status_ref", Expected: "KAT status JSON object", Actual: err.Error()}
	}
	for key := range raw {
		if gjcKATStatusAllowedFields[key] {
			continue
		}
		if isGJCKATAuthorityClaimKey(key) {
			return gjcKATStatusEvidence{}, &Problem{Code: "gjc_kat_authority_claim", Message: "KAT status contains unsupported approval authority field", Hint: "KAT/GJC/KAH evidence is candidate or factual only; remove review, MAR, waiver, approval, accepted, or final claims.", Field: "kat.status." + key, Expected: "no approval, acceptance, waiver, MAR, or final claim fields", Actual: key}
		}
		return gjcKATStatusEvidence{}, &Problem{Code: "gjc_kat_status_invalid", Message: "KAT status contains unsupported field", Hint: "Attach only the GAJAE KAT status fields understood by KAH.", Field: "kat.status." + key, Expected: "supported KAT status field", Actual: key}
	}
	var status gjcKATStatusEvidence
	if err := json.Unmarshal(data, &status); err != nil {
		return status, &Problem{Code: "gjc_kat_status_invalid_json", Message: "KAT status evidence is not valid JSON", Hint: "Attach the KAT status JSON emitted under the selected run id.", Field: "kat.status_ref", Expected: "KAT status JSON object", Actual: err.Error()}
	}
	if status.CommandExitCode == nil && status.ExitCode != nil {
		status.CommandExitCode = status.ExitCode
	}
	return status, nil
}

func isGJCKATV010Status(status gjcKATStatusEvidence) bool {
	return strings.TrimSpace(status.SchemaVersion) == "" && (strings.TrimSpace(status.SummaryPath) != "" || strings.TrimSpace(status.RawLogPath) != "" || strings.TrimSpace(status.KATStatusHash) != "" || status.ExitCode != nil)
}

func validateGJCKATStatusShape(status gjcKATStatusEvidence, path string) error {
	isV010 := isGJCKATV010Status(status)
	if strings.TrimSpace(status.SchemaVersion) == "" && !isV010 {
		return &Problem{Code: "gjc_kat_status_invalid", Message: "KAT status schema version is missing", Hint: "Attach complete KAT status JSON evidence.", Path: path, Field: "kat.status.schema_version", Expected: "non-empty schema version", Actual: "missing"}
	}
	if !validGJCKATStatus(status.Status) {
		return &Problem{Code: "gjc_kat_status_unsupported", Message: "KAT status is unsupported", Hint: "Use factual KAT command states only; KAT cannot approve review, MAR, waiver, or final completion.", Path: path, Field: "kat.status.status", Expected: "passed, failed, or error", Actual: status.Status}
	}
	if !validGJCKATExtractorStatus(status.ExtractorStatus) {
		return &Problem{Code: "gjc_kat_extractor_status_unsupported", Message: "KAT extractor status is unsupported", Hint: "Use precise, partial, degraded, or no_match as factual extractor evidence.", Path: path, Field: "kat.status.extractor_status", Expected: "precise, partial, degraded, or no_match", Actual: status.ExtractorStatus}
	}
	if status.CommandExitCode == nil {
		return &Problem{Code: "gjc_kat_status_invalid", Message: "KAT status command exit code is missing", Hint: "Attach complete KAT status JSON evidence.", Path: path, Field: "kat.status.command_exit_code", Expected: "integer command exit code", Actual: "missing"}
	}
	if isV010 {
		if strings.TrimSpace(status.SummaryPath) == "" || strings.TrimSpace(status.RawLogPath) == "" {
			return &Problem{Code: "gjc_kat_status_invalid", Message: "KAT v0.1 status is missing artifact paths", Hint: "Attach KAT status JSON with summary_path and raw_log_path.", Path: path, Field: "kat.status.paths", Expected: "summary_path and raw_log_path", Actual: "missing"}
		}
		if !gjcChecksumPattern.MatchString(strings.TrimSpace(status.SummarySHA256)) {
			return &Problem{Code: "gjc_checksum_malformed", Message: "KAT summary checksum is malformed", Hint: "Use KAT v0.1 summary_sha256 sha256:<64 lowercase hex characters>.", Path: path, Field: "kat.status.summary_sha256", Expected: "sha256:<64hex>", Actual: status.SummarySHA256}
		}
		if !gjcChecksumPattern.MatchString(strings.TrimSpace(status.RawLogSHA256)) {
			return &Problem{Code: "gjc_checksum_malformed", Message: "KAT raw-log checksum is malformed", Hint: "Use KAT v0.1 raw_log_sha256 sha256:<64 lowercase hex characters>.", Path: path, Field: "kat.status.raw_log_sha256", Expected: "sha256:<64hex>", Actual: status.RawLogSHA256}
		}
		if strings.TrimSpace(status.KATStatusHash) != "" && !gjcChecksumPattern.MatchString(strings.TrimSpace(status.KATStatusHash)) {
			return &Problem{Code: "gjc_checksum_malformed", Message: "KAT status self-hash is malformed", Hint: "KAT status_hash is accepted only as factual KAT self-integrity metadata.", Path: path, Field: "kat.status.status_hash", Expected: "sha256:<64hex>", Actual: status.KATStatusHash}
		}
	}
	if status.SelfApproval || status.FinalAccepted || status.ReviewApproved || status.WaiverApproved {
		return &Problem{Code: "gjc_kat_authority_claim", Message: "KAT status claims unsupported approval authority", Hint: "KAT/GJC/KAH evidence is candidate or factual only; KAS/Blue/color/MAR/final gates decide acceptance.", Path: path, Field: "kat.status.authority", Expected: "no self-approval, review approval, waiver, or final acceptance claims", Actual: "approval claim present"}
	}
	return nil
}

func validateGJCKATSourceStatusHash(status gjcKATStatusEvidence, expectedHash string, path string) error {
	sourceHash := strings.TrimSpace(status.SourceStatusHash)
	if sourceHash == "" {
		return &Problem{Code: "gjc_kat_status_invalid", Message: "KAT status source status hash is missing", Hint: "KAT evidence must bind to the current pre-attachment GJC status hash.", Path: path, Field: "kat.status.source_status_hash", Expected: "sha256:<64hex>", Actual: "missing"}
	}
	if !gjcChecksumPattern.MatchString(sourceHash) {
		return &Problem{Code: "gjc_checksum_malformed", Message: "KAT status source status hash is malformed", Hint: "Use sha256:<64 lowercase hex characters> from the pre-attachment GJC status.", Path: path, Field: "kat.status.source_status_hash", Expected: "sha256:<64hex>", Actual: sourceHash}
	}
	if sourceHash != expectedHash {
		return &Problem{Code: "gjc_kat_source_status_hash_mismatch", Message: "KAT status source status hash does not match bound GJC status", Hint: "Attach KAT evidence generated from the current pre-attachment GJC status evidence.", Path: path, Field: "kat.status.source_status_hash", Expected: expectedHash, Actual: sourceHash}
	}
	return nil
}

func validGJCKATStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "passed", "failed", "error":
		return true
	default:
		return false
	}
}

func validGJCKATExtractorStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case "precise", "partial", "degraded", "no_match":
		return true
	default:
		return false
	}
}

func validGJCKATAttachmentStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case gjcKATAttachmentReady, gjcKATAttachmentFailed:
		return true
	default:
		return false
	}
}

func isGJCKATAuthorityClaimKey(key string) bool {
	normalized := strings.ToLower(strings.TrimSpace(key))
	for _, token := range []string{"approval", "approved", "accepted", "acceptance", "final", "waiver", "mar"} {
		if strings.Contains(normalized, token) {
			return true
		}
	}
	return false
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
	notificationStatus := gjcNotificationMetadataOnly
	if notificationRef == "" || notificationRef == gjcNoWakeClaimNotification {
		notificationRef = gjcNoWakeClaimNotification
		notificationStatus = gjcNotificationNoWakeClaim
	}
	return GJCCallback{
		TaskID:              taskID,
		Status:              callbackStatus,
		IdempotencyKey:      idempotencyKey,
		SourceStatusHash:    sourceHash,
		LastCallbackStatus:  result,
		NotificationRef:     notificationRef,
		NotificationStatus:  notificationStatus,
		WakeEvidenceStatus:  gjcWakeEvidenceMissing,
		LastNotifiedHash:    sourceHash,
		SameThreadWakeClaim: false,
		UpdatedAt:           options.Now().UTC().Format(time.RFC3339),
	}, nil
}

func validGJCNotificationStatus(status string) bool {
	switch status {
	case gjcNotificationNoWakeClaim, gjcNotificationMetadataOnly:
		return true
	default:
		return false
	}
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
	if !validGJCNotificationStatus(callback.NotificationStatus) {
		return &Problem{Code: "gjc_callback_notification_status_unsupported", Message: "GJC callback notification status is unsupported", Hint: "Use no_wake_claim or metadata_recorded_no_wake_claim; callback metadata is not wake readiness.", Field: "callback.notification_status", Expected: "no_wake_claim or metadata_recorded_no_wake_claim", Actual: callback.NotificationStatus}
	}
	if strings.TrimSpace(callback.WakeEvidenceStatus) != gjcWakeEvidenceMissing {
		return &Problem{Code: "gjc_callback_wake_evidence_unsupported", Message: "GJC callback wake evidence status is unsupported", Hint: "Same-thread wake readiness remains no-wake-claim until explicit watcher/origin evidence support is implemented and verified.", Field: "callback.wake_evidence_status", Expected: gjcWakeEvidenceMissing, Actual: callback.WakeEvidenceStatus}
	}
	if callback.SameThreadWakeClaim {
		return &Problem{Code: "gjc_callback_wake_claim_unsupported", Message: "GJC callback cannot claim same-thread wake readiness", Hint: "Record notification metadata only; KAS/Blue may claim wake readiness only after explicit watcher/origin evidence exists.", Field: "callback.same_thread_wake_claim", Expected: "false", Actual: "true"}
	}
	return nil
}

func validateGJCKATEvidence(root Root, runID string, evidence *GJCKATEvidence) error {
	if evidence == nil {
		return nil
	}
	if evidence.RunID != runID {
		return &Problem{Code: "gjc_kat_run_id_mismatch", Message: "persisted KAT evidence run id does not match selected run", Hint: "Do not copy KAT evidence across runs.", Field: "kat.run_id", Expected: runID, Actual: evidence.RunID}
	}
	sourceStatusHash, err := validateGJCKATSourceStatusRef(root, runID, evidence.SourceStatusRef, evidence.SourceStatusHash)
	if err != nil {
		return err
	}
	statusRef, statusData, err := validateGJCKATOptionRef(root, runID, evidence.StatusRef.Path, evidence.StatusRef.SHA256, "kat.status_ref")
	if err != nil {
		return err
	}
	summaryRef, _, err := validateGJCKATOptionRef(root, runID, evidence.SummaryRef.Path, evidence.SummaryRef.SHA256, "kat.summary_ref")
	if err != nil {
		return err
	}
	summaryMDRef, _, err := validateGJCKATOptionRef(root, runID, evidence.SummaryMDRef.Path, evidence.SummaryMDRef.SHA256, "kat.summary_md_ref")
	if err != nil {
		return err
	}
	rawLogRef, _, err := validateGJCKATOptionRef(root, runID, evidence.RawLogRef.Path, evidence.RawLogRef.SHA256, "kat.raw_log_ref")
	if err != nil {
		return err
	}
	katStatus, err := parseGJCKATStatus(statusData)
	if err != nil {
		return err
	}
	if strings.TrimSpace(katStatus.RunID) != "" && katStatus.RunID != runID {
		return &Problem{Code: "gjc_kat_run_id_mismatch", Message: "persisted KAT status run id does not match selected run", Hint: "Do not copy KAT evidence across runs.", Path: statusRef.Path, Field: "kat.status.run_id", Expected: runID, Actual: katStatus.RunID}
	}
	if err := validateGJCKATStatusShape(katStatus, statusRef.Path); err != nil {
		return err
	}
	if evidence.StatusHash != evidence.StatusRef.SHA256 {
		return &Problem{Code: "gjc_checksum_mismatch", Message: "persisted KAT status hash does not match status ref", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Field: "kat.status_hash", Expected: evidence.StatusRef.SHA256, Actual: evidence.StatusHash}
	}
	if evidence.RawLogHash != evidence.RawLogRef.SHA256 {
		return &Problem{Code: "gjc_checksum_mismatch", Message: "persisted KAT raw-log hash does not match raw-log ref", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Field: "kat.raw_log_hash", Expected: evidence.RawLogRef.SHA256, Actual: evidence.RawLogHash}
	}
	if !gjcChecksumPattern.MatchString(evidence.SourceStatusHash) {
		return &Problem{Code: "gjc_checksum_malformed", Message: "persisted KAT source status hash is malformed", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Field: "kat.source_status_hash", Expected: "sha256:<64hex>", Actual: evidence.SourceStatusHash}
	}
	if evidence.SourceStatusHash != sourceStatusHash {
		return &Problem{Code: "gjc_kat_source_status_ref_mismatch", Message: "persisted KAT source status hash does not match source status evidence", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Field: "kat.source_status_hash", Expected: sourceStatusHash, Actual: evidence.SourceStatusHash}
	}
	if isGJCKATV010Status(katStatus) && strings.TrimSpace(katStatus.SourceStatusHash) == "" {
		katStatus.SourceStatusHash = evidence.SourceStatusHash
	}
	if err := validateGJCKATSourceStatusHash(katStatus, evidence.SourceStatusHash, statusRef.Path); err != nil {
		return err
	}
	if evidence.StatusRef != statusRef || evidence.SummaryRef != summaryRef || evidence.SummaryMDRef != summaryMDRef || evidence.RawLogRef != rawLogRef {
		return &Problem{Code: "gjc_checksum_mismatch", Message: "persisted KAT refs do not match normalized run-local evidence", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Field: "kat.refs", Expected: "normalized run-local refs", Actual: "stale refs"}
	}
	if evidence.ExtractorStatus != katStatus.ExtractorStatus {
		return &Problem{Code: "gjc_kat_status_invalid", Message: "persisted KAT extractor status does not match status_ref content", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Path: statusRef.Path, Field: "kat.extractor_status", Expected: katStatus.ExtractorStatus, Actual: evidence.ExtractorStatus}
	}
	if evidence.CommandExitCode != *katStatus.CommandExitCode {
		return &Problem{Code: "gjc_kat_status_invalid", Message: "persisted KAT command exit code does not match status_ref content", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Path: statusRef.Path, Field: "kat.command_exit_code", Expected: fmt.Sprintf("%d", *katStatus.CommandExitCode), Actual: fmt.Sprintf("%d", evidence.CommandExitCode)}
	}
	if !validGJCKATExtractorStatus(evidence.ExtractorStatus) {
		return &Problem{Code: "gjc_kat_extractor_status_unsupported", Message: "persisted KAT extractor status is unsupported", Hint: "Use precise, partial, degraded, or no_match as factual extractor evidence.", Field: "kat.extractor_status", Expected: "precise, partial, degraded, or no_match", Actual: evidence.ExtractorStatus}
	}
	if !validGJCKATAttachmentStatus(evidence.AttachmentStatus) {
		return &Problem{Code: "gjc_kat_attachment_status_unsupported", Message: "persisted KAT attachment status is unsupported", Hint: "Use factual KAT evidence attachment states only.", Field: "kat.attachment_status", Expected: "kat_evidence_ready or kat_evidence_failed", Actual: evidence.AttachmentStatus}
	}
	return nil
}

func validateGJCKATSourceStatusRef(root Root, runID string, ref GJCArtifactRef, expectedHash string) (string, error) {
	expectedPath := filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "gjc", "kat-source-status.json"))
	if strings.TrimSpace(ref.Path) != expectedPath {
		return "", &Problem{Code: "gjc_kat_source_status_ref_mismatch", Message: "KAT source status ref is not the canonical pre-attachment source evidence", Hint: "Regenerate KAT attachment evidence through the KAH wrapper; do not rebind source_status_ref to another run-local artifact.", Field: "kat.source_status_ref.path", Expected: expectedPath, Actual: ref.Path}
	}
	sourceRef, sourceData, err := validateGJCKATOptionRef(root, runID, ref.Path, ref.SHA256, "kat.source_status_ref")
	if err != nil {
		return "", err
	}
	var sourceStatus GJCStatus
	if err := json.Unmarshal(sourceData, &sourceStatus); err != nil {
		return "", &Problem{Code: "gjc_kat_source_status_invalid_json", Message: "KAT source status evidence is not valid GJC status JSON", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Path: sourceRef.Path, Field: "kat.source_status_ref", Expected: "pre-attachment GJC status JSON", Actual: err.Error()}
	}
	if sourceStatus.RunID != runID {
		return "", &Problem{Code: "gjc_kat_run_id_mismatch", Message: "KAT source status run id does not match selected run", Hint: "Do not copy KAT source evidence across runs.", Path: sourceRef.Path, Field: "kat.source_status_ref.run_id", Expected: runID, Actual: sourceStatus.RunID}
	}
	if sourceStatus.CommandKind != gjcCommandUltragoal || sourceStatus.Process.Status != "ultragoal_ready" {
		return "", &Problem{Code: "gjc_kat_source_status_ref_mismatch", Message: "KAT source status evidence is not the ultragoal_ready pre-attachment status", Hint: "Regenerate KAT attachment evidence after start-ultragoal records candidate status.", Path: sourceRef.Path, Field: "kat.source_status_ref.process.status", Expected: "ultragoal_ready", Actual: sourceStatus.Process.Status}
	}
	if sourceStatus.KAT != nil {
		return "", &Problem{Code: "gjc_kat_source_status_ref_mismatch", Message: "KAT source status evidence already contains KAT attachment data", Hint: "Regenerate KAT attachment evidence with the pre-attachment source status.", Path: sourceRef.Path, Field: "kat.source_status_ref.kat", Expected: "absent pre-attachment KAT section", Actual: "present"}
	}
	if !gjcChecksumPattern.MatchString(sourceStatus.StatusHash) {
		return "", &Problem{Code: "gjc_checksum_malformed", Message: "KAT source status hash is malformed", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Path: sourceRef.Path, Field: "kat.source_status_ref.status_hash", Expected: "sha256:<64hex>", Actual: sourceStatus.StatusHash}
	}
	computed, err := computeGJCStatusHash(sourceStatus)
	if err != nil {
		return "", err
	}
	if sourceStatus.StatusHash != computed {
		return "", &Problem{Code: "gjc_kat_source_status_ref_mismatch", Message: "KAT source status hash does not match source status content", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Path: sourceRef.Path, Field: "kat.source_status_ref.status_hash", Expected: computed, Actual: sourceStatus.StatusHash}
	}
	if expectedHash != "" && sourceStatus.StatusHash != expectedHash {
		return "", &Problem{Code: "gjc_kat_source_status_ref_mismatch", Message: "KAT source status evidence does not match persisted source status hash", Hint: "Regenerate KAT attachment evidence through the KAH wrapper.", Path: sourceRef.Path, Field: "kat.source_status_ref.status_hash", Expected: expectedHash, Actual: sourceStatus.StatusHash}
	}
	return sourceStatus.StatusHash, nil
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

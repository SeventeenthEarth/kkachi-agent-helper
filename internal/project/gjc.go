package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const (
	GJCSchemaVersion    = "kah.gajae_gjc_delegation.v1"
	GJCDefaultRealHome  = "/Users/draccoon"
	GJCStatusRunning    = "running"
	GJCStatusBlocked    = "blocked"
	GJCStatusFailed     = "failed"
	GJCStatusCancelled  = "cancelled"
	GJCActorGJC         = "gjc"
	GJCActorKAS         = "kas"
	GJCActorColor       = "color"
	GJCActorMAR         = "mar"
	GJCActorUser        = "user"
	GJCActorKAT         = "kat"
	GJCActorNone        = "none"
	gjcSessionFileName  = "session.json"
	gjcStatusFileName   = "status.json"
	gjcReceiptFileName  = "receipt.json"
	gjcEventStarted     = "gjc.started"
	gjcEventFailed      = "gjc.failed"
	gjcEventCallback    = "gjc.callback_recorded"
	gjcEventPlanLocked  = "gjc.plan_locked"
	gjcEventKATAttached = "gjc.kat_attached"
	gjcCommandDeep      = "start-deep-interview"
	gjcCommandRalplan   = "start-ralplan"
	gjcCommandUltragoal = "start-ultragoal"
)

var (
	gjcSessionIDPattern   = regexp.MustCompile(`^gjc-run-\d{8}T\d{6}Z-[0-9a-f]{12}$`)
	gjcChecksumPattern    = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
	gjcNativeStagePattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_.-]*$`)
	gjcRunCommand         = defaultGJCRunner
)

type GJCStartOptions struct {
	RunID        string
	TaskID       string
	Packet       string
	CommandKind  string
	RealUserHome string
	GJCCommand   string
	Now          func() time.Time
}

type GJCStatusOptions struct {
	RunID string
}

type GJCStartResult struct {
	Status  GJCStatus `json:"status"`
	EventID string    `json:"event_id,omitempty"`
}

type GJCStatusResult struct {
	Status GJCStatus `json:"status"`
}

type GJCStatus struct {
	SchemaVersion        string           `json:"schema_version"`
	RunID                string           `json:"run_id"`
	TaskID               string           `json:"task_id"`
	CommandKind          string           `json:"command_kind"`
	RealUserHome         string           `json:"real_user_home"`
	GJCSessionID         string           `json:"gjc_session_id"`
	Process              GJCProcessStatus `json:"process"`
	Packet               GJCArtifactRef   `json:"packet_ref"`
	NativeInput          *GJCArtifactRef  `json:"native_input_ref,omitempty"`
	Receipt              *GJCArtifactRef  `json:"receipt_ref,omitempty"`
	Artifacts            []GJCArtifactRef `json:"artifact_refs"`
	Plan                 GJCPlanEvidence  `json:"plan,omitempty"`
	KAT                  *GJCKATEvidence  `json:"kat,omitempty"`
	Callback             *GJCCallback     `json:"callback,omitempty"`
	CurrentRequiredActor string           `json:"current_required_actor"`
	CurrentWaitReason    *string          `json:"current_wait_reason"`
	StatusPath           string           `json:"status_path"`
	StatusHash           string           `json:"status_hash"`
	Error                *GJCStatusError  `json:"error,omitempty"`
	RecoveryHint         string           `json:"recovery_hint,omitempty"`
	UpdatedAt            string           `json:"updated_at"`
}

type GJCProcessStatus struct {
	Status   string `json:"status"`
	PID      int    `json:"pid,omitempty"`
	ExitCode int    `json:"exit_code"`
}

type GJCArtifactRef struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
}

type GJCPlanEvidence struct {
	Artifact           string `json:"artifact,omitempty"`
	ArtifactHash       string `json:"artifact_hash,omitempty"`
	LockStatus         string `json:"lock_status,omitempty"`
	AcceptedPlanHash   string `json:"accepted_plan_hash,omitempty"`
	ApprovalRef        string `json:"approval_ref,omitempty"`
	ConflictReportPath string `json:"conflict_report_path,omitempty"`
}

type GJCKATEvidence struct {
	RunID            string         `json:"run_id"`
	StatusRef        GJCArtifactRef `json:"status_ref"`
	SourceStatusRef  GJCArtifactRef `json:"source_status_ref"`
	SummaryRef       GJCArtifactRef `json:"summary_ref"`
	SummaryMDRef     GJCArtifactRef `json:"summary_md_ref"`
	RawLogRef        GJCArtifactRef `json:"raw_log_ref"`
	StatusHash       string         `json:"status_hash"`
	RawLogHash       string         `json:"raw_log_hash"`
	SourceStatusHash string         `json:"source_status_hash"`
	ExtractorStatus  string         `json:"extractor_status"`
	CommandExitCode  int            `json:"command_exit_code"`
	AttachmentStatus string         `json:"attachment_status"`
	UpdatedAt        string         `json:"updated_at"`
}

type GJCCallback struct {
	TaskID              string `json:"task_id"`
	Status              string `json:"status"`
	IdempotencyKey      string `json:"idempotency_key"`
	SourceStatusHash    string `json:"source_status_hash"`
	LastCallbackStatus  string `json:"last_callback_status"`
	NotificationRef     string `json:"notification_ref"`
	NotificationStatus  string `json:"notification_status"`
	WakeEvidenceStatus  string `json:"wake_evidence_status"`
	LastNotifiedHash    string `json:"last_notified_hash"`
	SameThreadWakeClaim bool   `json:"same_thread_wake_claim"`
	UpdatedAt           string `json:"updated_at"`
}

type GJCStatusError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type gjcSession struct {
	SchemaVersion string `json:"schema_version"`
	RunID         string `json:"run_id"`
	SessionID     string `json:"gjc_session_id"`
	CreatedAt     string `json:"created_at"`
}

type gjcRunnerInvocation struct {
	Command      string
	Args         []string
	Dir          string
	Env          []string
	RealUserHome string
	SessionID    string
}

type gjcRunnerResult struct {
	PID      int
	ExitCode int
	Stdout   []byte
	Stderr   []byte
}

type gjcNativeRalplanInput struct {
	Stage    string
	StageN   string
	Artifact SafePath
}

type gjcNativeUltragoalInput struct {
	Mode  string
	Brief string
}

type gjcCommandInput struct {
	Args        []string
	NativeInput *GJCArtifactRef
}

type gjcReceipt struct {
	Status               string           `json:"status"`
	Artifacts            []GJCArtifactRef `json:"artifact_refs"`
	ArtifactRefs         []GJCArtifactRef `json:"artifacts"`
	CurrentRequiredActor string           `json:"current_required_actor"`
	CurrentWaitReason    *string          `json:"current_wait_reason"`
	RunID                string           `json:"run_id"`
	Path                 string           `json:"path"`
	Stage                string           `json:"stage"`
	StageN               any              `json:"stage_n"`
	SHA256               string           `json:"sha256"`
	CreatedAt            string           `json:"created_at"`
	Deduplicated         bool             `json:"deduplicated"`
	OK                   bool             `json:"ok"`
	GoalsCount           int              `json:"goals_count"`
	GoalIDs              []string         `json:"goal_ids"`
	GoalsPath            string           `json:"goals_path"`
}

func StartGJC(root Root, options GJCStartOptions) (GJCStartResult, error) {
	var result GJCStartResult
	err := withProjectWriteLock(root, "gjc "+options.CommandKind, options.RunID, func() error {
		var err error
		result, err = startGJCUnlocked(root, options)
		return err
	})
	return result, err
}

func ShowGJCStatus(root Root, options GJCStatusOptions) (GJCStatusResult, error) {
	metadata, _, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return GJCStatusResult{}, err
	}
	statusPath, err := gjcStatusPath(root, metadata.RunID)
	if err != nil {
		return GJCStatusResult{}, err
	}
	data, err := os.ReadFile(statusPath.Absolute)
	if os.IsNotExist(err) {
		return GJCStatusResult{}, &Problem{Code: "gjc_status_missing", Message: "GJC status ledger is missing", Hint: "Run gjc start-deep-interview, start-ralplan, or start-ultragoal before reading status.", Path: statusPath.Relative, Field: "status_path", Expected: "existing run-local GJC status", Actual: "missing"}
	}
	if err != nil {
		return GJCStatusResult{}, &Problem{Code: "gjc_status_read_failed", Message: "cannot read GJC status ledger", Hint: "Check run-local GJC evidence permissions before retrying.", Path: statusPath.Relative, Field: "status_path", Expected: "readable status JSON", Actual: err.Error()}
	}
	var status GJCStatus
	if err := json.Unmarshal(data, &status); err != nil {
		return GJCStatusResult{}, &Problem{Code: "gjc_status_invalid_json", Message: "GJC status ledger is not valid JSON", Hint: "Regenerate the GJC status evidence from a valid wrapper start.", Path: statusPath.Relative, Field: "json", Expected: "GJC status JSON object", Actual: err.Error()}
	}
	if err := validatePersistedGJCStatus(root, metadata, status, statusPath.Relative); err != nil {
		return GJCStatusResult{}, err
	}
	return GJCStatusResult{Status: status}, nil
}

func startGJCUnlocked(root Root, options GJCStartOptions) (GJCStartResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	metadata, _, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return GJCStartResult{}, err
	}
	if strings.TrimSpace(options.TaskID) == "" {
		if metadata.TaskID == nil || strings.TrimSpace(*metadata.TaskID) == "" {
			return GJCStartResult{}, &Problem{Code: "gjc_task_required", Message: "GJC wrapper requires a task id", Hint: "Pass --task with the Kkachi task id supplied by KAS.", Field: "task_id", Expected: "non-empty task id", Actual: "missing"}
		}
		options.TaskID = strings.TrimSpace(*metadata.TaskID)
	}
	commandKind, err := normalizeGJCCommandKind(options.CommandKind)
	if err != nil {
		return GJCStartResult{}, err
	}
	realHome, err := normalizeGJCRealHome(options.RealUserHome)
	if err != nil {
		return GJCStartResult{}, err
	}
	packet, err := validateGJCRunRef(root, metadata.RunID, options.Packet, "packet")
	if err != nil {
		return GJCStartResult{}, err
	}
	packetData, err := os.ReadFile(packet.Absolute)
	if err != nil {
		return GJCStartResult{}, &Problem{Code: "gjc_artifact_read_failed", Message: "cannot read GJC packet_ref", Hint: "Regenerate or repair the run-local KAS packet before starting GJC.", Path: packet.Relative, Field: "packet_ref.path", Expected: "readable packet file", Actual: err.Error()}
	}
	packetRef := GJCArtifactRef{Path: packet.Relative, SHA256: sha256String(packetData)}
	commandInput, err := gjcArgsForCommand(root, metadata.RunID, commandKind, packet.Absolute, packetData)
	if err != nil {
		return GJCStartResult{}, err
	}
	session, err := loadOrCreateGJCSession(root, metadata.RunID, options.Now)
	if err != nil {
		return GJCStartResult{}, err
	}
	gjcCommand := strings.TrimSpace(options.GJCCommand)
	if gjcCommand == "" {
		gjcCommand = "gjc"
	}
	invocation := gjcRunnerInvocation{
		Command:      gjcCommand,
		Args:         commandInput.Args,
		Dir:          root.Path,
		Env:          gjcEnv(realHome, session.SessionID),
		RealUserHome: realHome,
		SessionID:    session.SessionID,
	}
	runnerResult, runErr := gjcRunCommand(invocation)
	occurredAt := options.Now().UTC().Format(time.RFC3339)
	status := GJCStatus{
		SchemaVersion:        GJCSchemaVersion,
		RunID:                metadata.RunID,
		TaskID:               strings.TrimSpace(options.TaskID),
		CommandKind:          commandKind,
		RealUserHome:         realHome,
		GJCSessionID:         session.SessionID,
		Process:              GJCProcessStatus{Status: GJCStatusRunning, PID: runnerResult.PID, ExitCode: runnerResult.ExitCode},
		Packet:               packetRef,
		NativeInput:          commandInput.NativeInput,
		CurrentRequiredActor: GJCActorGJC,
		StatusPath:           mustGJCStatusRelative(metadata.RunID),
		UpdatedAt:            occurredAt,
	}
	if runErr != nil {
		status.Process.Status = GJCStatusFailed
		status.Error = &GJCStatusError{Code: problemCode(runErr, "gjc_command_failed"), Message: problemMessage(runErr)}
		status.RecoveryHint = "Ensure gjc is installed and callable without mutating profile, provider, auth, token, gateway, or model settings."
		return writeGJCStatusAndEvent(root, status, gjcEventFailed, runErr)
	}
	if runnerResult.ExitCode != 0 {
		status.Process.Status = GJCStatusFailed
		status.Error = &GJCStatusError{Code: "gjc_command_nonzero", Message: firstNonEmptyLine(runnerResult.Stderr, "GJC command exited non-zero")}
		status.RecoveryHint = "Inspect run-local GJC status and stderr evidence, then rerun after fixing the GJC-side failure."
		return writeGJCStatusAndEvent(root, status, gjcEventFailed, &Problem{Code: "gjc_command_nonzero", Message: "GJC command exited non-zero", Hint: status.RecoveryHint, Field: "exit_code", Expected: "0", Actual: fmt.Sprintf("%d", runnerResult.ExitCode)})
	}
	receipt, err := parseGJCReceipt(root, metadata.RunID, commandKind, runnerResult.Stdout)
	if err != nil {
		status.Process.Status = GJCStatusFailed
		status.Error = &GJCStatusError{Code: problemCode(err, "gjc_json_invalid"), Message: problemMessage(err)}
		status.RecoveryHint = "Configure GJC to emit the GAJAE MVP JSON receipt with status, artifact_refs, current_required_actor, and run-local sha256 hashes."
		return writeGJCStatusAndEvent(root, status, gjcEventFailed, err)
	}
	if err := applyGJCReceipt(root, metadata.RunID, commandKind, &status, receipt); err != nil {
		status.Process.Status = GJCStatusFailed
		status.Error = &GJCStatusError{Code: problemCode(err, "gjc_receipt_invalid"), Message: problemMessage(err)}
		status.RecoveryHint = "Regenerate the GJC receipt with supported candidate status and existing run-local artifact refs plus matching sha256 hashes."
		return writeGJCStatusAndEvent(root, status, gjcEventFailed, err)
	}
	receiptRef, err := writeGJCReceipt(root, metadata.RunID, runnerResult.Stdout)
	if err != nil {
		status.Process.Status = GJCStatusFailed
		status.Error = &GJCStatusError{Code: problemCode(err, "gjc_receipt_write_failed"), Message: problemMessage(err)}
		status.RecoveryHint = "Check run-local GJC receipt evidence permissions before retrying."
		return writeGJCStatusAndEvent(root, status, gjcEventFailed, err)
	}
	status.Receipt = &receiptRef
	return writeGJCStatusAndEvent(root, status, gjcEventStarted, nil)
}

func writeGJCStatusAndEvent(root Root, status GJCStatus, eventType string, returnedErr error) (GJCStartResult, error) {
	statusPath, err := gjcStatusPath(root, status.RunID)
	if err != nil {
		return GJCStartResult{}, err
	}
	status.StatusPath = statusPath.Relative
	statusHash, err := computeGJCStatusHash(status)
	if err != nil {
		return GJCStartResult{}, err
	}
	status.StatusHash = statusHash
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return GJCStartResult{}, &Problem{Code: "gjc_status_encode_failed", Message: "cannot encode GJC status ledger", Hint: "Report the unsupported status payload to KAH maintainers.", Field: "status", Expected: "JSON-encodable status", Actual: err.Error()}
	}
	data = append(data, '\n')
	appendResult, err := appendEventWithPreparedStatusMutation(root, AppendEventOptions{Type: eventType, RunID: status.RunID, Payload: gjcEventPayload(status), Now: timestampFunc(status.UpdatedAt)}, func(_ map[string]any, _ string, _ string) (preparedEventStatusMutation, error) {
		return preparedEventStatusMutation{Payload: gjcEventPayload(status), BeforeAppend: func() error { return writeExistingFileAtomically(statusPath, data) }}, nil
	})
	if err != nil {
		return GJCStartResult{}, err
	}
	result := GJCStartResult{Status: status, EventID: appendResult.EventID}
	if returnedErr != nil {
		return result, returnedErr
	}
	return result, nil
}

func applyGJCReceipt(root Root, runID string, commandKind string, status *GJCStatus, receipt gjcReceipt) error {
	candidateStatus := strings.TrimSpace(receipt.Status)
	if !allowedGJCStatusForCommand(commandKind, candidateStatus) {
		return &Problem{Code: "gjc_status_unsupported", Message: "GJC receipt status is not supported for this command", Hint: "Use GAJAE candidate statuses only; do not emit plan/review/MAR/final acceptance from KAH.", Field: "status", Expected: expectedGJCStatuses(commandKind), Actual: candidateStatus}
	}
	artifacts := receipt.Artifacts
	if len(artifacts) == 0 {
		artifacts = receipt.ArtifactRefs
	}
	if len(artifacts) == 0 {
		return &Problem{Code: "gjc_artifact_refs_missing", Message: "GJC receipt lacks artifact refs", Hint: "Emit at least one run-local artifact ref with path and sha256 hash.", Field: "artifact_refs", Expected: "one or more run-local artifact refs", Actual: "missing"}
	}
	for i, ref := range artifacts {
		if err := validateGJCArtifactRef(root, runID, ref, fmt.Sprintf("artifact_refs[%d]", i)); err != nil {
			return err
		}
	}
	actor := strings.TrimSpace(receipt.CurrentRequiredActor)
	if actor == "" {
		actor = defaultGJCRequiredActor(candidateStatus)
	}
	if !validGJCActor(actor) {
		return &Problem{Code: "gjc_required_actor_unsupported", Message: "GJC receipt required actor is not supported", Hint: "Use gjc, kas, color, mar, user, kat, or none.", Field: "current_required_actor", Expected: "supported GAJAE actor", Actual: actor}
	}
	status.Process.Status = candidateStatus
	status.Artifacts = artifacts
	status.CurrentRequiredActor = actor
	status.CurrentWaitReason = receipt.CurrentWaitReason
	status.Error = nil
	status.RecoveryHint = ""
	return applyGJCPlanEvidence(root, runID, commandKind, status)
}

func parseGJCReceipt(root Root, runID string, commandKind string, stdout []byte) (gjcReceipt, error) {
	var receipt gjcReceipt
	trimmed := strings.TrimSpace(string(stdout))
	if trimmed == "" {
		return receipt, &Problem{Code: "gjc_json_missing", Message: "GJC command did not emit a JSON receipt", Hint: "Configure GJC to emit the GAJAE MVP JSON receipt on stdout.", Field: "stdout", Expected: "JSON receipt", Actual: "empty"}
	}
	if err := json.Unmarshal([]byte(trimmed), &receipt); err != nil {
		return receipt, &Problem{Code: "gjc_json_invalid", Message: "GJC receipt is not valid JSON", Hint: "Configure GJC to emit a compact JSON object on stdout.", Field: "stdout", Expected: "JSON receipt object", Actual: err.Error()}
	}
	if commandKind == gjcCommandRalplan && strings.TrimSpace(receipt.Status) == "" {
		return adaptNativeRalplanReceipt(root, runID, receipt)
	}
	if commandKind == gjcCommandUltragoal && strings.TrimSpace(receipt.Status) == "" {
		return adaptNativeUltragoalReceipt(root, runID, receipt)
	}
	return receipt, nil
}

func adaptNativeRalplanReceipt(root Root, runID string, receipt gjcReceipt) (gjcReceipt, error) {
	if strings.TrimSpace(receipt.RunID) == "" || strings.TrimSpace(receipt.Path) == "" || strings.TrimSpace(receipt.SHA256) == "" {
		return receipt, &Problem{Code: "gjc_ralplan_native_output_invalid", Message: "native ralplan output is missing required fields", Hint: "Use GJC native JSON output with run_id, path, and sha256 fields from `gjc ralplan --write --json`.", Field: "native_ralplan_output", Expected: "run_id, path, and sha256", Actual: "missing required field"}
	}
	expectedRunID := "gjc-" + runID
	if strings.TrimSpace(receipt.RunID) != expectedRunID {
		return receipt, &Problem{Code: "gjc_ralplan_native_output_invalid", Message: "native ralplan output run id does not match the selected GJC session", Hint: "Do not consume cross-session GJC ralplan artifacts.", Field: "run_id", Expected: expectedRunID, Actual: receipt.RunID}
	}
	planRef, err := copyGJCRalplanNativeArtifact(root, runID, receipt.Path, receipt.SHA256)
	if err != nil {
		return receipt, err
	}
	waitReason := "plan_review_required"
	receipt.Status = "ralplan_ready"
	receipt.Artifacts = []GJCArtifactRef{planRef}
	receipt.CurrentRequiredActor = GJCActorKAS
	receipt.CurrentWaitReason = &waitReason
	return receipt, nil
}

func copyGJCRalplanNativeArtifact(root Root, runID string, source string, declaredHash string) (GJCArtifactRef, error) {
	normalizedHash, err := normalizeGJCRalplanNativeHash(declaredHash)
	if err != nil {
		return GJCArtifactRef{}, err
	}
	data, cleanedSource, err := readGJCNativeSessionFile(root, runID, source, "ralplan")
	if err != nil {
		return GJCArtifactRef{}, err
	}
	actualHash := sha256String(data)
	if actualHash != normalizedHash {
		return GJCArtifactRef{}, &Problem{Code: "gjc_checksum_mismatch", Message: "native ralplan output checksum does not match file content", Hint: "Do not consume stale or tampered GJC ralplan artifacts.", Path: cleanedSource, Field: "sha256", Expected: actualHash, Actual: normalizedHash}
	}
	destBase := "ralplan-" + filepath.Base(cleanedSource)
	if destBase == "ralplan-." || destBase == "ralplan-"+string(filepath.Separator) {
		destBase = "ralplan-plan.md"
	}
	dest, err := ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "plan", destBase)))
	if err != nil {
		return GJCArtifactRef{}, err
	}
	if err := writeExistingFileAtomically(dest, data); err != nil {
		return GJCArtifactRef{}, err
	}
	return GJCArtifactRef{Path: dest.Relative, SHA256: actualHash}, nil
}

func normalizeGJCRalplanNativeHash(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", &Problem{Code: "gjc_ralplan_native_output_invalid", Message: "native ralplan output checksum is missing", Hint: "Use the sha256 emitted by GJC native ralplan JSON output.", Field: "sha256", Expected: "sha256:<64hex>", Actual: "missing"}
	}
	if strings.HasPrefix(strings.ToLower(trimmed), "sha256:") {
		trimmed = strings.TrimPrefix(strings.ToLower(trimmed), "sha256:")
	}
	normalized := "sha256:" + strings.ToLower(trimmed)
	if !gjcChecksumPattern.MatchString(normalized) {
		return "", &Problem{Code: "gjc_ralplan_native_output_invalid", Message: "native ralplan output checksum is malformed", Hint: "Use a 64-character sha256 hex digest, with or without sha256: prefix.", Field: "sha256", Expected: "sha256:<64hex>", Actual: value}
	}
	return normalized, nil
}

func readGJCNativeSessionFile(root Root, runID string, source string, artifactName string) ([]byte, string, error) {
	cleanedSource := filepath.Clean(source)
	if !filepath.IsAbs(cleanedSource) {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_invalid", Message: "native " + artifactName + " output path is not absolute", Hint: "Use GJC native JSON output paths emitted under the selected repository .gjc session.", Field: "path", Expected: "absolute path under repository .gjc sessions", Actual: source}
	}
	expectedSessionID := "gjc-" + runID
	allowedPrefixes := []string{
		filepath.Join(root.Path, ".gjc", "sessions", expectedSessionID) + string(filepath.Separator),
		filepath.Join(root.Path, ".gjc", "_session-"+expectedSessionID) + string(filepath.Separator),
	}
	allowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(cleanedSource, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_invalid", Message: "native " + artifactName + " output path is outside the selected repository GJC session area", Hint: "Do not consume external, cross-repository, or cross-session GJC artifacts.", Field: "path", Expected: strings.Join(allowedPrefixes, " or ") + "...", Actual: cleanedSource}
	}
	if err := rejectGJCNativeSymlinkComponents(root, cleanedSource, artifactName); err != nil {
		return nil, cleanedSource, err
	}
	originalInfo, err := os.Lstat(cleanedSource)
	if err != nil {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_missing", Message: "cannot stat native " + artifactName + " output", Hint: "Ensure GJC emitted the artifact before adapting evidence.", Path: cleanedSource, Field: "path", Expected: "selected-session regular file", Actual: err.Error()}
	}
	if originalInfo.Mode()&os.ModeSymlink != 0 {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_invalid", Message: "native " + artifactName + " output is a symlink", Hint: "Do not consume symlinked or rebound GJC artifacts.", Path: cleanedSource, Field: "path", Expected: "regular file", Actual: originalInfo.Mode().String()}
	}
	if !originalInfo.Mode().IsRegular() {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_invalid", Message: "native " + artifactName + " output is not a regular file", Hint: "Do not consume directories, devices, or special files as GJC artifacts.", Path: cleanedSource, Field: "path", Expected: "regular file", Actual: originalInfo.Mode().String()}
	}
	resolvedSource, err := filepath.EvalSymlinks(cleanedSource)
	if err != nil {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_missing", Message: "cannot resolve native " + artifactName + " output", Hint: "Ensure GJC emitted real files, not stale or broken links, before adapting evidence.", Path: cleanedSource, Field: "path", Expected: "resolvable selected-session file", Actual: err.Error()}
	}
	resolvedAllowed := false
	for _, prefix := range allowedPrefixes {
		resolvedPrefix, err := filepath.EvalSymlinks(strings.TrimSuffix(prefix, string(filepath.Separator)))
		if err == nil && (resolvedSource == resolvedPrefix || strings.HasPrefix(resolvedSource, resolvedPrefix+string(filepath.Separator))) {
			resolvedAllowed = true
			break
		}
	}
	if !resolvedAllowed {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_invalid", Message: "native " + artifactName + " output resolves outside the selected repository GJC session area", Hint: "Do not consume symlinked or rebound GJC artifacts.", Field: "path", Expected: strings.Join(allowedPrefixes, " or ") + "...", Actual: resolvedSource}
	}
	info, err := os.Stat(resolvedSource)
	if err != nil {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_missing", Message: "cannot stat native " + artifactName + " output", Hint: "Ensure GJC emitted readable selected-session files before adapting evidence.", Path: cleanedSource, Field: "path", Expected: "readable selected-session file", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_invalid", Message: "native " + artifactName + " output is not a regular file", Hint: "Do not consume directories, devices, or special files as GJC artifacts.", Path: cleanedSource, Field: "path", Expected: "regular file", Actual: info.Mode().String()}
	}
	data, err := os.ReadFile(resolvedSource)
	if err != nil {
		return nil, cleanedSource, &Problem{Code: "gjc_" + artifactName + "_native_output_missing", Message: "cannot read native " + artifactName + " output", Hint: "Ensure GJC emitted readable native output before adapting evidence.", Path: cleanedSource, Field: "path", Expected: "readable native GJC output", Actual: err.Error()}
	}
	return data, cleanedSource, nil
}

func rejectGJCNativeSymlinkComponents(root Root, cleanedSource string, artifactName string) error {
	dir := filepath.Dir(cleanedSource)
	relativeDir, err := filepath.Rel(root.Path, dir)
	if err != nil || relativeDir == ".." || strings.HasPrefix(relativeDir, ".."+string(filepath.Separator)) || filepath.IsAbs(relativeDir) {
		actual := dir
		if err != nil {
			actual = err.Error()
		}
		return &Problem{Code: "gjc_" + artifactName + "_native_output_invalid", Message: "native " + artifactName + " output path is outside the selected repository", Hint: "Do not consume external or symlink-escaped GJC artifacts.", Field: "path", Expected: "path under repository root", Actual: actual}
	}
	if relativeDir == "." {
		return nil
	}
	current := root.Path
	for _, component := range strings.Split(relativeDir, string(filepath.Separator)) {
		if component == "" || component == "." {
			continue
		}
		current = filepath.Join(current, component)
		info, err := os.Lstat(current)
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			return &Problem{Code: "gjc_" + artifactName + "_native_output_missing", Message: "cannot inspect native " + artifactName + " output path component", Hint: "Ensure GJC emitted readable selected-session directories before adapting evidence.", Path: current, Field: "path", Expected: "inspectable non-symlink directory", Actual: err.Error()}
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return &Problem{Code: "gjc_" + artifactName + "_native_output_invalid", Message: "native " + artifactName + " output path component is a symlink", Hint: "Do not consume symlinked or rebound GJC session artifacts.", Path: current, Field: "path", Expected: "non-symlink selected-session path", Actual: info.Mode().String()}
		}
	}
	return nil
}

func adaptNativeUltragoalReceipt(root Root, runID string, receipt gjcReceipt) (gjcReceipt, error) {
	if !receipt.OK || strings.TrimSpace(receipt.GoalsPath) == "" || receipt.GoalsCount <= 0 {
		return receipt, nil
	}
	goalsRef, err := copyGJCUltragoalNativeArtifact(root, runID, receipt.GoalsPath, "ultragoal-goals.json")
	if err != nil {
		return receipt, err
	}
	ledgerSource := filepath.Join(filepath.Dir(receipt.GoalsPath), "ledger.jsonl")
	ledgerRef, err := copyGJCUltragoalNativeArtifact(root, runID, ledgerSource, "ultragoal-ledger.jsonl")
	if err != nil {
		return receipt, err
	}
	receipt.Status = "ultragoal_ready"
	receipt.Artifacts = []GJCArtifactRef{goalsRef, ledgerRef}
	receipt.CurrentRequiredActor = GJCActorKAS
	return receipt, nil
}

func copyGJCUltragoalNativeArtifact(root Root, runID string, source string, destBase string) (GJCArtifactRef, error) {
	cleanedSource := filepath.Clean(source)
	if !filepath.IsAbs(cleanedSource) {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_invalid", Message: "native ultragoal output path is not absolute", Hint: "Use GJC native JSON output paths emitted by `gjc ultragoal create-goals --json`.", Field: "goals_path", Expected: "absolute path under repository .gjc sessions", Actual: source}
	}
	expectedSessionID := "gjc-" + runID
	allowedPrefixes := []string{
		filepath.Join(root.Path, ".gjc", "sessions", expectedSessionID) + string(filepath.Separator),
		filepath.Join(root.Path, ".gjc", "_session-"+expectedSessionID) + string(filepath.Separator),
	}
	allowed := false
	for _, prefix := range allowedPrefixes {
		if strings.HasPrefix(cleanedSource, prefix) {
			allowed = true
			break
		}
	}
	if !allowed {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_invalid", Message: "native ultragoal output path is outside the selected repository GJC session area", Hint: "Do not consume external, cross-repository, or cross-session GJC artifacts.", Field: "goals_path", Expected: strings.Join(allowedPrefixes, " or ") + "...", Actual: cleanedSource}
	}
	originalInfo, err := os.Lstat(cleanedSource)
	if err != nil {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_missing", Message: "cannot stat native ultragoal output", Hint: "Ensure GJC emitted goals.json and ledger.jsonl before adapting evidence.", Path: cleanedSource, Field: "goals_path", Expected: "selected-session regular file", Actual: err.Error()}
	}
	if originalInfo.Mode()&os.ModeSymlink != 0 {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_invalid", Message: "native ultragoal output is a symlink", Hint: "Do not consume symlinked or rebound GJC artifacts.", Path: cleanedSource, Field: "goals_path", Expected: "regular file", Actual: originalInfo.Mode().String()}
	}
	if !originalInfo.Mode().IsRegular() {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_invalid", Message: "native ultragoal output is not a regular file", Hint: "Do not consume directories, devices, or special files as GJC artifacts.", Path: cleanedSource, Field: "goals_path", Expected: "regular file", Actual: originalInfo.Mode().String()}
	}
	resolvedSource, err := filepath.EvalSymlinks(cleanedSource)
	if err != nil {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_missing", Message: "cannot resolve native ultragoal output", Hint: "Ensure GJC emitted real files, not stale or broken links, before adapting evidence.", Path: cleanedSource, Field: "goals_path", Expected: "resolvable selected-session file", Actual: err.Error()}
	}
	resolvedAllowed := false
	for _, prefix := range allowedPrefixes {
		resolvedPrefix, err := filepath.EvalSymlinks(strings.TrimSuffix(prefix, string(filepath.Separator)))
		if err == nil && (resolvedSource == resolvedPrefix || strings.HasPrefix(resolvedSource, resolvedPrefix+string(filepath.Separator))) {
			resolvedAllowed = true
			break
		}
	}
	if !resolvedAllowed {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_invalid", Message: "native ultragoal output resolves outside the selected repository GJC session area", Hint: "Do not consume symlinked or rebound GJC artifacts.", Field: "goals_path", Expected: strings.Join(allowedPrefixes, " or ") + "...", Actual: resolvedSource}
	}
	info, err := os.Stat(resolvedSource)
	if err != nil {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_missing", Message: "cannot stat native ultragoal output", Hint: "Ensure GJC emitted goals.json and ledger.jsonl before adapting evidence.", Path: cleanedSource, Field: "goals_path", Expected: "readable selected-session file", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_invalid", Message: "native ultragoal output is not a regular file", Hint: "Do not consume directories, devices, or special files as GJC artifacts.", Path: cleanedSource, Field: "goals_path", Expected: "regular file", Actual: info.Mode().String()}
	}
	data, err := os.ReadFile(resolvedSource)
	if err != nil {
		return GJCArtifactRef{}, &Problem{Code: "gjc_ultragoal_native_output_missing", Message: "cannot read native ultragoal output", Hint: "Ensure GJC emitted goals.json and ledger.jsonl before adapting evidence.", Path: cleanedSource, Field: "goals_path", Expected: "readable native GJC output", Actual: err.Error()}
	}
	dest, err := ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "gjc", destBase)))
	if err != nil {
		return GJCArtifactRef{}, err
	}
	if err := writeExistingFileAtomically(dest, data); err != nil {
		return GJCArtifactRef{}, err
	}
	return GJCArtifactRef{Path: dest.Relative, SHA256: sha256String(data)}, nil
}

func loadOrCreateGJCSession(root Root, runID string, now func() time.Time) (gjcSession, error) {
	path, err := gjcSessionPath(root, runID)
	if err != nil {
		return gjcSession{}, err
	}
	data, err := os.ReadFile(path.Absolute)
	if os.IsNotExist(err) {
		session := gjcSession{SchemaVersion: GJCSchemaVersion, RunID: runID, SessionID: "gjc-" + runID, CreatedAt: now().UTC().Format(time.RFC3339)}
		data, err := json.MarshalIndent(session, "", "  ")
		if err != nil {
			return gjcSession{}, &Problem{Code: "gjc_session_encode_failed", Message: "cannot encode GJC session evidence", Hint: "Report the unsupported session payload to KAH maintainers.", Field: "session", Expected: "JSON-encodable session", Actual: err.Error()}
		}
		if err := writeNewFileAtomically(path, append(data, '\n')); err != nil {
			return gjcSession{}, err
		}
		return session, nil
	}
	if err != nil {
		return gjcSession{}, &Problem{Code: "gjc_session_read_failed", Message: "cannot read GJC session evidence", Hint: "Check run-local GJC session file permissions before retrying.", Path: path.Relative, Field: "session", Expected: "readable session JSON", Actual: err.Error()}
	}
	var session gjcSession
	if err := json.Unmarshal(data, &session); err != nil {
		return gjcSession{}, &Problem{Code: "gjc_session_invalid_json", Message: "GJC session evidence is not valid JSON", Hint: "Remove or repair the malformed run-local GJC session evidence before retrying.", Path: path.Relative, Field: "json", Expected: "GJC session JSON object", Actual: err.Error()}
	}
	if err := validateGJCSession(runID, session, path.Relative); err != nil {
		return gjcSession{}, err
	}
	return session, nil
}

func validatePersistedGJCStatus(root Root, metadata RunMetadata, status GJCStatus, relative string) error {
	if status.SchemaVersion != GJCSchemaVersion {
		return &Problem{Code: "gjc_status_invalid", Message: "GJC status schema version is unsupported", Hint: "Regenerate status with the current KAH GJC wrapper.", Path: relative, Field: "schema_version", Expected: GJCSchemaVersion, Actual: status.SchemaVersion}
	}
	if status.RunID != metadata.RunID {
		return &Problem{Code: "gjc_status_invalid", Message: "GJC status run id does not match the selected run", Hint: "Do not copy GJC status ledgers across runs.", Path: relative, Field: "run_id", Expected: metadata.RunID, Actual: status.RunID}
	}
	session, err := readGJCSession(root, metadata.RunID)
	if err != nil {
		return err
	}
	if status.GJCSessionID != session.SessionID {
		return &Problem{Code: "gjc_session_mismatch", Message: "GJC status session id does not match persisted session evidence", Hint: "Regenerate the GJC status ledger from the current run-local session.", Path: relative, Field: "gjc_session_id", Expected: session.SessionID, Actual: status.GJCSessionID}
	}
	if !gjcChecksumPattern.MatchString(status.StatusHash) {
		return &Problem{Code: "gjc_checksum_malformed", Message: "GJC status hash is malformed", Hint: "Regenerate the GJC status ledger through the KAH GJC wrapper.", Path: relative, Field: "status_hash", Expected: "sha256:<64hex>", Actual: status.StatusHash}
	}
	expectedStatusHash, err := computeGJCStatusHash(status)
	if err != nil {
		return err
	}
	if status.StatusHash != expectedStatusHash {
		return &Problem{Code: "gjc_status_hash_mismatch", Message: "GJC status hash does not match persisted status content", Hint: "Regenerate the GJC status ledger through the KAH GJC wrapper before consuming status evidence.", Path: relative, Field: "status_hash", Expected: expectedStatusHash, Actual: status.StatusHash}
	}
	if _, err := normalizeGJCCommandKind(status.CommandKind); err != nil {
		return err
	}
	if _, err := normalizeGJCRealHome(status.RealUserHome); err != nil {
		return err
	}
	if !allowedGJCStatusForCommand(status.CommandKind, status.Process.Status) {
		return &Problem{Code: "gjc_status_unsupported", Message: "GJC status ledger contains unsupported process status", Hint: "Regenerate status with a supported GAJAE candidate status.", Path: relative, Field: "process.status", Expected: expectedGJCStatuses(status.CommandKind), Actual: status.Process.Status}
	}
	if !validGJCActor(status.CurrentRequiredActor) {
		return &Problem{Code: "gjc_required_actor_unsupported", Message: "GJC status ledger contains unsupported required actor", Hint: "Use gjc, kas, color, mar, user, kat, or none.", Path: relative, Field: "current_required_actor", Expected: "supported GAJAE actor", Actual: status.CurrentRequiredActor}
	}
	if err := validateGJCPacketRef(root, metadata.RunID, status.Packet); err != nil {
		return err
	}
	if status.NativeInput != nil {
		if err := validateGJCArtifactRef(root, metadata.RunID, *status.NativeInput, "native_input_ref"); err != nil {
			return err
		}
	}
	if status.Receipt != nil {
		if err := validateGJCArtifactRef(root, metadata.RunID, *status.Receipt, "receipt_ref"); err != nil {
			return err
		}
	}
	if err := validateGJCPlanEvidence(root, metadata.RunID, status); err != nil {
		return err
	}
	if err := validateGJCKATEvidence(root, metadata.RunID, status.KAT); err != nil {
		return err
	}
	for i, ref := range status.Artifacts {
		if err := validateGJCArtifactRef(root, metadata.RunID, ref, fmt.Sprintf("artifact_refs[%d]", i)); err != nil {
			return err
		}
	}
	if err := validateGJCCallback(status.Callback); err != nil {
		return err
	}
	return nil
}

func readGJCSession(root Root, runID string) (gjcSession, error) {
	path, err := gjcSessionPath(root, runID)
	if err != nil {
		return gjcSession{}, err
	}
	data, err := os.ReadFile(path.Absolute)
	if os.IsNotExist(err) {
		return gjcSession{}, &Problem{Code: "gjc_session_missing", Message: "GJC session evidence is missing", Hint: "Run a GJC start command to create run-local session evidence before reading status.", Path: path.Relative, Field: "gjc_session_id", Expected: "persisted run-local session", Actual: "missing"}
	}
	if err != nil {
		return gjcSession{}, &Problem{Code: "gjc_session_read_failed", Message: "cannot read GJC session evidence", Hint: "Check run-local GJC session file permissions before retrying.", Path: path.Relative, Field: "session", Expected: "readable session JSON", Actual: err.Error()}
	}
	var session gjcSession
	if err := json.Unmarshal(data, &session); err != nil {
		return gjcSession{}, &Problem{Code: "gjc_session_invalid_json", Message: "GJC session evidence is not valid JSON", Hint: "Remove or repair the malformed run-local GJC session evidence before retrying.", Path: path.Relative, Field: "json", Expected: "GJC session JSON object", Actual: err.Error()}
	}
	if err := validateGJCSession(runID, session, path.Relative); err != nil {
		return gjcSession{}, err
	}
	return session, nil
}

func validateGJCSession(runID string, session gjcSession, relative string) error {
	if session.SchemaVersion != GJCSchemaVersion {
		return &Problem{Code: "gjc_session_invalid", Message: "GJC session schema version is unsupported", Hint: "Regenerate session evidence with the current KAH wrapper.", Path: relative, Field: "schema_version", Expected: GJCSchemaVersion, Actual: session.SchemaVersion}
	}
	if session.RunID != runID {
		return &Problem{Code: "gjc_session_invalid", Message: "GJC session run id does not match the selected run", Hint: "Do not copy GJC session evidence across runs.", Path: relative, Field: "run_id", Expected: runID, Actual: session.RunID}
	}
	if !gjcSessionIDPattern.MatchString(session.SessionID) {
		return &Problem{Code: "gjc_session_invalid", Message: "GJC session id is malformed", Hint: "Regenerate session evidence through the KAH GJC wrapper.", Path: relative, Field: "gjc_session_id", Expected: "gjc-run-YYYYMMDDTHHMMSSZ-<12hex>", Actual: session.SessionID}
	}
	return nil
}

func validateGJCPacketRef(root Root, runID string, ref GJCArtifactRef) error {
	path, err := validateGJCRunRef(root, runID, ref.Path, "packet_ref.path")
	if err != nil {
		return err
	}
	if !gjcChecksumPattern.MatchString(ref.SHA256) {
		return &Problem{Code: "gjc_checksum_malformed", Message: "GJC packet checksum is malformed", Hint: "Use sha256:<64 lowercase hex characters> for the KAS packet_ref.", Path: path.Relative, Field: "packet_ref.sha256", Expected: "sha256:<64hex>", Actual: ref.SHA256}
	}
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return &Problem{Code: "gjc_artifact_read_failed", Message: "cannot read GJC packet_ref", Hint: "Regenerate or repair the run-local KAS packet before consuming GJC status.", Path: path.Relative, Field: "packet_ref.path", Expected: "readable packet file", Actual: err.Error()}
	}
	actual := sha256String(data)
	if actual != ref.SHA256 {
		return &Problem{Code: "gjc_checksum_mismatch", Message: "GJC packet_ref checksum does not match file content", Hint: "Regenerate the KAS packet_ref and rerun GJC before consuming status.", Path: path.Relative, Field: "packet_ref.sha256", Expected: actual, Actual: ref.SHA256}
	}
	return nil
}

func validateGJCArtifactRef(root Root, runID string, ref GJCArtifactRef, field string) error {
	path, err := validateGJCRunRef(root, runID, ref.Path, field+".path")
	if err != nil {
		return err
	}
	if !gjcChecksumPattern.MatchString(ref.SHA256) {
		return &Problem{Code: "gjc_checksum_malformed", Message: "GJC artifact checksum is malformed", Hint: "Use sha256:<64 lowercase hex characters> for every GJC artifact ref.", Path: path.Relative, Field: field + ".sha256", Expected: "sha256:<64hex>", Actual: ref.SHA256}
	}
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return &Problem{Code: "gjc_artifact_read_failed", Message: "cannot read GJC artifact ref", Hint: "Ensure GJC receipt refs point to readable run-local evidence files.", Path: path.Relative, Field: field + ".path", Expected: "readable artifact file", Actual: err.Error()}
	}
	actual := sha256String(data)
	if actual != ref.SHA256 {
		return &Problem{Code: "gjc_checksum_mismatch", Message: "GJC artifact checksum does not match file content", Hint: "Regenerate the GJC receipt after artifact content changes.", Path: path.Relative, Field: field + ".sha256", Expected: actual, Actual: ref.SHA256}
	}
	return nil
}

func validateGJCRunRef(root Root, runID string, value string, field string) (SafePath, error) {
	path, err := ResolveRelativePath(root, value)
	if err != nil {
		return SafePath{}, err
	}
	prefix := filepath.ToSlash(filepath.Join(RunRootPath, runID)) + "/"
	if path.Relative != strings.TrimSuffix(prefix, "/") && !strings.HasPrefix(path.Relative, prefix) {
		return SafePath{}, &Problem{Code: "gjc_ref_cross_run", Message: "GJC reference must stay within the selected run root", Hint: "Use a repository-relative path under .kkachi/runs/<run_id>/.", Path: path.Relative, Field: field, Expected: prefix + "...", Actual: path.Relative}
	}
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		return SafePath{}, &Problem{Code: "gjc_ref_missing", Message: "GJC reference is missing", Hint: "Create the packet or artifact under the selected run root before starting GJC.", Path: path.Relative, Field: field, Expected: "existing regular file", Actual: "missing"}
	}
	if err != nil {
		return SafePath{}, &Problem{Code: "gjc_ref_inspection_failed", Message: "cannot inspect GJC reference", Hint: "Check run-local evidence permissions before retrying.", Path: path.Relative, Field: field, Expected: "inspectable regular file", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		return SafePath{}, &Problem{Code: "gjc_ref_invalid", Message: "GJC reference must be a regular file", Hint: "Use a regular packet or artifact file under the selected run root.", Path: path.Relative, Field: field, Expected: "regular file", Actual: fileKind(info)}
	}
	return path, nil
}

func normalizeGJCCommandKind(value string) (string, error) {
	value = strings.TrimSpace(value)
	switch value {
	case gjcCommandDeep, gjcCommandRalplan, gjcCommandUltragoal:
		return value, nil
	default:
		return "", &Problem{Code: "gjc_command_unsupported", Message: "GJC command kind is not supported", Hint: "Use start-deep-interview, start-ralplan, or start-ultragoal for GAJAE-002.", Field: "command_kind", Expected: "start-deep-interview, start-ralplan, or start-ultragoal", Actual: value}
	}
}

func normalizeGJCRealHome(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = GJCDefaultRealHome
	}
	cleaned := filepath.Clean(value)
	if !filepath.IsAbs(cleaned) || cleaned != value || cleaned == string(filepath.Separator) {
		return "", &Problem{Code: "gjc_home_unsafe", Message: "GJC real-user HOME is unsafe", Hint: "Use the approved absolute real-user home for this local operator environment.", Field: "real_user_home", Expected: GJCDefaultRealHome, Actual: value}
	}
	return cleaned, nil
}

func gjcArgsForCommand(root Root, runID string, commandKind string, packet string, packetData []byte) (gjcCommandInput, error) {
	switch commandKind {
	case gjcCommandDeep:
		return gjcCommandInput{Args: []string{"deep-interview", "--packet", packet, "--json"}}, nil
	case gjcCommandRalplan:
		native, err := parseGJCRalplanNativeInput(root, runID, packetData)
		if err != nil {
			return gjcCommandInput{}, err
		}
		return gjcCommandInput{Args: []string{"ralplan", "--write", "--stage", native.Stage, "--stage_n", native.StageN, "--artifact", native.Artifact.Absolute, "--json"}}, nil
	case gjcCommandUltragoal:
		native, err := parseGJCUltragoalNativeInput(packetData)
		if err != nil {
			return gjcCommandInput{}, err
		}
		nativeInput, err := writeGJCUltragoalBriefInput(root, runID, native)
		if err != nil {
			return gjcCommandInput{}, err
		}
		path, err := validateGJCRunRef(root, runID, nativeInput.Path, "native_input_ref.path")
		if err != nil {
			return gjcCommandInput{}, err
		}
		return gjcCommandInput{Args: []string{"ultragoal", "create-goals", "--brief-file", path.Absolute, "--json"}, NativeInput: &nativeInput}, nil
	default:
		return gjcCommandInput{Args: []string{commandKind, "--packet", packet, "--json"}}, nil
	}
}

func parseGJCRalplanNativeInput(root Root, runID string, packetData []byte) (gjcNativeRalplanInput, error) {
	fields, found, err := extractGJCRalplanNativeInputFields(packetData)
	if err != nil {
		return gjcNativeRalplanInput{}, err
	}
	if !found {
		return gjcNativeRalplanInput{}, &Problem{Code: "gjc_ralplan_native_input_missing", Message: "ralplan packet lacks native_ralplan_input", Hint: "Regenerate the KAS ralplan packet with native_ralplan_input.stage, native_ralplan_input.stage_n, and native_ralplan_input.artifact before starting GJC.", Field: "native_ralplan_input", Expected: "stage, stage_n, and artifact fields", Actual: "missing"}
	}
	stage := strings.TrimSpace(fields["stage"])
	stageN := strings.TrimSpace(fields["stage_n"])
	artifact := strings.TrimSpace(fields["artifact"])
	if stage == "" || stageN == "" || artifact == "" {
		return gjcNativeRalplanInput{}, &Problem{Code: "gjc_ralplan_native_input_invalid", Message: "ralplan native input is incomplete", Hint: "Provide all required native_ralplan_input fields: stage, stage_n, and artifact.", Field: "native_ralplan_input", Expected: "non-empty stage, stage_n, and artifact", Actual: fmt.Sprintf("stage=%q stage_n=%q artifact=%q", stage, stageN, artifact)}
	}
	if !gjcNativeStagePattern.MatchString(stage) {
		return gjcNativeRalplanInput{}, &Problem{Code: "gjc_ralplan_native_input_invalid", Message: "ralplan native stage is malformed", Hint: "Use a compact GJC stage token such as planner.", Field: "native_ralplan_input.stage", Expected: "alphanumeric stage token", Actual: stage}
	}
	stageNumber, err := strconv.Atoi(stageN)
	if err != nil || stageNumber < 1 {
		return gjcNativeRalplanInput{}, &Problem{Code: "gjc_ralplan_native_input_invalid", Message: "ralplan native stage number is malformed", Hint: "Use a positive integer for native_ralplan_input.stage_n.", Field: "native_ralplan_input.stage_n", Expected: "positive integer", Actual: stageN}
	}
	artifactPath, err := validateGJCRunRef(root, runID, artifact, "native_ralplan_input.artifact")
	if err != nil {
		return gjcNativeRalplanInput{}, err
	}
	return gjcNativeRalplanInput{Stage: stage, StageN: strconv.Itoa(stageNumber), Artifact: artifactPath}, nil
}

func extractGJCRalplanNativeInputFields(packetData []byte) (map[string]string, bool, error) {
	fields := map[string]string{}
	lines := strings.Split(string(packetData), "\n")
	blockCount := 0
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "native_ralplan_input:" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " 	"))
		if indent != 0 {
			return nil, true, &Problem{Code: "gjc_ralplan_native_input_invalid", Message: "ralplan native input block is nested", Hint: "Declare native_ralplan_input once at the packet top level.", Field: fmt.Sprintf("native_ralplan_input line %d", idx+1), Expected: "top-level native_ralplan_input block", Actual: "nested block"}
		}
		blockCount++
	}
	if blockCount > 1 {
		return nil, true, &Problem{Code: "gjc_ralplan_native_input_invalid", Message: "ralplan packet has duplicate native input blocks", Hint: "Declare native_ralplan_input exactly once at the packet top level.", Field: "native_ralplan_input", Expected: "single block", Actual: "duplicate blocks"}
	}
	inBlock := false
	blockIndent := -1
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " 	"))
		if !inBlock {
			if trimmed == "native_ralplan_input:" {
				inBlock = true
				blockIndent = indent
			}
			continue
		}
		if indent <= blockIndent {
			break
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return nil, true, &Problem{Code: "gjc_ralplan_native_input_invalid", Message: "ralplan native input has a malformed field", Hint: "Use simple key: value lines under native_ralplan_input.", Field: fmt.Sprintf("native_ralplan_input line %d", idx+1), Expected: "key: value", Actual: trimmed}
		}
		key = strings.TrimSpace(key)
		value = normalizeGJCPacketScalar(value)
		switch key {
		case "stage", "stage_n", "artifact":
			if _, exists := fields[key]; exists {
				return nil, true, &Problem{Code: "gjc_ralplan_native_input_invalid", Message: "ralplan native input has a duplicate field", Hint: "Declare each native_ralplan_input field exactly once: stage, stage_n, and artifact.", Field: "native_ralplan_input." + key, Expected: "single field declaration", Actual: "duplicate"}
			}
			fields[key] = value
		}
	}
	return fields, inBlock, nil
}

func parseGJCUltragoalNativeInput(packetData []byte) (gjcNativeUltragoalInput, error) {
	fields, found, err := extractGJCUltragoalNativeInputFields(packetData)
	if err != nil {
		return gjcNativeUltragoalInput{}, err
	}
	if !found {
		return gjcNativeUltragoalInput{}, &Problem{Code: "gjc_ultragoal_native_input_missing", Message: "ultragoal packet lacks native_ultragoal_input", Hint: "Regenerate the KAS ultragoal packet with native_ultragoal_input.mode=brief_file and native_ultragoal_input.brief before starting GJC.", Field: "native_ultragoal_input", Expected: "mode and brief fields", Actual: "missing"}
	}
	mode := strings.TrimSpace(fields["mode"])
	brief := strings.TrimSpace(fields["brief"])
	if mode == "" || brief == "" {
		return gjcNativeUltragoalInput{}, &Problem{Code: "gjc_ultragoal_native_input_invalid", Message: "ultragoal native input is incomplete", Hint: "Provide native_ultragoal_input.mode and native_ultragoal_input.brief.", Field: "native_ultragoal_input", Expected: "non-empty mode and brief", Actual: fmt.Sprintf("mode=%q brief_empty=%t", mode, brief == "")}
	}
	if mode != "brief_file" {
		return gjcNativeUltragoalInput{}, &Problem{Code: "gjc_ultragoal_native_input_invalid", Message: "ultragoal native input mode is unsupported", Hint: "Use native_ultragoal_input.mode: brief_file; do not fall back to --brief or --from-stdin.", Field: "native_ultragoal_input.mode", Expected: "brief_file", Actual: mode}
	}
	return gjcNativeUltragoalInput{Mode: mode, Brief: strings.TrimSpace(brief) + "\n"}, nil
}

func extractGJCUltragoalNativeInputFields(packetData []byte) (map[string]string, bool, error) {
	fields := map[string]string{}
	lines := strings.Split(string(packetData), "\n")
	blockCount := 0
	for idx, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "native_ultragoal_input:" {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if indent != 0 {
			return nil, true, &Problem{Code: "gjc_ultragoal_native_input_invalid", Message: "ultragoal native input block is nested", Hint: "Declare native_ultragoal_input once at the packet top level.", Field: fmt.Sprintf("native_ultragoal_input line %d", idx+1), Expected: "top-level native_ultragoal_input block", Actual: "nested block"}
		}
		blockCount++
	}
	if blockCount > 1 {
		return nil, true, &Problem{Code: "gjc_ultragoal_native_input_invalid", Message: "ultragoal packet has duplicate native input blocks", Hint: "Declare native_ultragoal_input exactly once at the packet top level.", Field: "native_ultragoal_input", Expected: "single block", Actual: "duplicate blocks"}
	}
	inBlock := false
	blockIndent := -1
	for idx := 0; idx < len(lines); idx++ {
		line := lines[idx]
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " \t"))
		if !inBlock {
			if trimmed == "native_ultragoal_input:" {
				inBlock = true
				blockIndent = indent
			}
			continue
		}
		if indent <= blockIndent {
			break
		}
		key, value, ok := strings.Cut(trimmed, ":")
		if !ok {
			return nil, true, &Problem{Code: "gjc_ultragoal_native_input_invalid", Message: "ultragoal native input has a malformed field", Hint: "Use simple key: value lines under native_ultragoal_input.", Field: fmt.Sprintf("native_ultragoal_input line %d", idx+1), Expected: "key: value", Actual: trimmed}
		}
		key = strings.TrimSpace(key)
		value = normalizeGJCPacketScalar(value)
		switch key {
		case "mode":
			if _, exists := fields[key]; exists {
				return nil, true, &Problem{Code: "gjc_ultragoal_native_input_invalid", Message: "ultragoal native input has a duplicate field", Hint: "Declare each native_ultragoal_input field exactly once: mode and brief.", Field: "native_ultragoal_input." + key, Expected: "single field declaration", Actual: "duplicate"}
			}
			fields[key] = value
		case "brief":
			if _, exists := fields[key]; exists {
				return nil, true, &Problem{Code: "gjc_ultragoal_native_input_invalid", Message: "ultragoal native input has a duplicate field", Hint: "Declare each native_ultragoal_input field exactly once: mode and brief.", Field: "native_ultragoal_input." + key, Expected: "single field declaration", Actual: "duplicate"}
			}
			if value != "|" {
				fields[key] = value
				continue
			}
			fieldIndent := indent
			var blockLines []string
			contentIndent := -1
			for idx+1 < len(lines) {
				next := lines[idx+1]
				nextTrimmed := strings.TrimSpace(next)
				nextIndent := len(next) - len(strings.TrimLeft(next, " \t"))
				if nextTrimmed != "" && nextIndent <= fieldIndent {
					break
				}
				idx++
				if nextTrimmed == "" {
					blockLines = append(blockLines, "")
					continue
				}
				if contentIndent == -1 {
					contentIndent = nextIndent
				}
				if len(next) >= contentIndent {
					blockLines = append(blockLines, next[contentIndent:])
				} else {
					blockLines = append(blockLines, strings.TrimSpace(next))
				}
			}
			fields[key] = strings.TrimRight(strings.Join(blockLines, "\n"), "\n")
		}
	}
	return fields, inBlock, nil
}

func writeGJCUltragoalBriefInput(root Root, runID string, native gjcNativeUltragoalInput) (GJCArtifactRef, error) {
	path, err := ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "gjc", "ultragoal-brief-input.md")))
	if err != nil {
		return GJCArtifactRef{}, err
	}
	data := []byte(native.Brief)
	if err := writeExistingFileAtomically(path, data); err != nil {
		return GJCArtifactRef{}, err
	}
	return GJCArtifactRef{Path: path.Relative, SHA256: sha256String(data)}, nil
}

func normalizeGJCPacketScalar(value string) string {
	value = strings.TrimSpace(stripGJCPacketInlineComment(value))
	if len(value) >= 2 {
		if (value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'') {
			return strings.TrimSpace(value[1 : len(value)-1])
		}
	}
	return strings.TrimSpace(value)
}

func stripGJCPacketInlineComment(value string) string {
	inSingle := false
	inDouble := false
	for idx, r := range value {
		switch r {
		case '\'':
			if !inDouble {
				inSingle = !inSingle
			}
		case '"':
			if !inSingle {
				inDouble = !inDouble
			}
		case '#':
			if !inSingle && !inDouble && (idx == 0 || value[idx-1] == ' ' || value[idx-1] == '	') {
				return value[:idx]
			}
		}
	}
	return value
}

func gjcEnv(home string, sessionID string) []string {
	env := make([]string, 0, len(os.Environ())+2)
	for _, item := range os.Environ() {
		if strings.HasPrefix(item, "HOME=") || strings.HasPrefix(item, "GJC_SESSION_ID=") {
			continue
		}
		env = append(env, item)
	}
	env = append(env, "HOME="+home, "GJC_SESSION_ID="+sessionID)
	return env
}

func defaultGJCRunner(invocation gjcRunnerInvocation) (gjcRunnerResult, error) {
	if _, err := exec.LookPath(invocation.Command); err != nil {
		return gjcRunnerResult{ExitCode: -1}, &Problem{Code: "gjc_command_missing", Message: "GJC command is not available", Hint: "Install or expose gjc on PATH before using the KAH wrapper; do not mutate provider/auth/token/gateway/model settings as fallback.", Field: "command", Expected: "callable gjc executable", Actual: err.Error()}
	}
	cmd := exec.Command(invocation.Command, invocation.Args...)
	cmd.Dir = invocation.Dir
	cmd.Env = invocation.Env
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return gjcRunnerResult{ExitCode: -1}, err
	}
	result := gjcRunnerResult{PID: cmd.Process.Pid}
	waitErr := cmd.Wait()
	result.Stdout = stdout.Bytes()
	result.Stderr = stderr.Bytes()
	result.ExitCode = cmd.ProcessState.ExitCode()
	if _, ok := waitErr.(*exec.ExitError); ok {
		return result, nil
	}
	return result, waitErr
}

func allowedGJCStatusForCommand(commandKind string, status string) bool {
	switch status {
	case GJCStatusRunning, GJCStatusBlocked, GJCStatusFailed, GJCStatusCancelled:
		return true
	case "deep_interview_ready":
		return commandKind == gjcCommandDeep
	case "ralplan_ready":
		return commandKind == gjcCommandRalplan
	case "ultragoal_ready":
		return commandKind == gjcCommandUltragoal
	default:
		return false
	}
}

func expectedGJCStatuses(commandKind string) string {
	switch commandKind {
	case gjcCommandDeep:
		return "running, deep_interview_ready, blocked, failed, or cancelled"
	case gjcCommandRalplan:
		return "running, ralplan_ready, blocked, failed, or cancelled"
	case gjcCommandUltragoal:
		return "running, ultragoal_ready, blocked, failed, or cancelled"
	default:
		return "supported GAJAE candidate status"
	}
}

func defaultGJCRequiredActor(status string) string {
	switch status {
	case "deep_interview_ready", "ralplan_ready", "ultragoal_ready":
		return GJCActorKAS
	case GJCStatusBlocked:
		return GJCActorUser
	case GJCStatusFailed, GJCStatusCancelled:
		return GJCActorNone
	default:
		return GJCActorGJC
	}
}

func validGJCActor(actor string) bool {
	switch actor {
	case GJCActorGJC, GJCActorKAS, GJCActorColor, GJCActorMAR, GJCActorUser, GJCActorKAT, GJCActorNone:
		return true
	default:
		return false
	}
}

func gjcSessionPath(root Root, runID string) (SafePath, error) {
	return ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "gjc", gjcSessionFileName)))
}

func gjcStatusPath(root Root, runID string) (SafePath, error) {
	return ResolveRelativePath(root, mustGJCStatusRelative(runID))
}

func gjcReceiptPath(root Root, runID string) (SafePath, error) {
	return ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "gjc", gjcReceiptFileName)))
}

func mustGJCStatusRelative(runID string) string {
	return filepath.ToSlash(filepath.Join(RunRootPath, runID, "artifacts", "gjc", gjcStatusFileName))
}

func gjcEventPayload(status GJCStatus) map[string]any {
	payload := map[string]any{
		"schema_version":         status.SchemaVersion,
		"run_id":                 status.RunID,
		"task_id":                status.TaskID,
		"command_kind":           status.CommandKind,
		"real_user_home":         status.RealUserHome,
		"gjc_session_id":         status.GJCSessionID,
		"process":                status.Process,
		"packet_ref":             status.Packet,
		"native_input_ref":       status.NativeInput,
		"receipt_ref":            status.Receipt,
		"artifact_refs":          status.Artifacts,
		"plan":                   status.Plan,
		"kat":                    status.KAT,
		"callback":               status.Callback,
		"current_required_actor": status.CurrentRequiredActor,
		"current_wait_reason":    status.CurrentWaitReason,
		"status_path":            status.StatusPath,
		"status_hash":            status.StatusHash,
		"recovery_hint":          status.RecoveryHint,
	}
	if status.Error != nil {
		payload["error"] = status.Error
	}
	return payload
}

func sha256String(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func computeGJCStatusHash(status GJCStatus) (string, error) {
	status.StatusHash = ""
	data, err := json.MarshalIndent(status, "", "  ")
	if err != nil {
		return "", &Problem{Code: "gjc_status_encode_failed", Message: "cannot encode GJC status ledger", Hint: "Report the unsupported status payload to KAH maintainers.", Field: "status", Expected: "JSON-encodable status", Actual: err.Error()}
	}
	data = append(data, '\n')
	return sha256String(data), nil
}

func firstNonEmptyLine(data []byte, fallback string) string {
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return fallback
}

func problemCode(err error, fallback string) string {
	if p, ok := err.(*Problem); ok {
		return p.Code
	}
	return fallback
}

func problemMessage(err error) string {
	if p, ok := err.(*Problem); ok {
		return p.Message
	}
	return err.Error()
}

func timestampFunc(value string) func() time.Time {
	return func() time.Time {
		parsed, err := time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Now().UTC()
		}
		return parsed
	}
}

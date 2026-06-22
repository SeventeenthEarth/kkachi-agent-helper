package project

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	artifactWrittenEventType    = "artifact.written"
	artifactOperationWrite      = "write"
	artifactOperationAppend     = "append"
	artifactOperationSetStatus  = "set-status"
	artifactKindCanonical       = "canonical"
	artifactActionCreated       = "created"
	artifactActionReinitialized = "reinitialized"
	artifactActionPreserved     = "preserved"
	artifactActionMissing       = "missing"
	artifactActionPresent       = "present"
)

var canonicalArtifactPaths = []string{
	"intake-classification.md",
	"sot-basis.md",
	"task-brief.md",
	"acceptance-criteria.md",
	"plan.md",
	"checklist.md",
	"selected-cli.json",
	"capability-check.md",
	"bridge-session-snapshot.json",
	"bridge-events.md",
	"token-economy-evidence.json",
	"multi-agent-review/status.json",
	policyPromotionArtifact,
	designEvidenceArtifact,
	"prompt.md",
	"context-pack.md",
	"cli-output.md",
	"diff.patch",
	"impl-log.md",
	"test-log.md",
	"verification.md",
	"review.md",
	"docs-update.md",
	"sot-update.md",
	"roadmap-update.md",
	"improvement-note.md",
	"feedback-request.md",
	"feedback-1.md",
	"feedback-triage-1.md",
	"handle-feedback-1.md",
	"redteam/plan-review.md",
	"redteam/impl-review.md",
	"redteam/test-review.md",
	"redteam/qa-review.md",
	"redteam/shaping-review.md",
	"redteam/final-gate-review.md",
	"discovery/existing-docs-review.md",
	"discovery/problem-framing.md",
	"discovery/research-notes.md",
	"discovery/strategy-options.md",
	"discovery/selected-strategy.md",
	"discovery/task-breakdown.md",
	"discovery/implementation-readiness.md",
	"discovery/handoff-to-development.md",
	"final-report.md",
}

var canonicalArtifactPathSet = stringSet(canonicalArtifactPaths)

type ArtifactInitOptions struct {
	RunID string
	Now   func() time.Time
}

type ArtifactInitResult struct {
	RunID             string
	RunPath           string
	EventID           string
	Created           []ArtifactStatus
	Reinitialized     []ArtifactStatus
	Preserved         []ArtifactStatus
	RequiredArtifacts []string
	Artifacts         []ArtifactStatus
}

type ArtifactListResult struct {
	RunID     string
	Artifacts []ArtifactStatus
}

type ArtifactMutateOptions struct {
	RunID    string
	Artifact string
	From     string
	Status   string
	Reason   string
	Now      func() time.Time
}

type ArtifactMutationResult struct {
	RunID        string `json:"run_id"`
	Path         string `json:"path"`
	ArtifactKind string `json:"artifact_kind"`
	Operation    string `json:"operation"`
	Bytes        int64  `json:"bytes"`
	SourcePath   string `json:"source_path,omitempty"`
	Status       string `json:"status,omitempty"`
	Reason       string `json:"reason,omitempty"`
	EventID      string `json:"event_id"`
}

type ArtifactStatus struct {
	Path     string `json:"path"`
	Required bool   `json:"required"`
	Exists   bool   `json:"exists"`
	Empty    bool   `json:"empty"`
	Bytes    int64  `json:"bytes"`
	Action   string `json:"action"`
}

func InitArtifacts(root Root, options ArtifactInitOptions) (ArtifactInitResult, error) {
	var result ArtifactInitResult
	err := withProjectWriteLock(root, "artifact init", options.RunID, func() error {
		var err error
		result, err = initArtifactsUnlocked(root, options)
		return err
	})
	return result, err
}

func initArtifactsUnlocked(root Root, options ArtifactInitOptions) (ArtifactInitResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	metadata, metadataPath, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return ArtifactInitResult{}, err
	}
	if metadata.State == RunStateClosed || metadata.State == RunStateAborted {
		return ArtifactInitResult{}, &Problem{Code: "run_artifact_init_invalid_state", Message: "cannot initialize artifacts for a finished run", Hint: "Create a new run or inspect the existing artifacts without mutating them.", Path: metadataPath.Relative, Field: "state", Expected: "created or active", Actual: metadata.State}
	}
	if err := preflightEventCoherence(root); err != nil {
		return ArtifactInitResult{}, err
	}

	required := ArtifactManifest(metadata)
	requiredSet := stringSet(required)
	artifacts, err := inspectArtifacts(root, metadata.RunID, requiredSet)
	if err != nil {
		return ArtifactInitResult{}, err
	}

	planned := make([]ArtifactStatus, len(artifacts))
	copy(planned, artifacts)
	for i := range planned {
		if !planned[i].Exists {
			planned[i].Action = artifactActionCreated
		} else if planned[i].Empty {
			planned[i].Action = artifactActionReinitialized
		} else {
			planned[i].Action = artifactActionPreserved
		}
	}

	metadata.RequiredArtifacts = required
	payload := map[string]any{
		"run_id":             metadata.RunID,
		"artifact_count":     len(planned),
		"required_artifacts": required,
		"created":            artifactPathsByAction(planned, artifactActionCreated),
		"reinitialized":      artifactPathsByAction(planned, artifactActionReinitialized),
		"preserved":          artifactPathsByAction(planned, artifactActionPreserved),
	}
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: artifactWrittenEventType, RunID: metadata.RunID, Payload: payload, Now: options.Now}, func(map[string]any, string) error {
		if err := writeArtifactBaselines(root, metadata, planned); err != nil {
			return err
		}
		return writeRunMetadataExisting(metadataPath, metadata)
	})
	if err != nil {
		return ArtifactInitResult{}, err
	}

	updated, err := inspectArtifacts(root, metadata.RunID, requiredSet)
	if err != nil {
		return ArtifactInitResult{}, err
	}
	for i := range updated {
		updated[i].Action = planned[i].Action
	}
	result := ArtifactInitResult{RunID: metadata.RunID, RunPath: filepath.ToSlash(filepath.Dir(metadataPath.Relative)), EventID: appendResult.EventID, RequiredArtifacts: required, Artifacts: updated}
	for _, artifact := range updated {
		switch artifact.Action {
		case artifactActionCreated:
			result.Created = append(result.Created, artifact)
		case artifactActionReinitialized:
			result.Reinitialized = append(result.Reinitialized, artifact)
		case artifactActionPreserved:
			result.Preserved = append(result.Preserved, artifact)
		}
	}
	return result, nil
}

func WriteArtifact(root Root, options ArtifactMutateOptions) (ArtifactMutationResult, error) {
	return mutateArtifact(root, artifactOperationWrite, options)
}

func AppendArtifact(root Root, options ArtifactMutateOptions) (ArtifactMutationResult, error) {
	return mutateArtifact(root, artifactOperationAppend, options)
}

func SetArtifactStatus(root Root, options ArtifactMutateOptions) (ArtifactMutationResult, error) {
	return mutateArtifact(root, artifactOperationSetStatus, options)
}

func mutateArtifact(root Root, operation string, options ArtifactMutateOptions) (ArtifactMutationResult, error) {
	var result ArtifactMutationResult
	err := withProjectWriteLock(root, "artifact "+operation, options.RunID, func() error {
		var err error
		result, err = mutateArtifactUnlocked(root, operation, options)
		return err
	})
	return result, err
}

func mutateArtifactUnlocked(root Root, operation string, options ArtifactMutateOptions) (ArtifactMutationResult, error) {
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	metadata, metadataPath, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return ArtifactMutationResult{}, err
	}
	if metadata.State == RunStateClosed || metadata.State == RunStateAborted {
		return ArtifactMutationResult{}, &Problem{Code: "run_artifact_mutation_invalid_state", Message: "cannot mutate artifacts for a finished run", Hint: "Create a new run or inspect the existing artifacts without mutating them.", Path: metadataPath.Relative, Field: "state", Expected: "created or active", Actual: metadata.State}
	}
	if err := preflightEventCoherence(root); err != nil {
		return ArtifactMutationResult{}, err
	}
	artifact := strings.TrimSpace(options.Artifact)
	path, err := artifactPath(root, metadata.RunID, artifact)
	if err != nil {
		return ArtifactMutationResult{}, err
	}
	if operation == artifactOperationWrite {
		if err := validateArtifactTargetWritable(path); err != nil {
			return ArtifactMutationResult{}, err
		}
	}

	var sourcePath string
	var content []byte
	switch operation {
	case artifactOperationWrite, artifactOperationAppend:
		content, sourcePath, err = readArtifactSource(root, options.From)
		if err != nil {
			return ArtifactMutationResult{}, err
		}
		if operation == artifactOperationAppend {
			existing, err := readExistingArtifactForAppend(path)
			if err != nil {
				return ArtifactMutationResult{}, err
			}
			content = append(existing, content...)
		}
	case artifactOperationSetStatus:
		content, err = artifactContentWithStatus(path, options.Status, options.Reason)
		if err != nil {
			return ArtifactMutationResult{}, err
		}
	default:
		return ArtifactMutationResult{}, &Problem{Code: "artifact_operation_invalid", Message: "artifact mutation operation is not supported", Hint: "Use artifact write, artifact append, or artifact set-status.", Field: "operation", Expected: "write, append, or set-status", Actual: operation}
	}

	payload := map[string]any{
		"run_id":        metadata.RunID,
		"path":          artifact,
		"artifact_kind": artifactKindCanonical,
		"operation":     operation,
		"bytes":         len(content),
	}
	if sourcePath != "" {
		payload["source_path"] = sourcePath
	}
	status := strings.TrimSpace(options.Status)
	reason := strings.TrimSpace(options.Reason)
	if operation == artifactOperationSetStatus {
		payload["status"] = status
		if reason != "" {
			payload["reason"] = reason
		}
	}

	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: artifactWrittenEventType, RunID: metadata.RunID, Payload: payload, Now: options.Now}, func(map[string]any, string) error {
		return writeExistingFileAtomically(path, content)
	})
	if err != nil {
		return ArtifactMutationResult{}, err
	}
	return ArtifactMutationResult{RunID: metadata.RunID, Path: artifact, ArtifactKind: artifactKindCanonical, Operation: operation, Bytes: int64(len(content)), SourcePath: sourcePath, Status: status, Reason: reason, EventID: appendResult.EventID}, nil
}

func ListArtifacts(root Root, runID string) (ArtifactListResult, error) {
	if err := preflightEventCoherence(root); err != nil {
		return ArtifactListResult{}, err
	}
	metadata, _, err := ReadRunMetadata(root, runID)
	if err != nil {
		return ArtifactListResult{}, err
	}
	required := metadata.RequiredArtifacts
	if len(required) == 0 {
		required = ArtifactManifest(metadata)
	}
	artifacts, err := inspectArtifacts(root, metadata.RunID, stringSet(required))
	if err != nil {
		return ArtifactListResult{}, err
	}
	return ArtifactListResult{RunID: metadata.RunID, Artifacts: artifacts}, nil
}

func ArtifactManifest(metadata RunMetadata) []string {
	required := []string{"intake-classification.md", "acceptance-criteria.md", "test-log.md", "verification.md", "docs-update.md", "final-report.md"}
	if metadata.TaskID != nil && (strings.TrimSpace(*metadata.TaskID) == "token-001" || strings.TrimSpace(*metadata.TaskID) == "token-002") {
		required = append(required, "token-economy-evidence.json")
	}
	if metadata.TaskID != nil && multiAgentReviewTaskID(*metadata.TaskID) {
		required = append(required, "multi-agent-review/status.json")
	}
	if metadata.TaskID != nil && strings.TrimSpace(*metadata.TaskID) == policyPromotionTaskID {
		required = append(required, policyPromotionArtifact)
	}
	if metadata.TaskID != nil && strings.HasPrefix(strings.TrimSpace(*metadata.TaskID), "DESIGN-") {
		required = append(required, designEvidenceArtifact)
	}
	if metadata.WorkPath == "A_development_execution" {
		required = append(required, "sot-basis.md", "roadmap-update.md", "plan.md", "checklist.md")
	} else if metadata.WorkPath == "B_discovery_shaping" {
		required = append(required,
			"discovery/existing-docs-review.md",
			"discovery/problem-framing.md",
			"discovery/research-notes.md",
			"discovery/strategy-options.md",
			"discovery/selected-strategy.md",
			"discovery/task-breakdown.md",
			"discovery/implementation-readiness.md",
			"discovery/handoff-to-development.md",
			"sot-update.md",
			"roadmap-update.md",
		)
	}

	if metadata.WorkMode == "standard" {
		required = append(required, "task-brief.md", "prompt.md", "context-pack.md")
	}

	switch metadata.ExecutionMode {
	case "production_write", "readiness_hardening":
		required = append(required, "diff.patch", "impl-log.md", "review.md", "redteam/impl-review.md", "redteam/test-review.md", "redteam/final-gate-review.md")
	case "adapter_qa":
		required = append(required, "selected-cli.json", "capability-check.md", "bridge-session-snapshot.json", "bridge-events.md", "cli-output.md", "redteam/qa-review.md")
	case "research":
		required = append(required, "discovery/research-notes.md", "discovery/strategy-options.md", "discovery/selected-strategy.md")
	case "docs_only":
		required = append(required, "sot-update.md", "roadmap-update.md")
	case "verification":
		required = append(required, "review.md")
	}
	if metadata.BackendEvidence == BackendEvidenceRequired {
		required = append(required, backendGateArtifacts...)
	}
	if metadata.Redteam != nil {
		required = append(required, "redteam/plan-review.md", "redteam/shaping-review.md", "redteam/final-gate-review.md")
	}
	return uniqueSortedByCanonical(required)
}

func inspectArtifacts(root Root, runID string, required map[string]bool) ([]ArtifactStatus, error) {
	statuses := make([]ArtifactStatus, 0, len(canonicalArtifactPaths))
	for _, path := range canonicalArtifactPaths {
		safe, err := artifactPath(root, runID, path)
		if err != nil {
			return nil, err
		}
		status := ArtifactStatus{Path: path, Required: required[path], Exists: false, Empty: true, Bytes: 0, Action: artifactActionMissing}
		info, err := os.Lstat(safe.Absolute)
		if os.IsNotExist(err) {
			statuses = append(statuses, status)
			continue
		}
		if err != nil {
			return nil, &Problem{Code: "artifact_inspection_failed", Message: "cannot inspect artifact path", Hint: "Check run artifact permissions before initializing artifacts.", Path: safe.Relative, Field: "path", Expected: "inspectable artifact path", Actual: err.Error()}
		}
		if !info.Mode().IsRegular() {
			return nil, artifactPathInvalidProblem(safe, info, "Move the conflicting path before initializing run artifacts.")
		}
		status.Exists = true
		status.Bytes = info.Size()
		status.Empty = info.Size() == 0
		status.Action = artifactActionPresent
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func writeArtifactBaselines(root Root, metadata RunMetadata, planned []ArtifactStatus) error {
	for _, artifact := range planned {
		if artifact.Action == artifactActionPreserved {
			continue
		}
		path, err := artifactPath(root, metadata.RunID, artifact.Path)
		if err != nil {
			return err
		}
		content, err := artifactBaselineContent(metadata, artifact.Path)
		if err != nil {
			return err
		}
		if artifact.Action == artifactActionCreated {
			if err := writeNewFileAtomically(path, content); err != nil {
				return err
			}
			continue
		}
		if err := writeExistingFileAtomically(path, content); err != nil {
			return err
		}
	}
	return nil
}

func artifactPath(root Root, runID string, artifact string) (SafePath, error) {
	if !runIDPattern.MatchString(runID) {
		return SafePath{}, invalidRunField("", "run_id", "run-YYYYMMDDTHHMMSSZ-<12hex>", runID)
	}
	artifact = strings.TrimSpace(artifact)
	if !canonicalArtifactPathSet[artifact] {
		return SafePath{}, &Problem{Code: "artifact_path_invalid", Message: "artifact path is not part of the canonical run manifest", Hint: "Use artifact list to inspect helper-managed artifact paths.", Field: "path", Expected: "canonical artifact path from docs/specs.md", Actual: artifact}
	}
	return ResolveRelativePath(root, filepath.ToSlash(filepath.Join(RunRootPath, runID, artifact)))
}

func validateArtifactTargetWritable(path SafePath) error {
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return artifactInspectionProblem(path, "Check run artifact permissions before mutating.", err)
	}
	if info.Mode().IsRegular() {
		return nil
	}
	return artifactPathInvalidProblem(path, info, "Move the conflicting path before mutating run artifacts.")
}

func artifactInspectionProblem(path SafePath, hint string, err error) *Problem {
	return &Problem{Code: "artifact_inspection_failed", Message: "cannot inspect artifact path", Hint: hint, Path: path.Relative, Field: "path", Expected: "inspectable artifact path", Actual: err.Error()}
}

func artifactPathInvalidProblem(path SafePath, info os.FileInfo, hint string) *Problem {
	return &Problem{Code: "artifact_path_invalid", Message: "artifact path must be a regular file", Hint: hint, Path: path.Relative, Field: "path", Expected: "regular file or absent path", Actual: fileKind(info)}
}

func fileKind(info os.FileInfo) string {
	if info.IsDir() {
		return "directory"
	}
	return "non-regular"
}

func readArtifactSource(root Root, source string) ([]byte, string, error) {
	path, err := ResolveRelativePath(root, source)
	if err != nil {
		return nil, "", err
	}
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		return nil, "", &Problem{Code: "artifact_source_missing", Message: "artifact source file is missing", Hint: "Pass --from with an existing repository-relative regular file.", Path: path.Relative, Field: "from", Expected: "existing regular file", Actual: "missing"}
	}
	if err != nil {
		return nil, "", &Problem{Code: "artifact_source_inspection_failed", Message: "cannot inspect artifact source file", Hint: "Check source file permissions before retrying.", Path: path.Relative, Field: "from", Expected: "inspectable regular file", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		return nil, "", &Problem{Code: "artifact_source_invalid", Message: "artifact source must be a regular file", Hint: "Pass --from with a repository-relative regular file.", Path: path.Relative, Field: "from", Expected: "regular file", Actual: fileKind(info)}
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		return nil, "", &Problem{Code: "artifact_source_read_failed", Message: "cannot read artifact source file", Hint: "Check source file permissions before retrying.", Path: path.Relative, Field: "from", Expected: "readable regular file", Actual: err.Error()}
	}
	return content, path.Relative, nil
}

func readExistingArtifactForAppend(path SafePath) ([]byte, error) {
	return readOptionalArtifact(path, "appending")
}

func readOptionalArtifact(path SafePath, action string) ([]byte, error) {
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, artifactInspectionProblem(path, "Check run artifact permissions before "+action+".", err)
	}
	if !info.Mode().IsRegular() {
		return nil, artifactPathInvalidProblem(path, info, "Move the conflicting path before mutating run artifacts.")
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		return nil, &Problem{Code: "artifact_read_failed", Message: "cannot read artifact before " + action, Hint: "Check run artifact permissions before retrying.", Path: path.Relative, Field: "path", Expected: "readable regular file", Actual: err.Error()}
	}
	return content, nil
}

func artifactContentWithStatus(path SafePath, status string, reason string) ([]byte, error) {
	status = strings.TrimSpace(status)
	if !validArtifactStatus(status) {
		return nil, &Problem{Code: "artifact_status_invalid", Message: "artifact status is not supported", Hint: "Use pending, complete, or not_applicable.", Path: path.Relative, Field: "status", Expected: "pending, complete, or not_applicable", Actual: status}
	}
	if schemaOwnedJSONArtifact(path.Relative) {
		return nil, &Problem{Code: "artifact_status_not_applicable", Message: "artifact lifecycle status is not applicable to schema-owned JSON artifacts", Hint: "Use artifact write with valid JSON schema fields and rely on the matching gate for completion validation.", Path: path.Relative, Field: "path", Expected: "markdown lifecycle artifact", Actual: path.Relative}
	}
	reason = strings.TrimSpace(reason)
	if status == "not_applicable" && reason == "" {
		return nil, &Problem{Code: "artifact_reason_required", Message: "not_applicable artifact status requires a reason", Hint: "Pass --reason with KHS's explicit reason.", Path: path.Relative, Field: "reason", Expected: "non-empty reason", Actual: "missing"}
	}
	if strings.HasSuffix(path.Relative, ".patch") {
		return nil, &Problem{Code: "artifact_status_unsupported", Message: "artifact status mutation is not supported for patch artifacts", Hint: "Use artifact write for patch evidence content.", Path: path.Relative, Field: "path", Expected: ".md or .json artifact", Actual: path.Relative}
	}
	content, err := readArtifactForStatus(path)
	if err != nil {
		return nil, err
	}
	if strings.HasSuffix(path.Relative, ".json") {
		return jsonArtifactContentWithStatus(path, content, status, reason)
	}
	return markdownArtifactContentWithStatus(content, status, reason), nil
}

func validArtifactStatus(status string) bool {
	switch status {
	case "pending", "complete", "not_applicable":
		return true
	default:
		return false
	}
}

func schemaOwnedJSONArtifact(relative string) bool {
	return strings.HasSuffix(relative, "/selected-cli.json") ||
		strings.HasSuffix(relative, "/bridge-session-snapshot.json") ||
		strings.HasSuffix(relative, "/token-economy-evidence.json") ||
		strings.HasSuffix(relative, "/multi-agent-review/status.json") ||
		strings.HasSuffix(relative, "/policy-promotion-evidence.json") ||
		strings.HasSuffix(relative, "/design-evidence.json")
}

func readArtifactForStatus(path SafePath) ([]byte, error) {
	return readOptionalArtifact(path, "setting status")
}

func markdownArtifactContentWithStatus(content []byte, status string, reason string) []byte {
	if len(content) == 0 {
		content = []byte("Status: " + status + "\n")
		if status == "not_applicable" {
			content = append(content, []byte("Reason: "+reason+"\n")...)
		}
		return content
	}
	lines := strings.Split(string(content), "\n")
	statusSet := false
	reasonSet := false
	for i, line := range lines {
		key, _, ok := strings.Cut(strings.TrimSpace(line), ":")
		if !ok {
			continue
		}
		switch normalizeMarkdownFieldKey(key) {
		case "status":
			lines[i] = "Status: " + status
			statusSet = true
		case "reason":
			if status == "not_applicable" {
				lines[i] = "Reason: " + reason
				reasonSet = true
			}
		}
	}
	insert := []string{}
	if !statusSet {
		insert = append(insert, "Status: "+status)
	}
	if status == "not_applicable" && !reasonSet {
		insert = append(insert, "Reason: "+reason)
	}
	if len(insert) > 0 {
		lines = append(insert, lines...)
	}
	result := strings.Join(lines, "\n")
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	return []byte(result)
}

func jsonArtifactContentWithStatus(path SafePath, content []byte, status string, reason string) ([]byte, error) {
	payload := map[string]any{}
	if len(bytes.TrimSpace(content)) > 0 {
		if err := json.Unmarshal(content, &payload); err != nil {
			return nil, &Problem{Code: "artifact_json_invalid", Message: "cannot parse JSON artifact before setting status", Hint: "Repair the JSON artifact or use artifact write with valid JSON content.", Path: path.Relative, Field: "json", Expected: "JSON object", Actual: err.Error()}
		}
	}
	payload["status"] = status
	if status == "not_applicable" {
		payload["reason"] = reason
	} else {
		delete(payload, "reason")
	}
	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return nil, &Problem{Code: "artifact_json_encode_failed", Message: "cannot encode JSON artifact status", Hint: "Preserve stderr for diagnosis if this repeats.", Path: path.Relative, Field: "json", Expected: "JSON-encodable object", Actual: err.Error()}
	}
	return append(data, '\n'), nil
}

func artifactBaselineContent(metadata RunMetadata, artifact string) ([]byte, error) {
	if artifact == designEvidenceArtifact {
		return designEvidenceBaseline(metadata)
	}
	if strings.HasSuffix(artifact, ".json") {
		payload := map[string]any{
			"version":  "0.1",
			"status":   "pending",
			"run_id":   metadata.RunID,
			"artifact": artifact,
		}
		data, err := json.MarshalIndent(payload, "", "  ")
		if err != nil {
			return nil, &Problem{Code: "artifact_baseline_encode_failed", Message: "cannot encode artifact baseline JSON", Hint: "Retry artifact initialization and preserve stderr if the problem repeats.", Field: "artifact", Expected: "JSON-encodable baseline payload", Actual: err.Error()}
		}
		return append(data, '\n'), nil
	}
	if strings.HasSuffix(artifact, ".patch") {
		return []byte(fmt.Sprintf("# %s\n\nNo patch evidence recorded yet.\n", artifact)), nil
	}
	return []byte(fmt.Sprintf("# %s\n\nStatus: pending\nRun: %s\n\nRecord Kkachi evidence here. Use explicit not-applicable reasons when this artifact is intentionally out of scope.\n", artifact, metadata.RunID)), nil
}

func artifactPathsByAction(artifacts []ArtifactStatus, action string) []string {
	paths := []string{}
	for _, artifact := range artifacts {
		if artifact.Action == action {
			paths = append(paths, artifact.Path)
		}
	}
	return paths
}

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[value] = true
	}
	return set
}

func uniqueSortedByCanonical(values []string) []string {
	want := stringSet(values)
	result := []string{}
	for _, path := range canonicalArtifactPaths {
		if want[path] {
			result = append(result, path)
		}
	}
	return result
}

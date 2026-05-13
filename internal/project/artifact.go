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
	artifactWrittenEventType    = "artifact.written"
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
			actual := "non-regular"
			if info.IsDir() {
				actual = "directory"
			}
			return nil, &Problem{Code: "artifact_path_invalid", Message: "artifact path must be a regular file", Hint: "Move the conflicting path before initializing run artifacts.", Path: safe.Relative, Field: "path", Expected: "regular file or absent path", Actual: actual}
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

func artifactBaselineContent(metadata RunMetadata, artifact string) ([]byte, error) {
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

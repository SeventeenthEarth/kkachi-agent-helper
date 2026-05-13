package project

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

var backendGateArtifacts = []string{
	"selected-cli.json",
	"capability-check.md",
	"bridge-session-snapshot.json",
	"bridge-events.md",
}

type backendIdentity struct {
	BackendType string
	AdapterType string
}

func checkBackendGate(root Root, metadata RunMetadata, metadataRelative string) (GateCheckResult, error) {
	if !backendGateRequired(metadata) {
		return gateResultFromChecks(metadata.RunID, GateBackend, []GateCheck{{
			Name:    "backend_manifest",
			Status:  GateStatusPass,
			Path:    metadataRelative,
			Message: "backend gate is not applicable because the run manifest does not require backend artifacts",
			Field:   "backend_evidence",
			Actual:  metadata.BackendEvidence,
		}}), nil
	}

	checks := []GateCheck{checkBackendManifest(metadata, metadataRelative)}
	selectedCheck, identity := checkSelectedCLIArtifact(root, metadata.RunID)
	checks = append(checks, selectedCheck)
	checks = append(checks,
		checkCapabilityCheckArtifact(root, metadata.RunID, identity),
		checkBridgeSessionSnapshotArtifact(root, metadata.RunID, identity),
		checkBridgeEventsArtifact(root, metadata.RunID),
	)
	return gateResultFromChecks(metadata.RunID, GateBackend, checks), nil
}

func backendGateRequired(metadata RunMetadata) bool {
	if metadata.BackendEvidence == BackendEvidenceRequired {
		return true
	}
	return backendArtifactsRequired(metadata.RequiredArtifacts)
}

func backendArtifactsRequired(required []string) bool {
	requiredSet := stringSet(required)
	for _, artifact := range backendGateArtifacts {
		if requiredSet[artifact] {
			return true
		}
	}
	return false
}

func checkBackendManifest(metadata RunMetadata, metadataRelative string) GateCheck {
	requiredSet := stringSet(metadata.RequiredArtifacts)
	missing := []string{}
	for _, artifact := range backendGateArtifacts {
		if !requiredSet[artifact] {
			missing = append(missing, artifact)
		}
	}
	if len(missing) > 0 {
		return GateCheck{Name: "backend_manifest", Status: GateStatusFail, Path: metadataRelative, Message: "run manifest requires backend evidence but is missing canonical backend artifacts", Hint: "Run artifact init for an adapter_qa run or repair run-metadata.json.required_artifacts to match the manifest.", Field: "required_artifacts", Expected: strings.Join(backendGateArtifacts, ","), Actual: "missing " + strings.Join(missing, ",")}
	}
	return GateCheck{Name: "backend_manifest", Status: GateStatusPass, Path: metadataRelative, Message: "run manifest requires backend evidence artifacts", Field: "required_artifacts", Actual: strings.Join(backendGateArtifacts, ",")}
}

func checkSelectedCLIArtifact(root Root, runID string) (GateCheck, backendIdentity) {
	var identity backendIdentity
	path, content, failure, ok := readGateArtifact(root, runID, "selected-cli.json", "selected_cli")
	if !ok {
		return failure, identity
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		return GateCheck{Name: "selected_cli", Status: GateStatusFail, Path: path, Message: "selected CLI artifact must be valid JSON", Hint: "Record a selected-cli.json object with version, status, backend_type, adapter_type, source_ledger_ref, and caveats.", Field: "json", Expected: "valid JSON object", Actual: "malformed"}, identity
	}
	if payload == nil {
		return GateCheck{Name: "selected_cli", Status: GateStatusFail, Path: path, Message: "selected CLI artifact must be a JSON object", Hint: "Record selected CLI evidence as an object, not null or an array.", Field: "json", Expected: "object", Actual: "null"}, identity
	}
	_, versionOK := nonEmptyJSONString(payload, "version")
	status, statusOK := nonEmptyJSONString(payload, "status")
	backendType, backendOK := nonEmptyJSONString(payload, "backend_type")
	adapterType, adapterOK := nonEmptyJSONString(payload, "adapter_type")
	_, sourceOK := nonEmptyJSONString(payload, "source_ledger_ref")
	caveatsValue, caveatsPresent := payload["caveats"]
	caveatsOK, caveatsInvalid := validCaveats(caveatsValue, caveatsPresent)
	identity = backendIdentity{BackendType: backendType, AdapterType: adapterType}
	missing := []string{}
	if !versionOK {
		missing = append(missing, "version")
	}
	if !statusOK {
		missing = append(missing, "status")
	}
	if !backendOK {
		missing = append(missing, "backend_type")
	}
	if !adapterOK {
		missing = append(missing, "adapter_type")
	}
	if !sourceOK {
		missing = append(missing, "source_ledger_ref")
	}
	if !caveatsOK && !caveatsInvalid {
		missing = append(missing, "caveats")
	}
	if len(missing) > 0 {
		return GateCheck{Name: "selected_cli", Status: GateStatusFail, Path: path, Message: "selected CLI artifact is missing required fields", Hint: "Record all selected-cli.json fields required by the backend gate.", Field: strings.Join(missing, ","), Expected: "non-empty required selected CLI fields", Actual: "missing or invalid"}, identity
	}
	if caveatsInvalid {
		return GateCheck{Name: "selected_cli", Status: GateStatusFail, Path: path, Message: "selected CLI caveats must be a string array", Hint: "Record caveats as an array of strings; use an empty array when there are no caveats.", Field: "caveats", Expected: "array of strings", Actual: "invalid"}, identity
	}
	selectedStatus := strings.ToLower(status)
	switch selectedStatus {
	case "supported", "degraded":
		return GateCheck{Name: "selected_cli", Status: GateStatusPass, Path: path, Message: "selected CLI status is acceptable", Field: "status", Expected: "supported or degraded", Actual: selectedStatus}, identity
	case "unsupported", "pending":
		return GateCheck{Name: "selected_cli", Status: GateStatusFail, Path: path, Message: "selected CLI status cannot pass the backend gate", Hint: "Use commander-owned backend selection evidence with status supported or degraded before checking this gate.", Field: "status", Expected: "supported or degraded", Actual: selectedStatus}, identity
	default:
		return GateCheck{Name: "selected_cli", Status: GateStatusFail, Path: path, Message: "selected CLI status is invalid", Hint: "Use status supported or degraded for acceptable backend evidence.", Field: "status", Expected: "supported or degraded", Actual: selectedStatus}, identity
	}
}

func checkCapabilityCheckArtifact(root Root, runID string, identity backendIdentity) GateCheck {
	path, content, failure, ok := readGateArtifact(root, runID, "capability-check.md", "capability_check")
	if !ok {
		return failure
	}
	base := validateMarkdownGateArtifact(markdownGateArtifactRule{Name: "capability_check", Artifact: "capability-check.md"}, path, content)
	if base.Status != GateStatusPass {
		return base
	}
	text := strings.ToLower(string(content))
	missing := []string{}
	if identity.BackendType == "" || !strings.Contains(text, strings.ToLower(identity.BackendType)) {
		missing = append(missing, "backend_type")
	}
	if identity.AdapterType == "" || !strings.Contains(text, strings.ToLower(identity.AdapterType)) {
		missing = append(missing, "adapter_type")
	}
	if len(missing) > 0 {
		return GateCheck{Name: "capability_check", Status: GateStatusFail, Path: path, Message: "capability check does not link to the selected backend identity", Hint: "Mention the selected backend_type and adapter_type in capability-check.md.", Field: strings.Join(missing, ","), Expected: identity.BackendType + "/" + identity.AdapterType, Actual: "missing selected identity link"}
	}
	return GateCheck{Name: "capability_check", Status: GateStatusPass, Path: path, Message: "capability check is complete and links to selected backend identity"}
}

func checkBridgeSessionSnapshotArtifact(root Root, runID string, identity backendIdentity) GateCheck {
	path, content, failure, ok := readGateArtifact(root, runID, "bridge-session-snapshot.json", "bridge_session_snapshot")
	if !ok {
		return failure
	}
	var payload map[string]any
	if err := json.Unmarshal(content, &payload); err != nil {
		return GateCheck{Name: "bridge_session_snapshot", Status: GateStatusFail, Path: path, Message: "bridge session snapshot must be valid JSON", Hint: "Record a bridge-session-snapshot.json object with session identity and no open pendings.", Field: "json", Expected: "valid JSON object", Actual: "malformed"}
	}
	if payload == nil {
		return GateCheck{Name: "bridge_session_snapshot", Status: GateStatusFail, Path: path, Message: "bridge session snapshot must be a JSON object", Hint: "Record bridge snapshot evidence as an object.", Field: "json", Expected: "object", Actual: "null"}
	}
	_, sessionOK := nonEmptyJSONString(payload, "session_id")
	backendType, backendOK := nonEmptyJSONString(payload, "backend_type")
	adapterType, adapterOK := nonEmptyJSONString(payload, "adapter_type")
	_, stateOK := nonEmptyJSONString(payload, "state")
	_, lifecycleOK := nonEmptyJSONString(payload, "lifecycle_class")
	openPendings, pendingsOK := jsonNumberAsInt(payload["open_pendings"])
	missing := []string{}
	if !sessionOK {
		missing = append(missing, "session_id")
	}
	if !backendOK {
		missing = append(missing, "backend_type")
	}
	if !adapterOK {
		missing = append(missing, "adapter_type")
	}
	if !stateOK {
		missing = append(missing, "state")
	}
	if !lifecycleOK {
		missing = append(missing, "lifecycle_class")
	}
	if !pendingsOK {
		missing = append(missing, "open_pendings")
	}
	if len(missing) > 0 {
		return GateCheck{Name: "bridge_session_snapshot", Status: GateStatusFail, Path: path, Message: "bridge session snapshot is missing required fields", Hint: "Record session_id, backend_type, adapter_type, state, lifecycle_class, and open_pendings.", Field: strings.Join(missing, ","), Expected: "non-empty required bridge snapshot fields", Actual: "missing or invalid"}
	}
	if backendType != identity.BackendType || adapterType != identity.AdapterType {
		return GateCheck{Name: "bridge_session_snapshot", Status: GateStatusFail, Path: path, Message: "bridge session snapshot identity does not match selected CLI", Hint: "Record bridge snapshot evidence from the same backend_type and adapter_type selected in selected-cli.json.", Field: "backend_type,adapter_type", Expected: identity.BackendType + "/" + identity.AdapterType, Actual: backendType + "/" + adapterType}
	}
	if openPendings != 0 {
		return GateCheck{Name: "bridge_session_snapshot", Status: GateStatusFail, Path: path, Message: "bridge session snapshot has open pendings", Hint: "Resolve or close bridge pending operations before passing the backend gate.", Field: "open_pendings", Expected: "0", Actual: fmt.Sprintf("%d", openPendings)}
	}
	return GateCheck{Name: "bridge_session_snapshot", Status: GateStatusPass, Path: path, Message: "bridge session snapshot matches selected backend identity and has no open pendings"}
}

func checkBridgeEventsArtifact(root Root, runID string) GateCheck {
	path, content, failure, ok := readGateArtifact(root, runID, "bridge-events.md", "bridge_events")
	if !ok {
		return failure
	}
	base := validateMarkdownGateArtifact(markdownGateArtifactRule{Name: "bridge_events", Artifact: "bridge-events.md"}, path, content)
	if base.Status != GateStatusPass {
		return base
	}
	if !hasMarkdownEvidenceBody(content) {
		return GateCheck{Name: "bridge_events", Status: GateStatusFail, Path: path, Message: "bridge events artifact lacks bridge behavior evidence", Hint: "Record non-empty bridge behavior evidence after Status: complete.", Field: "content", Expected: "non-empty bridge behavior evidence", Actual: "missing"}
	}
	return GateCheck{Name: "bridge_events", Status: GateStatusPass, Path: path, Message: "bridge events artifact is complete and records bridge behavior evidence"}
}

func readGateArtifact(root Root, runID string, artifact string, checkName string) (string, []byte, GateCheck, bool) {
	path, err := artifactPath(root, runID, artifact)
	if err != nil {
		return "", nil, GateCheck{Name: checkName, Status: GateStatusFail, Message: "gate artifact path is invalid", Hint: "Use artifact init to create canonical artifact paths.", Field: "path", Expected: artifact, Actual: err.Error()}, false
	}
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		return path.Relative, nil, GateCheck{Name: checkName, Status: GateStatusFail, Path: path.Relative, Message: "required gate artifact is missing", Hint: "Run artifact init, then record the required backend evidence.", Field: "path", Expected: "existing regular file", Actual: "missing"}, false
	}
	if err != nil {
		return path.Relative, nil, GateCheck{Name: checkName, Status: GateStatusFail, Path: path.Relative, Message: "cannot inspect gate artifact", Hint: "Check run artifact permissions before checking gates.", Field: "path", Expected: "inspectable regular file", Actual: err.Error()}, false
	}
	if !info.Mode().IsRegular() {
		actual := "non-regular"
		if info.IsDir() {
			actual = "directory"
		}
		return path.Relative, nil, GateCheck{Name: checkName, Status: GateStatusFail, Path: path.Relative, Message: "gate artifact must be a regular file", Hint: "Move the conflicting path and re-run artifact init.", Field: "path", Expected: "regular file", Actual: actual}, false
	}
	if info.Size() == 0 {
		return path.Relative, nil, GateCheck{Name: checkName, Status: GateStatusFail, Path: path.Relative, Message: "gate artifact is empty", Hint: "Record the required backend evidence before checking this gate.", Field: "path", Expected: "non-empty file", Actual: "empty"}, false
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		return path.Relative, nil, GateCheck{Name: checkName, Status: GateStatusFail, Path: path.Relative, Message: "cannot read gate artifact", Hint: "Check run artifact permissions before checking gates.", Field: "path", Expected: "readable file", Actual: err.Error()}, false
	}
	return path.Relative, content, GateCheck{}, true
}

func validCaveats(value any, present bool) (bool, bool) {
	if !present {
		return false, false
	}
	if value == nil {
		return false, true
	}
	items, ok := value.([]any)
	if !ok {
		return false, true
	}
	for _, item := range items {
		if _, ok := item.(string); !ok {
			return false, true
		}
	}
	return true, false
}

func nonEmptyJSONString(payload map[string]any, field string) (string, bool) {
	value, ok := payload[field]
	if !ok {
		return "", false
	}
	text, ok := value.(string)
	if !ok {
		return "", false
	}
	return strings.TrimSpace(text), strings.TrimSpace(text) != ""
}

func jsonNumberAsInt(value any) (int, bool) {
	switch typed := value.(type) {
	case float64:
		if typed != float64(int(typed)) {
			return 0, false
		}
		return int(typed), true
	case int:
		return typed, true
	default:
		return 0, false
	}
}

func hasMarkdownEvidenceBody(content []byte) bool {
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "status:") || strings.HasPrefix(lower, "run:") || strings.HasPrefix(lower, "reason:") {
			continue
		}
		if strings.Contains(lower, "record kkachi evidence here") || strings.Contains(lower, "use explicit not-applicable reasons") || (strings.Contains(lower, "no ") && strings.Contains(lower, " recorded yet")) {
			continue
		}
		return true
	}
	return false
}

package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

const (
	WorkflowCatalogProposalSchemaVersion = "workflow-catalog-proposal/v1"

	workflowCatalogProposalDir        = ".kkachi/workflow-catalog/proposals"
	workflowCatalogPromotionBackupDir = ".kkachi/backups/workflow-catalog-promotions"
	workflowCatalogProposalEventType  = "workflow_catalog.proposal_recorded"
	workflowCatalogApplyEventType     = "workflow_catalog.applied"
	workflowCatalogNextActionProposal = "Record hash-bound approval evidence, then run workflow catalog apply with --proposal, --approval, and --proposal-hash."
	workflowCatalogNextActionApplied  = "Workflow catalog proposal applied; rerun workflow catalog validate or workflow create with an explicit workflow id."

	kasWorkflowPromotePacketSchema = "kas-workflow-promote-packet/v1"
	workflowCatalogMissingChecksum = "missing"
)

var (
	workflowCatalogProposalIDPattern = regexp.MustCompile(`^wcat-prop-(\d{6})\.json$`)
	workflowCatalogHashPattern       = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

type WorkflowCatalogProposeOptions struct {
	Packet string
	Reason string
	Now    func() time.Time
}

type WorkflowCatalogApplyOptions struct {
	Proposal     string
	Approval     string
	ProposalHash string
	Now          func() time.Time
}

type WorkflowCatalogProposalResult struct {
	SchemaVersion        string                               `json:"schema_version"`
	Status               string                               `json:"status"`
	ProposalID           string                               `json:"proposal_id"`
	ProposalPath         string                               `json:"proposal_path"`
	ProposalHash         string                               `json:"proposal_hash"`
	TargetPaths          []string                             `json:"target_paths"`
	BaseChecksums        map[string]string                    `json:"base_checksums"`
	CandidateChecksums   map[string]string                    `json:"candidate_checksums"`
	ChangedPaths         []WorkflowCatalogChangedPath         `json:"changed_paths"`
	ApprovalRequired     bool                                 `json:"approval_required"`
	ApprovalRequirements WorkflowCatalogApprovalRequirements  `json:"approval_requirements"`
	SourcePacket         WorkflowCatalogSourcePacket          `json:"source_packet"`
	NoWrite              WorkflowCatalogNoWriteEvidence       `json:"no_write"`
	ValidationSummary    WorkflowCatalogProposalValidation    `json:"validation_summary"`
	Conflicts            []WorkflowCatalogPromotionConflict   `json:"conflicts"`
	Diagnostics          []WorkflowCatalogPromotionDiagnostic `json:"diagnostics"`
	EventID              string                               `json:"event_id,omitempty"`
	NextAction           string                               `json:"next_action"`
}

type WorkflowCatalogApplyResult struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	ProposalID    string            `json:"proposal_id"`
	ApprovalRef   string            `json:"approval_ref"`
	ProposalHash  string            `json:"proposal_hash"`
	AppliedPaths  []string          `json:"applied_paths"`
	BackupPaths   map[string]string `json:"backup_paths"`
	RecoveryRefs  map[string]string `json:"recovery_refs"`
	NewChecksums  map[string]string `json:"new_checksums"`
	EventIDs      []string          `json:"event_ids"`
	NextAction    string            `json:"next_action"`
}

type WorkflowCatalogProposalRecord struct {
	SchemaVersion        string                               `json:"schema_version"`
	Status               string                               `json:"status"`
	ProposalID           string                               `json:"proposal_id"`
	ProposalPath         string                               `json:"proposal_path"`
	ProposalHash         string                               `json:"proposal_hash"`
	CreatedAt            string                               `json:"created_at"`
	Reason               string                               `json:"reason"`
	SourcePacket         WorkflowCatalogSourcePacket          `json:"source_packet"`
	TargetPaths          []string                             `json:"target_paths"`
	BaseChecksums        map[string]string                    `json:"base_checksums"`
	CandidateChecksums   map[string]string                    `json:"candidate_checksums"`
	Candidates           []WorkflowCatalogCandidateContent    `json:"candidates"`
	ChangedPaths         []WorkflowCatalogChangedPath         `json:"changed_paths"`
	ValidationSummary    WorkflowCatalogProposalValidation    `json:"validation_summary"`
	Conflicts            []WorkflowCatalogPromotionConflict   `json:"conflicts"`
	Diagnostics          []WorkflowCatalogPromotionDiagnostic `json:"diagnostics"`
	ApprovalRequired     bool                                 `json:"approval_required"`
	ApprovalRequirements WorkflowCatalogApprovalRequirements  `json:"approval_requirements"`
	NoWrite              WorkflowCatalogNoWriteEvidence       `json:"no_write"`
	NextAction           string                               `json:"next_action"`
}

type WorkflowCatalogSourcePacket struct {
	SchemaVersion    string `json:"schema_version"`
	Path             string `json:"path"`
	ApprovalHash     string `json:"approval_hash,omitempty"`
	Canonicalization string `json:"canonicalization,omitempty"`
}

type WorkflowCatalogCandidateContent struct {
	Path    string `json:"path"`
	Kind    string `json:"kind"`
	Content string `json:"content"`
	SHA256  string `json:"sha256"`
}

type WorkflowCatalogChangedPath struct {
	Path   string `json:"path"`
	Action string `json:"action"`
	Kind   string `json:"kind"`
}

type WorkflowCatalogPromotionConflict struct {
	Code    string `json:"code"`
	Path    string `json:"path,omitempty"`
	Message string `json:"message"`
}

type WorkflowCatalogPromotionDiagnostic struct {
	Level   string `json:"level"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Path    string `json:"path,omitempty"`
	Field   string `json:"field,omitempty"`
}

type WorkflowCatalogNoWriteEvidence struct {
	Guaranteed                   bool `json:"guaranteed"`
	ProjectWriteCount            int  `json:"project_write_count"`
	CandidateFileWriteCount      int  `json:"candidate_file_write_count"`
	KAHStateWriteCount           int  `json:"kah_state_write_count"`
	KABRuntimeMutationCount      int  `json:"kab_runtime_mutation_count"`
	HermesRuntimeMutationCount   int  `json:"hermes_runtime_mutation_count"`
	ProfileWriteCount            int  `json:"profile_write_count"`
	AuthProviderConfigWriteCount int  `json:"auth_provider_config_write_count"`
}

type WorkflowCatalogProposalValidation struct {
	WorkflowDAG          TaskDAGResult              `json:"workflow_dag"`
	Catalog              WorkflowCatalogResult      `json:"catalog"`
	NodeContractRegistry NodeContractRegistryResult `json:"node_contract_registry"`
}

type WorkflowCatalogApprovalRequirements struct {
	Required                          bool   `json:"required"`
	ProposalHashRequired              bool   `json:"proposal_hash_required"`
	ProposalHash                      string `json:"proposal_hash"`
	SourcePacketApprovalHash          string `json:"source_packet_approval_hash,omitempty"`
	HashBoundApprovalEvidenceRequired bool   `json:"hash_bound_approval_evidence_required"`
}

type workflowCatalogPacketEnvelope struct {
	MachinePacket workflowCatalogKASPacket `json:"machine_packet"`
}

type workflowCatalogKASPacket struct {
	SchemaVersion    string                               `json:"schema_version"`
	Canonicalization string                               `json:"canonicalization"`
	TargetPaths      []string                             `json:"target_paths"`
	CandidatePaths   workflowCatalogKASCandidatePaths     `json:"candidate_paths"`
	GeneratedContent []WorkflowCatalogCandidateContent    `json:"generated_content"`
	BaseChecksums    map[string]string                    `json:"base_checksums"`
	ChangedPaths     []WorkflowCatalogChangedPath         `json:"changed_paths"`
	Conflicts        []WorkflowCatalogPromotionConflict   `json:"conflicts"`
	Diagnostics      []WorkflowCatalogPromotionDiagnostic `json:"diagnostics"`
	NoWrite          WorkflowCatalogNoWriteEvidence       `json:"no_write"`
	ApprovalHash     string                               `json:"approval_hash"`
}

type workflowCatalogKASCandidatePaths struct {
	WorkflowDAG          string `json:"workflow_dag"`
	Catalog              string `json:"catalog"`
	NodeContractRegistry string `json:"node_contract_registry"`
	TriggerSkill         string `json:"trigger_skill,omitempty"`
}

func ProposeWorkflowCatalogPromotion(root Root, options WorkflowCatalogProposeOptions) (WorkflowCatalogProposalResult, error) {
	var result WorkflowCatalogProposalResult
	err := withProjectWriteLock(root, "workflow catalog propose", "", func() error {
		var err error
		result, err = proposeWorkflowCatalogPromotionUnlocked(root, options)
		return err
	})
	return result, err
}

func proposeWorkflowCatalogPromotionUnlocked(root Root, options WorkflowCatalogProposeOptions) (WorkflowCatalogProposalResult, error) {
	packetRef := strings.TrimSpace(options.Packet)
	reason := strings.TrimSpace(options.Reason)
	if packetRef == "" {
		return WorkflowCatalogProposalResult{}, &Problem{Code: "workflow_catalog_packet_required", Message: "workflow catalog proposal packet is required", Hint: "Pass --packet with a repository-relative KAS WFLOW-009 promote packet.", Field: "packet", Expected: "non-empty packet path", Actual: "empty"}
	}
	if reason == "" {
		return WorkflowCatalogProposalResult{}, &Problem{Code: "workflow_catalog_proposal_reason_required", Message: "workflow catalog proposal reason is required", Hint: "Pass --reason to explain why KAS supplied this promotion candidate.", Field: "reason", Expected: "non-empty proposal reason", Actual: "empty"}
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if err := preflightEventCoherence(root); err != nil {
		return WorkflowCatalogProposalResult{}, err
	}
	packet, source, err := readWorkflowCatalogPromotionPacket(root, packetRef)
	if err != nil {
		return WorkflowCatalogProposalResult{}, err
	}
	if err := validateWorkflowCatalogPromotionPacket(packet, source.Path); err != nil {
		return WorkflowCatalogProposalResult{}, err
	}
	targetPaths, candidates, candidateChecksums, err := normalizeWorkflowCatalogCandidates(root, packet)
	if err != nil {
		return WorkflowCatalogProposalResult{}, err
	}
	baseChecksums, err := workflowCatalogCurrentChecksums(root, targetPaths)
	if err != nil {
		return WorkflowCatalogProposalResult{}, err
	}
	if err := compareWorkflowCatalogBaseChecksums(packet.BaseChecksums, baseChecksums); err != nil {
		return WorkflowCatalogProposalResult{}, err
	}
	validation, err := validateWorkflowCatalogCandidates(candidates, packet.CandidatePaths)
	if err != nil {
		return WorkflowCatalogProposalResult{}, err
	}
	proposalHash := packet.ApprovalHash
	if proposalHash == "" {
		proposalHash = canonicalWorkflowCatalogProposalHash(source, targetPaths, baseChecksums, candidateChecksums, packet.ChangedPaths)
	}
	if err := validateWorkflowCatalogHash("proposal_hash", proposalHash); err != nil {
		return WorkflowCatalogProposalResult{}, err
	}

	proposalID, proposalPath, err := nextWorkflowCatalogProposalPath(root)
	if err != nil {
		return WorkflowCatalogProposalResult{}, err
	}
	created := options.Now().UTC()
	approval := WorkflowCatalogApprovalRequirements{
		Required:                          true,
		ProposalHashRequired:              true,
		ProposalHash:                      proposalHash,
		SourcePacketApprovalHash:          packet.ApprovalHash,
		HashBoundApprovalEvidenceRequired: true,
	}
	noWrite := packet.NoWrite
	record := WorkflowCatalogProposalRecord{
		SchemaVersion:        WorkflowCatalogProposalSchemaVersion,
		Status:               WorkflowCatalogStatusPass,
		ProposalID:           proposalID,
		ProposalPath:         proposalPath.Relative,
		ProposalHash:         proposalHash,
		CreatedAt:            created.Format(time.RFC3339),
		Reason:               reason,
		SourcePacket:         source,
		TargetPaths:          targetPaths,
		BaseChecksums:        baseChecksums,
		CandidateChecksums:   candidateChecksums,
		Candidates:           candidates,
		ChangedPaths:         sortedWorkflowCatalogChangedPaths(packet.ChangedPaths),
		ValidationSummary:    validation,
		Conflicts:            packet.Conflicts,
		Diagnostics:          packet.Diagnostics,
		ApprovalRequired:     true,
		ApprovalRequirements: approval,
		NoWrite:              noWrite,
		NextAction:           workflowCatalogNextActionProposal,
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return WorkflowCatalogProposalResult{}, &Problem{Code: "workflow_catalog_proposal_encode_failed", Message: "cannot encode workflow catalog proposal record", Hint: "Inspect proposal fields for unsupported JSON values.", Field: "proposal", Expected: "JSON-encodable proposal record", Actual: err.Error()}
	}
	data = append(data, '\n')
	payload := map[string]any{
		"proposal_id":                  proposalID,
		"proposal_path":                proposalPath.Relative,
		"proposal_hash":                proposalHash,
		"source_packet":                source,
		"target_paths":                 targetPaths,
		"base_checksums":               baseChecksums,
		"candidate_checksums":          candidateChecksums,
		"approval_required":            true,
		"proposal_hash_required":       true,
		"hash_bound_approval_evidence": approval.HashBoundApprovalEvidenceRequired,
		"reason":                       reason,
		"target_file_write_count":      noWrite.ProjectWriteCount,
		"candidate_file_write_count":   noWrite.CandidateFileWriteCount,
		"kah_state_write_count":        noWrite.KAHStateWriteCount,
	}
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: workflowCatalogProposalEventType, Payload: payload, Now: func() time.Time { return created }}, func(map[string]any, string) error {
		return writeNewFileAtomically(proposalPath, data)
	})
	if err != nil {
		return WorkflowCatalogProposalResult{}, err
	}
	return WorkflowCatalogProposalResult{
		SchemaVersion:        record.SchemaVersion,
		Status:               record.Status,
		ProposalID:           record.ProposalID,
		ProposalPath:         record.ProposalPath,
		ProposalHash:         record.ProposalHash,
		TargetPaths:          record.TargetPaths,
		BaseChecksums:        record.BaseChecksums,
		CandidateChecksums:   record.CandidateChecksums,
		ChangedPaths:         record.ChangedPaths,
		ApprovalRequired:     record.ApprovalRequired,
		ApprovalRequirements: record.ApprovalRequirements,
		SourcePacket:         record.SourcePacket,
		NoWrite:              record.NoWrite,
		ValidationSummary:    record.ValidationSummary,
		Conflicts:            record.Conflicts,
		Diagnostics:          record.Diagnostics,
		EventID:              appendResult.EventID,
		NextAction:           record.NextAction,
	}, nil
}

func ApplyWorkflowCatalogPromotion(root Root, options WorkflowCatalogApplyOptions) (WorkflowCatalogApplyResult, error) {
	var result WorkflowCatalogApplyResult
	err := withProjectWriteLock(root, "workflow catalog apply", "", func() error {
		var err error
		result, err = applyWorkflowCatalogPromotionUnlocked(root, options)
		return err
	})
	return result, err
}

func applyWorkflowCatalogPromotionUnlocked(root Root, options WorkflowCatalogApplyOptions) (WorkflowCatalogApplyResult, error) {
	proposalID := strings.TrimSpace(options.Proposal)
	approvalRef := strings.TrimSpace(options.Approval)
	proposalHash := strings.TrimSpace(options.ProposalHash)
	if proposalID == "" {
		return WorkflowCatalogApplyResult{}, &Problem{Code: "workflow_catalog_proposal_required", Message: "workflow catalog apply proposal is required", Hint: "Pass --proposal with a workflow catalog proposal id such as wcat-prop-000001.", Field: "proposal", Expected: "non-empty proposal id", Actual: "empty"}
	}
	if !isWorkflowCatalogProposalID(proposalID) {
		return WorkflowCatalogApplyResult{}, &Problem{Code: "workflow_catalog_proposal_invalid", Message: "workflow catalog proposal id is invalid", Hint: "Use a proposal id returned by workflow catalog propose, such as wcat-prop-000001.", Field: "proposal", Expected: "wcat-prop- followed by six digits", Actual: proposalID}
	}
	if approvalRef == "" {
		return WorkflowCatalogApplyResult{}, &Problem{Code: "workflow_catalog_apply_requires_approval", Message: "workflow catalog apply approval evidence is required", Hint: "Pass --approval with hash-bound approval evidence from the KAS WFLOW-009 review.", Field: "approval", Expected: "non-empty approval evidence reference", Actual: "empty"}
	}
	if proposalHash == "" {
		return WorkflowCatalogApplyResult{}, &Problem{Code: "workflow_catalog_proposal_hash_required", Message: "workflow catalog apply proposal hash is required", Hint: "Pass --proposal-hash sha256:<64hex> from workflow catalog propose.", Field: "proposal_hash", Expected: "sha256:<64hex>", Actual: "missing"}
	}
	if err := validateWorkflowCatalogHash("proposal_hash", proposalHash); err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	if err := preflightEventCoherence(root); err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	record, proposalPath, err := readWorkflowCatalogProposalRecord(root, proposalID)
	if err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	if err := validateWorkflowCatalogProposalRecord(record, proposalID, proposalPath.Relative); err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	if proposalHash != record.ProposalHash {
		return WorkflowCatalogApplyResult{}, &Problem{Code: "workflow_catalog_proposal_hash_mismatch", Message: "workflow catalog proposal hash does not match the proposal record", Hint: "Use the exact proposal_hash emitted by workflow catalog propose.", Path: proposalPath.Relative, Field: "proposal_hash", Expected: record.ProposalHash, Actual: proposalHash}
	}
	if err := validateWorkflowCatalogSourceApprovalBinding(root, record, proposalPath.Relative, approvalRef); err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	currentChecksums, err := workflowCatalogCurrentChecksums(root, record.TargetPaths)
	if err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	if err := compareWorkflowCatalogBaseChecksums(record.BaseChecksums, currentChecksums); err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	if err := validateWorkflowCatalogCandidateChecksums(record.Candidates, record.CandidateChecksums); err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	validation, err := validateWorkflowCatalogCandidates(record.Candidates, workflowCatalogCandidatePathsFromRecord(record))
	if err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	if !workflowCatalogValidationOK(validation) {
		return WorkflowCatalogApplyResult{}, &Problem{Code: "workflow_catalog_candidate_invalid", Message: "workflow catalog candidate content no longer validates", Hint: "Record a fresh proposal with valid workflow DAG, catalog, and node-contract registry content.", Path: proposalPath.Relative, Field: "candidate", Expected: "valid candidate content", Actual: "invalid"}
	}
	targets, err := workflowCatalogCandidateTargetPaths(root, record.Candidates)
	if err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	backupPaths := map[string]string{}
	recoveryRefs := map[string]string{}
	newChecksums := map[string]string{}
	appliedPaths := append([]string{}, record.TargetPaths...)
	sort.Strings(appliedPaths)
	appendResult, err := appendEventWithPreparedStatusMutation(root, AppendEventOptions{Type: workflowCatalogApplyEventType, Now: options.Now}, func(_ map[string]any, nextID string, _ string) (preparedEventStatusMutation, error) {
		for _, target := range targets {
			checksum := workflowCatalogChecksum([]byte(target.content))
			newChecksums[target.path.Relative] = checksum
			if workflowCatalogFileExists(target.path) {
				backup, err := ResolveRelativePath(root, fmt.Sprintf("%s/%s/%s-%s", workflowCatalogPromotionBackupDir, record.ProposalID, nextID, filepath.Base(target.path.Relative)))
				if err != nil {
					return preparedEventStatusMutation{}, err
				}
				backupPaths[target.path.Relative] = backup.Relative
				recoveryRefs[target.path.Relative] = backup.Relative + "#restore-original-workflow-catalog-target"
			}
		}
		payload := map[string]any{
			"proposal_id":         record.ProposalID,
			"proposal_path":       record.ProposalPath,
			"proposal_hash":       record.ProposalHash,
			"approval_ref":        approvalRef,
			"source_packet":       record.SourcePacket,
			"applied_paths":       appliedPaths,
			"base_checksums":      record.BaseChecksums,
			"candidate_checksums": record.CandidateChecksums,
			"new_checksums":       newChecksums,
			"backup_paths":        backupPaths,
			"recovery_refs":       recoveryRefs,
			"proposal_created_at": record.CreatedAt,
			"proposal_reason":     record.Reason,
		}
		return preparedEventStatusMutation{
			Payload: payload,
			BeforeAppend: func() error {
				for _, target := range targets {
					if backupRelative := backupPaths[target.path.Relative]; backupRelative != "" {
						if err := writeWorkflowCatalogPromotionBackup(root, target.path, backupRelative); err != nil {
							return err
						}
					}
				}
				for _, target := range targets {
					if workflowCatalogFileExists(target.path) {
						if err := writeExistingFileAtomically(target.path, []byte(target.content)); err != nil {
							return err
						}
					} else {
						if err := writeNewFileAtomically(target.path, []byte(target.content)); err != nil {
							return err
						}
					}
				}
				return nil
			},
		}, nil
	})
	if err != nil {
		return WorkflowCatalogApplyResult{}, err
	}
	return WorkflowCatalogApplyResult{
		SchemaVersion: WorkflowCatalogProposalSchemaVersion,
		Status:        WorkflowCatalogStatusPass,
		ProposalID:    record.ProposalID,
		ApprovalRef:   approvalRef,
		ProposalHash:  proposalHash,
		AppliedPaths:  appliedPaths,
		BackupPaths:   backupPaths,
		RecoveryRefs:  recoveryRefs,
		NewChecksums:  newChecksums,
		EventIDs:      []string{appendResult.EventID},
		NextAction:    workflowCatalogNextActionApplied,
	}, nil
}

type workflowCatalogCandidateTarget struct {
	path    SafePath
	content string
}

func readWorkflowCatalogPromotionPacket(root Root, packetRef string) (workflowCatalogKASPacket, WorkflowCatalogSourcePacket, error) {
	path, err := ResolveRelativePath(root, packetRef)
	if err != nil {
		return workflowCatalogKASPacket{}, WorkflowCatalogSourcePacket{}, &Problem{Code: "workflow_catalog_packet_unsafe_path", Message: "workflow catalog promotion packet path is unsafe", Hint: "Use a repository-confined KAS WFLOW-009 packet path.", Path: packetRef, Field: "packet", Expected: "repository-confined packet path", Actual: err.Error()}
	}
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return workflowCatalogKASPacket{}, WorkflowCatalogSourcePacket{}, &Problem{Code: "workflow_catalog_packet_read_failed", Message: "cannot read workflow catalog promotion packet", Hint: "Check the KAS WFLOW-009 packet path before proposing.", Path: path.Relative, Field: "packet", Expected: "readable JSON packet", Actual: err.Error()}
	}
	var direct workflowCatalogKASPacket
	if err := json.Unmarshal(data, &direct); err != nil {
		return workflowCatalogKASPacket{}, WorkflowCatalogSourcePacket{}, &Problem{Code: "workflow_catalog_packet_invalid_json", Message: "workflow catalog promotion packet is not valid JSON", Hint: "Pass the JSON packet emitted by KAS WFLOW-009.", Path: path.Relative, Field: "packet", Expected: "valid JSON", Actual: err.Error()}
	}
	packet := direct
	if direct.SchemaVersion == "" {
		var envelope workflowCatalogPacketEnvelope
		if err := json.Unmarshal(data, &envelope); err != nil {
			return workflowCatalogKASPacket{}, WorkflowCatalogSourcePacket{}, &Problem{Code: "workflow_catalog_packet_invalid_json", Message: "workflow catalog promotion packet envelope is not valid JSON", Hint: "Pass either the KAS machine_packet object or the full KAS dry-run JSON containing machine_packet.", Path: path.Relative, Field: "packet", Expected: "valid KAS packet envelope", Actual: err.Error()}
		}
		packet = envelope.MachinePacket
	}
	packet.ApprovalHash = strings.TrimSpace(packet.ApprovalHash)
	packet.Canonicalization = strings.TrimSpace(packet.Canonicalization)
	return packet, WorkflowCatalogSourcePacket{SchemaVersion: packet.SchemaVersion, Path: path.Relative, ApprovalHash: packet.ApprovalHash, Canonicalization: packet.Canonicalization}, nil
}

func validateWorkflowCatalogPromotionPacket(packet workflowCatalogKASPacket, packetPath string) error {
	if packet.SchemaVersion != kasWorkflowPromotePacketSchema {
		return &Problem{Code: "workflow_catalog_schema_unsupported", Message: "workflow catalog promotion packet schema is unsupported", Hint: "Use a KAS WFLOW-009 packet with schema_version kas-workflow-promote-packet/v1.", Path: packetPath, Field: "schema_version", Expected: kasWorkflowPromotePacketSchema, Actual: packet.SchemaVersion}
	}
	if strings.TrimSpace(packet.ApprovalHash) == "" {
		return &Problem{Code: "workflow_catalog_approval_hash_required", Message: "workflow catalog promotion packet lacks hash-bound approval evidence", Hint: "Regenerate the KAS WFLOW-009 packet with approval_hash populated so KAH apply can require an exact --proposal-hash match.", Path: packetPath, Field: "approval_hash", Expected: "sha256:<64hex>", Actual: "missing"}
	}
	packet.ApprovalHash = strings.TrimSpace(packet.ApprovalHash)
	if err := validateWorkflowCatalogHash("approval_hash", packet.ApprovalHash); err != nil {
		return err
	}
	if len(packet.Conflicts) > 0 {
		return &Problem{Code: "workflow_catalog_conflicts_present", Message: "workflow catalog promotion packet contains conflicts", Hint: "Resolve KAS WFLOW-009 conflicts before recording a KAH proposal.", Path: packetPath, Field: "conflicts", Expected: "no conflicts", Actual: fmt.Sprintf("%d conflicts", len(packet.Conflicts))}
	}
	for _, diagnostic := range packet.Diagnostics {
		if strings.EqualFold(strings.TrimSpace(diagnostic.Level), "error") {
			return &Problem{Code: "workflow_catalog_diagnostics_present", Message: "workflow catalog promotion packet contains error diagnostics", Hint: "Resolve KAS WFLOW-009 diagnostics before recording a KAH proposal.", Path: diagnostic.Path, Field: diagnostic.Field, Expected: "no error diagnostics", Actual: diagnostic.Code}
		}
	}
	if !packet.NoWrite.Guaranteed || packet.NoWrite.ProjectWriteCount != 0 || packet.NoWrite.CandidateFileWriteCount != 0 || packet.NoWrite.KAHStateWriteCount != 0 || packet.NoWrite.KABRuntimeMutationCount != 0 || packet.NoWrite.HermesRuntimeMutationCount != 0 || packet.NoWrite.ProfileWriteCount != 0 || packet.NoWrite.AuthProviderConfigWriteCount != 0 {
		return &Problem{Code: "workflow_catalog_packet_not_no_write", Message: "workflow catalog promotion packet does not prove a no-write posture", Hint: "Use a KAS WFLOW-009 dry-run packet with zero project, candidate-file, KAH, KAB, Hermes, profile, auth, or provider writes.", Path: packetPath, Field: "no_write", Expected: "guaranteed no-write evidence with all write counts at zero", Actual: "not guaranteed"}
	}
	return nil
}

func normalizeWorkflowCatalogCandidates(root Root, packet workflowCatalogKASPacket) ([]string, []WorkflowCatalogCandidateContent, map[string]string, error) {
	if len(packet.GeneratedContent) == 0 {
		return nil, nil, nil, &Problem{Code: "workflow_catalog_candidate_invalid", Message: "workflow catalog promotion packet lacks generated content", Hint: "KAS WFLOW-009 must supply complete target file content.", Field: "generated_content", Expected: "workflow DAG, catalog, and node-contract registry content", Actual: "empty"}
	}
	allowedKinds := map[string]bool{"workflow_dag": true, "workflow_catalog": true, "node_contract_registry": true, "trigger_skill": true}
	seenPaths := map[string]bool{}
	seenKinds := map[string]bool{}
	candidates := append([]WorkflowCatalogCandidateContent{}, packet.GeneratedContent...)
	for i := range candidates {
		candidates[i].Path = filepath.ToSlash(strings.TrimSpace(candidates[i].Path))
		candidates[i].Kind = strings.TrimSpace(candidates[i].Kind)
		candidates[i].SHA256 = strings.TrimSpace(candidates[i].SHA256)
		if candidates[i].Path == "" || candidates[i].Kind == "" {
			return nil, nil, nil, &Problem{Code: "workflow_catalog_candidate_invalid", Message: "workflow catalog candidate path and kind are required", Hint: "Regenerate the KAS WFLOW-009 packet with complete generated_content entries.", Field: "generated_content", Expected: "path and kind", Actual: "missing"}
		}
		if !allowedKinds[candidates[i].Kind] {
			return nil, nil, nil, &Problem{Code: "workflow_catalog_candidate_invalid", Message: "workflow catalog candidate kind is unsupported", Hint: "Use workflow_dag, workflow_catalog, node_contract_registry, or trigger_skill generated content only.", Path: candidates[i].Path, Field: "kind", Expected: "workflow_dag, workflow_catalog, node_contract_registry, or trigger_skill", Actual: candidates[i].Kind}
		}
		if seenPaths[candidates[i].Path] {
			return nil, nil, nil, &Problem{Code: "workflow_catalog_target_path_ambiguous", Message: "workflow catalog candidate path is duplicated", Hint: "KAS WFLOW-009 must supply exactly one content item per target path.", Path: candidates[i].Path, Field: "path", Expected: "unique target path", Actual: candidates[i].Path}
		}
		seenPaths[candidates[i].Path] = true
		if seenKinds[candidates[i].Kind] && candidates[i].Kind != "trigger_skill" {
			return nil, nil, nil, &Problem{Code: "workflow_catalog_target_path_ambiguous", Message: "workflow catalog candidate kind is duplicated", Hint: "KAS WFLOW-009 must supply one workflow DAG, one catalog, and one node-contract registry target.", Path: candidates[i].Path, Field: "kind", Expected: "unique required content kind", Actual: candidates[i].Kind}
		}
		seenKinds[candidates[i].Kind] = true
		if err := validateWorkflowCatalogTargetPath(root, candidates[i].Path, candidates[i].Kind); err != nil {
			return nil, nil, nil, err
		}
		if err := validateWorkflowCatalogHash("generated_content.sha256", candidates[i].SHA256); err != nil {
			return nil, nil, nil, err
		}
		if got := workflowCatalogChecksum([]byte(candidates[i].Content)); got != candidates[i].SHA256 {
			return nil, nil, nil, &Problem{Code: "workflow_catalog_candidate_checksum_mismatch", Message: "workflow catalog candidate checksum does not match generated content", Hint: "Regenerate the KAS WFLOW-009 packet after content changes.", Path: candidates[i].Path, Field: "sha256", Expected: candidates[i].SHA256, Actual: got}
		}
	}
	for _, required := range []string{"workflow_dag", "workflow_catalog", "node_contract_registry"} {
		if !seenKinds[required] {
			return nil, nil, nil, &Problem{Code: "workflow_catalog_candidate_invalid", Message: "workflow catalog promotion packet lacks required candidate content", Hint: "KAS WFLOW-009 must supply workflow DAG, workflow catalog, and node-contract registry content.", Field: "generated_content.kind", Expected: required, Actual: "missing"}
		}
	}
	targetPaths := append([]string{}, packet.TargetPaths...)
	for i := range targetPaths {
		targetPaths[i] = filepath.ToSlash(strings.TrimSpace(targetPaths[i]))
	}
	sort.Strings(targetPaths)
	candidatePaths := make([]string, 0, len(candidates))
	candidateChecksums := map[string]string{}
	for _, candidate := range candidates {
		candidatePaths = append(candidatePaths, candidate.Path)
		candidateChecksums[candidate.Path] = candidate.SHA256
	}
	sort.Strings(candidatePaths)
	if !slices.Equal(targetPaths, candidatePaths) {
		return nil, nil, nil, &Problem{Code: "workflow_catalog_mixed_target_paths", Message: "workflow catalog packet target paths do not match generated content paths", Hint: "Regenerate KAS WFLOW-009 packet so target_paths exactly match generated_content paths.", Field: "target_paths", Expected: strings.Join(candidatePaths, ","), Actual: strings.Join(targetPaths, ",")}
	}
	return targetPaths, sortedWorkflowCatalogCandidates(candidates), candidateChecksums, nil
}

func validateWorkflowCatalogTargetPath(root Root, relative string, kind string) error {
	path, err := ResolveRelativePath(root, relative)
	if err != nil {
		return &Problem{Code: "workflow_catalog_target_path_unsafe", Message: "workflow catalog target path is unsafe", Hint: "KAS WFLOW-009 target paths must stay under the project-local workflow catalog promotion surface.", Path: relative, Field: "path", Expected: "repository-confined workflow catalog target path", Actual: err.Error()}
	}
	switch kind {
	case "workflow_catalog":
		if path.Relative != WorkflowCatalogDefaultPath {
			return &Problem{Code: "workflow_catalog_target_path_unsafe", Message: "workflow catalog target path is outside the catalog file", Hint: "The workflow catalog target must be .kkachi/workflow-catalog.yaml.", Path: path.Relative, Field: "path", Expected: WorkflowCatalogDefaultPath, Actual: path.Relative}
		}
	case "workflow_dag", "node_contract_registry":
		if !strings.HasPrefix(path.Relative, ".kkachi/workflows/") || !(strings.HasSuffix(path.Relative, ".yaml") || strings.HasSuffix(path.Relative, ".yml")) {
			return &Problem{Code: "workflow_catalog_target_path_unsafe", Message: "workflow catalog target path is outside project-local workflow files", Hint: "Workflow DAG and node-contract registry targets must be YAML files under .kkachi/workflows/.", Path: path.Relative, Field: "path", Expected: ".kkachi/workflows/<name>.yaml", Actual: path.Relative}
		}
	case "trigger_skill":
		if !strings.HasPrefix(path.Relative, ".kkachi/workflow-triggers/") || filepath.Base(path.Relative) != "SKILL.md" {
			return &Problem{Code: "workflow_catalog_target_path_unsafe", Message: "workflow trigger target path is outside project-local trigger evidence", Hint: "Thin trigger targets must be .kkachi/workflow-triggers/<workflow-id>-trigger/SKILL.md.", Path: path.Relative, Field: "path", Expected: ".kkachi/workflow-triggers/<workflow-id>-trigger/SKILL.md", Actual: path.Relative}
		}
	}
	return nil
}

func validateWorkflowCatalogCandidates(candidates []WorkflowCatalogCandidateContent, paths workflowCatalogKASCandidatePaths) (WorkflowCatalogProposalValidation, error) {
	tempDir, err := os.MkdirTemp("", "kah-workflow-catalog-candidate-*")
	if err != nil {
		return WorkflowCatalogProposalValidation{}, &Problem{Code: "workflow_catalog_candidate_validation_failed", Message: "cannot create temporary validation workspace", Hint: "Check temporary directory permissions.", Field: "tempdir", Expected: "writable temporary directory", Actual: err.Error()}
	}
	defer os.RemoveAll(tempDir)
	tempRoot := Root{Path: tempDir}
	for _, candidate := range candidates {
		path, err := ResolveRelativePath(tempRoot, candidate.Path)
		if err != nil {
			return WorkflowCatalogProposalValidation{}, err
		}
		if err := os.MkdirAll(filepath.Dir(path.Absolute), 0o755); err != nil {
			return WorkflowCatalogProposalValidation{}, &Problem{Code: "workflow_catalog_candidate_validation_failed", Message: "cannot create temporary candidate directory", Hint: "Check temporary directory permissions.", Path: path.Relative, Field: "path", Expected: "writable temporary candidate directory", Actual: err.Error()}
		}
		if err := os.WriteFile(path.Absolute, []byte(candidate.Content), 0o600); err != nil {
			return WorkflowCatalogProposalValidation{}, &Problem{Code: "workflow_catalog_candidate_validation_failed", Message: "cannot write temporary candidate content", Hint: "Check temporary directory permissions.", Path: path.Relative, Field: "path", Expected: "writable temporary candidate file", Actual: err.Error()}
		}
	}
	workflowPath := strings.TrimSpace(paths.WorkflowDAG)
	catalogPath := strings.TrimSpace(paths.Catalog)
	registryPath := strings.TrimSpace(paths.NodeContractRegistry)
	if workflowPath == "" || catalogPath == "" || registryPath == "" {
		workflowPath, catalogPath, registryPath = workflowCatalogRequiredPathsFromCandidates(candidates)
	}
	taskDAG, err := ValidateTaskDAG(tempRoot, workflowPath)
	if err != nil {
		return WorkflowCatalogProposalValidation{}, err
	}
	catalog, err := ValidateWorkflowCatalog(tempRoot, WorkflowCatalogOptions{File: catalogPath})
	if err != nil {
		return WorkflowCatalogProposalValidation{}, err
	}
	registry, err := ValidateNodeContractRegistry(tempRoot, registryPath, taskDAG.WorkflowID, taskDAG.Nodes)
	if err != nil {
		return WorkflowCatalogProposalValidation{}, err
	}
	if !taskDAG.OK || !catalog.OK || !registry.OK {
		return WorkflowCatalogProposalValidation{}, &Problem{Code: "workflow_catalog_candidate_invalid", Message: "workflow catalog promotion candidate is invalid", Hint: "Repair KAS-supplied workflow DAG, catalog, or node-contract registry content before proposing.", Field: "candidate", Expected: "valid workflow DAG, catalog, and node-contract registry", Actual: "invalid"}
	}
	return WorkflowCatalogProposalValidation{WorkflowDAG: taskDAG, Catalog: catalog, NodeContractRegistry: registry}, nil
}

func workflowCatalogRequiredPathsFromCandidates(candidates []WorkflowCatalogCandidateContent) (string, string, string) {
	var workflowPath, catalogPath, registryPath string
	for _, candidate := range candidates {
		switch candidate.Kind {
		case "workflow_dag":
			workflowPath = candidate.Path
		case "workflow_catalog":
			catalogPath = candidate.Path
		case "node_contract_registry":
			registryPath = candidate.Path
		}
	}
	return workflowPath, catalogPath, registryPath
}

func workflowCatalogCurrentChecksums(root Root, paths []string) (map[string]string, error) {
	result := map[string]string{}
	for _, relative := range paths {
		if err := validateWorkflowCatalogTargetPath(root, relative, workflowCatalogKindFromPath(relative)); err != nil {
			return nil, err
		}
		path, err := ResolveRelativePath(root, relative)
		if err != nil {
			return nil, err
		}
		data, err := os.ReadFile(path.Absolute)
		if errors.Is(err, os.ErrNotExist) {
			result[path.Relative] = workflowCatalogMissingChecksum
			continue
		}
		if err != nil {
			return nil, &Problem{Code: "workflow_catalog_base_read_failed", Message: "cannot read workflow catalog target for base checksum", Hint: "Check project-local workflow target permissions.", Path: path.Relative, Field: "path", Expected: "readable existing target or missing target", Actual: err.Error()}
		}
		result[path.Relative] = workflowCatalogChecksum(data)
	}
	return result, nil
}

func compareWorkflowCatalogBaseChecksums(expected map[string]string, actual map[string]string) error {
	for path, actualChecksum := range actual {
		expectedChecksum := strings.TrimSpace(expected[path])
		if expectedChecksum == "" {
			expectedChecksum = workflowCatalogMissingChecksum
		}
		if expectedChecksum != actualChecksum {
			return &Problem{Code: "workflow_catalog_base_checksum_mismatch", Message: "current workflow catalog target no longer matches proposal base", Hint: "Record a fresh workflow catalog proposal against the current project-local targets.", Path: path, Field: "base_checksums", Expected: expectedChecksum, Actual: actualChecksum}
		}
	}
	return nil
}

func validateWorkflowCatalogCandidateChecksums(candidates []WorkflowCatalogCandidateContent, expected map[string]string) error {
	for _, candidate := range candidates {
		got := workflowCatalogChecksum([]byte(candidate.Content))
		want := strings.TrimSpace(expected[candidate.Path])
		if want == "" {
			want = strings.TrimSpace(candidate.SHA256)
		}
		if got != want {
			return &Problem{Code: "workflow_catalog_candidate_checksum_mismatch", Message: "workflow catalog candidate checksum does not match proposal record", Hint: "Record a fresh proposal after changing candidate content.", Path: candidate.Path, Field: "candidate_checksums", Expected: want, Actual: got}
		}
	}
	return nil
}

func nextWorkflowCatalogProposalPath(root Root) (string, SafePath, error) {
	dir, err := ResolveRelativePath(root, workflowCatalogProposalDir)
	if err != nil {
		return "", SafePath{}, err
	}
	entries, err := os.ReadDir(dir.Absolute)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", SafePath{}, &Problem{Code: "workflow_catalog_proposal_list_failed", Message: "cannot list workflow catalog proposal directory", Hint: "Check repository permissions before proposing.", Path: dir.Relative, Field: "path", Expected: "listable proposal directory", Actual: err.Error()}
	}
	next := 1
	for _, entry := range entries {
		matches := workflowCatalogProposalIDPattern.FindStringSubmatch(entry.Name())
		if len(matches) != 2 {
			continue
		}
		var n int
		_, _ = fmt.Sscanf(matches[1], "%d", &n)
		if n >= next {
			next = n + 1
		}
	}
	proposalID := fmt.Sprintf("wcat-prop-%06d", next)
	path, err := ResolveRelativePath(root, workflowCatalogProposalDir+"/"+proposalID+".json")
	return proposalID, path, err
}

func readWorkflowCatalogProposalRecord(root Root, proposalID string) (WorkflowCatalogProposalRecord, SafePath, error) {
	path, err := ResolveRelativePath(root, workflowCatalogProposalDir+"/"+proposalID+".json")
	if err != nil {
		return WorkflowCatalogProposalRecord{}, SafePath{}, err
	}
	data, err := os.ReadFile(path.Absolute)
	if errors.Is(err, os.ErrNotExist) {
		return WorkflowCatalogProposalRecord{}, path, &Problem{Code: "workflow_catalog_proposal_missing", Message: "workflow catalog proposal record is missing", Hint: "Run workflow catalog propose first and pass the returned proposal id.", Path: path.Relative, Field: "proposal", Expected: "existing workflow catalog proposal record", Actual: "missing"}
	}
	if err != nil {
		return WorkflowCatalogProposalRecord{}, path, &Problem{Code: "workflow_catalog_proposal_read_failed", Message: "cannot read workflow catalog proposal record", Hint: "Check repository permissions before applying workflow catalog proposals.", Path: path.Relative, Field: "path", Expected: "readable workflow catalog proposal record", Actual: err.Error()}
	}
	var record WorkflowCatalogProposalRecord
	if err := json.Unmarshal(data, &record); err != nil {
		return WorkflowCatalogProposalRecord{}, path, &Problem{Code: "workflow_catalog_proposal_invalid_json", Message: "workflow catalog proposal record is not valid JSON", Hint: "Record proposals with workflow catalog propose so the strict proposal schema is preserved.", Path: path.Relative, Field: "json", Expected: "valid proposal JSON", Actual: err.Error()}
	}
	return record, path, nil
}

func validateWorkflowCatalogProposalRecord(record WorkflowCatalogProposalRecord, proposalID string, proposalPath string) error {
	if record.SchemaVersion != WorkflowCatalogProposalSchemaVersion {
		return &Problem{Code: "workflow_catalog_schema_unsupported", Message: "workflow catalog proposal schema version is invalid", Hint: "Record a fresh proposal with the current helper before applying.", Path: proposalPath, Field: "schema_version", Expected: WorkflowCatalogProposalSchemaVersion, Actual: record.SchemaVersion}
	}
	if record.Status != WorkflowCatalogStatusPass {
		return &Problem{Code: "workflow_catalog_proposal_invalid", Message: "workflow catalog proposal status is invalid", Hint: "Only passing workflow catalog proposal records can be applied.", Path: proposalPath, Field: "status", Expected: WorkflowCatalogStatusPass, Actual: record.Status}
	}
	if record.ProposalID != proposalID {
		return &Problem{Code: "workflow_catalog_proposal_id_mismatch", Message: "workflow catalog proposal id does not match the requested proposal", Hint: "Inspect the proposal record and rerun apply with the matching proposal id.", Path: proposalPath, Field: "proposal_id", Expected: proposalID, Actual: record.ProposalID}
	}
	if record.ProposalPath != proposalPath {
		return &Problem{Code: "workflow_catalog_proposal_path_mismatch", Message: "workflow catalog proposal path does not match its stored record", Hint: "Record a fresh proposal with workflow catalog propose before applying.", Path: proposalPath, Field: "proposal_path", Expected: proposalPath, Actual: record.ProposalPath}
	}
	if err := validateWorkflowCatalogHash("proposal_hash", record.ProposalHash); err != nil {
		return err
	}
	if len(record.TargetPaths) == 0 || len(record.Candidates) == 0 {
		return &Problem{Code: "workflow_catalog_proposal_invalid", Message: "workflow catalog proposal record is missing target or candidate evidence", Hint: "Record a fresh proposal with workflow catalog propose.", Path: proposalPath, Field: "targets", Expected: "target paths and candidate content", Actual: "missing"}
	}
	if !record.ApprovalRequired || !record.ApprovalRequirements.ProposalHashRequired {
		return &Problem{Code: "workflow_catalog_proposal_invalid", Message: "workflow catalog proposal is missing approval/hash requirements", Hint: "Record a fresh proposal with the current helper before applying.", Path: proposalPath, Field: "approval_requirements", Expected: "approval and proposal hash required", Actual: "not required"}
	}
	return nil
}

func validateWorkflowCatalogSourceApprovalBinding(root Root, record WorkflowCatalogProposalRecord, proposalPath string, approvalRef string) error {
	sourceHash := record.SourcePacket.ApprovalHash
	if sourceHash == "" {
		return nil
	}
	if record.ProposalHash != sourceHash {
		return &Problem{Code: "workflow_catalog_source_approval_hash_mismatch", Message: "workflow catalog proposal hash does not match the source KAS packet approval hash", Hint: "Record a fresh proposal from the current KAS WFLOW-009 packet.", Path: proposalPath, Field: "source_packet.approval_hash", Expected: sourceHash, Actual: record.ProposalHash}
	}
	if !workflowCatalogApprovalBindsHash(root, approvalRef, sourceHash) {
		return &Problem{Code: "workflow_catalog_hash_bound_approval_missing", Message: "workflow catalog approval evidence is not bound to the source KAS packet approval hash", Hint: "Use dry-run:<proposal-hash> or a repository evidence file that contains the source KAS approval hash.", Field: "approval", Expected: "hash-bound approval evidence containing " + sourceHash, Actual: approvalRef}
	}
	return nil
}

func workflowCatalogValidationOK(validation WorkflowCatalogProposalValidation) bool {
	return validation.WorkflowDAG.OK && validation.Catalog.OK && validation.NodeContractRegistry.OK
}

func isWorkflowCatalogProposalID(value string) bool {
	return workflowCatalogProposalIDPattern.MatchString(value + ".json")
}

func workflowCatalogCandidateTargetPaths(root Root, candidates []WorkflowCatalogCandidateContent) ([]workflowCatalogCandidateTarget, error) {
	targets := make([]workflowCatalogCandidateTarget, 0, len(candidates))
	for _, candidate := range sortedWorkflowCatalogCandidates(candidates) {
		path, err := ResolveRelativePath(root, candidate.Path)
		if err != nil {
			return nil, err
		}
		targets = append(targets, workflowCatalogCandidateTarget{path: path, content: candidate.Content})
	}
	return targets, nil
}

func workflowCatalogCandidatePathsFromRecord(record WorkflowCatalogProposalRecord) workflowCatalogKASCandidatePaths {
	var paths workflowCatalogKASCandidatePaths
	for _, candidate := range record.Candidates {
		switch candidate.Kind {
		case "workflow_dag":
			paths.WorkflowDAG = candidate.Path
		case "workflow_catalog":
			paths.Catalog = candidate.Path
		case "node_contract_registry":
			paths.NodeContractRegistry = candidate.Path
		case "trigger_skill":
			paths.TriggerSkill = candidate.Path
		}
	}
	return paths
}

func workflowCatalogApprovalBindsHash(root Root, approvalRef string, hash string) bool {
	approvalRef = strings.TrimSpace(approvalRef)
	if approvalRef == hash || approvalRef == "dry-run:"+hash || strings.Contains(approvalRef, hash) {
		return true
	}
	path, err := ResolveRelativePath(root, approvalRef)
	if err != nil {
		return false
	}
	data, err := os.ReadFile(path.Absolute)
	return err == nil && strings.Contains(string(data), hash)
}

func writeWorkflowCatalogPromotionBackup(root Root, target SafePath, backupRelative string) error {
	data, err := os.ReadFile(target.Absolute)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return &Problem{Code: "workflow_catalog_backup_failed", Message: "cannot read current workflow catalog target for backup", Hint: "Check repository permissions before applying the workflow catalog proposal.", Path: target.Relative, Field: "path", Expected: "readable existing target", Actual: err.Error()}
	}
	backup, err := ResolveRelativePath(root, backupRelative)
	if err != nil {
		return err
	}
	if err := writeNewFileAtomically(backup, data); err != nil {
		return &Problem{Code: "workflow_catalog_backup_failed", Message: "cannot write workflow catalog promotion backup", Hint: "Check repository permissions and preserve stderr for diagnosis.", Path: backup.Relative, Field: "backup", Expected: "writable backup file", Actual: err.Error()}
	}
	return nil
}

func workflowCatalogFileExists(path SafePath) bool {
	info, err := os.Stat(path.Absolute)
	return err == nil && !info.IsDir()
}

func validateWorkflowCatalogHash(field string, hash string) error {
	if workflowCatalogHashPattern.MatchString(strings.TrimSpace(hash)) {
		return nil
	}
	return &Problem{Code: "workflow_catalog_proposal_hash_malformed", Message: "workflow catalog hash is malformed", Hint: "Use sha256:<64 lowercase hex characters>.", Field: field, Expected: "sha256:<64hex>", Actual: hash}
}

func canonicalWorkflowCatalogProposalHash(source WorkflowCatalogSourcePacket, targetPaths []string, baseChecksums map[string]string, candidateChecksums map[string]string, changedPaths []WorkflowCatalogChangedPath) string {
	payload := map[string]any{
		"schema_version":      WorkflowCatalogProposalSchemaVersion,
		"source_packet":       source,
		"target_paths":        targetPaths,
		"base_checksums":      baseChecksums,
		"candidate_checksums": candidateChecksums,
		"changed_paths":       sortedWorkflowCatalogChangedPaths(changedPaths),
	}
	data, _ := json.Marshal(payload)
	return workflowCatalogChecksum(data)
}

func workflowCatalogChecksum(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func workflowCatalogKindFromPath(path string) string {
	switch {
	case path == WorkflowCatalogDefaultPath:
		return "workflow_catalog"
	case strings.HasPrefix(path, ".kkachi/workflow-triggers/"):
		return "trigger_skill"
	case strings.HasSuffix(path, "-node-contracts.yaml") || strings.HasSuffix(path, "-node-contracts.yml"):
		return "node_contract_registry"
	default:
		return "workflow_dag"
	}
}

func sortedWorkflowCatalogCandidates(candidates []WorkflowCatalogCandidateContent) []WorkflowCatalogCandidateContent {
	result := append([]WorkflowCatalogCandidateContent{}, candidates...)
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

func sortedWorkflowCatalogChangedPaths(paths []WorkflowCatalogChangedPath) []WorkflowCatalogChangedPath {
	result := append([]WorkflowCatalogChangedPath{}, paths...)
	sort.Slice(result, func(i, j int) bool { return result[i].Path < result[j].Path })
	return result
}

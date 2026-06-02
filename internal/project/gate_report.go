package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type gateReport struct {
	RunID           string               `json:"run_id"`
	Gate            string               `json:"gate"`
	Status          string               `json:"status"`
	EventID         string               `json:"event_id"`
	GeneratedAt     string               `json:"generated_at"`
	ReportPath      string               `json:"report_path"`
	Checks          []GateCheck          `json:"checks"`
	Evidence        []gateReportEvidence `json:"evidence,omitempty"`
	MissingEvidence []string             `json:"missing_evidence"`
}

type gateReportEvidence struct {
	Path   string `json:"path"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256"`
}

func gateReportPath(root Root, runID string, gate string) (SafePath, error) {
	if strings.ContainsAny(gate, `/\:*?"<>|`) {
		return SafePath{}, &Problem{Code: "gate_report_path_invalid", Message: "gate name is not safe for a gate report filename", Hint: "Use a registry gate name without path separators or filesystem metacharacters.", Field: "gate", Expected: "safe gate registry name", Actual: gate}
	}
	return ResolveRelativePath(root, fmt.Sprintf("%s/%s/gate-reports/%s.json", RunRootPath, runID, gate))
}

func writeGateReport(root Root, path SafePath, result GateCheckResult, generatedAt string) (string, error) {
	evidence, err := collectGateReportEvidence(root, result)
	if err != nil {
		return "", err
	}
	report := gateReport{
		RunID:           result.RunID,
		Gate:            result.Gate,
		Status:          result.Status,
		EventID:         result.EventID,
		GeneratedAt:     generatedAt,
		ReportPath:      path.Relative,
		Checks:          result.Checks,
		Evidence:        evidence,
		MissingEvidence: result.MissingEvidence,
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", &Problem{Code: "gate_report_encode_failed", Message: "cannot encode gate report", Hint: "Inspect gate check result fields for unsupported JSON values.", Path: path.Relative, Field: "gate_report", Expected: "JSON object", Actual: err.Error()}
	}
	data = append(data, '\n')
	if err := writeExistingFileAtomically(path, data); err != nil {
		return "", err
	}
	return path.Relative, nil
}

func collectGateReportEvidence(root Root, result GateCheckResult) ([]gateReportEvidence, error) {
	evidence := []gateReportEvidence{}
	seen := map[string]bool{}
	for _, check := range result.Checks {
		path := strings.TrimSpace(check.Path)
		if check.Status != GateStatusPass || path == "" || !gateReportEvidencePath(result.RunID, path) || seen[path] {
			continue
		}
		item, err := hashGateReportEvidence(root, path)
		if err != nil {
			return nil, err
		}
		evidence = append(evidence, item)
		seen[path] = true
	}
	return evidence, nil
}

func gateReportEvidencePath(runID string, path string) bool {
	prefix := fmt.Sprintf("%s/%s/", RunRootPath, runID)
	if !strings.HasPrefix(path, prefix) {
		return false
	}
	rest := strings.TrimPrefix(path, prefix)
	return rest != "run-metadata.json" && !strings.HasPrefix(rest, "gate-reports/")
}

func hashGateReportEvidence(root Root, relative string) (gateReportEvidence, error) {
	path, err := ResolveRelativePath(root, relative)
	if err != nil {
		return gateReportEvidence{}, err
	}
	info, err := os.Lstat(path.Absolute)
	if err != nil {
		return gateReportEvidence{}, &Problem{Code: "gate_report_evidence_inspection_failed", Message: "cannot inspect gate report evidence", Hint: "Preserve gate evidence files and rerun the affected gate after repairing the artifact.", Path: path.Relative, Field: "path", Expected: "inspectable evidence file", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		return gateReportEvidence{}, &Problem{Code: "gate_report_evidence_invalid", Message: "gate report evidence must be a regular file", Hint: "Move the conflicting path and rerun the affected gate.", Path: path.Relative, Field: "path", Expected: "regular file", Actual: fileKind(info)}
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		return gateReportEvidence{}, &Problem{Code: "gate_report_evidence_read_failed", Message: "cannot read gate report evidence", Hint: "Check artifact permissions and rerun the affected gate.", Path: path.Relative, Field: "path", Expected: "readable evidence file", Actual: err.Error()}
	}
	sum := sha256.Sum256(content)
	return gateReportEvidence{Path: path.Relative, Size: info.Size(), SHA256: hex.EncodeToString(sum[:])}, nil
}

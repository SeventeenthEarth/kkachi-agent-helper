package project

import (
	"encoding/json"
	"fmt"
	"strings"
)

type gateReport struct {
	RunID           string      `json:"run_id"`
	Gate            string      `json:"gate"`
	Status          string      `json:"status"`
	EventID         string      `json:"event_id"`
	GeneratedAt     string      `json:"generated_at"`
	ReportPath      string      `json:"report_path"`
	Checks          []GateCheck `json:"checks"`
	MissingEvidence []string    `json:"missing_evidence"`
}

func gateReportPath(root Root, runID string, gate string) (SafePath, error) {
	if strings.ContainsAny(gate, `/\:*?"<>|`) {
		return SafePath{}, &Problem{Code: "gate_report_path_invalid", Message: "gate name is not safe for a gate report filename", Hint: "Use a registry gate name without path separators or filesystem metacharacters.", Field: "gate", Expected: "safe gate registry name", Actual: gate}
	}
	return ResolveRelativePath(root, fmt.Sprintf("%s/%s/gate-reports/%s.json", RunRootPath, runID, gate))
}

func writeGateReport(path SafePath, result GateCheckResult, generatedAt string) (string, error) {
	report := gateReport{
		RunID:           result.RunID,
		Gate:            result.Gate,
		Status:          result.Status,
		EventID:         result.EventID,
		GeneratedAt:     generatedAt,
		ReportPath:      path.Relative,
		Checks:          result.Checks,
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

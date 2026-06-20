package project

import (
	"os"
	"path/filepath"
	"strings"
)

const ToolchainProbeSchemaVersion = "kah.toolchain_probe.v1"

// ToolchainProbeOptions carries CLI/helper facts that project inspection cannot infer.
type ToolchainProbeOptions struct {
	HelperCommand string
	HelperVersion string
	BinaryPath    string
}

// ToolchainProbeResult is the stable read-only KAH fact payload consumed by KAS TOLMR.
type ToolchainProbeResult struct {
	OK            bool                       `json:"ok"`
	SchemaVersion string                     `json:"schema_version"`
	NoWrite       ToolchainProbeNoWrite      `json:"no_write"`
	KAH           ToolchainProbeKAH          `json:"kah"`
	Project       ToolchainProbeProject      `json:"project"`
	Doctor        ToolchainProbeDoctor       `json:"doctor"`
	Diagnostics   []ToolchainProbeDiagnostic `json:"diagnostics"`
}

type ToolchainProbeNoWrite struct {
	Guaranteed bool `json:"guaranteed"`
	WriteCount int  `json:"write_count"`
}

type ToolchainProbeKAH struct {
	HelperCommand string `json:"helper_command"`
	Version       string `json:"version"`
	BinaryPath    string `json:"binary_path"`
}

type ToolchainProbeProject struct {
	Root                 string `json:"root"`
	KkachiDir            string `json:"kkachi_dir"`
	KkachiDirPresent     bool   `json:"kkachi_dir_present"`
	ProjectInitialized   bool   `json:"project_initialized"`
	WorkflowGraphPresent bool   `json:"workflow_graph_present"`
}

type ToolchainProbeDoctor struct {
	Status      string   `json:"status"`
	ReasonCodes []string `json:"reason_codes"`
}

type ToolchainProbeDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// ProbeToolchain reads deterministic local helper/project facts without mutating state.
func ProbeToolchain(root Root, options ToolchainProbeOptions) (ToolchainProbeResult, error) {
	if strings.TrimSpace(root.Path) == "" {
		return ToolchainProbeResult{}, problem("repo_root_required", "repository root is required", "Pass --project-root or run from a readable project directory.")
	}
	canonicalRoot, err := canonicalExistingDirectory(root.Path)
	if err != nil {
		return ToolchainProbeResult{}, err
	}
	root = Root{Path: canonicalRoot}

	kahVersion := strings.TrimSpace(options.HelperVersion)
	if kahVersion == "" {
		kahVersion = "unknown"
	}
	helperCommand := strings.TrimSpace(options.HelperCommand)
	if helperCommand == "" {
		helperCommand = "kkachi-agent-helper"
	}
	binaryPath := strings.TrimSpace(options.BinaryPath)
	if binaryPath == "" {
		binaryPath = "unknown"
	}

	kkachiDir := filepath.Join(root.Path, ".kkachi")
	kkachiDirPresent := directoryExists(kkachiDir)
	workflowGraphPresent := fileExists(filepath.Join(root.Path, WorkflowGraphDefaultPath))

	doctorStatus := "UNKNOWN"
	reasonCodes := []string{}
	projectInitialized := false
	if !kkachiDirPresent {
		doctorStatus = "FAIL"
		reasonCodes = append(reasonCodes, "kkachi_dir_missing")
	} else {
		report, err := Doctor(root)
		if err != nil {
			doctorStatus = "UNKNOWN"
			reasonCodes = append(reasonCodes, "doctor_unavailable")
		} else {
			doctorStatus = toolchainDoctorStatus(report.Health)
			reasonCodes = toolchainDoctorReasonCodes(report.Checks)
			projectInitialized = toolchainProjectInitialized(report.Checks)
		}
	}

	return ToolchainProbeResult{
		OK:            true,
		SchemaVersion: ToolchainProbeSchemaVersion,
		NoWrite:       ToolchainProbeNoWrite{Guaranteed: true, WriteCount: 0},
		KAH:           ToolchainProbeKAH{HelperCommand: helperCommand, Version: kahVersion, BinaryPath: binaryPath},
		Project: ToolchainProbeProject{
			Root:                 root.Path,
			KkachiDir:            kkachiDir,
			KkachiDirPresent:     kkachiDirPresent,
			ProjectInitialized:   projectInitialized,
			WorkflowGraphPresent: workflowGraphPresent,
		},
		Doctor:      ToolchainProbeDoctor{Status: doctorStatus, ReasonCodes: reasonCodes},
		Diagnostics: []ToolchainProbeDiagnostic{},
	}, nil
}

func canonicalExistingDirectory(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", &Problem{Code: "invalid_project_root", Message: "cannot resolve project root", Hint: "Pass an existing project directory.", Path: path, Field: "project_root", Expected: "existing directory", Actual: err.Error()}
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", &Problem{Code: "invalid_project_root", Message: "cannot resolve project root", Hint: "Pass an existing project directory and avoid broken symlinks.", Path: absolute, Field: "project_root", Expected: "existing directory", Actual: err.Error()}
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return "", &Problem{Code: "invalid_project_root", Message: "cannot inspect project root", Hint: "Pass an existing project directory.", Path: resolved, Field: "project_root", Expected: "directory", Actual: err.Error()}
	}
	if !info.IsDir() {
		return "", &Problem{Code: "invalid_project_root", Message: "project root is not a directory", Hint: "Pass an existing project directory.", Path: resolved, Field: "project_root", Expected: "directory", Actual: "file"}
	}
	return filepath.Clean(resolved), nil
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func toolchainDoctorStatus(health string) string {
	switch health {
	case HealthOK:
		return "PASS"
	case HealthWarning:
		return "WARN"
	case HealthFail:
		return "FAIL"
	default:
		return "UNKNOWN"
	}
}

func toolchainDoctorReasonCodes(checks []DiagnosticCheck) []string {
	reasonCodes := []string{}
	seen := map[string]bool{}
	for _, check := range checks {
		if check.Status == CheckPass {
			continue
		}
		code := strings.TrimSpace(check.Name)
		if code == "" {
			code = "doctor_check_failed"
		} else {
			code += "_" + check.Status
		}
		if !seen[code] {
			seen[code] = true
			reasonCodes = append(reasonCodes, code)
		}
	}
	return reasonCodes
}

func toolchainProjectInitialized(checks []DiagnosticCheck) bool {
	required := map[string]bool{
		"config": false,
		"status": false,
		"events": false,
		"paths":  false,
	}
	for _, check := range checks {
		if _, ok := required[check.Name]; !ok {
			continue
		}
		if check.Status != CheckPass {
			return false
		}
		required[check.Name] = true
	}
	for _, passed := range required {
		if !passed {
			return false
		}
	}
	return true
}

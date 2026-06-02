package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateArtifactsIntakePassesWithCompletedClassification(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	writeIntakeClassification(t, repo, created.Metadata, "")
	before := len(runEventLines(t, repo))

	result, err := ValidateArtifacts(root, ArtifactValidateOptions{RunID: created.Metadata.RunID})
	if err != nil {
		t.Fatalf("ValidateArtifacts() error = %v", err)
	}
	if result.RunID != created.Metadata.RunID || result.Gate != ArtifactGateIntake || result.Status != ValidationStatusPass {
		t.Fatalf("result = %#v, want passing intake result", result)
	}
	if len(result.Checks) == 0 || !validationCheckStatus(result.Checks, "required_artifacts", ValidationStatusPass) || !validationCheckStatus(result.Checks, "urgency", ValidationStatusPass) {
		t.Fatalf("checks = %#v, want required artifacts and urgency pass checks", result.Checks)
	}
	if after := len(runEventLines(t, repo)); after != before {
		t.Fatalf("event count changed from %d to %d; validate must be read-only", before, after)
	}
}

func TestValidateArtifactsRejectsPendingIntakeAndManifestMismatch(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	metadata.RequiredArtifacts = []string{}
	writeRunMetadataForTest(t, repo, metadata)

	result, err := ValidateArtifacts(root, ArtifactValidateOptions{RunID: created.Metadata.RunID, Gate: ArtifactGateIntake})
	if err != nil {
		t.Fatalf("ValidateArtifacts() error = %v", err)
	}
	if result.Status != ValidationStatusFail || !validationCheckStatus(result.Checks, "required_artifacts", ValidationStatusFail) || !validationCheckStatus(result.Checks, "intake_status", ValidationStatusFail) {
		t.Fatalf("checks = %#v, want manifest and pending intake failures", result.Checks)
	}
}

func TestValidateArtifactsChecksPathSOTEligibility(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.WorkPath = "B_discovery_shaping"
	options.SOTPolicy = "minimal_sot_before_code"
	options.ExecutionMode = "research"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	metadata := created.Metadata
	metadata.SOTPolicy = "existing_sot_basis"
	writeRunMetadataForTest(t, repo, metadata)
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	writeIntakeClassification(t, repo, metadata, "")

	result, err := ValidateArtifacts(root, ArtifactValidateOptions{RunID: created.Metadata.RunID})
	if err != nil {
		t.Fatalf("ValidateArtifacts() error = %v", err)
	}
	if result.Status != ValidationStatusFail || !validationCheckStatus(result.Checks, "work_path_sot_policy", ValidationStatusFail) {
		t.Fatalf("checks = %#v, want Path B SOT policy failure", result.Checks)
	}
}

func TestValidateArtifactsRequiresLightModeReason(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.WorkMode = "light"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	writeIntakeClassification(t, repo, created.Metadata, "")
	result, err := ValidateArtifacts(root, ArtifactValidateOptions{RunID: created.Metadata.RunID})
	if err != nil {
		t.Fatalf("ValidateArtifacts() error = %v", err)
	}
	if result.Status != ValidationStatusFail || !validationCheckStatus(result.Checks, "light_mode_reason", ValidationStatusFail) {
		t.Fatalf("checks = %#v, want missing light reason failure", result.Checks)
	}

	writeIntakeClassification(t, repo, created.Metadata, "Light Mode Reason: low-risk documentation-only update\n")
	result, err = ValidateArtifacts(root, ArtifactValidateOptions{RunID: created.Metadata.RunID})
	if err != nil {
		t.Fatalf("ValidateArtifacts(with reason) error = %v", err)
	}
	if result.Status != ValidationStatusPass || !validationCheckStatus(result.Checks, "light_mode_reason", ValidationStatusPass) {
		t.Fatalf("checks = %#v, want light reason pass", result.Checks)
	}
}

func TestValidateArtifactsPathSOTEligibilityMatrix(t *testing.T) {
	tests := []struct {
		name       string
		workPath   string
		sotPolicy  string
		wantStatus string
	}{
		{name: "path a existing sot", workPath: "A_development_execution", sotPolicy: "existing_sot_basis", wantStatus: ValidationStatusPass},
		{name: "path a minimal sot", workPath: "A_development_execution", sotPolicy: "minimal_sot_before_code", wantStatus: ValidationStatusFail},
		{name: "path a full sot", workPath: "A_development_execution", sotPolicy: "full_sot_before_code", wantStatus: ValidationStatusFail},
		{name: "path b existing sot", workPath: "B_discovery_shaping", sotPolicy: "existing_sot_basis", wantStatus: ValidationStatusFail},
		{name: "path b minimal sot", workPath: "B_discovery_shaping", sotPolicy: "minimal_sot_before_code", wantStatus: ValidationStatusPass},
		{name: "path b full sot", workPath: "B_discovery_shaping", sotPolicy: "full_sot_before_code", wantStatus: ValidationStatusPass},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			options := deterministicCreateRunOptions()
			options.WorkPath = tt.workPath
			options.SOTPolicy = tt.sotPolicy
			mutateToInvalidPathBExistingSOT := tt.workPath == "B_discovery_shaping" && tt.sotPolicy == "existing_sot_basis"
			if mutateToInvalidPathBExistingSOT {
				options.SOTPolicy = "minimal_sot_before_code"
			}
			if tt.workPath == "B_discovery_shaping" {
				options.ExecutionMode = "research"
			}
			created, err := CreateRun(root, options)
			if err != nil {
				t.Fatalf("CreateRun() error = %v", err)
			}
			metadata := created.Metadata
			if mutateToInvalidPathBExistingSOT {
				metadata.SOTPolicy = tt.sotPolicy
				writeRunMetadataForTest(t, repo, metadata)
			}
			if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
				t.Fatalf("InitArtifacts() error = %v", err)
			}
			writeIntakeClassification(t, repo, metadata, "")

			result, err := ValidateArtifacts(root, ArtifactValidateOptions{RunID: created.Metadata.RunID})
			if err != nil {
				t.Fatalf("ValidateArtifacts() error = %v", err)
			}
			if result.Status != tt.wantStatus {
				t.Fatalf("status = %q checks=%#v, want %q", result.Status, result.Checks, tt.wantStatus)
			}
			if !validationCheckStatus(result.Checks, "work_path_sot_policy", tt.wantStatus) {
				t.Fatalf("checks = %#v, want work_path_sot_policy %s", result.Checks, tt.wantStatus)
			}
		})
	}
}

func TestValidateArtifactsRejectsIntakeFieldMismatches(t *testing.T) {
	tests := []struct {
		name  string
		field string
		line  string
	}{
		{name: "work path", field: "work_path", line: "Work Path: B_discovery_shaping"},
		{name: "work mode", field: "work_mode", line: "Work Mode: light"},
		{name: "sot policy", field: "sot_policy", line: "SOT Policy: minimal_sot_before_code"},
		{name: "urgency", field: "urgency", line: "Urgency: critical"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			created, err := CreateRun(root, deterministicCreateRunOptions())
			if err != nil {
				t.Fatalf("CreateRun() error = %v", err)
			}
			if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
				t.Fatalf("InitArtifacts() error = %v", err)
			}
			writeIntakeClassificationWithOverrides(t, repo, created.Metadata, map[string]string{tt.field: tt.line}, "")

			result, err := ValidateArtifacts(root, ArtifactValidateOptions{RunID: created.Metadata.RunID})
			if err != nil {
				t.Fatalf("ValidateArtifacts() error = %v", err)
			}
			if result.Status != ValidationStatusFail || !validationCheckStatus(result.Checks, tt.field, ValidationStatusFail) {
				t.Fatalf("checks = %#v, want %s mismatch failure", result.Checks, tt.field)
			}
		})
	}
}

func TestValidateArtifactsRejectsMissingEmptyAndNonRegularIntake(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, path string)
		want  string
	}{
		{name: "missing", setup: func(t *testing.T, path string) { t.Helper(); mustRemove(t, path) }, want: "missing"},
		{name: "empty", setup: func(t *testing.T, path string) { t.Helper(); mustWriteFile(t, path, nil) }, want: "empty"},
		{name: "directory", setup: func(t *testing.T, path string) { t.Helper(); mustRemove(t, path); mustMkdir(t, path) }, want: "directory"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := initializedRepo(t)
			root, _ := DiscoverRoot(repo)
			created, err := CreateRun(root, deterministicCreateRunOptions())
			if err != nil {
				t.Fatalf("CreateRun() error = %v", err)
			}
			if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
				t.Fatalf("InitArtifacts() error = %v", err)
			}
			path := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "intake-classification.md")
			tt.setup(t, path)

			result, err := ValidateArtifacts(root, ArtifactValidateOptions{RunID: created.Metadata.RunID})
			if err != nil {
				t.Fatalf("ValidateArtifacts() error = %v", err)
			}
			if result.Status != ValidationStatusFail || !validationCheckStatus(result.Checks, "intake_artifact", ValidationStatusFail) || !validationCheckActual(result.Checks, "intake_artifact", tt.want) {
				t.Fatalf("checks = %#v, want intake_artifact failure actual %q", result.Checks, tt.want)
			}
		})
	}
}

func TestNotApplicableReasonRequiresStatusAndReasonFields(t *testing.T) {
	ok, reason := NotApplicableReason([]byte("Status: not_applicable\nReason: bridge backend not used\n"))
	if !ok || reason != "bridge backend not used" {
		t.Fatalf("NotApplicableReason() = %v, %q; want valid reason", ok, reason)
	}
	ok, reason = NotApplicableReason([]byte("Status: not_applicable\nReason:   \n"))
	if ok || reason != "" {
		t.Fatalf("NotApplicableReason(empty reason) = %v, %q; want invalid", ok, reason)
	}
	ok, _ = NotApplicableReason([]byte("Status: complete\nReason: no\n"))
	if ok {
		t.Fatalf("NotApplicableReason(complete) = true, want false")
	}
}

func writeIntakeClassification(t *testing.T, repo string, metadata RunMetadata, extra string) {
	t.Helper()
	writeIntakeClassificationWithOverrides(t, repo, metadata, nil, extra)
}

func writeIntakeClassificationWithOverrides(t *testing.T, repo string, metadata RunMetadata, overrides map[string]string, extra string) {
	t.Helper()
	lines := map[string]string{
		"work_path":  "Work Path: " + metadata.WorkPath,
		"work_mode":  "Work Mode: " + metadata.WorkMode,
		"sot_policy": "SOT Policy: " + metadata.SOTPolicy,
		"urgency":    "Urgency: " + metadata.Urgency,
	}
	for key, line := range overrides {
		lines[key] = line
	}
	content := strings.Join([]string{
		"# intake-classification.md",
		"",
		"Status: complete",
		lines["work_path"],
		lines["work_mode"],
		lines["sot_policy"],
		lines["urgency"],
		strings.TrimRight(extra, "\n"),
		"",
	}, "\n")
	path := filepath.Join(repo, ".kkachi", "runs", metadata.RunID, "intake-classification.md")
	mustWriteFile(t, path, []byte(content))
}

func mustWriteFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func mustRemove(t *testing.T, path string) {
	t.Helper()
	if err := os.Remove(path); err != nil {
		t.Fatalf("remove %s: %v", path, err)
	}
}

func writeRunMetadataForTest(t *testing.T, repo string, metadata RunMetadata) {
	t.Helper()
	path := filepath.Join(repo, ".kkachi", "runs", metadata.RunID, "run-metadata.json")
	if err := writeRunMetadataExisting(SafePath{Relative: filepath.ToSlash(filepath.Join(".kkachi", "runs", metadata.RunID, "run-metadata.json")), Absolute: path}, metadata); err != nil {
		t.Fatalf("write metadata: %v", err)
	}
}

func validationCheckStatus(checks []ArtifactValidationCheck, name string, status string) bool {
	for _, check := range checks {
		if check.Name == name && check.Status == status {
			return true
		}
	}
	return false
}

func validationCheckActual(checks []ArtifactValidationCheck, name string, actual string) bool {
	for _, check := range checks {
		if check.Name == name && check.Actual == actual {
			return true
		}
	}
	return false
}

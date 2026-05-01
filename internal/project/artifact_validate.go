package project

import (
	"fmt"
	"os"
	"reflect"
	"strings"
)

const (
	ArtifactGateIntake = "intake"

	ValidationStatusPass = "pass"
	ValidationStatusFail = "fail"

	intakeClassificationArtifact = "intake-classification.md"
)

type ArtifactValidateOptions struct {
	RunID string
	Gate  string
}

type ArtifactValidateResult struct {
	RunID  string                    `json:"run_id"`
	Gate   string                    `json:"gate"`
	Status string                    `json:"status"`
	Checks []ArtifactValidationCheck `json:"checks"`
}

type ArtifactValidationCheck struct {
	Name     string `json:"name"`
	Status   string `json:"status"`
	Path     string `json:"path,omitempty"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Field    string `json:"field,omitempty"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

func ValidateArtifacts(root Root, options ArtifactValidateOptions) (ArtifactValidateResult, error) {
	gate := strings.TrimSpace(options.Gate)
	if gate == "" {
		gate = ArtifactGateIntake
	}
	if gate != ArtifactGateIntake {
		return ArtifactValidateResult{}, &Problem{Code: "artifact_gate_unsupported", Message: "artifact validation gate is not supported", Hint: "Use artifact validate <run_id> --gate intake, or omit --gate to validate intake.", Field: "gate", Expected: ArtifactGateIntake, Actual: gate}
	}
	if err := preflightEventCoherence(root); err != nil {
		return ArtifactValidateResult{}, err
	}
	metadata, metadataPath, err := ReadRunMetadata(root, options.RunID)
	if err != nil {
		return ArtifactValidateResult{}, err
	}

	result := ArtifactValidateResult{RunID: metadata.RunID, Gate: gate, Status: ValidationStatusPass}
	add := func(check ArtifactValidationCheck) {
		if check.Status == "" {
			check.Status = ValidationStatusPass
		}
		if check.Status == ValidationStatusFail {
			result.Status = ValidationStatusFail
		}
		result.Checks = append(result.Checks, check)
	}

	add(validateRequiredArtifacts(metadata, metadataPath.Relative))
	for _, check := range validatePathModeEligibility(metadata, metadataPath.Relative) {
		add(check)
	}

	intakePath, content, check := readValidationArtifact(root, metadata.RunID, intakeClassificationArtifact)
	add(check)
	if check.Status == ValidationStatusPass {
		for _, c := range validateIntakeClassification(metadata, intakePath, content) {
			add(c)
		}
	}
	return result, nil
}

func validateRequiredArtifacts(metadata RunMetadata, metadataRelative string) ArtifactValidationCheck {
	expected := ArtifactManifest(metadata)
	if reflect.DeepEqual(metadata.RequiredArtifacts, expected) {
		return ArtifactValidationCheck{Name: "required_artifacts", Status: ValidationStatusPass, Path: metadataRelative, Message: "required artifacts match the run manifest"}
	}
	actual := fmt.Sprintf("%d artifacts", len(metadata.RequiredArtifacts))
	if len(metadata.RequiredArtifacts) == 0 {
		actual = "empty"
	}
	return ArtifactValidationCheck{Name: "required_artifacts", Status: ValidationStatusFail, Path: metadataRelative, Message: "required artifacts do not match the run manifest", Hint: "Run artifact init for this run and preserve the generated required_artifacts order.", Field: "required_artifacts", Expected: strings.Join(expected, ","), Actual: actual}
}

func validatePathModeEligibility(metadata RunMetadata, metadataRelative string) []ArtifactValidationCheck {
	return []ArtifactValidationCheck{
		validateSOTPolicyEligibility(metadata, metadataRelative),
		validateLightModeSafetyArtifacts(metadata, metadataRelative),
	}
}

func validateSOTPolicyEligibility(metadata RunMetadata, metadataRelative string) ArtifactValidationCheck {
	if metadata.WorkPath == "A_development_execution" && metadata.SOTPolicy != "existing_sot_basis" {
		return ArtifactValidationCheck{Name: "work_path_sot_policy", Status: ValidationStatusFail, Path: metadataRelative, Message: "Path A requires an existing SOT basis", Hint: "Use Path B when SOT must be created before development execution.", Field: "sot_policy", Expected: "existing_sot_basis", Actual: metadata.SOTPolicy}
	}
	if metadata.WorkPath == "B_discovery_shaping" && !allowed(metadata.SOTPolicy, "minimal_sot_before_code", "full_sot_before_code") {
		return ArtifactValidationCheck{Name: "work_path_sot_policy", Status: ValidationStatusFail, Path: metadataRelative, Message: "Path B requires an SOT creation policy", Hint: "Use Path A only when a durable SOT basis already exists.", Field: "sot_policy", Expected: "minimal_sot_before_code or full_sot_before_code", Actual: metadata.SOTPolicy}
	}
	return ArtifactValidationCheck{Name: "work_path_sot_policy", Status: ValidationStatusPass, Path: metadataRelative, Message: "work path and SOT policy are eligible"}
}

func validateLightModeSafetyArtifacts(metadata RunMetadata, metadataRelative string) ArtifactValidationCheck {
	safetyArtifacts := []string{intakeClassificationArtifact, "acceptance-criteria.md", "test-log.md", "verification.md", "docs-update.md", "final-report.md"}
	missing := []string{}
	required := stringSet(metadata.RequiredArtifacts)
	for _, artifact := range safetyArtifacts {
		if !required[artifact] {
			missing = append(missing, artifact)
		}
	}
	if len(missing) == 0 {
		return ArtifactValidationCheck{Name: "light_mode_safety_artifacts", Status: ValidationStatusPass, Path: metadataRelative, Message: "safety artifact requirements are retained"}
	}
	return ArtifactValidationCheck{Name: "light_mode_safety_artifacts", Status: ValidationStatusFail, Path: metadataRelative, Message: "required safety artifacts are missing", Hint: "Re-run artifact init so Light mode reduces depth without removing safety evidence.", Field: "required_artifacts", Expected: strings.Join(safetyArtifacts, ","), Actual: strings.Join(missing, ",")}
}

func readValidationArtifact(root Root, runID string, artifact string) (string, []byte, ArtifactValidationCheck) {
	path, err := artifactPath(root, runID, artifact)
	if err != nil {
		return "", nil, ArtifactValidationCheck{Name: "intake_artifact", Status: ValidationStatusFail, Message: "intake artifact path is invalid", Hint: "Use artifact init to create canonical artifact paths.", Field: "path", Expected: artifact, Actual: err.Error()}
	}
	info, err := os.Lstat(path.Absolute)
	if os.IsNotExist(err) {
		return path.Relative, nil, ArtifactValidationCheck{Name: "intake_artifact", Status: ValidationStatusFail, Path: path.Relative, Message: "intake classification artifact is missing", Hint: "Run artifact init, then record the intake classification fields.", Field: "path", Expected: "existing regular file", Actual: "missing"}
	}
	if err != nil {
		return path.Relative, nil, ArtifactValidationCheck{Name: "intake_artifact", Status: ValidationStatusFail, Path: path.Relative, Message: "cannot inspect intake classification artifact", Hint: "Check run artifact permissions before validating.", Field: "path", Expected: "inspectable regular file", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		actual := "non-regular"
		if info.IsDir() {
			actual = "directory"
		}
		return path.Relative, nil, ArtifactValidationCheck{Name: "intake_artifact", Status: ValidationStatusFail, Path: path.Relative, Message: "intake classification artifact must be a regular file", Hint: "Move the conflicting path and re-run artifact init.", Field: "path", Expected: "regular file", Actual: actual}
	}
	if info.Size() == 0 {
		return path.Relative, nil, ArtifactValidationCheck{Name: "intake_artifact", Status: ValidationStatusFail, Path: path.Relative, Message: "intake classification artifact is empty", Hint: "Record the intake classification fields before validation.", Field: "path", Expected: "non-empty file", Actual: "empty"}
	}
	content, err := os.ReadFile(path.Absolute)
	if err != nil {
		return path.Relative, nil, ArtifactValidationCheck{Name: "intake_artifact", Status: ValidationStatusFail, Path: path.Relative, Message: "cannot read intake classification artifact", Hint: "Check run artifact permissions before validating.", Field: "path", Expected: "readable file", Actual: err.Error()}
	}
	return path.Relative, content, ArtifactValidationCheck{Name: "intake_artifact", Status: ValidationStatusPass, Path: path.Relative, Message: "intake classification artifact is present"}
}

func validateIntakeClassification(metadata RunMetadata, intakeRelative string, content []byte) []ArtifactValidationCheck {
	fields := parseMarkdownFields(string(content))
	checks := []ArtifactValidationCheck{}
	status := strings.ToLower(strings.TrimSpace(fields["status"]))
	switch status {
	case "complete":
		checks = append(checks, ArtifactValidationCheck{Name: "intake_status", Status: ValidationStatusPass, Path: intakeRelative, Message: "intake classification is complete"})
	case "":
		checks = append(checks, ArtifactValidationCheck{Name: "intake_status", Status: ValidationStatusFail, Path: intakeRelative, Message: "intake classification status is missing", Hint: "Set Status: complete after recording the intake classification.", Field: "status", Expected: "complete", Actual: "missing"})
	case "pending":
		checks = append(checks, ArtifactValidationCheck{Name: "intake_status", Status: ValidationStatusFail, Path: intakeRelative, Message: "intake classification still has the baseline pending status", Hint: "Replace the baseline with completed intake classification evidence.", Field: "status", Expected: "complete", Actual: status})
	case "not_applicable":
		_, reason := NotApplicableReason(content)
		actual := "missing reason"
		if reason != "" {
			actual = "not_applicable"
		}
		checks = append(checks, ArtifactValidationCheck{Name: "intake_status", Status: ValidationStatusFail, Path: intakeRelative, Message: "intake classification cannot be marked not applicable", Hint: "Every run must record work path classification; use Status: complete.", Field: "status", Expected: "complete", Actual: actual})
	default:
		checks = append(checks, ArtifactValidationCheck{Name: "intake_status", Status: ValidationStatusFail, Path: intakeRelative, Message: "intake classification status is invalid", Hint: "Use Status: complete for intake classification evidence.", Field: "status", Expected: "complete", Actual: status})
	}

	checks = append(checks,
		validateIntakeField(fields, intakeRelative, "work_path", "Work Path", metadata.WorkPath),
		validateIntakeField(fields, intakeRelative, "work_mode", "Work Mode", metadata.WorkMode),
		validateIntakeField(fields, intakeRelative, "sot_policy", "SOT Policy", metadata.SOTPolicy),
		validateIntakeField(fields, intakeRelative, "urgency", "Urgency", metadata.Urgency),
	)
	if metadata.WorkMode == "light" {
		reason := strings.TrimSpace(fields["light_mode_reason"])
		if reason == "" {
			checks = append(checks, ArtifactValidationCheck{Name: "light_mode_reason", Status: ValidationStatusFail, Path: intakeRelative, Message: "Light mode requires an explicit reason", Hint: "Add Light Mode Reason: <why reduced depth is still safe>.", Field: "light_mode_reason", Expected: "non-empty reason", Actual: "missing"})
		} else {
			checks = append(checks, ArtifactValidationCheck{Name: "light_mode_reason", Status: ValidationStatusPass, Path: intakeRelative, Message: "Light mode reason is recorded"})
		}
	}
	return checks
}

func validateIntakeField(fields map[string]string, path string, key string, label string, expected string) ArtifactValidationCheck {
	actual := strings.TrimSpace(fields[key])
	if actual == expected {
		return ArtifactValidationCheck{Name: key, Status: ValidationStatusPass, Path: path, Message: label + " matches run metadata"}
	}
	if actual == "" {
		actual = "missing"
	}
	return ArtifactValidationCheck{Name: key, Status: ValidationStatusFail, Path: path, Message: label + " does not match run metadata", Hint: "Record the exact run metadata value in intake-classification.md.", Field: key, Expected: expected, Actual: actual}
}

func NotApplicableReason(content []byte) (bool, string) {
	fields := parseMarkdownFields(string(content))
	if strings.ToLower(strings.TrimSpace(fields["status"])) != "not_applicable" {
		return false, ""
	}
	reason := strings.TrimSpace(fields["reason"])
	return reason != "", reason
}

func parseMarkdownFields(content string) map[string]string {
	fields := map[string]string{}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = normalizeMarkdownFieldKey(key)
		if key == "" {
			continue
		}
		fields[key] = strings.TrimSpace(value)
	}
	return fields
}

func normalizeMarkdownFieldKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	var b strings.Builder
	previousUnderscore := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			previousUnderscore = false
			continue
		}
		if !previousUnderscore {
			b.WriteByte('_')
			previousUnderscore = true
		}
	}
	return strings.Trim(b.String(), "_")
}

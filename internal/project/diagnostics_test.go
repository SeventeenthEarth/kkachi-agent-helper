package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestExportDiagnosticsUsesActiveRunWhenRunOmittedAndReportsMissingArtifacts(t *testing.T) {
	_, root := newInitializedDiagnosticsRoot(t)
	created := createDiagnosticsRun(t, root, "Diagnostics active default", "111111111111")
	if _, err := ActivateRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: fixedDiagnosticsTime}); err != nil {
		t.Fatalf("ActivateRun() error = %v", err)
	}

	bundle, err := ExportDiagnostics(root, DiagnosticsExportOptions{Now: fixedDiagnosticsTime})
	if err != nil {
		t.Fatalf("ExportDiagnostics() error = %v", err)
	}
	if bundle.RunID != created.Metadata.RunID {
		t.Fatalf("RunID = %q, want active run %q", bundle.RunID, created.Metadata.RunID)
	}
	if bundle.Project.Status.Status != "present" || bundle.Project.Events.Status != "present" || len(bundle.SchemaVersions) != len(canonicalSchemaNames) {
		t.Fatalf("project diagnostics = %#v schemas=%#v, want present project files and schemas", bundle.Project, bundle.SchemaVersions)
	}
	if len(bundle.GateReports) != 0 {
		t.Fatalf("GateReports = %#v, want empty when no reports exist", bundle.GateReports)
	}
	statuses := map[string]string{}
	for _, artifact := range bundle.SelectedArtifacts {
		statuses[artifact.Path] = artifact.Status
	}
	metadataPath := filepath.ToSlash(filepath.Join(RunRootPath, created.Metadata.RunID, "run-metadata.json"))
	if statuses[metadataPath] != "present" {
		t.Fatalf("run metadata status = %q, want present", statuses[metadataPath])
	}
	selectedCLIPath := filepath.ToSlash(filepath.Join(RunRootPath, created.Metadata.RunID, "selected-cli.json"))
	if statuses[selectedCLIPath] != "missing" {
		t.Fatalf("selected-cli status = %q, want missing before artifact init", statuses[selectedCLIPath])
	}
}

func TestExportDiagnosticsProjectLevelWhenNoRunAndOutputOverwriteRefused(t *testing.T) {
	repo, root := newInitializedDiagnosticsRoot(t)

	bundle, err := ExportDiagnostics(root, DiagnosticsExportOptions{Now: fixedDiagnosticsTime})
	if err != nil {
		t.Fatalf("ExportDiagnostics() error = %v", err)
	}
	if bundle.RunID != "" || len(bundle.GateReports) != 0 || len(bundle.SelectedArtifacts) != 0 {
		t.Fatalf("bundle = %#v, want project-level diagnostics only", bundle)
	}

	outputPath := filepath.Join(repo, "diagnostics", "bundle.json")
	mustMkdir(t, filepath.Dir(outputPath))
	if err := os.WriteFile(outputPath, []byte("existing\n"), 0o600); err != nil {
		t.Fatalf("write existing output: %v", err)
	}
	_, err = ExportDiagnostics(root, DiagnosticsExportOptions{Output: "diagnostics/bundle.json", Now: fixedDiagnosticsTime})
	assertProblemCode(t, err, "diagnostics_output_exists")
	if got := readText(t, outputPath); got != "existing\n" {
		t.Fatalf("existing output = %q, want unchanged", got)
	}
}

func TestExportDiagnosticsRedactsMalformedAndNestedContent(t *testing.T) {
	repo, root := newInitializedDiagnosticsRoot(t)
	created := createDiagnosticsRun(t, root, "Diagnostics redaction", "222222222222")
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: fixedDiagnosticsTime}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	secret := "sk-abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	selectedCLI := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "selected-cli.json")
	if err := os.WriteFile(selectedCLI, []byte(`{"version":"0.1","nested":{"api_token":"`+secret+`"}}`+"\n"), 0o600); err != nil {
		t.Fatalf("write selected-cli: %v", err)
	}
	verification := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "verification.md")
	if err := os.WriteFile(verification, []byte("Status: complete\nAuthorization: Bearer "+secret+"\n"), 0o600); err != nil {
		t.Fatalf("write verification: %v", err)
	}
	badSnapshot := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, "bridge-session-snapshot.json")
	if err := os.WriteFile(badSnapshot, []byte(`{"api_token":"`+secret+`"`), 0o600); err != nil {
		t.Fatalf("write malformed snapshot: %v", err)
	}

	bundle, err := ExportDiagnostics(root, DiagnosticsExportOptions{RunID: created.Metadata.RunID, Now: fixedDiagnosticsTime})
	if err != nil {
		t.Fatalf("ExportDiagnostics() error = %v", err)
	}
	for _, artifact := range bundle.SelectedArtifacts {
		if artifact.Path == filepath.ToSlash(filepath.Join(RunRootPath, created.Metadata.RunID, "selected-cli.json")) {
			content := artifact.Content.(map[string]any)
			nested := content["nested"].(map[string]any)
			if nested["api_token"] != RedactedPlaceholder {
				t.Fatalf("selected-cli content = %#v, want nested api_token redacted", artifact.Content)
			}
		}
		if artifact.Path == filepath.ToSlash(filepath.Join(RunRootPath, created.Metadata.RunID, "verification.md")) {
			content := artifact.Content.(string)
			if strings.Contains(content, secret) || !strings.Contains(content, RedactedPlaceholder) {
				t.Fatalf("verification content = %q, want redacted secret", content)
			}
		}
		if artifact.Path == filepath.ToSlash(filepath.Join(RunRootPath, created.Metadata.RunID, "bridge-session-snapshot.json")) {
			if artifact.Status != "invalid" || strings.Contains(artifact.Error, secret) {
				t.Fatalf("snapshot artifact = %#v, want invalid with redacted error", artifact)
			}
		}
	}
}

func newInitializedDiagnosticsRoot(t *testing.T) (string, Root) {
	t.Helper()

	repo := t.TempDir()
	mustMkdir(t, filepath.Join(repo, ".git"))
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}
	if _, err := InitProject(root, deterministicInitOptions()); err != nil {
		t.Fatalf("InitProject() error = %v", err)
	}
	return repo, root
}

func createDiagnosticsRun(t *testing.T, root Root, title string, suffix string) CreateRunResult {
	t.Helper()

	created, err := CreateRun(root, CreateRunOptions{
		Title:         title,
		WorkPath:      "A_development_execution",
		WorkMode:      "standard",
		Urgency:       "normal",
		SOTPolicy:     "existing_sot_basis",
		ExecutionMode: "adapter_qa",
		Commander:     "Gongmyeong",
		TaskID:        "pilot-002",
		Now:           fixedDiagnosticsTime,
		RandomHex:     func(int) (string, error) { return suffix, nil },
	})
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	return created
}

func fixedDiagnosticsTime() time.Time {
	return time.Date(2026, 5, 4, 1, 2, 3, 0, time.UTC)
}

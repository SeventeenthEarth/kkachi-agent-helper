package project

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInitArtifactsCreatesCanonicalFilesAndUpdatesMetadata(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	result, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	if result.RunID != created.Metadata.RunID || result.EventID != "evt-000003" {
		t.Fatalf("result = %#v, want run id and artifact event", result)
	}
	if len(result.Created) != len(canonicalArtifactPaths) || len(result.RequiredArtifacts) == 0 {
		t.Fatalf("created=%d required=%d, want canonical files and required manifest", len(result.Created), len(result.RequiredArtifacts))
	}
	for _, artifact := range canonicalArtifactPaths {
		path := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID, filepath.FromSlash(artifact))
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("missing artifact %s: %v", artifact, err)
		}
		if info.Size() == 0 {
			t.Fatalf("artifact %s is empty, want baseline content", artifact)
		}
	}
	metadata := readRunMetadata(t, repo, created.Metadata.RunID)
	if len(metadata.RequiredArtifacts) != len(result.RequiredArtifacts) || !containsString(metadata.RequiredArtifacts, "diff.patch") || !containsString(metadata.RequiredArtifacts, "task-brief.md") || !containsString(metadata.RequiredArtifacts, "redteam/final-gate-review.md") {
		t.Fatalf("required_artifacts = %#v, want production write manifest", metadata.RequiredArtifacts)
	}
	lines := runEventLines(t, repo)
	if len(lines) != 3 || !strings.Contains(lines[2], `"type":"artifact.written"`) || !strings.Contains(lines[2], `"artifact_count"`) {
		t.Fatalf("events = %#v, want artifact.written", lines)
	}
}

func TestInitArtifactsPreservesNonEmptyAndReinitializesEmpty(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runDir := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID)
	if err := os.WriteFile(filepath.Join(runDir, "plan.md"), []byte("custom plan\n"), 0o600); err != nil {
		t.Fatalf("write custom plan: %v", err)
	}
	if err := os.WriteFile(filepath.Join(runDir, "checklist.md"), nil, 0o600); err != nil {
		t.Fatalf("write empty checklist: %v", err)
	}

	result, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	if !containsArtifactPath(result.Preserved, "plan.md") || !containsArtifactPath(result.Reinitialized, "checklist.md") {
		t.Fatalf("preserved=%#v reinitialized=%#v, want non-empty preserved and empty reinitialized", result.Preserved, result.Reinitialized)
	}
	if got := readText(t, filepath.Join(runDir, "plan.md")); got != "custom plan\n" {
		t.Fatalf("plan.md = %q, want preserved custom content", got)
	}
	if got := readText(t, filepath.Join(runDir, "checklist.md")); !strings.Contains(got, "Status: pending") {
		t.Fatalf("checklist.md = %q, want baseline content", got)
	}
}

func TestListArtifactsUsesManifestBeforeAndAfterInit(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	options := deterministicCreateRunOptions()
	options.WorkPath = "B_discovery_shaping"
	options.ExecutionMode = "research"
	created, err := CreateRun(root, options)
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}

	before, err := ListArtifacts(root, created.Metadata.RunID)
	if err != nil {
		t.Fatalf("ListArtifacts(before) error = %v", err)
	}
	if before.RunID != created.Metadata.RunID {
		t.Fatalf("before.RunID = %q, want %q", before.RunID, created.Metadata.RunID)
	}
	if len(before.Artifacts) != len(canonicalArtifactPaths) || !artifactRequired(before.Artifacts, "discovery/handoff-to-development.md") || !artifactRequired(before.Artifacts, "discovery/research-notes.md") {
		t.Fatalf("before = %#v, want Path B research required discovery artifacts", before)
	}
	if before.Artifacts[0].Exists {
		t.Fatalf("before.Artifacts[0] = %#v, want missing before init", before.Artifacts[0])
	}
	if _, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("InitArtifacts() error = %v", err)
	}
	after, err := ListArtifacts(root, created.Metadata.RunID)
	if err != nil {
		t.Fatalf("ListArtifacts(after) error = %v", err)
	}
	if !after.Artifacts[0].Exists || after.Artifacts[0].Empty {
		t.Fatalf("after.Artifacts[0] = %#v, want initialized artifact", after.Artifacts[0])
	}
}

func TestInitArtifactsRejectsFinishedRun(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := CloseRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("CloseRun() error = %v", err)
	}
	_, err = InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	assertProblemCode(t, err, "run_artifact_init_invalid_state")
}

func TestInitArtifactsRejectsAbortedRun(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	if _, err := AbortRun(root, RunLifecycleOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)}); err != nil {
		t.Fatalf("AbortRun() error = %v", err)
	}
	_, err = InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	assertProblemCode(t, err, "run_artifact_init_invalid_state")
}

func TestInitArtifactsSecondRunPreservesAllExistingArtifacts(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	first, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	if err != nil {
		t.Fatalf("InitArtifacts(first) error = %v", err)
	}
	second, err := InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(5)})
	if err != nil {
		t.Fatalf("InitArtifacts(second) error = %v", err)
	}
	if len(first.Created) != len(canonicalArtifactPaths) {
		t.Fatalf("first.Created=%d, want canonical artifact count", len(first.Created))
	}
	if len(second.Created) != 0 || len(second.Reinitialized) != 0 || len(second.Preserved) != len(canonicalArtifactPaths) {
		t.Fatalf("second created=%d reinitialized=%d preserved=%d, want all preserved", len(second.Created), len(second.Reinitialized), len(second.Preserved))
	}
	if second.EventID != "evt-000004" {
		t.Fatalf("second.EventID = %q, want evt-000004", second.EventID)
	}
}

func TestInitArtifactsRejectsSymlinkArtifactPath(t *testing.T) {
	repo := initializedRepo(t)
	root, _ := DiscoverRoot(repo)
	created, err := CreateRun(root, deterministicCreateRunOptions())
	if err != nil {
		t.Fatalf("CreateRun() error = %v", err)
	}
	runDir := filepath.Join(repo, ".kkachi", "runs", created.Metadata.RunID)
	if err := os.Symlink(filepath.Join(repo, ".kkachi", "status.json"), filepath.Join(runDir, "plan.md")); err != nil {
		t.Fatalf("create artifact symlink: %v", err)
	}
	_, err = InitArtifacts(root, ArtifactInitOptions{RunID: created.Metadata.RunID, Now: testRunNow(4)})
	assertProblemCode(t, err, "artifact_path_invalid")
}

func TestArtifactManifestExecutionModes(t *testing.T) {
	tests := []struct {
		name          string
		executionMode string
		want          []string
	}{
		{name: "production write", executionMode: "production_write", want: []string{"diff.patch", "impl-log.md", "review.md", "redteam/impl-review.md", "redteam/test-review.md", "redteam/final-gate-review.md"}},
		{name: "readiness hardening", executionMode: "readiness_hardening", want: []string{"diff.patch", "impl-log.md", "review.md", "redteam/impl-review.md", "redteam/test-review.md", "redteam/final-gate-review.md"}},
		{name: "adapter qa", executionMode: "adapter_qa", want: []string{"selected-cli.json", "capability-check.md", "bridge-session-snapshot.json", "bridge-events.md", "cli-output.md", "redteam/qa-review.md"}},
		{name: "research", executionMode: "research", want: []string{"discovery/research-notes.md", "discovery/strategy-options.md", "discovery/selected-strategy.md"}},
		{name: "docs only", executionMode: "docs_only", want: []string{"sot-update.md", "roadmap-update.md"}},
		{name: "verification", executionMode: "verification", want: []string{"review.md"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := RunMetadata{
				WorkPath:      "A_development_execution",
				WorkMode:      "standard",
				ExecutionMode: tt.executionMode,
			}
			got := ArtifactManifest(metadata)
			for _, want := range tt.want {
				if !containsString(got, want) {
					t.Fatalf("ArtifactManifest(%s) = %#v, missing %s", tt.executionMode, got, want)
				}
			}
			if hasDuplicates(got) {
				t.Fatalf("ArtifactManifest(%s) = %#v, want no duplicates", tt.executionMode, got)
			}
		})
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func hasDuplicates(values []string) bool {
	seen := map[string]bool{}
	for _, value := range values {
		if seen[value] {
			return true
		}
		seen[value] = true
	}
	return false
}

func containsArtifactPath(values []ArtifactStatus, target string) bool {
	for _, value := range values {
		if value.Path == target {
			return true
		}
	}
	return false
}

func artifactRequired(values []ArtifactStatus, target string) bool {
	for _, value := range values {
		if value.Path == target {
			return value.Required
		}
	}
	return false
}

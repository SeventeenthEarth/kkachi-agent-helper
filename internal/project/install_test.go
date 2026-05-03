package project

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const installOwnerMarker = "<!-- kkachi-agent-helper:managed -->"

func TestPlanInstallDryRunClassifiesPathActions(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "skills/create/SKILL.md", installOwnerMarker+"\ncreate\n")
	writeInstallSourceFile(t, source, "skills/update/SKILL.md", installOwnerMarker+"\nnew\n")
	writeInstallSourceFile(t, source, "skills/unchanged/SKILL.md", installOwnerMarker+"\nsame\n")
	writeInstallSourceFile(t, source, "skills/preserve/SKILL.md", installOwnerMarker+"\nupstream\n")
	writeInstallSourceFile(t, source, "skills/conflict/SKILL.md", installOwnerMarker+"\nconflict\n")
	writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{
		manifestItem(t, source, "skills/create/SKILL.md", ".codex/skills/create/SKILL.md"),
		manifestItem(t, source, "skills/update/SKILL.md", ".codex/skills/update/SKILL.md"),
		manifestItem(t, source, "skills/unchanged/SKILL.md", ".codex/skills/unchanged/SKILL.md"),
		manifestItem(t, source, "skills/preserve/SKILL.md", ".codex/skills/preserve/SKILL.md"),
		manifestItem(t, source, "skills/conflict/SKILL.md", ".codex/skills/conflict/SKILL.md"),
	})
	writeRepoFile(t, repo, ".codex/skills/update/SKILL.md", installOwnerMarker+"\nold\n")
	writeRepoFile(t, repo, ".codex/skills/unchanged/SKILL.md", installOwnerMarker+"\nsame\n")
	writeRepoFile(t, repo, ".codex/skills/preserve/SKILL.md", "user custom\n")
	writeInstallSourceFile(t, source, "skills/body-marker/SKILL.md", installOwnerMarker+"\nbody marker upstream\n")
	appendInstallManifestItem(t, source, InstallKindSkills, manifestItem(t, source, "skills/body-marker/SKILL.md", ".codex/skills/body-marker/SKILL.md"))
	writeRepoFile(t, repo, ".codex/skills/body-marker/SKILL.md", "user custom before marker\n"+installOwnerMarker+"\n")
	mustMkdir(t, filepath.Join(repo, ".codex", "skills", "conflict", "SKILL.md"))
	beforeEvents := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))

	result, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
	if err != nil {
		t.Fatalf("PlanInstall() error = %v", err)
	}
	if result.Summary.Create != 1 || result.Summary.Update != 1 || result.Summary.Unchanged != 1 || result.Summary.Preserve != 2 || result.Summary.Conflict != 1 {
		t.Fatalf("summary = %#v, want create/update/unchanged/conflict and two preserve actions", result.Summary)
	}
	if result.Create[0].Target != ".codex/skills/create/SKILL.md" || result.Update[0].Target != ".codex/skills/update/SKILL.md" || !installActionTargetListed(result.Preserve, ".codex/skills/preserve/SKILL.md") || !installActionTargetListed(result.Preserve, ".codex/skills/body-marker/SKILL.md") || result.Conflict[0].Target != ".codex/skills/conflict/SKILL.md" {
		t.Fatalf("actions = %#v/%#v/%#v/%#v", result.Create, result.Update, result.Preserve, result.Conflict)
	}
	if got := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); got != beforeEvents {
		t.Fatalf("dry-run mutated events\nbefore=%s\nafter=%s", beforeEvents, got)
	}
	if got := readText(t, filepath.Join(repo, ".codex", "skills", "update", "SKILL.md")); got != installOwnerMarker+"\nold\n" {
		t.Fatalf("dry-run mutated target = %q", got)
	}
}

func TestPlanInstallRejectsInvalidContracts(t *testing.T) {
	_, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "skills/a/SKILL.md", installOwnerMarker+"\na\n")
	valid := manifestItem(t, source, "skills/a/SKILL.md", ".codex/skills/a/SKILL.md")

	t.Run("kind mismatch", func(t *testing.T) {
		writeInstallManifest(t, source, InstallKindTemplates, []InstallManifestItem{valid})
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_manifest_kind_mismatch")
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		bad := valid
		bad.SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{bad})
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_checksum_mismatch")
	})

	t.Run("duplicate target", func(t *testing.T) {
		writeInstallSourceFile(t, source, "skills/b/SKILL.md", installOwnerMarker+"\nb\n")
		duplicate := manifestItem(t, source, "skills/b/SKILL.md", valid.Target)
		writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{valid, duplicate})
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_duplicate_target")
	})

	t.Run("source escape", func(t *testing.T) {
		bad := valid
		bad.Source = "../escape.md"
		bad.SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{bad})
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_source_item_invalid")
	})

	t.Run("target escape", func(t *testing.T) {
		bad := valid
		bad.Target = "../escape.md"
		writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{bad})
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "path_escape")
	})
}

func TestApplyInstallWritesHelperOwnedTargetsAndRecordsEvent(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "skills/create/SKILL.md", installOwnerMarker+"\ncreate\n")
	writeInstallSourceFile(t, source, "skills/update/SKILL.md", installOwnerMarker+"\nnew\n")
	writeInstallSourceFile(t, source, "skills/unchanged/SKILL.md", installOwnerMarker+"\nsame\n")
	writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{
		manifestItem(t, source, "skills/create/SKILL.md", ".codex/skills/create/SKILL.md"),
		manifestItem(t, source, "skills/update/SKILL.md", ".codex/skills/update/SKILL.md"),
		manifestItem(t, source, "skills/unchanged/SKILL.md", ".codex/skills/unchanged/SKILL.md"),
	})
	writeRepoFile(t, repo, ".codex/skills/update/SKILL.md", installOwnerMarker+"\nold\n")
	writeRepoFile(t, repo, ".codex/skills/unchanged/SKILL.md", installOwnerMarker+"\nsame\n")

	result, err := ApplyInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, HelperVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("ApplyInstall() error = %v", err)
	}
	if result.Status != InstallStatusApplied || result.EventID != "evt-000002" || result.Summary.Create != 1 || result.Summary.Update != 1 || result.Summary.Unchanged != 1 {
		t.Fatalf("result = %#v, want applied create/update/unchanged", result)
	}
	if got := readText(t, filepath.Join(repo, ".codex", "skills", "create", "SKILL.md")); got != installOwnerMarker+"\ncreate\n" {
		t.Fatalf("created target = %q", got)
	}
	if got := readText(t, filepath.Join(repo, ".codex", "skills", "update", "SKILL.md")); got != installOwnerMarker+"\nnew\n" {
		t.Fatalf("updated target = %q", got)
	}
	if events := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); !strings.Contains(events, `"type":"install.applied"`) {
		t.Fatalf("events = %s, want install.applied", events)
	}
}

func TestApplyInstallBlocksPreserveBeforeWriting(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "skills/create/SKILL.md", installOwnerMarker+"\ncreate\n")
	writeInstallSourceFile(t, source, "skills/preserve/SKILL.md", installOwnerMarker+"\nupstream\n")
	writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{
		manifestItem(t, source, "skills/create/SKILL.md", ".codex/skills/create/SKILL.md"),
		manifestItem(t, source, "skills/preserve/SKILL.md", ".codex/skills/preserve/SKILL.md"),
	})
	writeRepoFile(t, repo, ".codex/skills/preserve/SKILL.md", "user custom\n")
	beforeEvents := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl"))

	_, err := ApplyInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, HelperVersion: "1.2.3"})
	assertProblemCode(t, err, "install_preflight_blocked")
	if _, statErr := os.Stat(filepath.Join(repo, ".codex", "skills", "create", "SKILL.md")); !os.IsNotExist(statErr) {
		t.Fatalf("create target stat = %v, want absent after blocked install", statErr)
	}
	if got := readText(t, filepath.Join(repo, ".kkachi", "events.jsonl")); got != beforeEvents {
		t.Fatalf("blocked install mutated events\nbefore=%s\nafter=%s", beforeEvents, got)
	}
}

func TestApplyInstallRespectsProjectWriteLock(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "skills/create/SKILL.md", installOwnerMarker+"\ncreate\n")
	writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{
		manifestItem(t, source, "skills/create/SKILL.md", ".codex/skills/create/SKILL.md"),
	})
	writeLockMetadata(t, repo, ProjectWriteLockName, LockMetadata{Version: LockVersion, LockName: ProjectWriteLockName, OwnerPID: os.Getpid(), Hostname: mustHostname(t), Command: "fresh install writer", CreatedAt: time.Now().UTC().Format(time.RFC3339)})

	_, err := ApplyInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, HelperVersion: "1.2.3"})
	assertProblemCode(t, err, "lock_conflict")
	if _, statErr := os.Stat(filepath.Join(repo, ".codex", "skills", "create", "SKILL.md")); !os.IsNotExist(statErr) {
		t.Fatalf("create target stat = %v, want absent under lock conflict", statErr)
	}
}

func TestInstallDriftCheckAndCompatibility(t *testing.T) {
	_, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "templates/README.md", installOwnerMarker+"\ntemplate\n")
	writeInstallManifest(t, source, InstallKindTemplates, []InstallManifestItem{manifestItem(t, source, "templates/README.md", "docs/kkachi/README.md")})

	drifted, err := ApplyInstall(root, InstallPlanOptions{Kind: InstallKindTemplates, Source: source, DriftCheck: true, HelperVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("drift ApplyInstall() error = %v", err)
	}
	if drifted.Status != InstallStatusDrifted || drifted.Summary.Create != 1 {
		t.Fatalf("drifted = %#v, want create drift", drifted)
	}
	if _, err := ApplyInstall(root, InstallPlanOptions{Kind: InstallKindTemplates, Source: source, HelperVersion: "1.2.3"}); err != nil {
		t.Fatalf("real install error = %v", err)
	}
	clean, err := ApplyInstall(root, InstallPlanOptions{Kind: InstallKindTemplates, Source: source, DriftCheck: true, HelperVersion: "1.2.3"})
	if err != nil {
		t.Fatalf("clean ApplyInstall() error = %v", err)
	}
	if clean.Status != InstallStatusClean || clean.Summary.Unchanged != 1 {
		t.Fatalf("clean = %#v, want clean unchanged", clean)
	}

	manifest := InstallManifest{Version: SchemaVersion, Kind: InstallKindTemplates, Package: InstallPackage{Name: "kkachi-test-pack", Version: "0.1.0"}, Compat: InstallCompat{RequiredHelper: ">=9.0.0"}, Items: []InstallManifestItem{manifestItem(t, source, "templates/README.md", "docs/kkachi/README.md")}}
	writeInstallManifestPayload(t, source, manifest)
	_, err = ApplyInstall(root, InstallPlanOptions{Kind: InstallKindTemplates, Source: source, HelperVersion: "1.2.3"})
	assertProblemCode(t, err, "install_compatibility_failed")
}

func TestInstallCompatibilityRejectsMalformedRangeAndMissingHelperVersion(t *testing.T) {
	_, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "templates/README.md", installOwnerMarker+"\ntemplate\n")
	item := manifestItem(t, source, "templates/README.md", "docs/kkachi/README.md")

	for _, tt := range []struct {
		name          string
		required      string
		helperVersion string
	}{
		{name: "malformed range", required: "^1.0.0", helperVersion: "1.2.3"},
		{name: "missing helper version", required: ">=0.1.0", helperVersion: ""},
		{name: "dev helper version", required: ">=0.1.0", helperVersion: "0.0.0-dev"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			manifest := InstallManifest{Version: SchemaVersion, Kind: InstallKindTemplates, Package: InstallPackage{Name: "kkachi-test-pack", Version: "0.1.0"}, Compat: InstallCompat{RequiredHelper: tt.required}, Items: []InstallManifestItem{item}}
			writeInstallManifestPayload(t, source, manifest)
			_, err := ApplyInstall(root, InstallPlanOptions{Kind: InstallKindTemplates, Source: source, HelperVersion: tt.helperVersion})
			assertProblemCode(t, err, "install_compatibility_failed")
		})
	}
}

func TestPlanInstallRejectsManifestAndSourceRootFailures(t *testing.T) {
	_, root, _ := initSchemaTestProject(t)

	t.Run("empty source", func(t *testing.T) {
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: "   ", DryRun: true})
		assertProblemCode(t, err, "install_source_required")
	})

	t.Run("missing source root", func(t *testing.T) {
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: filepath.Join(t.TempDir(), "missing"), DryRun: true})
		assertProblemCode(t, err, "install_source_invalid")
	})

	t.Run("source root is file", func(t *testing.T) {
		sourceFile := filepath.Join(t.TempDir(), "source-file")
		if err := os.WriteFile(sourceFile, []byte("not dir\n"), 0o600); err != nil {
			t.Fatalf("write source file: %v", err)
		}
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: sourceFile, DryRun: true})
		assertProblemCode(t, err, "install_source_invalid")
	})

	t.Run("missing manifest", func(t *testing.T) {
		source := t.TempDir()
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_manifest_read_failed")
	})

	t.Run("invalid manifest json", func(t *testing.T) {
		source := t.TempDir()
		if err := os.WriteFile(filepath.Join(source, InstallManifestFile), []byte("{bad json\n"), 0o600); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_manifest_invalid_json")
	})

	t.Run("missing required manifest fields", func(t *testing.T) {
		source := t.TempDir()
		if err := os.WriteFile(filepath.Join(source, InstallManifestFile), []byte(`{"version":"0.1","kind":"skills","package":{"name":"p","version":"0.1.0"},"compat":{},"items":[]}`+"\n"), 0o600); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_manifest_invalid")
	})

	t.Run("unknown manifest field", func(t *testing.T) {
		source := t.TempDir()
		if err := os.WriteFile(filepath.Join(source, InstallManifestFile), []byte(`{"version":"0.1","kind":"skills","package":{"name":"p","version":"0.1.0"},"compat":{"required_helper":">=0.1.0"},"items":[],"unexpected":true}`+"\n"), 0o600); err != nil {
			t.Fatalf("write manifest: %v", err)
		}
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_manifest_invalid_json")
	})
}

func TestPlanInstallRejectsSourceWithoutDeclaredOwnerMarker(t *testing.T) {
	_, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "skills/a/SKILL.md", "missing marker\n")
	writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{manifestItem(t, source, "skills/a/SKILL.md", ".codex/skills/a/SKILL.md")})

	_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true, HelperVersion: "1.2.3"})
	assertProblemCode(t, err, "install_owner_marker_missing")
}

func TestPlanInstallRejectsSourceAndTargetSafetyFailures(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "skills/a/SKILL.md", installOwnerMarker+"\na\n")
	valid := manifestItem(t, source, "skills/a/SKILL.md", ".codex/skills/a/SKILL.md")

	t.Run("missing source item", func(t *testing.T) {
		bad := valid
		bad.Source = "skills/missing/SKILL.md"
		bad.SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{bad})
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_source_item_invalid")
	})

	t.Run("source item is directory", func(t *testing.T) {
		mustMkdir(t, filepath.Join(source, "skills", "dir"))
		bad := valid
		bad.Source = "skills/dir"
		bad.SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{bad})
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_source_item_invalid")
	})

	t.Run("source symlink escapes source root", func(t *testing.T) {
		outside := t.TempDir()
		outsideFile := filepath.Join(outside, "outside.md")
		if err := os.WriteFile(outsideFile, []byte(installOwnerMarker+"\noutside\n"), 0o600); err != nil {
			t.Fatalf("write outside: %v", err)
		}
		link := filepath.Join(source, "skills", "escape.md")
		_ = os.Remove(link)
		if err := os.Symlink(outsideFile, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		bad := valid
		bad.Source = "skills/escape.md"
		bad.SHA256 = "0000000000000000000000000000000000000000000000000000000000000000"
		writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{bad})
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "install_source_item_invalid")
	})

	t.Run("target symlink escapes repo root", func(t *testing.T) {
		outside := t.TempDir()
		mustMkdir(t, filepath.Join(repo, ".codex"))
		link := filepath.Join(repo, ".codex", "escape")
		_ = os.Remove(link)
		if err := os.Symlink(outside, link); err != nil {
			t.Fatalf("symlink: %v", err)
		}
		bad := valid
		bad.Target = ".codex/escape/SKILL.md"
		writeInstallManifest(t, source, InstallKindSkills, []InstallManifestItem{bad})
		_, err := PlanInstall(root, InstallPlanOptions{Kind: InstallKindSkills, Source: source, DryRun: true})
		assertProblemCode(t, err, "symlink_escape")
	})
}

func TestInstallManifestSchemaValidation(t *testing.T) {
	repo, root, _ := initSchemaTestProject(t)
	source := t.TempDir()
	writeInstallSourceFile(t, source, "templates/README.md", installOwnerMarker+"\ntemplate\n")
	manifest := InstallManifest{
		Version: SchemaVersion,
		Kind:    InstallKindTemplates,
		Package: InstallPackage{Name: "kkachi-project-overlay", Version: "0.1.0"},
		Compat:  InstallCompat{RequiredHelper: ">=0.1.0", RequiredBridge: ">=0.1.0"},
		Items:   []InstallManifestItem{manifestItem(t, source, "templates/README.md", "docs/README.md")},
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	manifestPath := filepath.Join(repo, "kkachi-install-manifest.json")
	if err := os.WriteFile(manifestPath, append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	result, err := ValidateSchemaFile(root, SchemaValidateOptions{File: "kkachi-install-manifest.json", Schema: SchemaInstallManifest})
	if err != nil {
		t.Fatalf("ValidateSchemaFile() error = %v", err)
	}
	if result.Status != "pass" {
		t.Fatalf("result = %#v, want pass", result)
	}
}

func writeInstallSourceFile(t *testing.T, root string, relative string, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(relative))
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write source %s: %v", relative, err)
	}
}

func writeRepoFile(t *testing.T, repo string, relative string, content string) {
	t.Helper()
	path := filepath.Join(repo, filepath.FromSlash(relative))
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write repo %s: %v", relative, err)
	}
}

func manifestItem(t *testing.T, sourceRoot string, source string, target string) InstallManifestItem {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(sourceRoot, filepath.FromSlash(source)))
	if err != nil {
		t.Fatalf("read source %s: %v", source, err)
	}
	return InstallManifestItem{Source: source, Target: target, SHA256: sha256Hex(content), OwnerMarker: installOwnerMarker}
}

func writeInstallManifest(t *testing.T, root string, kind string, items []InstallManifestItem) {
	t.Helper()
	manifest := InstallManifest{Version: SchemaVersion, Kind: kind, Package: InstallPackage{Name: "kkachi-test-pack", Version: "0.1.0"}, Compat: InstallCompat{RequiredHelper: ">=0.1.0", RequiredBridge: ">=0.1.0", RequiredSkills: ">=0.1.0"}, Items: items}
	writeInstallManifestPayload(t, root, manifest)
}

func appendInstallManifestItem(t *testing.T, root string, kind string, item InstallManifestItem) {
	t.Helper()
	path := filepath.Join(root, InstallManifestFile)
	var manifest InstallManifest
	readJSONFile(t, path, &manifest)
	if manifest.Kind != kind {
		t.Fatalf("manifest kind = %q, want %q", manifest.Kind, kind)
	}
	manifest.Items = append(manifest.Items, item)
	writeInstallManifestPayload(t, root, manifest)
}

func writeInstallManifestPayload(t *testing.T, root string, manifest InstallManifest) {
	t.Helper()
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, InstallManifestFile), append(data, '\n'), 0o600); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
}

func installActionTargetListed(actions []InstallPlanAction, target string) bool {
	for _, action := range actions {
		if action.Target == target {
			return true
		}
	}
	return false
}

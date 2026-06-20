package project

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

func TestProbeToolchainInitializedProject(t *testing.T) {
	repo := initializedRepo(t)
	root, err := DiscoverRoot(repo)
	if err != nil {
		t.Fatalf("DiscoverRoot() error = %v", err)
	}

	result, err := ProbeToolchain(root, ToolchainProbeOptions{HelperCommand: "kkachi-agent-helper", HelperVersion: "1.2.3", BinaryPath: "/tmp/kah"})
	if err != nil {
		t.Fatalf("ProbeToolchain() error = %v", err)
	}

	if !result.OK || result.SchemaVersion != "kah.toolchain_probe.v1" {
		t.Fatalf("result = %#v, want ok v1 probe", result)
	}
	if !result.NoWrite.Guaranteed || result.NoWrite.WriteCount != 0 {
		t.Fatalf("no_write = %#v, want guaranteed zero writes", result.NoWrite)
	}
	if result.KAH.HelperCommand != "kkachi-agent-helper" || result.KAH.Version != "1.2.3" || result.KAH.BinaryPath != "/tmp/kah" {
		t.Fatalf("kah = %#v, want helper facts", result.KAH)
	}
	if result.Project.Root != canonicalPath(t, repo) || result.Project.KkachiDir != filepath.Join(canonicalPath(t, repo), ".kkachi") {
		t.Fatalf("project = %#v, want canonical project paths", result.Project)
	}
	if !result.Project.KkachiDirPresent || !result.Project.ProjectInitialized {
		t.Fatalf("project = %#v, want initialized facts", result.Project)
	}
	if result.Doctor.Status != "PASS" || len(result.Doctor.ReasonCodes) != 0 {
		t.Fatalf("doctor = %#v, want PASS with no reason codes", result.Doctor)
	}
}

func TestProbeToolchainInitializedProjectWithDoctorWarningKeepsInitializedSignal(t *testing.T) {
	repo := initializedRepo(t)
	writeLockMetadata(t, repo, ActiveRunLockName, LockMetadata{Version: LockVersion, LockName: ActiveRunLockName, OwnerPID: 999999, Hostname: "other-host", Command: "stale writer", CreatedAt: time.Now().UTC().Add(-31 * time.Minute).Format(time.RFC3339)})

	result, err := ProbeToolchain(Root{Path: repo}, ToolchainProbeOptions{})
	if err != nil {
		t.Fatalf("ProbeToolchain() error = %v", err)
	}

	if !result.Project.ProjectInitialized {
		t.Fatalf("project = %#v, want initialized signal even when doctor warns", result.Project)
	}
	if result.Doctor.Status != "WARN" || !slices.Contains(result.Doctor.ReasonCodes, "locks_warn") {
		t.Fatalf("doctor = %#v, want WARN locks_warn", result.Doctor)
	}
}

func TestProbeToolchainUninitializedProjectIsNoWrite(t *testing.T) {
	repo := t.TempDir()
	before := snapshotProjectTree(t, repo)

	result, err := ProbeToolchain(Root{Path: repo}, ToolchainProbeOptions{HelperVersion: "1.2.3", BinaryPath: "/tmp/kah"})
	if err != nil {
		t.Fatalf("ProbeToolchain() error = %v", err)
	}

	after := snapshotProjectTree(t, repo)
	if !slices.Equal(before, after) {
		t.Fatalf("tree changed after probe: before=%#v after=%#v", before, after)
	}
	if _, err := os.Stat(filepath.Join(repo, ".kkachi")); !os.IsNotExist(err) {
		t.Fatalf(".kkachi stat = %v, want absent", err)
	}
	if !result.OK || result.Project.KkachiDirPresent || result.Project.ProjectInitialized || result.Project.WorkflowGraphPresent {
		t.Fatalf("result = %#v, want successful uninitialized facts", result)
	}
	if result.Doctor.Status != "FAIL" || !slices.Contains(result.Doctor.ReasonCodes, "kkachi_dir_missing") {
		t.Fatalf("doctor = %#v, want missing .kkachi reason code", result.Doctor)
	}
}

func snapshotProjectTree(t *testing.T, root string) []string {
	t.Helper()

	var entries []string
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == root {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		entries = append(entries, filepath.ToSlash(relative))
		return nil
	}); err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	slices.Sort(entries)
	return entries
}

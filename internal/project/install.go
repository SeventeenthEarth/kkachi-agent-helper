package project

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	InstallKindSkills    = "skills"
	InstallKindTemplates = "templates"

	InstallManifestFile = "kkachi-install-manifest.json"

	installActionCreate    = "create"
	installActionUpdate    = "update"
	installActionUnchanged = "unchanged"
	installActionPreserve  = "preserve"
	installActionConflict  = "conflict"
)

type InstallPlanOptions struct {
	Kind   string
	Source string
	DryRun bool
}

type InstallManifest struct {
	Version string                `json:"version"`
	Kind    string                `json:"kind"`
	Package InstallPackage        `json:"package"`
	Compat  InstallCompat         `json:"compat"`
	Items   []InstallManifestItem `json:"items"`
}

type InstallPackage struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type InstallCompat struct {
	RequiredHelper string `json:"required_helper"`
	RequiredBridge string `json:"required_bridge,omitempty"`
	RequiredSkills string `json:"required_skills,omitempty"`
}

type InstallManifestItem struct {
	Source      string `json:"source"`
	Target      string `json:"target"`
	SHA256      string `json:"sha256"`
	OwnerMarker string `json:"owner_marker"`
}

type InstallPlanResult struct {
	DryRun       bool                `json:"dry_run"`
	Kind         string              `json:"kind"`
	Source       string              `json:"source"`
	ManifestPath string              `json:"manifest_path"`
	Package      InstallPackage      `json:"package"`
	Compat       InstallCompat       `json:"compat"`
	Summary      InstallPlanSummary  `json:"summary"`
	Create       []InstallPlanAction `json:"create"`
	Update       []InstallPlanAction `json:"update"`
	Unchanged    []InstallPlanAction `json:"unchanged"`
	Preserve     []InstallPlanAction `json:"preserve"`
	Conflict     []InstallPlanAction `json:"conflict"`
}

type InstallPlanSummary struct {
	Create    int `json:"create"`
	Update    int `json:"update"`
	Unchanged int `json:"unchanged"`
	Preserve  int `json:"preserve"`
	Conflict  int `json:"conflict"`
}

type InstallPlanAction struct {
	Target      string `json:"target"`
	Source      string `json:"source"`
	SHA256      string `json:"sha256"`
	OwnerMarker string `json:"owner_marker"`
	Reason      string `json:"reason"`
}

type installPlannedAction struct {
	Bucket string
	Action InstallPlanAction
}

func PlanInstall(root Root, options InstallPlanOptions) (InstallPlanResult, error) {
	if strings.TrimSpace(root.Path) == "" {
		return InstallPlanResult{}, problem("repo_root_required", "repository root is required", "Discover the repository root before planning installs.")
	}
	kind := strings.TrimSpace(options.Kind)
	if !allowed(kind, InstallKindSkills, InstallKindTemplates) {
		return InstallPlanResult{}, &Problem{Code: "install_kind_invalid", Message: "install kind is not supported", Hint: "Use install skills or install templates.", Field: "kind", Expected: InstallKindSkills + " or " + InstallKindTemplates, Actual: kind}
	}
	if !options.DryRun {
		return InstallPlanResult{}, &Problem{Code: "install_real_not_implemented", Message: "real install is not implemented yet", Hint: "Use --dry-run for packg-003; local install/update is reserved for packg-004.", Field: "--dry-run", Expected: "dry-run preview", Actual: "missing"}
	}
	sourceRoot, err := resolveInstallSourceRoot(options.Source)
	if err != nil {
		return InstallPlanResult{}, err
	}
	manifestPath := filepath.Join(sourceRoot, InstallManifestFile)
	manifest, err := readInstallManifest(manifestPath)
	if err != nil {
		return InstallPlanResult{}, err
	}
	if err := validateInstallManifest(manifestPath, manifest, kind); err != nil {
		return InstallPlanResult{}, err
	}
	result := InstallPlanResult{
		DryRun:       true,
		Kind:         kind,
		Source:       filepath.ToSlash(sourceRoot),
		ManifestPath: filepath.ToSlash(manifestPath),
		Package:      manifest.Package,
		Compat:       manifest.Compat,
		Create:       []InstallPlanAction{},
		Update:       []InstallPlanAction{},
		Unchanged:    []InstallPlanAction{},
		Preserve:     []InstallPlanAction{},
		Conflict:     []InstallPlanAction{},
	}
	seenTargets := map[string]struct{}{}
	for index, item := range manifest.Items {
		planned, err := planInstallItem(root, sourceRoot, item, index, seenTargets)
		if err != nil {
			return InstallPlanResult{}, err
		}
		appendInstallAction(&result, planned)
	}
	result.Summary = InstallPlanSummary{Create: len(result.Create), Update: len(result.Update), Unchanged: len(result.Unchanged), Preserve: len(result.Preserve), Conflict: len(result.Conflict)}
	return result, nil
}

func resolveInstallSourceRoot(source string) (string, error) {
	trimmed := strings.TrimSpace(source)
	if trimmed == "" {
		return "", &Problem{Code: "install_source_required", Message: "install source is required", Hint: "Use install skills --source <local-path> --dry-run.", Field: "--source", Expected: "local path", Actual: "missing"}
	}
	path := filepath.Clean(filepath.FromSlash(trimmed))
	if !filepath.IsAbs(path) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return "", &Problem{Code: "install_source_invalid", Message: "cannot resolve install source path", Hint: "Use a readable local path source.", Path: source, Field: "--source", Expected: "resolvable local path", Actual: err.Error()}
		}
		path = abs
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", &Problem{Code: "install_source_invalid", Message: "cannot resolve install source path", Hint: "Use a readable local directory source.", Path: source, Field: "--source", Expected: "readable local directory", Actual: err.Error()}
	}
	info, err := os.Lstat(resolved)
	if err != nil {
		return "", &Problem{Code: "install_source_invalid", Message: "cannot inspect install source path", Hint: "Use a readable local directory source.", Path: source, Field: "--source", Expected: "readable local directory", Actual: err.Error()}
	}
	if !info.IsDir() {
		return "", &Problem{Code: "install_source_invalid", Message: "install source must be a directory", Hint: "Point --source at a directory containing kkachi-install-manifest.json.", Path: source, Field: "--source", Expected: "directory", Actual: "file"}
	}
	return filepath.Clean(resolved), nil
}

func readInstallManifest(path string) (InstallManifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return InstallManifest{}, &Problem{Code: "install_manifest_read_failed", Message: "cannot read install manifest", Hint: "Create kkachi-install-manifest.json at the source root.", Path: filepath.ToSlash(path), Field: "manifest", Expected: "readable JSON manifest", Actual: err.Error()}
	}
	var manifest InstallManifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return InstallManifest{}, &Problem{Code: "install_manifest_invalid_json", Message: "install manifest is not valid JSON", Hint: "Fix kkachi-install-manifest.json to match the packg-003 manifest contract.", Path: filepath.ToSlash(path), Field: "json", Expected: "install manifest object", Actual: err.Error()}
	}
	return manifest, nil
}

func validateInstallManifest(path string, manifest InstallManifest, commandKind string) error {
	relativePath := filepath.ToSlash(path)
	if manifest.Version != SchemaVersion {
		return &Problem{Code: "install_manifest_invalid", Message: "install manifest version is unsupported", Hint: "Use manifest version " + SchemaVersion + ".", Path: relativePath, Field: "version", Expected: SchemaVersion, Actual: manifest.Version}
	}
	if manifest.Kind != commandKind {
		return &Problem{Code: "install_manifest_kind_mismatch", Message: "install manifest kind does not match command", Hint: "Use install " + manifest.Kind + " for this source or fix manifest kind.", Path: relativePath, Field: "kind", Expected: commandKind, Actual: manifest.Kind}
	}
	if strings.TrimSpace(manifest.Package.Name) == "" {
		return installManifestFieldProblem(relativePath, "package.name", "non-empty string", "missing")
	}
	if strings.TrimSpace(manifest.Package.Version) == "" {
		return installManifestFieldProblem(relativePath, "package.version", "non-empty string", "missing")
	}
	if strings.TrimSpace(manifest.Compat.RequiredHelper) == "" {
		return installManifestFieldProblem(relativePath, "compat.required_helper", "non-empty helper version range", "missing")
	}
	if len(manifest.Items) == 0 {
		return installManifestFieldProblem(relativePath, "items", "at least one item", "empty")
	}
	for i, item := range manifest.Items {
		prefix := fmt.Sprintf("items[%d].", i)
		if strings.TrimSpace(item.Source) == "" {
			return installManifestFieldProblem(relativePath, prefix+"source", "non-empty relative path", "missing")
		}
		if strings.TrimSpace(item.Target) == "" {
			return installManifestFieldProblem(relativePath, prefix+"target", "non-empty repository-relative path", "missing")
		}
		if !isSHA256Hex(item.SHA256) {
			return installManifestFieldProblem(relativePath, prefix+"sha256", "64 lowercase hex characters", item.SHA256)
		}
		if strings.TrimSpace(item.OwnerMarker) == "" {
			return installManifestFieldProblem(relativePath, prefix+"owner_marker", "non-empty helper-owned marker", "missing")
		}
	}
	return nil
}

func installManifestFieldProblem(path string, field string, expected string, actual string) *Problem {
	return &Problem{Code: "install_manifest_invalid", Message: "install manifest field is invalid", Hint: "Fix kkachi-install-manifest.json to match the packg-003 manifest contract.", Path: path, Field: field, Expected: expected, Actual: actual}
}

func appendInstallAction(result *InstallPlanResult, planned installPlannedAction) {
	switch planned.Bucket {
	case installActionCreate:
		result.Create = append(result.Create, planned.Action)
	case installActionUpdate:
		result.Update = append(result.Update, planned.Action)
	case installActionUnchanged:
		result.Unchanged = append(result.Unchanged, planned.Action)
	case installActionPreserve:
		result.Preserve = append(result.Preserve, planned.Action)
	default:
		result.Conflict = append(result.Conflict, planned.Action)
	}
}

func planInstallItem(root Root, sourceRoot string, item InstallManifestItem, index int, seenTargets map[string]struct{}) (installPlannedAction, error) {
	sourcePath, sourceRelative, err := resolveSourceFile(sourceRoot, item.Source, index)
	if err != nil {
		return installPlannedAction{}, err
	}
	content, err := os.ReadFile(sourcePath)
	if err != nil {
		return installPlannedAction{}, &Problem{Code: "install_source_read_failed", Message: "cannot read install source item", Hint: "Check the package item path and permissions.", Path: sourceRelative, Field: fmt.Sprintf("items[%d].source", index), Expected: "readable source file", Actual: err.Error()}
	}
	actualHash := sha256Hex(content)
	if actualHash != item.SHA256 {
		return installPlannedAction{}, &Problem{Code: "install_checksum_mismatch", Message: "install source checksum does not match manifest", Hint: "Regenerate the manifest checksum after changing source content.", Path: sourceRelative, Field: fmt.Sprintf("items[%d].sha256", index), Expected: item.SHA256, Actual: actualHash}
	}
	target, err := ResolveRelativePath(root, item.Target)
	if err != nil {
		return installPlannedAction{}, err
	}
	if _, ok := seenTargets[target.Relative]; ok {
		return installPlannedAction{}, &Problem{Code: "install_duplicate_target", Message: "install manifest declares a duplicate target", Hint: "Each install manifest item must target a distinct path.", Path: target.Relative, Field: fmt.Sprintf("items[%d].target", index), Expected: "unique target path", Actual: target.Relative}
	}
	seenTargets[target.Relative] = struct{}{}
	action := InstallPlanAction{Target: target.Relative, Source: sourceRelative, SHA256: item.SHA256, OwnerMarker: item.OwnerMarker}
	info, err := os.Lstat(target.Absolute)
	if os.IsNotExist(err) {
		action.Reason = "target absent; dry-run would create helper-owned file"
		return installPlannedAction{Bucket: installActionCreate, Action: action}, nil
	}
	if err != nil {
		return installPlannedAction{}, &Problem{Code: "install_target_inspection_failed", Message: "cannot inspect install target path", Hint: "Check target path permissions before installing.", Path: target.Relative, Field: "target", Expected: "inspectable target path", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		action.Reason = "target is not a regular file; dry-run reports conflict"
		return installPlannedAction{Bucket: installActionConflict, Action: action}, nil
	}
	existing, err := os.ReadFile(target.Absolute)
	if err != nil {
		return installPlannedAction{}, &Problem{Code: "install_target_read_failed", Message: "cannot read install target path", Hint: "Check target path permissions before installing.", Path: target.Relative, Field: "target", Expected: "readable target file", Actual: err.Error()}
	}
	if !hasInstallOwnerMarker(existing, item.OwnerMarker) {
		action.Reason = "target lacks owner marker; dry-run preserves user-owned file"
		return installPlannedAction{Bucket: installActionPreserve, Action: action}, nil
	}
	if bytes.Equal(existing, content) {
		action.Reason = "helper-owned target already matches source"
		return installPlannedAction{Bucket: installActionUnchanged, Action: action}, nil
	}
	action.Reason = "helper-owned target differs; dry-run would update"
	return installPlannedAction{Bucket: installActionUpdate, Action: action}, nil
}

func hasInstallOwnerMarker(content []byte, marker string) bool {
	marker = strings.TrimSpace(marker)
	if marker == "" {
		return false
	}
	for _, line := range bytes.Split(content, []byte("\n")) {
		trimmed := strings.TrimSpace(string(line))
		if trimmed == "" {
			continue
		}
		return trimmed == marker
	}
	return false
}

func resolveSourceFile(sourceRoot string, value string, index int) (string, string, error) {
	original := value
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", &Problem{Code: "install_source_item_invalid", Message: "install source item path is empty", Hint: "Use a relative source path inside the package source root.", Field: fmt.Sprintf("items[%d].source", index), Expected: "source-root-confined relative path", Actual: "empty"}
	}
	path := filepath.Clean(filepath.FromSlash(value))
	if filepath.IsAbs(path) || path == "." || path == ".." || strings.HasPrefix(path, ".."+string(filepath.Separator)) {
		return "", "", &Problem{Code: "install_source_item_invalid", Message: "install source item path must stay inside the source root", Hint: "Use a package-relative file path without parent traversal.", Path: original, Field: fmt.Sprintf("items[%d].source", index), Expected: "source-root-confined relative path", Actual: original}
	}
	absolute := filepath.Join(sourceRoot, path)
	resolvedRoot := sourceRoot
	if rootSymlink, err := filepath.EvalSymlinks(sourceRoot); err == nil {
		resolvedRoot = rootSymlink
	}
	if err := rejectSourceEscapingSymlinks(resolvedRoot, path); err != nil {
		return "", "", err
	}
	if !isWithinRoot(resolvedRoot, absolute) {
		return "", "", &Problem{Code: "install_source_item_invalid", Message: "install source item path must stay inside the source root", Hint: "Use a package-relative file path without parent traversal.", Path: original, Field: fmt.Sprintf("items[%d].source", index), Expected: "source-root-confined relative path", Actual: original}
	}
	info, err := os.Lstat(absolute)
	if err != nil {
		return "", "", &Problem{Code: "install_source_item_invalid", Message: "cannot inspect install source item", Hint: "Check that every manifest source item exists as a regular file.", Path: filepath.ToSlash(path), Field: fmt.Sprintf("items[%d].source", index), Expected: "regular source file", Actual: err.Error()}
	}
	if !info.Mode().IsRegular() {
		return "", "", &Problem{Code: "install_source_item_invalid", Message: "install source item must be a regular file", Hint: "Use regular files as install package items.", Path: filepath.ToSlash(path), Field: fmt.Sprintf("items[%d].source", index), Expected: "regular source file", Actual: "non-regular"}
	}
	return absolute, filepath.ToSlash(path), nil
}

func rejectSourceEscapingSymlinks(rootPath string, relative string) error {
	current := rootPath
	for _, part := range splitPath(relative) {
		current = filepath.Join(current, part)
		info, err := os.Lstat(current)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return &Problem{Code: "install_source_item_invalid", Message: "cannot inspect install source item path component", Hint: "Check package source permissions.", Path: filepath.ToSlash(current), Field: "source", Expected: "inspectable source path", Actual: err.Error()}
		}
		if info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		resolved, err := filepath.EvalSymlinks(current)
		if err != nil {
			return &Problem{Code: "install_source_item_invalid", Message: "cannot resolve install source symlink", Hint: "Remove broken symlinks from install package items.", Path: filepath.ToSlash(current), Field: "source", Expected: "resolvable symlink inside source root", Actual: err.Error()}
		}
		if !isWithinRoot(rootPath, resolved) {
			return &Problem{Code: "install_source_item_invalid", Message: "install source symlink escapes the source root", Hint: "Replace the symlink with a regular package file or an internal symlink.", Path: filepath.ToSlash(current), Field: "source", Expected: "symlink target inside source root", Actual: filepath.ToSlash(resolved)}
		}
	}
	return nil
}

func isSHA256Hex(value string) bool {
	if len(value) != 64 {
		return false
	}
	for _, r := range value {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}

func sha256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

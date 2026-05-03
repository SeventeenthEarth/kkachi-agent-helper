package project

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	schemaMigratedEventType = "schema.migrated"
	migrationNoopName       = "noop-0.1-to-0.1"
)

type SchemaMigrationOptions struct {
	From   string
	To     string
	DryRun bool
	Now    func() time.Time
}

type SchemaMigrationResult struct {
	DryRun       bool     `json:"dry_run"`
	FromVersion  string   `json:"from_version"`
	ToVersion    string   `json:"to_version"`
	Status       string   `json:"status"`
	Migration    string   `json:"migration"`
	WouldBackup  []string `json:"would_backup"`
	BackedUp     []string `json:"backed_up"`
	BackupPath   string   `json:"backup_path,omitempty"`
	WouldMigrate []string `json:"would_migrate"`
	Migrated     []string `json:"migrated"`
	Unchanged    []string `json:"unchanged"`
	EventID      string   `json:"event_id,omitempty"`
}

type schemaMigration struct {
	from string
	to   string
	name string
}

var schemaMigrations = []schemaMigration{{from: SchemaVersion, to: SchemaVersion, name: migrationNoopName}}

func MigrateSchemaState(root Root, options SchemaMigrationOptions) (SchemaMigrationResult, error) {
	if strings.TrimSpace(root.Path) == "" {
		return SchemaMigrationResult{}, problem("repo_root_required", "repository root is required", "Discover the repository root before migrating schema state.")
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	migration, err := resolveSchemaMigration(options.From, options.To)
	if err != nil {
		return SchemaMigrationResult{}, err
	}
	if options.DryRun {
		return migrateSchemaStateUnlocked(root, options, migration)
	}
	var result SchemaMigrationResult
	err = withProjectWriteLock(root, "schema migrate", "", func() error {
		var err error
		result, err = migrateSchemaStateUnlocked(root, options, migration)
		return err
	})
	return result, err
}

func resolveSchemaMigration(from string, to string) (schemaMigration, error) {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" {
		return schemaMigration{}, &Problem{Code: "schema_migration_from_required", Message: "schema migrate requires --from", Hint: "Use schema migrate --from 0.1 --to 0.1.", Field: "--from", Expected: "registered source version", Actual: "missing"}
	}
	if to == "" {
		return schemaMigration{}, &Problem{Code: "schema_migration_to_required", Message: "schema migrate requires --to", Hint: "Use schema migrate --from 0.1 --to 0.1.", Field: "--to", Expected: "registered target version", Actual: "missing"}
	}
	knownSource := false
	for _, migration := range schemaMigrations {
		if migration.from == from {
			knownSource = true
			if migration.to == to {
				return migration, nil
			}
		}
	}
	if !knownSource {
		return schemaMigration{}, &Problem{Code: "schema_migration_unknown_source_version", Message: "schema migration source version is not registered", Hint: "Use a helper release that knows how to migrate this source version, or restore supported helper state.", Field: "--from", Expected: registeredMigrationSources(), Actual: from}
	}
	return schemaMigration{}, &Problem{Code: "schema_migration_not_registered", Message: "schema migration path is not registered", Hint: "Use a registered migration path before changing schema versions.", Field: "--to", Expected: registeredMigrationTargets(from), Actual: to}
}

func migrateSchemaStateUnlocked(root Root, options SchemaMigrationOptions, migration schemaMigration) (SchemaMigrationResult, error) {
	if err := preflightEventCoherence(root); err != nil {
		return SchemaMigrationResult{}, err
	}
	paths, err := collectMigrationPaths(root)
	if err != nil {
		return SchemaMigrationResult{}, err
	}
	if err := verifyMigrationSourceVersions(root, paths, migration.from); err != nil {
		return SchemaMigrationResult{}, err
	}
	result := SchemaMigrationResult{
		DryRun:       options.DryRun,
		FromVersion:  migration.from,
		ToVersion:    migration.to,
		Status:       "pass",
		Migration:    migration.name,
		WouldBackup:  relativePaths(paths),
		WouldMigrate: []string{},
		Unchanged:    relativePaths(paths),
	}
	if options.DryRun {
		return result, nil
	}
	backupPath, backedUp, err := writeMigrationBackup(root, migration, paths, options.Now())
	if err != nil {
		return SchemaMigrationResult{}, err
	}
	result.BackupPath = backupPath
	result.BackedUp = backedUp
	appendResult, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: schemaMigratedEventType, Payload: map[string]any{"from_version": migration.from, "to_version": migration.to, "migration": migration.name, "backup_path": backupPath, "backed_up": backedUp, "migrated": result.Migrated, "unchanged": result.Unchanged}, Now: options.Now}, nil)
	if err != nil {
		return SchemaMigrationResult{}, err
	}
	result.EventID = appendResult.EventID
	return result, nil
}

func collectMigrationPaths(root Root) ([]SafePath, error) {
	relatives := []string{ConfigPath, StatusPath, EventsPath}
	relatives = append(relatives, schemaPaths...)
	runMetadataPaths, err := collectRunMetadataPaths(root)
	if err != nil {
		return nil, err
	}
	relatives = append(relatives, runMetadataPaths...)
	paths := make([]SafePath, 0, len(relatives))
	for _, relative := range relatives {
		path, err := ResolveRelativePath(root, relative)
		if err != nil {
			return nil, err
		}
		if _, err := os.Lstat(path.Absolute); os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, &Problem{Code: "schema_migration_path_inspection_failed", Message: "cannot inspect migration state path", Hint: "Check helper state permissions before migrating.", Path: path.Relative, Field: "path", Expected: "inspectable helper state path", Actual: err.Error()}
		}
		paths = append(paths, path)
	}
	sort.Slice(paths, func(i, j int) bool { return paths[i].Relative < paths[j].Relative })
	return paths, nil
}

func collectRunMetadataPaths(root Root) ([]string, error) {
	runRoot, err := ResolveRelativePath(root, RunRootPath)
	if err != nil {
		return nil, err
	}
	if _, err := os.Lstat(runRoot.Absolute); os.IsNotExist(err) {
		return nil, nil
	} else if err != nil {
		return nil, &Problem{Code: "schema_migration_path_inspection_failed", Message: "cannot inspect run root", Hint: "Check .kkachi/runs permissions before migrating.", Path: runRoot.Relative, Field: "path", Expected: "inspectable run root", Actual: err.Error()}
	}
	var relatives []string
	err = filepath.WalkDir(runRoot.Absolute, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return &Problem{Code: "schema_migration_path_inspection_failed", Message: "cannot inspect run metadata path", Hint: "Check run artifact permissions before migrating.", Path: filepath.ToSlash(path), Field: "path", Expected: "inspectable run path", Actual: err.Error()}
		}
		if entry.IsDir() {
			return nil
		}
		if entry.Name() != "run-metadata.json" {
			return nil
		}
		relative, relErr := filepath.Rel(root.Path, path)
		if relErr != nil {
			return &Problem{Code: "schema_migration_path_inspection_failed", Message: "cannot relativize run metadata path", Hint: "Check repository root and run artifact paths.", Path: filepath.ToSlash(path), Field: "path", Expected: "repository-relative path", Actual: relErr.Error()}
		}
		relatives = append(relatives, filepath.ToSlash(relative))
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(relatives)
	return relatives, nil
}

func verifyMigrationSourceVersions(root Root, paths []SafePath, from string) error {
	for _, path := range paths {
		version, err := migrationFileVersion(path)
		if err != nil {
			return err
		}
		if version != from {
			return &Problem{Code: "schema_migration_source_version_mismatch", Message: "helper state source version does not match requested migration", Hint: "Run schema migrate with the current state version or restore coherent helper state before migrating.", Path: path.Relative, Field: "version", Expected: from, Actual: version}
		}
	}
	return nil
}

func migrationFileVersion(path SafePath) (string, error) {
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return "", &Problem{Code: "schema_migration_read_failed", Message: "cannot read helper state for migration", Hint: "Check file permissions before migrating.", Path: path.Relative, Field: "path", Expected: "readable helper state file", Actual: err.Error()}
	}
	if path.Relative == ConfigPath {
		version := strings.TrimSpace(parseSimpleConfig(data)["version"])
		if version == "" {
			return "", &Problem{Code: "schema_migration_version_missing", Message: "helper state is missing a version", Hint: "Restore versioned helper state before migrating.", Path: path.Relative, Field: "version", Expected: "non-empty version", Actual: "missing"}
		}
		return version, nil
	}
	if path.Relative == EventsPath {
		return migrationEventLogVersion(path, data)
	}
	var object map[string]any
	if err := json.Unmarshal(data, &object); err != nil {
		return "", &Problem{Code: "schema_migration_invalid_json", Message: "helper state is not valid JSON", Hint: "Restore valid JSON helper state before migrating.", Path: path.Relative, Field: "json", Expected: "JSON object", Actual: err.Error()}
	}
	version, ok := object["version"].(string)
	if !ok || strings.TrimSpace(version) == "" {
		return "", &Problem{Code: "schema_migration_version_missing", Message: "helper state is missing a version", Hint: "Restore versioned helper state before migrating.", Path: path.Relative, Field: "version", Expected: "non-empty version", Actual: fmt.Sprintf("%v", object["version"])}
	}
	return strings.TrimSpace(version), nil
}

func migrationEventLogVersion(path SafePath, data []byte) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	scanner.Buffer(make([]byte, 0, 64*1024), MaxEventLineBytes+1)
	line := 0
	version := ""
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			return "", &Problem{Code: "schema_migration_invalid_event_log", Message: "event log contains an empty line", Hint: "Restore a valid event log before migrating.", Path: path.Relative, Field: fmt.Sprintf("line_%d", line), Expected: "JSON object event line", Actual: "empty"}
		}
		var object map[string]any
		if err := json.Unmarshal([]byte(text), &object); err != nil {
			return "", &Problem{Code: "schema_migration_invalid_event_log", Message: "event log contains invalid JSON", Hint: "Restore a valid event log before migrating.", Path: path.Relative, Field: fmt.Sprintf("line_%d", line), Expected: "JSON object event line", Actual: err.Error()}
		}
		lineVersion, ok := object["version"].(string)
		if !ok || strings.TrimSpace(lineVersion) == "" {
			return "", &Problem{Code: "schema_migration_version_missing", Message: "event log line is missing a version", Hint: "Restore versioned event log state before migrating.", Path: path.Relative, Field: fmt.Sprintf("line_%d.version", line), Expected: "non-empty version", Actual: fmt.Sprintf("%v", object["version"])}
		}
		lineVersion = strings.TrimSpace(lineVersion)
		if version == "" {
			version = lineVersion
			continue
		}
		if lineVersion != version {
			return "", &Problem{Code: "schema_migration_source_version_mismatch", Message: "event log contains mixed source versions", Hint: "Restore coherent event log state before migrating.", Path: path.Relative, Field: fmt.Sprintf("line_%d.version", line), Expected: version, Actual: lineVersion}
		}
	}
	if err := scanner.Err(); err != nil {
		return "", &Problem{Code: "schema_migration_read_failed", Message: "cannot scan event log for migration", Hint: "Check event log permissions before migrating.", Path: path.Relative, Field: "events", Expected: "readable event log", Actual: err.Error()}
	}
	if version == "" {
		return "", &Problem{Code: "schema_migration_version_missing", Message: "event log is empty", Hint: "Restore versioned event log state before migrating.", Path: path.Relative, Field: "events", Expected: "at least one versioned event", Actual: "empty"}
	}
	return version, nil
}

func writeMigrationBackup(root Root, migration schemaMigration, paths []SafePath, now time.Time) (string, []string, error) {
	stamp := now.UTC().Format("20060102T150405Z")
	baseRelative := fmt.Sprintf(".kkachi/backups/schema-migrations/%s-%s-to-%s", stamp, sanitizeMigrationVersion(migration.from), sanitizeMigrationVersion(migration.to))
	backupRoot, err := nextBackupRoot(root, baseRelative)
	if err != nil {
		return "", nil, err
	}
	if err := os.MkdirAll(backupRoot.Absolute, 0o700); err != nil {
		return "", nil, &Problem{Code: "schema_migration_backup_failed", Message: "cannot create migration backup directory", Hint: "Check repository permissions before migrating.", Path: backupRoot.Relative, Field: "path", Expected: "writable backup directory", Actual: err.Error()}
	}
	if err := syncDirectory(filepath.Dir(backupRoot.Absolute)); err != nil {
		return "", nil, &Problem{Code: "schema_migration_backup_failed", Message: "cannot sync migration backup parent", Hint: "Check repository storage health before retrying.", Path: filepath.ToSlash(filepath.Dir(backupRoot.Relative)), Field: "path", Expected: "synced backup parent", Actual: err.Error()}
	}
	backedUp := make([]string, 0, len(paths))
	for _, source := range paths {
		target := filepath.Join(backupRoot.Absolute, filepath.FromSlash(source.Relative))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return "", nil, &Problem{Code: "schema_migration_backup_failed", Message: "cannot create migration backup subdirectory", Hint: "Check repository permissions before migrating.", Path: backupRoot.Relative, Field: "path", Expected: "writable backup subdirectory", Actual: err.Error()}
		}
		if err := copyRegularFile(source.Absolute, target); err != nil {
			return "", nil, &Problem{Code: "schema_migration_backup_failed", Message: "cannot copy helper state into migration backup", Hint: "Preserve the partial backup for diagnosis and check repository permissions.", Path: source.Relative, Field: "path", Expected: "copyable regular file", Actual: err.Error()}
		}
		backedUp = append(backedUp, source.Relative)
	}
	return backupRoot.Relative, backedUp, nil
}

func nextBackupRoot(root Root, baseRelative string) (SafePath, error) {
	for i := 0; i < 100; i++ {
		relative := baseRelative
		if i > 0 {
			relative = fmt.Sprintf("%s-%02d", baseRelative, i)
		}
		path, err := ResolveRelativePath(root, relative)
		if err != nil {
			return SafePath{}, err
		}
		if _, err := os.Lstat(path.Absolute); os.IsNotExist(err) {
			return path, nil
		} else if err != nil {
			return SafePath{}, &Problem{Code: "schema_migration_backup_failed", Message: "cannot inspect migration backup directory", Hint: "Check repository permissions before migrating.", Path: path.Relative, Field: "path", Expected: "inspectable backup directory", Actual: err.Error()}
		}
	}
	return SafePath{}, &Problem{Code: "schema_migration_backup_failed", Message: "cannot allocate unique migration backup directory", Hint: "Retry migration after the helper clock advances or remove invalid partial backups after inspection.", Path: baseRelative, Field: "path", Expected: "available backup directory suffix", Actual: "all suffixes in use"}
}

func copyRegularFile(source string, target string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer output.Close()
	if _, err := io.Copy(output, input); err != nil {
		return err
	}
	return output.Sync()
}

func relativePaths(paths []SafePath) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		result = append(result, path.Relative)
	}
	return result
}

func registeredMigrationSources() string {
	seen := map[string]struct{}{}
	for _, migration := range schemaMigrations {
		seen[migration.from] = struct{}{}
	}
	return sortedKeys(seen)
}

func registeredMigrationTargets(from string) string {
	seen := map[string]struct{}{}
	for _, migration := range schemaMigrations {
		if migration.from == from {
			seen[migration.to] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

func sortedKeys(values map[string]struct{}) string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func sanitizeMigrationVersion(version string) string {
	version = strings.TrimSpace(version)
	var builder strings.Builder
	for _, r := range version {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '-' || r == '_' {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('-')
	}
	if builder.Len() == 0 {
		return "unknown"
	}
	return builder.String()
}

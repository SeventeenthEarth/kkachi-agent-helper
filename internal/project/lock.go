package project

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

const (
	LockVersion            = "0.1"
	ActiveRunLockName      = "active_run"
	ProjectWriteLockName   = "project_write"
	activeRunLockPath      = ".kkachi/active_run.lock"
	projectWriteLockPath   = ".kkachi/project_write.lock"
	LockStaleAfter         = 30 * time.Minute
	lockRecoveredEventType = "lock.recovered"
)

var lockPaths = []string{
	activeRunLockPath,
	projectWriteLockPath,
}

type LockMetadata struct {
	Version   string `json:"version"`
	LockName  string `json:"lock_name"`
	RunID     string `json:"run_id,omitempty"`
	OwnerPID  int    `json:"owner_pid"`
	Hostname  string `json:"hostname"`
	Command   string `json:"command"`
	CreatedAt string `json:"created_at"`
}

type LockRecoveryOptions struct {
	Target string
	Reason string
	RunID  string
	Now    func() time.Time
}

type LockRecoveryResult struct {
	Recovered []LockMetadata `json:"recovered"`
}

type lockHandle struct {
	path     SafePath
	metadata LockMetadata
	released bool
}

type lockInspection struct {
	path     SafePath
	metadata LockMetadata
	stale    bool
	reason   string
}

func withProjectWriteLock(root Root, command string, runID string, fn func() error) error {
	lock, err := acquireLock(root, ProjectWriteLockName, command, runID, time.Now().UTC())
	if err != nil {
		return err
	}
	err = fn()
	releaseErr := lock.release()
	if err != nil {
		// Preserve the primary mutation failure. A release failure after a
		// failed mutation leaves a recoverable stale lock that project doctor can
		// diagnose, while replacing the original error would hide the cause.
		return err
	}
	return releaseErr
}

func withRunLifecycleLocks(root Root, command string, runID string, fn func() error) error {
	projectLock, err := acquireLock(root, ProjectWriteLockName, command, runID, time.Now().UTC())
	if err != nil {
		return err
	}
	activeLock, err := acquireLock(root, ActiveRunLockName, command, runID, time.Now().UTC())
	if err != nil {
		projectReleaseErr := projectLock.release()
		if projectReleaseErr != nil {
			return projectReleaseErr
		}
		return err
	}
	err = fn()
	activeReleaseErr := activeLock.release()
	projectReleaseErr := projectLock.release()
	if err != nil {
		// Preserve the primary mutation failure. Release failures are still
		// self-healing through stale-lock recovery and should not mask the cause
		// of the failed lifecycle transition.
		return err
	}
	if activeReleaseErr != nil {
		return activeReleaseErr
	}
	return projectReleaseErr
}

func RecoverLocks(root Root, options LockRecoveryOptions) (LockRecoveryResult, error) {
	if strings.TrimSpace(root.Path) == "" {
		return LockRecoveryResult{}, problem("repo_root_required", "repository root is required", "Discover the repository root before recovering locks.")
	}
	options.Target = strings.TrimSpace(options.Target)
	options.Reason = strings.TrimSpace(options.Reason)
	if options.Reason == "" {
		return LockRecoveryResult{}, &Problem{Code: "lock_recovery_reason_required", Message: "lock recovery requires a reason", Hint: "Pass --reason with a concise explanation of why recovery is safe.", Field: "reason", Expected: "non-empty reason", Actual: "empty"}
	}
	if options.Now == nil {
		options.Now = func() time.Time { return time.Now().UTC() }
	}
	names, err := lockTargetNames(options.Target)
	if err != nil {
		return LockRecoveryResult{}, err
	}

	if options.Target == "active-run" {
		var result LockRecoveryResult
		err := withProjectWriteLock(root, "lock recover", options.RunID, func() error {
			var err error
			result, err = recoverLocksUnlocked(root, options, names, true)
			return err
		})
		return result, err
	}

	return recoverLocksUnlocked(root, options, names, false)
}

func recoverLocksUnlocked(root Root, options LockRecoveryOptions, names []string, projectWriteHeld bool) (LockRecoveryResult, error) {
	// If recovery is not explicitly recovering project_write.lock, a fresh project
	// writer must still block event/status mutation performed by recovery.
	if !projectWriteHeld && !containsLockName(names, ProjectWriteLockName) {
		projectPath, err := lockPath(root, ProjectWriteLockName)
		if err != nil {
			return LockRecoveryResult{}, err
		}
		if _, statErr := os.Lstat(projectPath.Absolute); statErr == nil {
			inspection, err := inspectLockFile(root, ProjectWriteLockName, options.Now())
			if err != nil {
				return LockRecoveryResult{}, err
			}
			if !inspection.stale {
				return LockRecoveryResult{}, lockConflictProblem(inspection)
			}
		} else if statErr != nil && !errors.Is(statErr, os.ErrNotExist) {
			return LockRecoveryResult{}, &Problem{Code: "lock_metadata_invalid", Message: "cannot inspect project write lock", Hint: "Preserve the lock for diagnosis and run project doctor.", Path: projectPath.Relative, Field: "path", Expected: "inspectable lock path", Actual: statErr.Error()}
		}
	}

	inspections := make([]lockInspection, 0, len(names))
	for _, name := range names {
		path, err := lockPath(root, name)
		if err != nil {
			return LockRecoveryResult{}, err
		}
		if _, statErr := os.Lstat(path.Absolute); errors.Is(statErr, os.ErrNotExist) {
			if options.Target == "all" {
				continue
			}
			return LockRecoveryResult{}, &Problem{Code: "lock_not_found", Message: "lock file was not found", Hint: "Run project doctor to inspect current lock state.", Path: path.Relative, Field: "path", Expected: "present stale lock", Actual: "absent"}
		} else if statErr != nil {
			return LockRecoveryResult{}, &Problem{Code: "lock_metadata_invalid", Message: "cannot inspect lock file", Hint: "Preserve the lock for diagnosis and run project doctor.", Path: path.Relative, Field: "path", Expected: "inspectable lock path", Actual: statErr.Error()}
		}
		inspection, err := inspectLockFile(root, name, options.Now())
		if err != nil {
			return LockRecoveryResult{}, err
		}
		if !inspection.stale {
			return LockRecoveryResult{}, lockConflictProblem(inspection)
		}
		inspections = append(inspections, inspection)
	}
	if len(inspections) == 0 {
		return LockRecoveryResult{}, &Problem{Code: "lock_not_found", Message: "no stale lock was found", Hint: "Run project doctor to inspect current lock state.", Field: "lock", Expected: "at least one stale lock", Actual: "none"}
	}

	result := LockRecoveryResult{Recovered: make([]LockMetadata, 0, len(inspections))}
	for _, inspection := range inspections {
		payload := map[string]any{
			"lock_name":    inspection.metadata.LockName,
			"path":         inspection.path.Relative,
			"reason":       options.Reason,
			"stale":        true,
			"stale_reason": inspection.reason,
			"owner":        inspection.metadata,
		}
		// Recovering project_write.lock cannot itself hold project_write.lock, so
		// recovery remains an explicit operator action that records the unlock
		// before removing a verified-stale lock. active-run-only recovery is
		// serialized by RecoverLocks before reaching this point.
		if _, err := appendEventWithStatusMutation(root, AppendEventOptions{Type: lockRecoveredEventType, RunID: options.RunID, Payload: payload, Now: options.Now}, nil); err != nil {
			return LockRecoveryResult{}, err
		}
		if err := removeLockIfIdentityMatches(inspection.path, inspection.metadata); err != nil {
			return LockRecoveryResult{}, err
		}
		result.Recovered = append(result.Recovered, inspection.metadata)
	}
	return result, nil
}

func acquireLock(root Root, name string, command string, runID string, now time.Time) (*lockHandle, error) {
	path, err := lockPath(root, name)
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path.Absolute), 0o755); err != nil {
		return nil, helperStateDirectoryProblem(path, err)
	}
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "unknown"
	}
	metadata := LockMetadata{Version: LockVersion, LockName: name, RunID: strings.TrimSpace(runID), OwnerPID: os.Getpid(), Hostname: hostname, Command: strings.TrimSpace(command), CreatedAt: now.UTC().Format(time.RFC3339)}
	if metadata.Command == "" {
		metadata.Command = "unknown"
	}
	content, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return nil, err
	}
	content = append(content, '\n')
	file, err := os.OpenFile(path.Absolute, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			inspection, inspectErr := inspectLockFile(root, name, time.Now().UTC())
			if inspectErr != nil {
				return nil, inspectErr
			}
			if inspection.stale {
				return nil, lockStaleProblem(inspection)
			}
			return nil, lockConflictProblem(inspection)
		}
		return nil, &Problem{Code: "lock_acquire_failed", Message: "cannot create lock file", Hint: "Check .kkachi permissions and retry.", Path: path.Relative, Field: "path", Expected: "atomic lock file create", Actual: err.Error()}
	}
	closed := false
	cleanup := func() {
		if !closed {
			_ = file.Close()
		}
		_ = removeLockIfIdentityMatches(path, metadata)
	}
	if _, err := file.Write(content); err != nil {
		cleanup()
		return nil, &Problem{Code: "lock_acquire_failed", Message: "cannot write lock metadata", Hint: "Check repository storage health before retrying.", Path: path.Relative, Field: "metadata", Expected: "durable lock metadata", Actual: err.Error()}
	}
	if err := file.Sync(); err != nil {
		cleanup()
		return nil, &Problem{Code: "lock_acquire_failed", Message: "cannot sync lock metadata", Hint: "Check repository storage health before retrying.", Path: path.Relative, Field: "metadata", Expected: "synced lock metadata", Actual: err.Error()}
	}
	if err := file.Close(); err != nil {
		closed = true
		_ = removeLockIfIdentityMatches(path, metadata)
		return nil, &Problem{Code: "lock_acquire_failed", Message: "cannot close lock file", Hint: "Check repository storage health before retrying.", Path: path.Relative, Field: "path", Expected: "closed lock file", Actual: err.Error()}
	}
	closed = true
	if err := syncDirectory(filepath.Dir(path.Absolute)); err != nil {
		_ = removeLockIfIdentityMatches(path, metadata)
		return nil, &Problem{Code: "lock_acquire_failed", Message: "cannot sync lock directory", Hint: "Check repository storage health before retrying.", Path: filepath.ToSlash(filepath.Dir(path.Relative)), Field: "path", Expected: "synced lock directory", Actual: err.Error()}
	}
	return &lockHandle{path: path, metadata: metadata}, nil
}

func (h *lockHandle) release() error {
	if h == nil || h.released {
		return nil
	}
	h.released = true
	return removeLockIfIdentityMatches(h.path, h.metadata)
}

func removeLockIfIdentityMatches(path SafePath, metadata LockMetadata) error {
	current, err := readLockMetadata(path, metadata.LockName)
	if err != nil {
		return err
	}
	if !sameLockIdentity(current, metadata) {
		return &Problem{Code: "lock_identity_mismatch", Message: "lock identity changed before release", Hint: "Do not remove a lock owned by another helper process; run project doctor for diagnosis.", Path: path.Relative, Field: "lock", Expected: lockIdentity(metadata), Actual: lockIdentity(current)}
	}
	if err := os.Remove(path.Absolute); err != nil {
		return &Problem{Code: "lock_release_failed", Message: "cannot remove lock file", Hint: "Run project doctor and recover stale locks only after confirming the writer is gone.", Path: path.Relative, Field: "path", Expected: "removed own lock", Actual: err.Error()}
	}
	return syncDirectory(filepath.Dir(path.Absolute))
}

func inspectLockFile(root Root, name string, now time.Time) (lockInspection, error) {
	path, err := lockPath(root, name)
	if err != nil {
		return lockInspection{}, err
	}
	info, err := os.Lstat(path.Absolute)
	if err != nil {
		return lockInspection{}, &Problem{Code: "lock_not_found", Message: "lock file was not found", Hint: "Run project doctor to inspect current lock state.", Path: path.Relative, Field: "path", Expected: "present lock", Actual: err.Error()}
	}
	if info.IsDir() || info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return lockInspection{}, &Problem{Code: "lock_metadata_invalid", Message: "lock path is not a regular file", Hint: "Preserve the path for diagnosis; do not remove it silently.", Path: path.Relative, Field: "path", Expected: "regular lock file", Actual: info.Mode().String()}
	}
	metadata, err := readLockMetadata(path, name)
	if err != nil {
		return lockInspection{}, err
	}
	stale, reason := lockIsStale(metadata, now)
	return lockInspection{path: path, metadata: metadata, stale: stale, reason: reason}, nil
}

func readLockMetadata(path SafePath, expectedName string) (LockMetadata, error) {
	data, err := os.ReadFile(path.Absolute)
	if err != nil {
		return LockMetadata{}, &Problem{Code: "lock_metadata_invalid", Message: "lock file exists but is unreadable", Hint: "Preserve the lock for diagnosis and inspect permissions before removing it.", Path: path.Relative, Field: "path", Expected: "readable lock metadata", Actual: err.Error()}
	}
	var metadata LockMetadata
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&metadata); err != nil {
		return LockMetadata{}, &Problem{Code: "lock_metadata_invalid", Message: "lock metadata is not valid JSON", Hint: "Preserve the lock and run project doctor before recovery.", Path: path.Relative, Field: "json", Expected: "lock metadata JSON object", Actual: err.Error()}
	}
	if err := validateLockMetadata(metadata, expectedName, path.Relative); err != nil {
		return LockMetadata{}, err
	}
	return metadata, nil
}

func validateLockMetadata(metadata LockMetadata, expectedName string, relative string) error {
	if metadata.Version != LockVersion {
		return &Problem{Code: "lock_metadata_invalid", Message: "lock metadata version is invalid", Hint: "Preserve the lock and run project doctor before recovery.", Path: relative, Field: "version", Expected: LockVersion, Actual: metadata.Version}
	}
	if metadata.LockName != expectedName {
		return &Problem{Code: "lock_metadata_invalid", Message: "lock metadata name does not match path", Hint: "Preserve the lock and run project doctor before recovery.", Path: relative, Field: "lock_name", Expected: expectedName, Actual: metadata.LockName}
	}
	if metadata.OwnerPID <= 0 {
		return &Problem{Code: "lock_metadata_invalid", Message: "lock metadata owner_pid is invalid", Hint: "Preserve the lock and run project doctor before recovery.", Path: relative, Field: "owner_pid", Expected: "positive process id", Actual: fmt.Sprintf("%d", metadata.OwnerPID)}
	}
	for _, field := range []struct{ name, value string }{{"hostname", metadata.Hostname}, {"command", metadata.Command}, {"created_at", metadata.CreatedAt}} {
		if strings.TrimSpace(field.value) == "" {
			return &Problem{Code: "lock_metadata_invalid", Message: "lock metadata is missing a required field", Hint: "Preserve the lock and run project doctor before recovery.", Path: relative, Field: field.name, Expected: "non-empty value", Actual: "empty"}
		}
	}
	if _, err := time.Parse(time.RFC3339, metadata.CreatedAt); err != nil {
		return &Problem{Code: "lock_metadata_invalid", Message: "lock metadata created_at is invalid", Hint: "Preserve the lock and run project doctor before recovery.", Path: relative, Field: "created_at", Expected: "RFC3339 timestamp", Actual: metadata.CreatedAt}
	}
	if metadata.RunID != "" {
		if strings.ContainsAny(metadata.RunID, "\r\n\x00") {
			return &Problem{Code: "lock_metadata_invalid", Message: "lock metadata run_id is invalid", Hint: "Preserve the lock and run project doctor before recovery.", Path: relative, Field: "run_id", Expected: "printable run id", Actual: "contains control character"}
		}
	}
	return nil
}

func lockIsStale(metadata LockMetadata, now time.Time) (bool, string) {
	createdAt, err := time.Parse(time.RFC3339, metadata.CreatedAt)
	if err != nil {
		return false, "invalid created_at"
	}
	hostname, _ := os.Hostname()
	if metadata.Hostname == hostname && !pidAlive(metadata.OwnerPID) {
		return true, "same-host owner pid is not alive"
	}
	if now.UTC().Sub(createdAt.UTC()) > LockStaleAfter {
		return true, "lock age exceeds 30 minutes"
	}
	return false, ""
}

func pidAlive(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	return processSignalMeansAlive(process.Signal(syscall.Signal(0)))
}

func processSignalMeansAlive(err error) bool {
	return err == nil || errors.Is(err, syscall.EPERM)
}

func lockPath(root Root, name string) (SafePath, error) {
	switch name {
	case ActiveRunLockName:
		return ResolveRelativePath(root, activeRunLockPath)
	case ProjectWriteLockName:
		return ResolveRelativePath(root, projectWriteLockPath)
	default:
		return SafePath{}, &Problem{Code: "lock_target_invalid", Message: "lock target is invalid", Hint: "Use active-run, project-write, or all.", Field: "lock", Expected: "active-run, project-write, or all", Actual: name}
	}
}

func lockTargetNames(target string) ([]string, error) {
	switch target {
	case "active-run":
		return []string{ActiveRunLockName}, nil
	case "project-write":
		return []string{ProjectWriteLockName}, nil
	case "all":
		return []string{ActiveRunLockName, ProjectWriteLockName}, nil
	default:
		return nil, &Problem{Code: "lock_target_invalid", Message: "lock recovery target is invalid", Hint: "Use lock recover <active-run|project-write|all> --reason <text>.", Field: "lock", Expected: "active-run, project-write, or all", Actual: target}
	}
}

func containsLockName(names []string, name string) bool {
	for _, candidate := range names {
		if candidate == name {
			return true
		}
	}
	return false
}

func lockConflictProblem(inspection lockInspection) *Problem {
	return &Problem{Code: "lock_conflict", Message: "helper state is locked by another active writer", Hint: "Wait for the writer to finish, or run lock recover after project doctor reports the lock stale.", Path: inspection.path.Relative, Field: "lock", Expected: "absent or stale lock", Actual: lockIdentity(inspection.metadata)}
}

func lockStaleProblem(inspection lockInspection) *Problem {
	return &Problem{Code: "lock_stale_recovery_required", Message: "stale helper lock requires explicit recovery", Hint: "Run lock recover for the stale lock target with --reason before retrying the mutating command.", Path: inspection.path.Relative, Field: "lock", Expected: "absent lock", Actual: lockIdentity(inspection.metadata) + " stale: " + inspection.reason}
}

func sameLockIdentity(a, b LockMetadata) bool {
	return a.Version == b.Version && a.LockName == b.LockName && a.RunID == b.RunID && a.OwnerPID == b.OwnerPID && a.Hostname == b.Hostname && a.Command == b.Command && a.CreatedAt == b.CreatedAt
}

func lockIdentity(metadata LockMetadata) string {
	return fmt.Sprintf("%s pid=%d host=%s command=%s run_id=%s created_at=%s", metadata.LockName, metadata.OwnerPID, metadata.Hostname, metadata.Command, metadata.RunID, metadata.CreatedAt)
}

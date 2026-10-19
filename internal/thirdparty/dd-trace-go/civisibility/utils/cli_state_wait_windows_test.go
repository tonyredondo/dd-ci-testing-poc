// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.

package utils

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A state file that another process holds without sharing it, as during the
// CLI's rename, is waited for instead of reported as unavailable.
func TestWaitCLIGitStateFileWaitsForASharedFile(t *testing.T) {
	dir := t.TempDir()
	if err := WriteCLIGitStateFile(dir, CLIGitUploadFile, CLIGitUpload{Bytes: 7}); err != nil {
		t.Fatal(err)
	}
	path, err := syscall.UTF16PtrFromString(filepath.Join(dir, CLIGitUploadFile))
	if err != nil {
		t.Fatal(err)
	}
	// Share mode 0: no other open succeeds while the handle is open.
	handle, err := syscall.CreateFile(path, syscall.GENERIC_READ, 0, nil, syscall.OPEN_EXISTING, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		t.Fatal(err)
	}
	var upload CLIGitUpload
	if err := ReadCLIGitStateFile(dir, CLIGitUploadFile, &upload); !transientStateFileError(err) {
		_ = syscall.CloseHandle(handle)
		t.Fatalf("a held file read as %v", err)
	}
	released := time.AfterFunc(200*time.Millisecond, func() { _ = syscall.CloseHandle(handle) })
	defer released.Stop()
	if err := WaitCLIGitStateFile(dir, CLIGitUploadFile, &upload, 10*time.Second); err != nil || upload.Bytes != 7 {
		t.Fatalf("upload %+v, %v", upload, err)
	}
}

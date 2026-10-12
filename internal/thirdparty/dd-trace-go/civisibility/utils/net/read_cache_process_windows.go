// Copyright 2026 Datadog, Inc. Licensed under the Apache License, Version 2.0.

package net

import (
	"errors"
	"syscall"
)

const (
	// processQueryLimitedInformation is PROCESS_QUERY_LIMITED_INFORMATION.
	processQueryLimitedInformation = 0x1000
	// stillActive is the exit code of a process that runs, STILL_ACTIVE.
	stillActive = 259
)

// processAlive reports whether the process pid runs, and whether that is
// known: a process that cannot be opened for lack of access exists.
func processAlive(pid int) (alive bool, known bool) {
	if pid <= 0 {
		return false, false
	}
	handle, err := syscall.OpenProcess(processQueryLimitedInformation, false, uint32(pid))
	switch {
	case errors.Is(err, syscall.ERROR_ACCESS_DENIED):
		return true, true
	case errors.Is(err, syscall.Errno(87)): // ERROR_INVALID_PARAMETER: no such process
		return false, true
	case err != nil:
		return false, false
	}
	defer syscall.CloseHandle(handle)
	var code uint32
	if err := syscall.GetExitCodeProcess(handle, &code); err != nil {
		return false, false
	}
	return code == stillActive, true
}

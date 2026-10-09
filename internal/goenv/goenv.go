// Package goenv restores Go command settings that ddtest replaces only for its
// own go test command, such as a temporary GOWORK.
//
// Go initializes ready packages in import-path order. This package imports
// only syscall and its path sorts before "os", so it initializes before os and
// therefore before any package that can start a go command, including client
// dependencies that do not import testing.
package goenv

import "syscall"

// SavedPrefix names the variables that carry a caller's original settings.
const SavedPrefix = "DDTEST_ORIGINAL_"

// Settings are the variables that ddtest may replace for go test.
var Settings = [...]string{"GOWORK", "GOFLAGS"}

func init() { Restore() }

// Save returns the environment entry that records name's current value:
// "=value" when it is set, or an empty value when it is unset.
func Save(name string) string {
	if value, ok := syscall.Getenv(name); ok {
		return SavedPrefix + name + "==" + value
	}
	return SavedPrefix + name + "="
}

// Restore applies and removes every saved setting. Without saved values, as in
// a binary started outside ddtest, it changes nothing.
func Restore() {
	for _, name := range Settings {
		key := SavedPrefix + name
		saved, ok := syscall.Getenv(key)
		if !ok {
			continue
		}
		_ = syscall.Unsetenv(key)
		if saved != "" && saved[0] == '=' {
			_ = syscall.Setenv(name, saved[1:])
		} else {
			_ = syscall.Unsetenv(name)
		}
	}
}

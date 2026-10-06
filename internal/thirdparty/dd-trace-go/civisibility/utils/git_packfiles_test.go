package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Pack files must not outlive their upload, and their temporary directory must
// never be created in the working tree: a temporary directory on another device
// falls back to the git directory, which shares a device with the objects.

// useTempDir makes os.TempDir return dir on every platform.
func useTempDir(t *testing.T, dir string) {
	t.Helper()
	for _, name := range []string{"TMPDIR", "TMP", "TEMP"} {
		t.Setenv(name, dir)
	}
}

// packDirectories lists the pack-file temporary directories directly in dir.
func packDirectories(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	var found []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".dd-pack-objects") {
			found = append(found, entry.Name())
		}
	}
	return found
}

func sameDirectory(t *testing.T, a, b string) bool {
	t.Helper()
	a, err := filepath.EvalSymlinks(a)
	require.NoError(t, err)
	b, err = filepath.EvalSymlinks(b)
	require.NoError(t, err)
	return a == b
}

func TestRemovePackFilesRemovesTemporaryDirectory(t *testing.T) {
	useLocalGitFixture(t)
	repo, err := os.Getwd()
	require.NoError(t, err)
	tmp := t.TempDir()
	useTempDir(t, tmp)

	files := CreatePackFiles(GetLastLocalGitCommitShas(), nil)
	require.NotEmpty(t, files)
	for _, file := range files {
		require.FileExists(t, file)
	}
	assert.True(t, sameDirectory(t, tmp, filepath.Dir(filepath.Dir(files[0]))))
	assert.Empty(t, packDirectories(t, repo), "working tree received a pack directory")

	// git also writes index files next to each pack; nothing may remain.
	RemovePackFiles(files)
	assert.Empty(t, packDirectories(t, tmp))
}

func TestPackFilesFallBackToGitDirectory(t *testing.T) {
	useLocalGitFixture(t)
	repo, err := os.Getwd()
	require.NoError(t, err)
	// The temporary directory cannot be created, as when git cannot move its
	// pack there from another device.
	useTempDir(t, filepath.Join(t.TempDir(), "missing"))

	files := CreatePackFiles(GetLastLocalGitCommitShas(), nil)
	require.NotEmpty(t, files)
	gitDir := filepath.Join(repo, ".git")
	assert.True(t, sameDirectory(t, gitDir, filepath.Dir(filepath.Dir(files[0]))))
	assert.Empty(t, packDirectories(t, repo), "working tree received a pack directory")

	RemovePackFiles(files)
	assert.Empty(t, packDirectories(t, gitDir))
}

func TestFailedPackFilesLeaveNoDirectory(t *testing.T) {
	resetGitCommandCachesForTesting(t)
	tmp, gitDir, bin := t.TempDir(), t.TempDir(), t.TempDir()
	useTempDir(t, tmp)
	t.Setenv("FAKE_GIT_COMMON_DIR", gitDir)
	for _, fake := range []struct{ name, script string }{
		{"git", `#!/bin/sh
if [ "$1" = "-c" ]; then
	shift 2
fi
case "$1" in
	rev-list)
		printf 'abc123\n'
		;;
	rev-parse)
		printf '%s\n' "$FAKE_GIT_COMMON_DIR"
		;;
	pack-objects)
		exit 1
		;;
esac
exit 0
`},
		{"git.bat", `@echo off
if "%1"=="-c" (
	shift
	shift
)
if "%1"=="rev-list" (
	echo abc123
	exit /b 0
)
if "%1"=="rev-parse" (
	echo %FAKE_GIT_COMMON_DIR%
	exit /b 0
)
if "%1"=="pack-objects" (
	exit /b 1
)
exit /b 0
`},
	} {
		require.NoError(t, os.WriteFile(filepath.Join(bin, fake.name), []byte(fake.script), 0o755))
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))

	// Both attempts fail: each removes its directory.
	assert.Empty(t, CreatePackFiles([]string{"HEAD"}, nil))
	assert.Empty(t, packDirectories(t, tmp))
	assert.Empty(t, packDirectories(t, gitDir))
}

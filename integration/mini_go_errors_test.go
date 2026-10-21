package integration

import (
	"testing"
)

// When go rejects the command or its environment while ddto prepares it, ddto
// runs go test natively: the output and exit status are go test's.
func TestMiniKeepsGoErrorStatus(t *testing.T) {
	dir, driver := prepareMiniFixture(t)
	for _, tc := range []struct {
		env  []string
		args []string
	}{
		{args: []string{"-p=0", "."}},
		{env: []string{"GOFLAGS=-badflag"}, args: []string{"."}},
		{env: []string{"GOFLAGS=-p=0"}, args: []string{"."}},
		{args: []string{"-C", "nonexistent", "."}},
		{args: []string{"-C", "go.mod", "."}},
		{args: []string{"-count=1", "-C", "other", "."}},
		{args: []string{"-modfile=missing.mod", "."}},
		{args: []string{"-modfile=.", "."}},
		{args: []string{"-cover", "-covermode=bad", "."}},
	} {
		env := testEnv(tc.env...)
		args := append([]string{"test"}, tc.args...)
		want, wantErr, wantCode := command(t, dir, env, "go", args...)
		got, gotErr, gotCode := command(t, dir, env, driver, args...)
		if wantCode == 0 || gotCode != wantCode || got != want || gotErr != wantErr {
			t.Fatalf("%v %v: ddto exit %d, go test %d\nddto stdout:\n%s\nddto stderr:\n%s\ngo test stdout:\n%s\ngo test stderr:\n%s", tc.env, tc.args, gotCode, wantCode, got, gotErr, want, wantErr)
		}
	}
}

package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/instrument"
)

func (p Plan) toolMode() string {
	if p.testify && p.coverOverlay {
		return "testify-cover"
	}
	if p.testify {
		return "testify"
	}
	if p.coverOverlay {
		return "cover"
	}
	return ""
}

// ToolNeedsPlan is the allocation-free dispatch before any plan I/O. Go probes
// compile/link versions without a package identity; those always stay native.
func ToolNeedsPlan(mode string, args []string, importPath string) bool {
	if len(args) == 0 {
		return false
	}
	tool := filepath.Base(args[0])
	tool = strings.TrimSuffix(tool, ".exe")
	if tool == "cover" {
		if mode != "cover" && mode != "testify-cover" {
			return false
		}
		if len(args) == 2 && args[1] == "-V=full" {
			return true
		}
		pkg, _, _ := strings.Cut(importPath, " [")
		return pkg == "testing"
	}
	if tool != "compile" || mode != "testify" && mode != "testify-cover" {
		return false
	}
	if len(args) == 2 && args[1] == "-V=full" {
		return false
	}
	pkg, _, _ := strings.Cut(importPath, " [")
	return pkg == instrument.TestifySuiteImport
}

// RunTool handles only a selected transform. CLI bypasses unrelated tools
// before entering here, with native process replacement on Unix.
func RunTool(ctx context.Context, overlay string, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		fmt.Fprintln(stderr, "ddtest: missing tool executable")
		return 2
	}
	if tool := strings.TrimSuffix(filepath.Base(args[0]), ".exe"); tool == "cover" {
		return RunCoverTool(ctx, overlay, args, stdin, stdout, stderr)
	}
	data, err := os.ReadFile(overlay)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	var plan Overlay
	if err := json.Unmarshal(data, &plan); err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	forwarded, cleanup, err := prepareTestifyCompile(plan.Testify, args)
	if err != nil {
		fmt.Fprintln(stderr, err)
		return 2
	}
	defer cleanup()
	cmd := exec.CommandContext(ctx, forwarded[0], forwarded[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		if exit, ok := err.(*exec.ExitError); ok {
			return exit.ExitCode()
		}
		fmt.Fprintln(stderr, err)
		return 2
	}
	return 0
}

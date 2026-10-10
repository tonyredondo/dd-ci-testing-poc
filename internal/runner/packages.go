package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"
)

// packageGraph indexes a go list -deps result, which lists each package once.
type packageGraph map[string]*goPackage

func newPackageGraph(packages []goPackage) packageGraph {
	graph := make(packageGraph, len(packages))
	for i := range packages {
		graph[packages[i].ImportPath] = &packages[i]
	}
	return graph
}

// imported returns every listed package that roots import, directly or
// indirectly: the union of their Deps. A root is included only when another
// root imports it.
func (g packageGraph) imported(roots ...*goPackage) map[string]bool {
	reached := map[string]bool{}
	var pending []string
	for _, root := range roots {
		pending = append(pending, root.Imports...)
	}
	for len(pending) != 0 {
		path := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		// Imports also name "C", which is not a package and is not listed.
		if p := g[path]; p != nil && !reached[path] {
			reached[path] = true
			pending = append(pending, p.Imports...)
		}
	}
	return reached
}

// readPackages decodes go list output as it arrives instead of retaining a
// second, complete JSON copy. It always drains stdout and waits for the command,
// including after a decoding failure. A command failure takes precedence over
// partial or malformed output, as it did with Cmd.Output.
func readPackages(ctx context.Context, cmd *exec.Cmd, failureContext string) ([]goPackage, error) {
	var stderr strings.Builder
	cmd.Stderr = &stderr
	// A canceled go list is killed; do not wait indefinitely for descendants
	// that might still hold its output pipes.
	cmd.WaitDelay = 5 * time.Second
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", failureContext, err, stderr.String())
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", failureContext, err, stderr.String())
	}
	// StdoutPipe is read by this goroutine, outside exec's copying goroutines.
	// WaitDelay cannot bound a read that prevents us from reaching Wait.
	closed := make(chan struct{})
	stop := context.AfterFunc(ctx, func() {
		_ = stdout.Close()
		close(closed)
	})
	defer func() {
		if !stop() {
			<-closed
		}
	}()
	var packages []goPackage
	decoder := json.NewDecoder(stdout)
	var decodeErr error
	for {
		var p goPackage
		if err := decoder.Decode(&p); err != nil {
			if err != io.EOF {
				decodeErr = err
			}
			break
		}
		packages = append(packages, p)
	}
	if decodeErr != nil {
		// A producer can keep writing after a malformed record. Draining avoids
		// blocking it on a full pipe while we wait for its exit status.
		_, _ = io.Copy(io.Discard, stdout)
	}
	if err := cmd.Wait(); err != nil {
		return nil, fmt.Errorf("%s: %w\n%s", failureContext, err, stderr.String())
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", failureContext, err)
	}
	if decodeErr != nil {
		return nil, decodeErr
	}
	return packages, nil
}

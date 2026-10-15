package runner

import (
	"os"
	"path/filepath"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/instrument"
)

const miniIntegrationsPackage = miniModule + "/internal/thirdparty/dd-trace-go/civisibility/integrations"

// signalHookFile is the overlay file that ddto adds to os/signal.
const signalHookFile = "zz_dd_ci_visibility_signal.go"

// prepareSignalHook edits os/signal so that Mini receives SIGINT and SIGTERM
// only when no other channel wants them, and adds instrument.SignalHook. It
// returns the edited os/signal package, or why the build keeps os/signal
// unchanged; Mini then handles no signal, as under go test.
func prepareSignalHook(listed []goPackage, replacements map[string]string, temp string, add func(logical, content string) error) (*goPackage, string, error) {
	var signal, integrations *goPackage
	for i := range listed {
		switch p := &listed[i]; {
		case p.Standard && p.ImportPath == "os/signal":
			signal = p
		case p.ImportPath == miniIntegrationsPackage:
			integrations = p
		}
	}
	if signal == nil || integrations == nil || signal.Error != nil || integrations.Error != nil {
		return nil, "os/signal or the runtime's integrations package not listed", nil
	}
	// A client that requires another version of this module selects its runtime.
	runtimeSources, err := packageSources(integrations, replacements)
	if err != nil {
		return nil, err.Error(), nil
	}
	if err := instrument.SignalHookRuntimeError(runtimeSources); err != nil {
		return nil, err.Error(), nil
	}
	sources, err := packageSources(signal, replacements)
	if err != nil {
		return nil, err.Error(), nil
	}
	edited, err := instrument.TransformSignal(sources)
	if err != nil {
		return nil, err.Error(), nil
	}
	for logical, src := range edited {
		backing := filepath.Join(temp, "signal-"+filepath.Base(logical))
		if err := os.WriteFile(backing, src, 0600); err != nil {
			return nil, "", err
		}
		replacements[logical] = backing
	}
	if err := add(filepath.Join(signal.Dir, signalHookFile), instrument.SignalHook); err != nil {
		return nil, "", err
	}
	return signal, "", nil
}

// packageSources reads the Go files of p that the build compiles, through the
// overlay's replacements. A file that the overlay deletes is left out.
func packageSources(p *goPackage, replacements map[string]string) (map[string][]byte, error) {
	sources := make(map[string][]byte, len(p.GoFiles))
	for _, name := range p.GoFiles {
		path := filepath.Join(p.Dir, name)
		actual, replaced := replacements[path]
		if !replaced {
			actual = path
		} else if actual == "" {
			continue
		}
		src, err := os.ReadFile(actual)
		if err != nil {
			return nil, err
		}
		sources[path] = src
	}
	return sources, nil
}

package runner

import (
	"os"
	"strings"
	"sync"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/goenv"
	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/dd-trace-go/civisibility/utils/githubjob"
)

// gitHubDiagnosticsDirs are the runner directories that hold Worker logs.
var gitHubDiagnosticsDirs = githubjob.DiagnosticsDirs

// handGitHubJobID starts reading the GitHub Actions job ID for Mini's test
// processes, each of which would otherwise scan the runner's diagnostics
// files. The returned function waits for the environment entry to add, if any.
// A numeric JOB_CHECK_RUN_ID takes precedence in every process, so it needs no
// scan; the runtime reads GitHub metadata only when GITHUB_SHA is set.
func handGitHubJobID(runtime Runtime, debug *cliDebug) func() []string {
	if _, github := os.LookupEnv("GITHUB_SHA"); runtime != Mini || !github || githubjob.Numeric(strings.TrimSpace(os.Getenv(githubjob.JobIDEnv))) {
		return func() []string { return nil }
	}
	found := make(chan string, 1)
	go func() {
		id, _ := githubjob.FromDiagnostics(gitHubDiagnosticsDirs(), debug.printf)
		found <- id
	}()
	return sync.OnceValue(func() []string {
		return []string{goenv.GitHubJobIDHandoff + "=" + <-found}
	})
}

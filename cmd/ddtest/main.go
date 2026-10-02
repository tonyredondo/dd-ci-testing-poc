package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/runner"
)

func main() {
	if len(os.Args) < 2 || os.Args[1] != "test" {
		fmt.Fprintln(os.Stderr, "usage: ddtest test [--runtime=sdk|mini] [go test flags] [packages]")
		os.Exit(2)
	}
	if _, defined := os.LookupEnv("DD_CIVISIBILITY_ENABLED"); !defined {
		if err := os.Setenv("DD_CIVISIBILITY_ENABLED", "parent"); err != nil {
			fmt.Fprintln(os.Stderr, "ddtest:", err)
			os.Exit(2)
		}
	}
	args := os.Args[2:]
	runtime := runner.SDK
	if len(args) > 0 && strings.HasPrefix(args[0], "--runtime=") {
		runtime = runner.Runtime(strings.TrimPrefix(args[0], "--runtime="))
		args = args[1:]
		if runtime != runner.SDK && runtime != runner.Mini {
			fmt.Fprintln(os.Stderr, "ddtest: runtime must be sdk or mini")
			os.Exit(2)
		}
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	os.Exit(runner.RunRuntime(ctx, args, runtime, os.Stdin, os.Stdout, os.Stderr))
}

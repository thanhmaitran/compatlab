package commands

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/thanhmaitran/compatlab/internal/execution"
	"github.com/thanhmaitran/compatlab/internal/mocking"
	"github.com/thanhmaitran/compatlab/internal/security"
	"github.com/thanhmaitran/compatlab/internal/workflows"
)

func runMock(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("mock", flag.ContinueOnError)
	flags.SetOutput(stderr)
	spec := flags.String("spec", "", "OpenAPI file to mock")
	addr := flags.String("addr", "127.0.0.1:8081", "listen address (loopback only)")
	if err := flags.Parse(args); err != nil {
		return flagExit(err)
	}
	if *spec == "" {
		return fail(stderr, usageErr("--spec is required"))
	}
	host, _, err := net.SplitHostPort(*addr)
	if err != nil {
		return fail(stderr, usageErr("invalid --addr %q: %v", *addr, err))
	}
	if ip := net.ParseIP(host); host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return fail(stderr, usageErr("refusing to listen on non-loopback address %q", *addr))
	}
	api, err := loadAPI(*spec)
	if err != nil {
		return fail(stderr, err)
	}
	srv := &http.Server{Addr: *addr, Handler: mocking.NewHandler(api, nil), ReadHeaderTimeout: 5 * time.Second}
	fmt.Fprintf(stdout, "Mock server for %s listening on http://%s (Ctrl+C to stop)\n", api.Title, *addr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fail(stderr, err)
	}
	return ExitOK
}

func runWorkflow(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "run" {
		return fail(stderr, usageErr("usage: compatlab workflow run --file <workflow.json> --target <url> --allow-host <host>"))
	}
	flags := flag.NewFlagSet("workflow run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	file := flags.String("file", "", "workflow JSON file")
	target := flags.String("target", "", "base URL of the API under test")
	var allow stringList
	flags.Var(&allow, "allow-host", "host (or host:port) the runner may contact (repeatable)")
	if err := flags.Parse(args[1:]); err != nil {
		return flagExit(err)
	}
	if *file == "" || *target == "" {
		return fail(stderr, usageErr("--file and --target are required"))
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		return fail(stderr, fmt.Errorf("%s: %w", *file, err))
	}
	wf, err := workflows.ParseFile(data)
	if err != nil {
		return fail(stderr, fmt.Errorf("%s: %w", *file, err))
	}
	runner := workflows.Runner{Config: execution.Config{Policy: security.TargetPolicy{AllowedHosts: allow}}}
	res, err := runner.Run(context.Background(), wf, *target)
	if err != nil {
		return fail(stderr, err)
	}
	verdict := "PASSED"
	if !res.Passed {
		verdict = "FAILED"
	}
	fmt.Fprintf(stdout, "Workflow %q: %s\n", res.Name, verdict)
	for _, s := range append(append([]workflows.StepResult{}, res.Steps...), res.Cleanup...) {
		fmt.Fprintf(stdout, "  %-8s %s (attempts: %d) %s\n", s.Status, s.Name, s.Attempts, s.Message)
	}
	if !res.Passed {
		return ExitRejected
	}
	return ExitOK
}

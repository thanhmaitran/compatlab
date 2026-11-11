package commands

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/execution"
	"github.com/thanhmaitran/compatlab/internal/generation"
	"github.com/thanhmaitran/compatlab/internal/security"
)

func runTest(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || (args[0] != "generate" && args[0] != "run") {
		return fail(stderr, usageErr("usage: compatlab test generate|run --spec <api> [options]"))
	}
	if args[0] == "generate" {
		return runTestGenerate(args[1:], stdout, stderr)
	}
	return runTestRun(args[1:], stdout, stderr)
}

func generateSuite(api *domain.NormalizedAPI, limit int, seed int64, priority []string) (*domain.TestSuite, error) {
	return generation.Generator{}.Generate(api, domain.GenerationConfig{Seed: seed, MaxCasesPerEndpoint: limit, PriorityEndpoints: priority})
}

func runTestGenerate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("test generate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	spec := flags.String("spec", "", "OpenAPI file")
	limit := flags.Int("max-per-endpoint", 0, "maximum cases per endpoint (0 = default)")
	seed := flags.Int64("seed", 0, "seed recorded in the suite")
	output := flags.String("output", "", "write to a file instead of stdout")
	var priority stringList
	flags.Var(&priority, "priority", "endpoint key whose tests run first (repeatable)")
	if err := flags.Parse(args); err != nil {
		return flagExit(err)
	}
	if *spec == "" {
		return fail(stderr, usageErr("--spec is required"))
	}
	api, err := loadAPI(*spec)
	if err != nil {
		return fail(stderr, err)
	}
	suite, err := generateSuite(api, *limit, *seed, priority)
	if err != nil {
		return fail(stderr, err)
	}
	data, err := json.MarshalIndent(suite, "", "  ")
	if err != nil {
		return fail(stderr, err)
	}
	if err := emit(*output, append(data, '\n'), stdout); err != nil {
		return fail(stderr, err)
	}
	return ExitOK
}

func runTestRun(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("test run", flag.ContinueOnError)
	flags.SetOutput(stderr)
	spec := flags.String("spec", "", "OpenAPI file")
	suiteFile := flags.String("suite", "", "suite JSON produced by `test generate` (default: generate from --spec)")
	target := flags.String("target", "", "base URL to test; without it the suite runs against an in-process mock")
	maxReq := flags.Int("max-requests", 0, "maximum requests per run (0 = default 1000)")
	timeoutMS := flags.Int("timeout-ms", 0, "per-request timeout in milliseconds (0 = default 10000)")
	limit := flags.Int("max-per-endpoint", 0, "maximum generated cases per endpoint (0 = default)")
	seed := flags.Int64("seed", 0, "seed recorded in a generated suite")
	output := flags.String("output", "", "write the run as JSON to a file")
	var allow stringList
	flags.Var(&allow, "allow-host", "host (or host:port) the runner may contact (repeatable)")
	if err := flags.Parse(args); err != nil {
		return flagExit(err)
	}
	if *spec == "" {
		return fail(stderr, usageErr("--spec is required"))
	}
	api, err := loadAPI(*spec)
	if err != nil {
		return fail(stderr, err)
	}
	var suite *domain.TestSuite
	if *suiteFile != "" {
		data, rerr := os.ReadFile(*suiteFile)
		if rerr != nil {
			return fail(stderr, fmt.Errorf("%s: %w", *suiteFile, rerr))
		}
		suite = &domain.TestSuite{}
		if jerr := json.Unmarshal(data, suite); jerr != nil {
			return fail(stderr, fmt.Errorf("%w: invalid suite %s: %v", domain.ErrInvalid, *suiteFile, jerr))
		}
	} else if suite, err = generateSuite(api, *limit, *seed, nil); err != nil {
		return fail(stderr, err)
	}

	var run *domain.TestRun
	if *target == "" {
		run, err = execution.RunOnMock(context.Background(), api, suite)
	} else {
		runner := execution.Runner{Config: execution.Config{
			Policy:      security.TargetPolicy{AllowedHosts: allow},
			MaxRequests: *maxReq,
			Timeout:     time.Duration(*timeoutMS) * time.Millisecond,
		}}
		run, err = runner.Run(context.Background(), api, suite, *target)
	}
	if err != nil {
		return fail(stderr, err)
	}

	names := map[string]string{}
	for _, c := range suite.Cases {
		names[c.ID] = c.Name
	}
	counts := map[domain.TestStatus]int{}
	for _, r := range run.Results {
		counts[r.Status]++
	}
	fmt.Fprintf(stdout, "Test run %s: %d passed, %d failed, %d errored (pass rate %.1f%%)\n",
		run.ID, counts[domain.TestStatusPassed], counts[domain.TestStatusFailed], counts[domain.TestStatusErrored], run.PassRate())
	for _, r := range run.Results {
		if r.Status == domain.TestStatusFailed || r.Status == domain.TestStatusErrored {
			fmt.Fprintf(stdout, "  %s  %s: %s\n", r.Status, names[r.CaseID], r.Message)
		}
	}
	if *output != "" {
		data, merr := json.MarshalIndent(run, "", "  ")
		if merr != nil {
			return fail(stderr, merr)
		}
		if werr := os.WriteFile(*output, append(data, '\n'), 0o644); werr != nil {
			return fail(stderr, werr)
		}
	}
	if counts[domain.TestStatusFailed]+counts[domain.TestStatusErrored] > 0 {
		return ExitRejected
	}
	return ExitOK
}

// Package commands implements the compatlab CLI. Run returns the process exit
// code so the CLI can be tested without spawning processes.
//
// Exit codes: 0 success / gate passed, 1 analysis or execution error,
// 2 invalid input or configuration, 3 policy rejected the release (or a
// contract failed verification).
package commands

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"sort"
	"strings"

	"github.com/thanhmaitran/compatlab/internal/compatibility"
	"github.com/thanhmaitran/compatlab/internal/compatibility/analyzer"
	"github.com/thanhmaitran/compatlab/internal/contracts"
	"github.com/thanhmaitran/compatlab/internal/domain"
	"github.com/thanhmaitran/compatlab/internal/policies"
	"github.com/thanhmaitran/compatlab/internal/reporting"
	"github.com/thanhmaitran/compatlab/internal/security"
	"github.com/thanhmaitran/compatlab/internal/specification"
)

// Version is overridden at build time with -ldflags "-X .../commands.Version=...".
var Version = "0.1.0-dev"

// Process exit codes.
const (
	ExitOK       = 0
	ExitError    = 1
	ExitInvalid  = 2
	ExitRejected = 3
)

var errUsage = errors.New("usage error")

type stringList []string

func (s *stringList) String() string     { return strings.Join(*s, ",") }
func (s *stringList) Set(v string) error { *s = append(*s, v); return nil }

// Run executes the CLI with the given arguments (without the program name).
func Run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return ExitInvalid
	}
	switch args[0] {
	case "help", "-h", "--help":
		usage(stdout)
		return ExitOK
	case "version":
		fmt.Fprintf(stdout, "compatlab %s\n", Version)
		return ExitOK
	case "spec":
		return runSpec(args[1:], stdout, stderr)
	case "diff", "analyze":
		return runAnalyze(args[0], args[1:], stdout, stderr)
	case "contract":
		return runContract(args[1:], stdout, stderr)
	case "gate":
		return runGate(args[1:], stdout, stderr)
	case "test":
		return runTest(args[1:], stdout, stderr)
	case "mock":
		return runMock(args[1:], stdout, stderr)
	case "workflow":
		return runWorkflow(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "unknown command %q\n\n", args[0])
	usage(stderr)
	return ExitInvalid
}

func usage(w io.Writer) {
	fmt.Fprint(w, `compatlab - API compatibility analysis

Usage:
  compatlab spec validate <file>
  compatlab spec inspect <file>
  compatlab diff    --base <old.yaml> --target <new.yaml> [--format text|json] [--output file]
  compatlab analyze --base <old.yaml> --target <new.yaml> [--format text|json|html] [--output file]
  compatlab contract verify --spec <api.yaml> --contract <file.json> [--contract ...]
  compatlab gate --base <old.yaml> --target <new.yaml> --policy <policy.yaml>
                 [--contract <file.json> ...] [--format text|json|html] [--output file]
  compatlab test generate --spec <api.yaml> [--max-per-endpoint N] [--seed N] [--priority "GET /path/{}"] [--output file]
  compatlab test run --spec <api.yaml> [--suite file] [--target url --allow-host host] [--output file]
  compatlab mock --spec <api.yaml> [--addr 127.0.0.1:8081]
  compatlab workflow run --file <workflow.json> --target <url> --allow-host <host>
  compatlab version

Exit codes: 0 ok, 1 error, 2 invalid input, 3 release rejected.
`)
}

func exitFor(err error) int {
	switch {
	case errors.Is(err, errUsage),
		errors.Is(err, domain.ErrInvalid),
		errors.Is(err, specification.ErrParse),
		errors.Is(err, specification.ErrValidation),
		errors.Is(err, specification.ErrUnsupported),
		errors.Is(err, specification.ErrTooLarge),
		errors.Is(err, specification.ErrReference),
		errors.Is(err, security.ErrTargetNotAllowed),
		errors.Is(err, fs.ErrNotExist):
		return ExitInvalid
	}
	return ExitError
}

func fail(stderr io.Writer, err error) int {
	fmt.Fprintf(stderr, "error: %v\n", err)
	return exitFor(err)
}

func flagExit(err error) int {
	if errors.Is(err, flag.ErrHelp) {
		return ExitOK
	}
	return ExitInvalid
}

func usageErr(format string, a ...any) error {
	return fmt.Errorf("%w: %s", errUsage, fmt.Sprintf(format, a...))
}

func loadAPI(path string) (*domain.NormalizedAPI, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	api, err := specification.NewParser().Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return api, nil
}

func loadContract(path string) (*domain.ConsumerContract, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	c, err := contracts.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return c, nil
}

func render(format string, r *domain.CompatibilityReport) ([]byte, error) {
	switch format {
	case "text":
		return []byte(reporting.Text(r)), nil
	case "json":
		return reporting.JSON(r)
	case "html":
		return reporting.HTML(r)
	}
	return nil, usageErr("unknown format %q (use text, json or html)", format)
}

func emit(output string, data []byte, stdout io.Writer) error {
	if output == "" {
		_, err := stdout.Write(data)
		return err
	}
	return os.WriteFile(output, data, 0o644)
}

// ---------- spec ----------

func runSpec(args []string, stdout, stderr io.Writer) int {
	if len(args) != 2 || (args[0] != "validate" && args[0] != "inspect") {
		return fail(stderr, usageErr("usage: compatlab spec validate|inspect <file>"))
	}
	api, err := loadAPI(args[1])
	if err != nil {
		return fail(stderr, err)
	}
	if args[0] == "validate" {
		fmt.Fprintf(stdout, "OK: %s %s (OpenAPI %s) - %d endpoints, %d schemas\n",
			api.Title, api.Version, api.OpenAPIVersion, len(api.Endpoints), len(api.Schemas))
		return ExitOK
	}
	fmt.Fprintf(stdout, "%s %s (OpenAPI %s)\n\nEndpoints\n", api.Title, api.Version, api.OpenAPIVersion)
	for _, e := range api.Endpoints {
		fmt.Fprintf(stdout, "  %-7s %s  (%d parameters, %d responses)\n", e.Method, e.Path, len(e.Parameters), len(e.Responses))
	}
	names := make([]string, 0, len(api.Schemas))
	for n := range api.Schemas {
		names = append(names, n)
	}
	sort.Strings(names)
	fmt.Fprintf(stdout, "\nSchemas\n")
	for _, n := range names {
		fmt.Fprintf(stdout, "  %s (%d properties)\n", n, len(api.Schemas[n].Properties))
	}
	return ExitOK
}

// ---------- diff / analyze ----------

func runAnalyze(name string, args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet(name, flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("base", "", "base (old) OpenAPI file")
	target := flags.String("target", "", "target (new) OpenAPI file")
	format := flags.String("format", "text", "output format: text, json or html (diff: text or json)")
	output := flags.String("output", "", "write to a file instead of stdout")
	if err := flags.Parse(args); err != nil {
		return flagExit(err)
	}
	if *base == "" || *target == "" {
		return fail(stderr, usageErr("--base and --target are required"))
	}
	oldAPI, err := loadAPI(*base)
	if err != nil {
		return fail(stderr, err)
	}
	newAPI, err := loadAPI(*target)
	if err != nil {
		return fail(stderr, err)
	}

	var data []byte
	if name == "diff" {
		oldAPI.Normalize()
		newAPI.Normalize()
		changes := analyzer.Diff(oldAPI, newAPI)
		switch *format {
		case "json":
			data, err = marshalChanges(changes)
		case "text":
			var sb strings.Builder
			for _, c := range changes {
				fmt.Fprintf(&sb, "%-20s %-7s %-28s %s\n", c.ChangeType, c.Method, c.Path, c.Description)
			}
			if len(changes) == 0 {
				sb.WriteString("No differences found.\n")
			}
			data = []byte(sb.String())
		default:
			err = usageErr("diff supports --format text or json")
		}
	} else {
		analysis, cerr := compatibility.NewEngine().Compare(oldAPI, newAPI)
		if cerr != nil {
			return fail(stderr, cerr)
		}
		data, err = render(*format, &domain.CompatibilityReport{Analysis: analysis, EngineVersion: Version})
	}
	if err != nil {
		return fail(stderr, err)
	}
	if err := emit(*output, data, stdout); err != nil {
		return fail(stderr, err)
	}
	return ExitOK
}

// ---------- contract ----------

func runContract(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] != "verify" {
		return fail(stderr, usageErr("usage: compatlab contract verify --spec <api> --contract <file> [--contract ...]"))
	}
	flags := flag.NewFlagSet("contract verify", flag.ContinueOnError)
	flags.SetOutput(stderr)
	spec := flags.String("spec", "", "provider OpenAPI file")
	var files stringList
	flags.Var(&files, "contract", "consumer contract file (repeatable)")
	if err := flags.Parse(args[1:]); err != nil {
		return flagExit(err)
	}
	if *spec == "" || len(files) == 0 {
		return fail(stderr, usageErr("--spec and at least one --contract are required"))
	}
	api, err := loadAPI(*spec)
	if err != nil {
		return fail(stderr, err)
	}
	failed := 0
	for _, path := range files {
		c, err := loadContract(path)
		if err != nil {
			return fail(stderr, err)
		}
		res, err := contracts.NewVerifier().Verify(c, api)
		if err != nil {
			return fail(stderr, err)
		}
		passed := 0
		for _, in := range res.Interactions {
			if in.Passed {
				passed++
			}
		}
		fmt.Fprintf(stdout, "%s -> %s: %d / %d interactions passed\n", res.Consumer, res.Provider, passed, len(res.Interactions))
		for _, in := range res.Interactions {
			if !in.Passed {
				fmt.Fprintf(stdout, "  FAIL %s: %s\n", in.Description, in.Message)
			}
		}
		if !res.Passed() {
			failed++
		}
	}
	if failed > 0 {
		return ExitRejected
	}
	return ExitOK
}

// ---------- gate ----------

func runGate(args []string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("gate", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("base", "", "base (old) OpenAPI file")
	target := flags.String("target", "", "target (new) OpenAPI file")
	policyPath := flags.String("policy", "", "release policy YAML file")
	format := flags.String("format", "text", "output format: text, json or html")
	output := flags.String("output", "", "write to a file instead of stdout")
	var contractFiles stringList
	flags.Var(&contractFiles, "contract", "consumer contract file (repeatable)")
	if err := flags.Parse(args); err != nil {
		return flagExit(err)
	}
	if *base == "" || *target == "" || *policyPath == "" {
		return fail(stderr, usageErr("--base, --target and --policy are required"))
	}
	policy, err := policies.LoadFile(*policyPath)
	if err != nil {
		return fail(stderr, err)
	}
	oldAPI, err := loadAPI(*base)
	if err != nil {
		return fail(stderr, err)
	}
	newAPI, err := loadAPI(*target)
	if err != nil {
		return fail(stderr, err)
	}
	analysis, err := compatibility.NewEngine().Compare(oldAPI, newAPI)
	if err != nil {
		return fail(stderr, err)
	}

	var loaded []*domain.ConsumerContract
	var verifications []domain.ContractVerification
	for _, path := range contractFiles {
		c, err := loadContract(path)
		if err != nil {
			return fail(stderr, err)
		}
		res, err := contracts.NewVerifier().Verify(c, newAPI)
		if err != nil {
			return fail(stderr, err)
		}
		loaded = append(loaded, c)
		verifications = append(verifications, *res)
	}

	report := &domain.CompatibilityReport{
		Analysis:      analysis,
		Contracts:     verifications,
		Impacts:       contracts.Impact(analysis, loaded),
		EngineVersion: Version,
	}
	eval, err := policies.Evaluator{}.Evaluate(policy, report)
	if err != nil {
		return fail(stderr, err)
	}
	report.Evaluation = eval

	data, err := render(*format, report)
	if err != nil {
		return fail(stderr, err)
	}
	if err := emit(*output, data, stdout); err != nil {
		return fail(stderr, err)
	}
	if eval.Decision == domain.DecisionBlock {
		return ExitRejected
	}
	return ExitOK
}

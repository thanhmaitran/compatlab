// Package generation derives deterministic API test cases from a normalized
// specification. Generation is bounded (MaxCasesPerEndpoint) to avoid
// combinatorial explosion and contains no randomness: the same specification
// and configuration always produce the same suite. The seed is recorded in the
// suite for future randomized strategies.
//
// Generated cases only describe what to send and what status to expect; they
// are executed later against an allowed target such as the mock server.
package generation

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// GeneratorVersion identifies the generation algorithm in produced suites.
const GeneratorVersion = "1.0.0"

// DefaultMaxCasesPerEndpoint applies when the configuration leaves the limit at 0.
const DefaultMaxCasesPerEndpoint = 50

// maxStringBoundary skips string-length boundary cases for huge limits so a
// hostile specification cannot make the generator allocate large strings.
const maxStringBoundary = 10000

// Generator implements domain.TestGenerator.
type Generator struct{}

var _ domain.TestGenerator = Generator{}

var basePriority = map[domain.TestCategory]int{
	domain.CategoryContract:            0,
	domain.CategoryAuthentication:      10,
	domain.CategoryStatus:              20,
	domain.CategorySchemaValidation:    30,
	domain.CategoryBoundary:            40,
	domain.CategoryParameterValidation: 45,
	domain.CategoryNegative:            50,
	domain.CategoryWorkflow:            60,
}

// Generate builds a prioritized, bounded test suite for the API.
func (Generator) Generate(api *domain.NormalizedAPI, cfg domain.GenerationConfig) (*domain.TestSuite, error) {
	if api == nil {
		return nil, fmt.Errorf("%w: API is required", domain.ErrInvalid)
	}
	if cfg.MaxCasesPerEndpoint < 0 {
		return nil, fmt.Errorf("%w: max_cases_per_endpoint must not be negative", domain.ErrInvalid)
	}
	limit := cfg.MaxCasesPerEndpoint
	if limit == 0 {
		limit = DefaultMaxCasesPerEndpoint
	}
	api.Normalize()
	prioritized := map[string]bool{}
	for _, k := range cfg.PriorityEndpoints {
		prioritized[k] = true
	}
	var all []domain.TestCase
	for i := range api.Endpoints {
		ep := &api.Endpoints[i]
		cases := endpointCases(ep)
		if prioritized[ep.Key()] {
			for j := range cases {
				cases[j].Priority -= 10
			}
		}
		sortCases(cases)
		if len(cases) > limit {
			cases = cases[:limit]
		}
		all = append(all, cases...)
	}
	sortCases(all)
	if all == nil {
		all = []domain.TestCase{}
	}
	return &domain.TestSuite{
		ID:               suiteID(api.SourceHash, cfg),
		Name:             api.Title + " generated tests",
		GeneratorVersion: GeneratorVersion,
		SpecHash:         api.SourceHash,
		Config:           cfg,
		Cases:            all,
	}, nil
}

func sortCases(cases []domain.TestCase) {
	slices.SortFunc(cases, func(a, b domain.TestCase) int {
		if c := cmp.Compare(a.Priority, b.Priority); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Path, b.Path); c != 0 {
			return c
		}
		if c := cmp.Compare(a.Method, b.Method); c != 0 {
			return c
		}
		return cmp.Compare(a.ID, b.ID)
	})
}

func hashHex(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

func suiteID(specHash string, cfg domain.GenerationConfig) string {
	prio := append([]string(nil), cfg.PriorityEndpoints...)
	sort.Strings(prio)
	return hashHex(GeneratorVersion, specHash, strconv.FormatInt(cfg.Seed, 10),
		strconv.Itoa(cfg.MaxCasesPerEndpoint), strings.Join(prio, ","))
}

type adder func(cat domain.TestCategory, target, variant, desc string, value any, status int)

func endpointCases(ep *domain.APIEndpoint) []domain.TestCase {
	var out []domain.TestCase
	add := adder(func(cat domain.TestCategory, target, variant, desc string, value any, status int) {
		out = append(out, domain.TestCase{
			ID:             hashHex(ep.Key(), ep.Path, string(cat), target, variant),
			Name:           fmt.Sprintf("%s %s: %s", ep.Method, ep.Path, desc),
			Category:       cat,
			Method:         ep.Method,
			Path:           ep.Path,
			Description:    desc,
			Target:         target,
			Variant:        variant,
			Value:          value,
			ExpectedStatus: status,
			Priority:       basePriority[cat],
		})
	})
	ok, bad := successStatus(ep), invalidStatus(ep)

	if len(ep.Security) > 0 {
		add(domain.CategoryAuthentication, "header.authorization", "missing", "request without credentials is rejected", nil, 401)
	}
	add(domain.CategoryStatus, "response", "success", "documented success status is returned", nil, ok)
	if schema := successSchema(ep); schema != nil {
		add(domain.CategorySchemaValidation, "response.body", "schema", "response body matches the documented schema", nil, ok)
	}
	for _, p := range ep.Parameters {
		target := string(p.Location) + "." + p.Name
		if p.Required && p.Location != domain.LocationPath {
			add(domain.CategoryParameterValidation, target, "missing", "required parameter "+p.Name+" is enforced", nil, bad)
		}
		valueCases(add, target, p.Schema, false, ok, bad)
	}
	if ep.RequestBody != nil && ep.RequestBody.Schema != nil {
		body := ep.RequestBody.Schema
		for _, name := range body.PropertyNames() {
			target := "body." + name
			if body.IsRequired(name) {
				add(domain.CategoryNegative, target, "missing", "required field "+name+" is enforced", nil, bad)
			}
			valueCases(add, target, body.Properties[name], true, ok, bad)
		}
	}
	return out
}

// valueCases adds boundary and negative cases for one input described by s.
func valueCases(add adder, target string, s *domain.SchemaDefinition, inBody bool, ok, bad int) {
	if s == nil {
		return
	}
	if v, has := wrongTypeValue(s.Type, inBody); has {
		add(domain.CategoryNegative, target, "wrong_type", target+" with the wrong type is rejected", v, bad)
	}
	if len(s.Enum) > 0 {
		add(domain.CategoryNegative, target, "invalid_enum", target+" outside its enum is rejected", "__invalid_enum_value__", bad)
	}
	switch s.Type {
	case domain.KindInteger, domain.KindNumber:
		if s.Minimum != nil {
			add(domain.CategoryBoundary, target, "min", target+" at its minimum is accepted", *s.Minimum, ok)
			add(domain.CategoryBoundary, target, "below_min", target+" below its minimum is rejected", *s.Minimum-1, bad)
		}
		if s.Maximum != nil {
			add(domain.CategoryBoundary, target, "max", target+" at its maximum is accepted", *s.Maximum, ok)
			add(domain.CategoryBoundary, target, "above_max", target+" above its maximum is rejected", *s.Maximum+1, bad)
		}
	case domain.KindString:
		if s.MinLength != nil && *s.MinLength <= maxStringBoundary {
			n := *s.MinLength
			add(domain.CategoryBoundary, target, "min_length", target+" at its minimum length is accepted", strings.Repeat("a", n), ok)
			if n > 0 {
				add(domain.CategoryBoundary, target, "below_min_length", target+" below its minimum length is rejected", strings.Repeat("a", n-1), bad)
			}
		}
		if s.MaxLength != nil && *s.MaxLength < maxStringBoundary {
			n := *s.MaxLength
			add(domain.CategoryBoundary, target, "max_length", target+" at its maximum length is accepted", strings.Repeat("a", n), ok)
			add(domain.CategoryBoundary, target, "above_max_length", target+" above its maximum length is rejected", strings.Repeat("a", n+1), bad)
		}
	}
}

// wrongTypeValue returns a value that violates the schema type. Strings are
// only meaningful as wrong-type targets in JSON bodies; query, header and path
// values are always strings on the wire.
func wrongTypeValue(kind domain.SchemaKind, inBody bool) (any, bool) {
	switch kind {
	case domain.KindInteger, domain.KindNumber:
		return "not-a-number", true
	case domain.KindString:
		return 12345, inBody
	case domain.KindBoolean:
		return "not-a-bool", true
	case domain.KindArray:
		return "not-an-array", true
	case domain.KindObject:
		return "not-an-object", true
	}
	return nil, false
}

func statusNumber(code string) (int, bool) { return domain.StatusNumber(code) }

func successStatus(ep *domain.APIEndpoint) int { return ep.SuccessStatus() }

func successSchema(ep *domain.APIEndpoint) *domain.SchemaDefinition {
	if r := ep.SuccessResponse(); r != nil {
		return r.Schema
	}
	return nil
}

func invalidStatus(ep *domain.APIEndpoint) int { return ep.ClientErrorStatus() }

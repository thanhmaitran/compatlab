package generation

import (
	"errors"
	"reflect"
	"testing"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

func sampleAPI(reverse bool) *domain.NormalizedAPI {
	one, hundred, eighteen := 1.0, 100.0, 18.0
	minLen, maxLen := 2, 5
	get := domain.APIEndpoint{
		Path: "/users/{id}", Method: "GET",
		Parameters: []domain.Parameter{
			{Name: "id", Location: domain.LocationPath, Required: true, Schema: &domain.SchemaDefinition{Type: domain.KindString}},
			{Name: "page", Location: domain.LocationQuery, Required: true,
				Schema: &domain.SchemaDefinition{Type: domain.KindInteger, Minimum: &one, Maximum: &hundred}},
		},
		Responses: []domain.Response{
			{StatusCode: "200", ContentType: "application/json", Schema: &domain.SchemaDefinition{Type: domain.KindObject}},
			{StatusCode: "400"},
		},
		Security: []domain.SecurityRequirement{{Name: "bearerAuth"}},
	}
	post := domain.APIEndpoint{
		Path: "/users", Method: "POST",
		RequestBody: &domain.RequestBody{Required: true, ContentType: "application/json", Schema: &domain.SchemaDefinition{
			Type: domain.KindObject, Required: []string{"name"},
			Properties: map[string]*domain.SchemaDefinition{
				"name":   {Type: domain.KindString, MinLength: &minLen, MaxLength: &maxLen},
				"age":    {Type: domain.KindInteger, Minimum: &eighteen, Maximum: &hundred},
				"role":   {Type: domain.KindString, Enum: []string{"admin", "member"}},
				"active": {Type: domain.KindBoolean},
			},
		}},
		Responses: []domain.Response{{StatusCode: "201"}, {StatusCode: "400"}},
	}
	eps := []domain.APIEndpoint{post, get}
	if reverse {
		eps = []domain.APIEndpoint{get, post}
	}
	return &domain.NormalizedAPI{Title: "Users", SourceHash: "abc123", Endpoints: eps}
}

func generate(t *testing.T, api *domain.NormalizedAPI, cfg domain.GenerationConfig) *domain.TestSuite {
	t.Helper()
	s, err := Generator{}.Generate(api, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func find(s *domain.TestSuite, method, target, variant string) *domain.TestCase {
	for i := range s.Cases {
		c := &s.Cases[i]
		if c.Method == method && c.Target == target && c.Variant == variant {
			return c
		}
	}
	return nil
}

func TestGenerationIsDeterministicAndOrderIndependent(t *testing.T) {
	cfg := domain.GenerationConfig{Seed: 7}
	a := generate(t, sampleAPI(false), cfg)
	b := generate(t, sampleAPI(false), cfg)
	c := generate(t, sampleAPI(true), cfg)
	if !reflect.DeepEqual(a, b) {
		t.Error("same input must produce the same suite")
	}
	if !reflect.DeepEqual(a, c) {
		t.Error("endpoint order in the source must not change the suite")
	}
}

func TestBoundaryCasesForNumbers(t *testing.T) {
	s := generate(t, sampleAPI(false), domain.GenerationConfig{})
	tests := []struct {
		variant string
		value   float64
		status  int
	}{{"min", 18, 201}, {"below_min", 17, 400}, {"max", 100, 201}, {"above_max", 101, 400}}
	for _, tc := range tests {
		c := find(s, "POST", "body.age", tc.variant)
		if c == nil {
			t.Fatalf("missing boundary case %s", tc.variant)
		}
		if c.Value != tc.value || c.ExpectedStatus != tc.status || c.Category != domain.CategoryBoundary {
			t.Errorf("%s: got value=%v status=%d category=%s", tc.variant, c.Value, c.ExpectedStatus, c.Category)
		}
	}
}

func TestBoundaryCasesForStringLengths(t *testing.T) {
	s := generate(t, sampleAPI(false), domain.GenerationConfig{})
	want := map[string]string{"min_length": "aa", "below_min_length": "a", "max_length": "aaaaa", "above_max_length": "aaaaaa"}
	for variant, value := range want {
		c := find(s, "POST", "body.name", variant)
		if c == nil || c.Value != value {
			t.Errorf("%s: got %+v, want value %q", variant, c, value)
		}
	}
}

func TestNegativeAndParameterCases(t *testing.T) {
	s := generate(t, sampleAPI(false), domain.GenerationConfig{})
	if c := find(s, "POST", "body.name", "missing"); c == nil || c.Value != nil || c.ExpectedStatus != 400 {
		t.Errorf("missing required field case wrong: %+v", c)
	}
	if find(s, "POST", "body.age", "missing") != nil {
		t.Error("optional fields must not get a missing-field case")
	}
	if c := find(s, "POST", "body.role", "invalid_enum"); c == nil {
		t.Error("enum fields need an invalid-enum case")
	}
	if c := find(s, "POST", "body.active", "wrong_type"); c == nil {
		t.Error("boolean fields need a wrong-type case")
	}
	if c := find(s, "GET", "query.page", "missing"); c == nil || c.Category != domain.CategoryParameterValidation {
		t.Errorf("required query parameter case wrong: %+v", c)
	}
	if find(s, "GET", "path.id", "missing") != nil {
		t.Error("path parameters cannot be omitted")
	}
	if find(s, "GET", "query.page", "wrong_type") == nil {
		t.Error("integer query parameter needs a wrong-type case")
	}
}

func TestAuthenticationCaseOnlyWhereSecured(t *testing.T) {
	s := generate(t, sampleAPI(false), domain.GenerationConfig{})
	if c := find(s, "GET", "header.authorization", "missing"); c == nil || c.ExpectedStatus != 401 {
		t.Errorf("secured endpoint needs a 401 case: %+v", c)
	}
	if find(s, "POST", "header.authorization", "missing") != nil {
		t.Error("unsecured endpoint must not get an authentication case")
	}
}

func TestStatusAndSchemaCases(t *testing.T) {
	s := generate(t, sampleAPI(false), domain.GenerationConfig{})
	if c := find(s, "POST", "response", "success"); c == nil || c.ExpectedStatus != 201 {
		t.Errorf("POST success status should be 201: %+v", c)
	}
	if find(s, "GET", "response.body", "schema") == nil {
		t.Error("GET documents a response schema, so it needs a schema case")
	}
	if find(s, "POST", "response.body", "schema") != nil {
		t.Error("POST documents no response schema")
	}
}

func TestLimitsAndPrioritization(t *testing.T) {
	s := generate(t, sampleAPI(false), domain.GenerationConfig{MaxCasesPerEndpoint: 3})
	perMethod := map[string]int{}
	for _, c := range s.Cases {
		perMethod[c.Method]++
	}
	for m, n := range perMethod {
		if n > 3 {
			t.Errorf("%s has %d cases, limit is 3", m, n)
		}
	}
	if find(s, "GET", "header.authorization", "missing") == nil {
		t.Error("the limit must keep the highest-priority cases (authentication)")
	}
	for i := 1; i < len(s.Cases); i++ {
		if s.Cases[i-1].Priority > s.Cases[i].Priority {
			t.Fatal("cases must be sorted by priority")
		}
	}
	p := generate(t, sampleAPI(false), domain.GenerationConfig{PriorityEndpoints: []string{"POST /users"}})
	if p.Cases[0].Method != "POST" {
		t.Errorf("prioritized endpoint must run first, got %s", p.Cases[0].Method)
	}
}

func TestIDsAreUniqueAndSuiteIDTracksConfig(t *testing.T) {
	s := generate(t, sampleAPI(false), domain.GenerationConfig{})
	seen := map[string]bool{}
	for _, c := range s.Cases {
		if seen[c.ID] {
			t.Fatalf("duplicate case ID %s", c.ID)
		}
		seen[c.ID] = true
		if !c.Category.Valid() {
			t.Errorf("invalid category %q", c.Category)
		}
	}
	if s.SpecHash != "abc123" || s.GeneratorVersion != GeneratorVersion {
		t.Errorf("suite metadata wrong: %+v", s)
	}
	other := generate(t, sampleAPI(false), domain.GenerationConfig{Seed: 99})
	if other.ID == s.ID {
		t.Error("suite ID must change with the configuration")
	}
}

func TestHugeStringLimitsAreSkipped(t *testing.T) {
	huge := 1_000_000_000
	api := &domain.NormalizedAPI{Endpoints: []domain.APIEndpoint{{
		Path: "/x", Method: "POST",
		RequestBody: &domain.RequestBody{Schema: &domain.SchemaDefinition{
			Type:       domain.KindObject,
			Properties: map[string]*domain.SchemaDefinition{"s": {Type: domain.KindString, MaxLength: &huge, MinLength: &huge}},
		}},
	}}}
	s := generate(t, api, domain.GenerationConfig{})
	if find(s, "POST", "body.s", "max_length") != nil || find(s, "POST", "body.s", "min_length") != nil {
		t.Error("string boundaries above the safety limit must be skipped")
	}
}

func TestInvalidInputs(t *testing.T) {
	if _, err := (Generator{}).Generate(nil, domain.GenerationConfig{}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("nil API: %v", err)
	}
	if _, err := (Generator{}).Generate(sampleAPI(false), domain.GenerationConfig{MaxCasesPerEndpoint: -1}); !errors.Is(err, domain.ErrInvalid) {
		t.Errorf("negative limit: %v", err)
	}
}

func TestStatusNumber(t *testing.T) {
	for code, want := range map[string]int{"200": 200, "2XX": 200, "4xx": 400, "201": 201} {
		if got, ok := statusNumber(code); !ok || got != want {
			t.Errorf("statusNumber(%q) = %d,%v want %d", code, got, ok, want)
		}
	}
	if _, ok := statusNumber("default"); ok {
		t.Error("default is not a numeric status")
	}
}

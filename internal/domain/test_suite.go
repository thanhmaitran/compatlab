package domain

// GenerationConfig bounds and seeds test generation. PriorityEndpoints lists
// endpoint keys (for example "GET /users/{}") whose tests should run first,
// typically the endpoints touched by breaking changes.
type GenerationConfig struct {
	Seed                int64    `json:"seed"`
	MaxCasesPerEndpoint int      `json:"max_cases_per_endpoint"`
	PriorityEndpoints   []string `json:"priority_endpoints,omitempty"`
}

// TestSuite is a reproducible group of test cases.
type TestSuite struct {
	ID               string           `json:"id"`
	Name             string           `json:"name"`
	GeneratorVersion string           `json:"generator_version"`
	SpecHash         string           `json:"spec_hash"`
	Config           GenerationConfig `json:"config"`
	Cases            []TestCase       `json:"cases"`
}

package domain

// SpecificationParser turns a raw OpenAPI document into the normalized model.
type SpecificationParser interface {
	Parse(data []byte) (*NormalizedAPI, error)
}

// CompatibilityAnalyzer compares two normalized APIs.
type CompatibilityAnalyzer interface {
	Compare(oldAPI, newAPI *NormalizedAPI) (*CompatibilityAnalysis, error)
}

// ContractVerifier checks a consumer contract against a provider API.
type ContractVerifier interface {
	Verify(contract *ConsumerContract, api *NormalizedAPI) (*ContractVerification, error)
}

// TestGenerator creates a reproducible test suite from an API.
type TestGenerator interface {
	Generate(api *NormalizedAPI, cfg GenerationConfig) (*TestSuite, error)
}

// PolicyEvaluator decides whether a release may proceed.
type PolicyEvaluator interface {
	Evaluate(policy *ReleasePolicy, report *CompatibilityReport) (*PolicyEvaluation, error)
}

// ReportGenerator renders an analysis (for example as JSON or HTML).
type ReportGenerator interface {
	Generate(analysis *CompatibilityAnalysis) ([]byte, error)
}

package domain

// CompatibilityReport aggregates every result needed for a release decision.
type CompatibilityReport struct {
	Analysis      *CompatibilityAnalysis `json:"analysis"`
	Contracts     []ContractVerification `json:"contracts,omitempty"`
	Impacts       []ConsumerImpact       `json:"consumer_impacts,omitempty"`
	TestRun       *TestRun               `json:"test_run,omitempty"`
	Evaluation    *PolicyEvaluation      `json:"evaluation,omitempty"`
	EngineVersion string                 `json:"engine_version,omitempty"`
}

// Package policies evaluates release policies against a compatibility report.
package policies

import (
	"fmt"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// Evaluator implements domain.PolicyEvaluator.
type Evaluator struct{}

var _ domain.PolicyEvaluator = Evaluator{}

// Evaluate returns BLOCK if any blocking rule is violated, WARN if only
// advisory findings exist, and PASS otherwise. Unless the policy sets
// include_forward, only backward-direction changes are considered.
func (Evaluator) Evaluate(policy *domain.ReleasePolicy, report *domain.CompatibilityReport) (*domain.PolicyEvaluation, error) {
	if policy == nil || report == nil || report.Analysis == nil {
		return nil, fmt.Errorf("%w: policy and analysis are required", domain.ErrInvalid)
	}
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	failOn := map[domain.ChangeClassification]bool{}
	for _, c := range policy.FailOn {
		failOn[c] = true
	}
	counts := map[domain.ChangeClassification]int{}
	for _, ch := range report.Analysis.Changes {
		if ch.Direction == domain.DirectionForward && !policy.IncludeForward {
			continue
		}
		counts[ch.Classification]++
	}

	var block, warn []string
	for _, c := range domain.AllClassifications() {
		n := counts[c]
		switch {
		case n == 0:
		case failOn[c]:
			block = append(block, fmt.Sprintf("%d %s change(s) violate the policy", n, c))
		case c != domain.NonBreaking:
			warn = append(warn, fmt.Sprintf("%d %s change(s) are present and need review", n, c))
		}
	}

	failedContracts := 0
	for _, v := range report.Contracts {
		if !v.Passed() {
			failedContracts++
		}
	}
	if policy.RequireContractTests {
		if len(report.Contracts) == 0 {
			block = append(block, "contract verification is required but no contracts were verified")
		}
		if failedContracts > 0 {
			block = append(block, fmt.Sprintf("%d consumer contract(s) failed verification", failedContracts))
		}
	} else if failedContracts > 0 {
		warn = append(warn, fmt.Sprintf("%d consumer contract(s) failed verification", failedContracts))
	}

	if policy.MinimumPassRate > 0 {
		if report.TestRun == nil {
			warn = append(warn, "no test run is available to check the minimum pass rate")
		} else if rate := report.TestRun.PassRate(); rate < policy.MinimumPassRate {
			block = append(block, fmt.Sprintf("test pass rate %.1f%% is below the required %.1f%%", rate, policy.MinimumPassRate))
		}
	}

	switch {
	case len(block) > 0:
		return &domain.PolicyEvaluation{Decision: domain.DecisionBlock, Reasons: append(block, warn...)}, nil
	case len(warn) > 0:
		return &domain.PolicyEvaluation{Decision: domain.DecisionWarn, Reasons: warn}, nil
	}
	return &domain.PolicyEvaluation{Decision: domain.DecisionPass}, nil
}

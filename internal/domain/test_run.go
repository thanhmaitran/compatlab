package domain

import "time"

// TestResult is the outcome of executing one test case.
type TestResult struct {
	CaseID   string        `json:"case_id"`
	Status   TestStatus    `json:"status"`
	Message  string        `json:"message,omitempty"`
	Duration time.Duration `json:"duration"`
}

// TestRun is one execution of a suite.
type TestRun struct {
	ID      string       `json:"id"`
	SuiteID string       `json:"suite_id"`
	Results []TestResult `json:"results"`
}

// PassRate returns the percentage of executed tests that passed. Pending,
// running and skipped results are not counted. It returns 0 if nothing ran.
func (r *TestRun) PassRate() float64 {
	passed, executed := 0, 0
	for _, res := range r.Results {
		switch res.Status {
		case TestStatusPassed:
			passed++
			executed++
		case TestStatusFailed, TestStatusErrored:
			executed++
		}
	}
	if executed == 0 {
		return 0
	}
	return float64(passed) / float64(executed) * 100
}

package domain

import (
	"errors"
	"testing"
	"time"
)

func TestPolicyValidate(t *testing.T) {
	ok := &ReleasePolicy{FailOn: []ChangeClassification{Breaking}, MinimumPassRate: 95}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid policy rejected: %v", err)
	}
	bad := []*ReleasePolicy{
		{FailOn: []ChangeClassification{"SCARY"}},
		{MinimumPassRate: 101},
		{MinimumPassRate: -1},
	}
	for i, p := range bad {
		if err := p.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("case %d: expected ErrInvalid, got %v", i, err)
		}
	}
}

func TestWorkflowValidate(t *testing.T) {
	ok := &Workflow{Name: "order", Steps: []WorkflowStep{{Name: "create"}, {Name: "get", Timeout: time.Second}}}
	if err := ok.Validate(); err != nil {
		t.Fatalf("valid workflow rejected: %v", err)
	}
	bad := []*Workflow{
		{Steps: []WorkflowStep{{Name: "a"}}},
		{Name: "x"},
		{Name: "x", Steps: []WorkflowStep{{Name: ""}}},
		{Name: "x", Steps: []WorkflowStep{{Name: "a"}, {Name: "a"}}},
		{Name: "x", Steps: []WorkflowStep{{Name: "a", Timeout: -1}}},
		{Name: "x", Steps: []WorkflowStep{{Name: "a", Retry: RetryPolicy{MaxAttempts: -1}}}},
	}
	for i, w := range bad {
		if err := w.Validate(); !errors.Is(err, ErrInvalid) {
			t.Errorf("case %d: expected ErrInvalid, got %v", i, err)
		}
	}
}

func TestTestRunPassRate(t *testing.T) {
	r := &TestRun{Results: []TestResult{
		{Status: TestStatusPassed}, {Status: TestStatusPassed}, {Status: TestStatusPassed},
		{Status: TestStatusFailed}, {Status: TestStatusSkipped}, {Status: TestStatusPending},
	}}
	if got := r.PassRate(); got != 75 {
		t.Errorf("PassRate() = %v, want 75", got)
	}
	if (&TestRun{}).PassRate() != 0 {
		t.Error("empty run must have pass rate 0")
	}
}

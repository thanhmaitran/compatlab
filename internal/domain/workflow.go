package domain

import (
	"fmt"
	"time"
)

// RetryPolicy controls retries of a failing workflow step.
type RetryPolicy struct {
	MaxAttempts int           `json:"max_attempts"`
	Backoff     time.Duration `json:"backoff"`
}

// Assertion checks a value taken from a step response.
type Assertion struct {
	Target   string `json:"target"`
	Operator string `json:"operator"`
	Expected string `json:"expected"`
}

// WorkflowStep is one API call in a stateful workflow. Extract maps a variable
// name to a response path so later steps can reuse the value.
type WorkflowStep struct {
	Name       string            `json:"name"`
	Method     string            `json:"method"`
	Headers    map[string]string `json:"headers,omitempty"`
	Path       string            `json:"path"`
	Body       any               `json:"body,omitempty"`
	Extract    map[string]string `json:"extract,omitempty"`
	Assertions []Assertion       `json:"assertions,omitempty"`
	Timeout    time.Duration     `json:"timeout"`
	Retry      RetryPolicy       `json:"retry"`
}

// Workflow is an ordered chain of dependent API operations.
type Workflow struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	OnFailure string         `json:"on_failure,omitempty"` // "stop" (default) or "continue"
	Steps     []WorkflowStep `json:"steps"`
	Cleanup   []WorkflowStep `json:"cleanup,omitempty"` // always run after the steps
}

// Validate checks names, uniqueness and non-negative limits.
func (w *Workflow) Validate() error {
	if w.Name == "" {
		return fmt.Errorf("%w: workflow name is required", ErrInvalid)
	}
	if len(w.Steps) == 0 {
		return fmt.Errorf("%w: workflow needs at least one step", ErrInvalid)
	}
	if w.OnFailure != "" && w.OnFailure != "stop" && w.OnFailure != "continue" {
		return fmt.Errorf("%w: on_failure must be \"stop\" or \"continue\"", ErrInvalid)
	}
	for i, s := range w.Cleanup {
		if s.Name == "" {
			return fmt.Errorf("%w: cleanup step %d has no name", ErrInvalid, i)
		}
	}
	seen := map[string]bool{}
	for i, s := range w.Steps {
		switch {
		case s.Name == "":
			return fmt.Errorf("%w: step %d has no name", ErrInvalid, i)
		case seen[s.Name]:
			return fmt.Errorf("%w: duplicate step name %q", ErrInvalid, s.Name)
		case s.Timeout < 0 || s.Retry.MaxAttempts < 0 || s.Retry.Backoff < 0:
			return fmt.Errorf("%w: step %q has a negative timeout or retry value", ErrInvalid, s.Name)
		}
		seen[s.Name] = true
	}
	return nil
}

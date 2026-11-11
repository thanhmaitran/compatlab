package workflows

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

type fileRetry struct {
	MaxAttempts int `json:"max_attempts"`
	BackoffMS   int `json:"backoff_ms"`
}

type fileStep struct {
	Name       string             `json:"name"`
	Method     string             `json:"method"`
	Path       string             `json:"path"`
	Headers    map[string]string  `json:"headers"`
	Body       any                `json:"body"`
	Extract    map[string]string  `json:"extract"`
	Assertions []domain.Assertion `json:"assertions"`
	TimeoutMS  int                `json:"timeout_ms"`
	Retry      fileRetry          `json:"retry"`
}

type file struct {
	Name      string     `json:"name"`
	OnFailure string     `json:"on_failure"`
	Steps     []fileStep `json:"steps"`
	Cleanup   []fileStep `json:"cleanup"`
}

func convert(steps []fileStep) []domain.WorkflowStep {
	out := make([]domain.WorkflowStep, 0, len(steps))
	for _, s := range steps {
		out = append(out, domain.WorkflowStep{
			Name: s.Name, Method: s.Method, Path: s.Path, Headers: s.Headers, Body: s.Body,
			Extract: s.Extract, Assertions: s.Assertions,
			Timeout: time.Duration(s.TimeoutMS) * time.Millisecond,
			Retry:   domain.RetryPolicy{MaxAttempts: s.Retry.MaxAttempts, Backoff: time.Duration(s.Retry.BackoffMS) * time.Millisecond},
		})
	}
	return out
}

// ParseFile reads the JSON workflow format, which uses millisecond fields
// (timeout_ms, retry.backoff_ms) instead of nanosecond durations.
func ParseFile(data []byte) (*domain.Workflow, error) {
	var f file
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return nil, fmt.Errorf("%w: invalid workflow: %v", domain.ErrInvalid, err)
	}
	wf := &domain.Workflow{Name: f.Name, OnFailure: f.OnFailure, Steps: convert(f.Steps), Cleanup: convert(f.Cleanup)}
	if err := wf.Validate(); err != nil {
		return nil, err
	}
	return wf, nil
}

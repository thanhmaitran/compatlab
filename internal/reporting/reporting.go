// Package reporting renders compatibility reports as text, JSON or HTML.
package reporting

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"sort"
	"strings"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// JSON renders the full report as indented JSON.
func JSON(r *domain.CompatibilityReport) ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

// JSONGenerator implements domain.ReportGenerator for an analysis.
type JSONGenerator struct{}

var _ domain.ReportGenerator = JSONGenerator{}

// Generate renders an analysis as JSON.
func (JSONGenerator) Generate(a *domain.CompatibilityAnalysis) ([]byte, error) {
	return JSON(&domain.CompatibilityReport{Analysis: a})
}

func decisionLabel(d domain.PolicyDecision) string {
	switch d {
	case domain.DecisionBlock:
		return "BLOCKED"
	case domain.DecisionWarn:
		return "PASSED WITH WARNINGS"
	case domain.DecisionPass:
		return "PASSED"
	}
	return string(d)
}

// Text renders a human-readable release check, suitable for CI logs.
func Text(r *domain.CompatibilityReport) string {
	var b strings.Builder
	b.WriteString("COMPATLAB RELEASE CHECK\n\n")
	a := r.Analysis
	if a == nil {
		b.WriteString("No analysis available.\n")
		return b.String()
	}
	fmt.Fprintf(&b, "Base:   %s\nTarget: %s\n\n", a.BaseVersion, a.TargetVersion)
	if a.Scores != nil {
		fmt.Fprintf(&b, "Compatibility score (CompatLab-defined, 0-100): backward %d, forward %d\n\n", a.Scores.Backward, a.Scores.Forward)
	}
	back, fwd := a.SummaryFor(domain.DirectionBackward), a.SummaryFor(domain.DirectionForward)
	fmt.Fprintf(&b, "Backward changes: %d breaking, %d conditional, %d non-breaking\n", back.Breaking, back.Conditional, back.NonBreaking)
	fmt.Fprintf(&b, "Forward changes:  %d breaking, %d conditional, %d non-breaking\n", fwd.Breaking, fwd.Conditional, fwd.NonBreaking)

	for _, cl := range []domain.ChangeClassification{domain.Breaking, domain.Conditional} {
		var rows []domain.ClassifiedChange
		for _, c := range a.Changes {
			if c.Direction == domain.DirectionBackward && c.Classification == cl {
				rows = append(rows, c)
			}
		}
		if len(rows) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s (backward)\n", cl)
		for _, c := range rows {
			fmt.Fprintf(&b, "  [%s] %s  %s %s  %s\n", c.Severity, c.RuleID, c.Method, c.Path, c.Description)
		}
	}

	if len(r.Impacts) > 0 {
		affected, review := map[string]int{}, map[string]int{}
		for _, im := range r.Impacts {
			if im.Level == domain.ImpactAffected {
				affected[im.Consumer]++
			} else {
				review[im.Consumer]++
			}
		}
		names := map[string]bool{}
		for n := range affected {
			names[n] = true
		}
		for n := range review {
			names[n] = true
		}
		sorted := make([]string, 0, len(names))
		for n := range names {
			sorted = append(sorted, n)
		}
		sort.Strings(sorted)
		b.WriteString("\nConsumer impact (from registered contracts)\n")
		for _, n := range sorted {
			fmt.Fprintf(&b, "  %s: %d affected, %d to review\n", n, affected[n], review[n])
		}
	}

	if len(r.Contracts) > 0 {
		b.WriteString("\nContract verification\n")
		for _, v := range r.Contracts {
			passed := 0
			for _, in := range v.Interactions {
				if in.Passed {
					passed++
				}
			}
			fmt.Fprintf(&b, "  %s -> %s: %d / %d interactions passed\n", v.Consumer, v.Provider, passed, len(v.Interactions))
			for _, in := range v.Interactions {
				if !in.Passed {
					fmt.Fprintf(&b, "    FAIL %s: %s\n", in.Description, in.Message)
				}
			}
		}
	}
	if r.TestRun != nil {
		fmt.Fprintf(&b, "\nTest pass rate: %.1f%%\n", r.TestRun.PassRate())
	}
	if r.Evaluation != nil {
		fmt.Fprintf(&b, "\nRelease decision: %s\n", decisionLabel(r.Evaluation.Decision))
		for _, reason := range r.Evaluation.Reasons {
			fmt.Fprintf(&b, "  - %s\n", reason)
		}
	}
	return b.String()
}

var htmlTemplate = template.Must(template.New("report").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><title>CompatLab report</title>
<style>
body{font-family:system-ui,Segoe UI,Arial,sans-serif;margin:2rem;color:#0f172a}
table{border-collapse:collapse;width:100%}
th,td{border:1px solid #cbd5e1;padding:.4rem .6rem;text-align:left;font-size:.9rem}
th{background:#f1f5f9}
.BREAKING{color:#b91c1c;font-weight:600}.CONDITIONAL{color:#b45309;font-weight:600}.NON_BREAKING{color:#15803d}
</style></head><body>
<h1>CompatLab report</h1>
{{with .Analysis}}
<p>Base <strong>{{.BaseVersion}}</strong> &rarr; Target <strong>{{.TargetVersion}}</strong></p>
{{with .Scores}}<p>Score (CompatLab-defined): backward {{.Backward}}, forward {{.Forward}}</p>{{end}}
<table><tr><th>Direction</th><th>Class</th><th>Severity</th><th>Rule</th><th>Operation</th><th>Description</th></tr>
{{range .Changes}}<tr><td>{{.Direction}}</td><td class="{{.Classification}}">{{.Classification}}</td><td>{{.Severity}}</td><td>{{.RuleID}}</td><td>{{.Method}} {{.Path}}</td><td>{{.Description}}</td></tr>
{{end}}</table>
{{end}}
{{with .Evaluation}}<h2>Release decision: {{.Decision}}</h2><ul>{{range .Reasons}}<li>{{.}}</li>{{end}}</ul>{{end}}
</body></html>
`))

// HTML renders the report as a standalone HTML page. All values are escaped.
func HTML(r *domain.CompatibilityReport) ([]byte, error) {
	var buf bytes.Buffer
	if err := htmlTemplate.Execute(&buf, r); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

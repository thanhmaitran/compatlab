// Package analytics turns stored analyses, test runs and gate reports into
// the historical metrics shown on the dashboard. It is a pure computation: the
// backend loads the data and passes it in.
package analytics

import (
	"sort"
	"time"

	"github.com/thanhmaitran/compatlab/internal/domain"
)

// AnalysisPoint is one stored analysis with the version labels resolved.
type AnalysisPoint struct {
	ID          string
	BaseLabel   string
	TargetLabel string
	CreatedAt   time.Time
	Analysis    *domain.CompatibilityAnalysis
}

// RunPoint is one stored test run.
type RunPoint struct {
	ID        string
	CreatedAt time.Time
	Run       *domain.TestRun
}

// ReportPoint is one stored release-gate report.
type ReportPoint struct {
	ID        string
	CreatedAt time.Time
	Report    *domain.CompatibilityReport
}

// EndpointChange counts how often an operation changed.
type EndpointChange struct {
	Method string `json:"method"`
	Path   string `json:"path"`
	Count  int    `json:"count"`
}

// Input is everything Compute needs.
type Input struct {
	Analyses     []AnalysisPoint
	Runs         []RunPoint
	Reports      []ReportPoint
	TopEndpoints []EndpointChange
}

// ScorePoint answers: is compatibility improving over time?
type ScorePoint struct {
	AnalysisID string    `json:"analysis_id"`
	Base       string    `json:"base"`
	Target     string    `json:"target"`
	CreatedAt  time.Time `json:"created_at"`
	Backward   int       `json:"backward"`
	Forward    int       `json:"forward"`
}

// BreakingPoint answers: which version introduced the most breaking changes?
type BreakingPoint struct {
	Target      string `json:"target"`
	Breaking    int    `json:"breaking"`
	Conditional int    `json:"conditional"`
	NonBreaking int    `json:"non_breaking"`
}

// PassRatePoint answers: is test quality improving?
type PassRatePoint struct {
	RunID     string    `json:"run_id"`
	CreatedAt time.Time `json:"created_at"`
	PassRate  float64   `json:"pass_rate"`
}

// ContractPoint answers: are consumer contracts still being honoured?
type ContractPoint struct {
	ReportID  string    `json:"report_id"`
	CreatedAt time.Time `json:"created_at"`
	Passed    int       `json:"passed"`
	Total     int       `json:"total"`
}

// GatePoint answers: how often does the release gate block?
type GatePoint struct {
	ReportID  string                `json:"report_id"`
	CreatedAt time.Time             `json:"created_at"`
	Decision  domain.PolicyDecision `json:"decision"`
}

// Summary is the analytics payload of one project.
type Summary struct {
	TotalAnalyses       int                    `json:"total_analyses"`
	ScoreTrend          []ScorePoint           `json:"score_trend"`
	BreakingByVersion   []BreakingPoint        `json:"breaking_by_version"`
	Distribution        domain.AnalysisSummary `json:"distribution"`
	PassRateTrend       []PassRatePoint        `json:"pass_rate_trend"`
	ContractTrend       []ContractPoint        `json:"contract_trend"`
	GateHistory         []GatePoint            `json:"gate_history"`
	GateBlocks          int                    `json:"gate_blocks"`
	TopChangedEndpoints []EndpointChange       `json:"top_changed_endpoints"`
}

// Compute derives the summary. All counts use the backward direction, which
// answers "will existing consumers break?"; forward scores are reported
// separately in the score trend. Slices are never nil so JSON renders [].
func Compute(in Input) Summary {
	s := Summary{
		ScoreTrend: []ScorePoint{}, BreakingByVersion: []BreakingPoint{}, PassRateTrend: []PassRatePoint{},
		ContractTrend: []ContractPoint{}, GateHistory: []GatePoint{}, TopChangedEndpoints: []EndpointChange{},
	}
	analyses := append([]AnalysisPoint(nil), in.Analyses...)
	sort.SliceStable(analyses, func(i, j int) bool { return analyses[i].CreatedAt.Before(analyses[j].CreatedAt) })
	for _, a := range analyses {
		if a.Analysis == nil {
			continue
		}
		s.TotalAnalyses++
		p := ScorePoint{AnalysisID: a.ID, Base: a.BaseLabel, Target: a.TargetLabel, CreatedAt: a.CreatedAt}
		if a.Analysis.Scores != nil {
			p.Backward, p.Forward = a.Analysis.Scores.Backward, a.Analysis.Scores.Forward
		}
		s.ScoreTrend = append(s.ScoreTrend, p)
		sum := a.Analysis.SummaryFor(domain.DirectionBackward)
		s.BreakingByVersion = append(s.BreakingByVersion, BreakingPoint{
			Target: a.TargetLabel, Breaking: sum.Breaking, Conditional: sum.Conditional, NonBreaking: sum.NonBreaking})
		s.Distribution.Breaking += sum.Breaking
		s.Distribution.Conditional += sum.Conditional
		s.Distribution.NonBreaking += sum.NonBreaking
	}

	runs := append([]RunPoint(nil), in.Runs...)
	sort.SliceStable(runs, func(i, j int) bool { return runs[i].CreatedAt.Before(runs[j].CreatedAt) })
	for _, r := range runs {
		if r.Run != nil {
			s.PassRateTrend = append(s.PassRateTrend, PassRatePoint{RunID: r.ID, CreatedAt: r.CreatedAt, PassRate: r.Run.PassRate()})
		}
	}

	reports := append([]ReportPoint(nil), in.Reports...)
	sort.SliceStable(reports, func(i, j int) bool { return reports[i].CreatedAt.Before(reports[j].CreatedAt) })
	for _, r := range reports {
		if r.Report == nil {
			continue
		}
		if len(r.Report.Contracts) > 0 {
			p := ContractPoint{ReportID: r.ID, CreatedAt: r.CreatedAt}
			for _, c := range r.Report.Contracts {
				for _, in := range c.Interactions {
					p.Total++
					if in.Passed {
						p.Passed++
					}
				}
			}
			s.ContractTrend = append(s.ContractTrend, p)
		}
		if r.Report.Evaluation != nil {
			s.GateHistory = append(s.GateHistory, GatePoint{ReportID: r.ID, CreatedAt: r.CreatedAt, Decision: r.Report.Evaluation.Decision})
			if r.Report.Evaluation.Decision == domain.DecisionBlock {
				s.GateBlocks++
			}
		}
	}
	s.TopChangedEndpoints = append(s.TopChangedEndpoints, in.TopEndpoints...)
	return s
}

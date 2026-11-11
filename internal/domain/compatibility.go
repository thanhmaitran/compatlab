package domain

// ClassifiedChange is a ChangeRecord evaluated by one compatibility rule for
// one direction. Every change yields one entry per direction.
type ClassifiedChange struct {
	ChangeRecord
	RuleID         string                 `json:"rule_id"`
	Classification ChangeClassification   `json:"classification"`
	Severity       Severity               `json:"severity"`
	Direction      CompatibilityDirection `json:"direction"`
	Explanation    string                 `json:"explanation,omitempty"`
	Remediation    string                 `json:"remediation,omitempty"`
}

// AnalysisSummary counts classified changes per classification.
type AnalysisSummary struct {
	Breaking    int `json:"breaking"`
	NonBreaking int `json:"non_breaking"`
	Conditional int `json:"conditional"`
}

// CompatibilityScores are CompatLab-defined 0-100 indicators, one per
// direction. They are not an industry standard and never replace the list of
// changes: any breaking change caps the score of its direction.
type CompatibilityScores struct {
	Backward int `json:"backward"`
	Forward  int `json:"forward"`
}

// CompatibilityAnalysis is the deterministic result of comparing two versions.
// It intentionally carries no timestamps so equal inputs give equal output.
type CompatibilityAnalysis struct {
	BaseVersion   string               `json:"base_version"`
	TargetVersion string               `json:"target_version"`
	Changes       []ClassifiedChange   `json:"changes"`
	Scores        *CompatibilityScores `json:"scores,omitempty"`
}

// SummaryFor counts the changes of one direction per classification.
func (a *CompatibilityAnalysis) SummaryFor(dir CompatibilityDirection) AnalysisSummary {
	var s AnalysisSummary
	for _, c := range a.Changes {
		if c.Direction != dir {
			continue
		}
		switch c.Classification {
		case Breaking:
			s.Breaking++
		case NonBreaking:
			s.NonBreaking++
		case Conditional:
			s.Conditional++
		}
	}
	return s
}

// Summary counts the changes of every direction per classification.
func (a *CompatibilityAnalysis) Summary() AnalysisSummary {
	var s AnalysisSummary
	for _, c := range a.Changes {
		switch c.Classification {
		case Breaking:
			s.Breaking++
		case NonBreaking:
			s.NonBreaking++
		case Conditional:
			s.Conditional++
		}
	}
	return s
}

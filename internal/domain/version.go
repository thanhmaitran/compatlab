package domain

// APIVersion labels a specification as a release of a project (for example v1.2.0).
type APIVersion struct {
	ID              string `json:"id"`
	ProjectID       string `json:"project_id"`
	Label           string `json:"label"`
	SpecificationID string `json:"specification_id"`
}

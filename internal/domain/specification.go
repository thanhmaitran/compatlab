package domain

// APISpecification is an imported OpenAPI document.
type APISpecification struct {
	ID        string              `json:"id"`
	ProjectID string              `json:"project_id"`
	Filename  string              `json:"filename"`
	Format    string              `json:"format"` // "yaml" or "json"
	Hash      string              `json:"hash"`   // SHA-256 of the raw document
	Status    SpecificationStatus `json:"status"`
}

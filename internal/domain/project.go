package domain

// APIProject represents one API managed in CompatLab.
type APIProject struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

package domain

// TestCase is one generated or hand-written API test. Target names the input
// being exercised (for example query.page or body.age) and Value is the value
// to send; a nil Value means the input is omitted.
type TestCase struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	Category       TestCategory `json:"category"`
	Method         string       `json:"method"`
	Path           string       `json:"path"`
	Description    string       `json:"description,omitempty"`
	Target         string       `json:"target,omitempty"`
	Variant        string       `json:"variant,omitempty"`
	Value          any          `json:"value,omitempty"`
	ExpectedStatus int          `json:"expected_status"`
	Priority       int          `json:"priority"` // lower runs first
}

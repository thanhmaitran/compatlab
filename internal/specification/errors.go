package specification

import "errors"

// Sentinel errors returned (wrapped) by the specification engine.
var (
	ErrParse       = errors.New("specification parse error")
	ErrValidation  = errors.New("specification validation error")
	ErrUnsupported = errors.New("unsupported specification")
	ErrTooLarge    = errors.New("specification too large")
	ErrReference   = errors.New("specification reference error")
)

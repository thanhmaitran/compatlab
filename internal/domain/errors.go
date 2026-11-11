package domain

import "errors"

// ErrInvalid is wrapped by every validation error returned from this package.
var ErrInvalid = errors.New("invalid domain value")

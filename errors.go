package uri

import "strconv"

// The error taxonomy mirrors MRI's: InvalidURIError (URI::InvalidURIError) for a
// string that is not a URI, InvalidComponentError (URI::InvalidComponentError)
// for an out-of-grammar component assignment, and BadURIError (URI::BadURIError)
// for an operation that is invalid for the URI's kind. All three satisfy the
// Error marker interface so callers can match the family with errors.As.

// Error is the common interface implemented by every error this package
// returns, corresponding to MRI's URI::Error module.
type Error interface {
	error
	uriError()
}

// InvalidURIError corresponds to URI::InvalidURIError: the input could not be
// parsed as a URI. Its message reproduces MRI's wording byte-for-byte.
type InvalidURIError struct {
	URI string
}

func (e *InvalidURIError) Error() string {
	return "bad URI (is not URI?): " + strconv.Quote(e.URI)
}
func (e *InvalidURIError) uriError() {}

// InvalidComponentError corresponds to URI::InvalidComponentError: a component
// value (scheme, host, port, ...) did not satisfy its grammar. The message
// matches MRI's "bad component(expected X component): Y".
type InvalidComponentError struct {
	Component string // e.g. "scheme", "host", "port"
	Value     string
	quoted    bool // MRI quotes the offending value for some components (port)
}

func (e *InvalidComponentError) Error() string {
	v := e.Value
	if e.quoted {
		v = strconv.Quote(v)
	}
	return "bad component(expected " + e.Component + " component): " + v
}
func (e *InvalidComponentError) uriError() {}

// BadURIError corresponds to URI::BadURIError: an operation is not valid for the
// receiver URI (for example resolving a reference against a non-absolute base).
type BadURIError struct {
	Message string
}

func (e *BadURIError) Error() string { return e.Message }
func (e *BadURIError) uriError()     {}

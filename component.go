package uri

import "strconv"

// The component setters validate their argument against the same grammars MRI
// enforces in URI::Generic's check_scheme/check_host/... methods, returning an
// InvalidComponentError (matching URI::InvalidComponentError) on a bad value.
// They mutate the receiver in place and return it for chaining.

// SetScheme validates and assigns the scheme. A scheme must be
// ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ); the empty string clears it.
func (u *URI) SetScheme(s string) error {
	if s != "" && schemeEnd(s+":") != len(s) {
		return &InvalidComponentError{Component: "scheme", Value: s}
	}
	u.Scheme = s
	return nil
}

// SetHost validates and assigns the host. A host may not contain
// whitespace or the authority delimiters "/?#@"; the empty string clears it.
func (u *URI) SetHost(h string) error {
	if !validHost(h) {
		return &InvalidComponentError{Component: "host", Value: h}
	}
	u.Host = h
	return nil
}

// SetPort validates and assigns the port from its string form, matching MRI's
// acceptance of an integer or an all-digit string. A non-numeric value is an
// InvalidComponentError whose message quotes the value, as MRI does.
func (u *URI) SetPort(p string) error {
	if p == "" {
		u.HasPort = false
		u.Port = 0
		return nil
	}
	if !allDigits(p) {
		return &InvalidComponentError{Component: "port", Value: p, quoted: true}
	}
	n, _ := strconv.Atoi(p)
	u.Port = n
	u.HasPort = true
	return nil
}

// SetUserinfo assigns the userinfo. MRI accepts any userinfo without "/?#@", so
// the same delimiter check as host applies.
func (u *URI) SetUserinfo(ui string) error {
	if !validHost(ui) {
		return &InvalidComponentError{Component: "userinfo", Value: ui}
	}
	u.Userinfo = ui
	return nil
}

// validHost reports whether s is acceptable as a host or userinfo: no
// whitespace and none of the authority delimiters "/?#@". The empty string is
// valid (it clears the component).
func validHost(s string) bool {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
			c == '/' || c == '?' || c == '#' || c == '@' {
			return false
		}
	}
	return true
}

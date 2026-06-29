package uri

import "strings"

// hexUpper holds the uppercase hex digits MRI uses in percent-encodings.
const hexUpper = "0123456789ABCDEF"

// Encode percent-encodes s the way URI.encode_www_form_component does: every
// byte outside the www-form unreserved set ([A-Za-z0-9*\-._]) is %-escaped, and
// a space becomes '+'. This is the per-component encoder used for query values.
//
// When an optional unsafe pattern is supplied, the older URI.escape semantics
// are used instead via Escape; Encode itself ignores it (kept variadic to match
// the documented rbgo binding signature).
func Encode(s string, unsafe ...string) string {
	if len(unsafe) > 0 {
		return Escape(s, unsafe[0])
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == ' ':
			b.WriteByte('+')
		case isWWWUnreserved(c):
			b.WriteByte(c)
		default:
			b.WriteByte('%')
			b.WriteByte(hexUpper[c>>4])
			b.WriteByte(hexUpper[c&0x0f])
		}
	}
	return b.String()
}

// Decode reverses Encode / URI.decode_www_form_component: '+' becomes a space
// and "%XX" sequences are decoded. A stray '%' not followed by two hex digits
// is left verbatim (matching MRI's lenient component decoder).
func Decode(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c == '+':
			b.WriteByte(' ')
		case c == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]):
			b.WriteByte(unhex(s[i+1])<<4 | unhex(s[i+2]))
			i += 2
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

// Escape implements the legacy URI.escape / DEFAULT_PARSER.escape: it %-escapes
// every byte that is *not* allowed, where the allowed set is "unreserved or
// reserved" minus the characters matched by the unsafe pattern. With no unsafe
// argument MRI escapes everything outside the default safe set (alphanumerics,
// "-_.!~*'()" and the reserved/path delimiters); to keep behavior deterministic
// and dependency-free, an explicit unsafe string lists the characters to escape.
func Escape(s string, unsafe string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if strings.IndexByte(unsafe, c) >= 0 {
			b.WriteByte('%')
			b.WriteByte(hexUpper[c>>4])
			b.WriteByte(hexUpper[c&0x0f])
		} else {
			b.WriteByte(c)
		}
	}
	return b.String()
}

// EscapeDefault is DEFAULT_PARSER.escape with no second argument: it %-escapes
// every byte outside the default "safe" set, which is the unreserved set plus
// the reserved path/query delimiters MRI leaves intact ("/?:@!$&'()*+,;=~").
func EscapeDefault(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if isEscapeSafe(c) {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(hexUpper[c>>4])
			b.WriteByte(hexUpper[c&0x0f])
		}
	}
	return b.String()
}

// Unescape reverses Escape / EscapeDefault: it decodes "%XX" sequences but,
// unlike Decode, does not treat '+' as a space (URI.unescape leaves '+' alone).
func Unescape(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '%' && i+2 < len(s) && isHex(s[i+1]) && isHex(s[i+2]) {
			b.WriteByte(unhex(s[i+1])<<4 | unhex(s[i+2]))
			i += 2
		} else {
			b.WriteByte(s[i])
		}
	}
	return b.String()
}

// EncodeWWWForm renders pairs as an application/x-www-form-urlencoded body,
// matching URI.encode_www_form: each key and value is component-encoded and the
// pairs are joined with '&'.
func EncodeWWWForm(pairs [][2]string) string {
	parts := make([]string, len(pairs))
	for i, kv := range pairs {
		parts[i] = Encode(kv[0]) + "=" + Encode(kv[1])
	}
	return strings.Join(parts, "&")
}

// DecodeWWWForm parses an application/x-www-form-urlencoded body into key/value
// pairs, matching URI.decode_www_form. The separator is '&' (MRI 4.0's
// default); each side is component-decoded. An empty input yields no pairs. A
// pair with no '=' decodes to a value of "".
func DecodeWWWForm(s string) ([][2]string, error) {
	if s == "" {
		return [][2]string{}, nil
	}
	out := [][2]string{}
	for _, field := range strings.Split(s, "&") {
		if field == "" {
			continue
		}
		k, v, _ := strings.Cut(field, "=")
		out = append(out, [2]string{Decode(k), Decode(v)})
	}
	return out, nil
}

// ---- character classes (encoding) -----------------------------------------

// isWWWUnreserved reports whether c is left verbatim by
// encode_www_form_component: alphanumerics and "*-._".
func isWWWUnreserved(c byte) bool {
	return isAlpha(c) || isDigit(c) || c == '*' || c == '-' || c == '.' || c == '_'
}

// isEscapeSafe reports whether c is left verbatim by EscapeDefault: the
// unreserved set plus the reserved delimiters MRI's default escape preserves.
func isEscapeSafe(c byte) bool {
	if isAlpha(c) || isDigit(c) {
		return true
	}
	return strings.IndexByte("-_.!~*'()/?:@!$&'()*+,;=", c) >= 0
}

func isHex(c byte) bool {
	return isDigit(c) || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F'
}

func unhex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

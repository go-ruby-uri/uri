// Package uri is a pure-Go, CGO-free port of Ruby's "uri" standard library,
// matching the behavior of MRI (CRuby) 4.0.x byte-for-byte across parsing,
// component access, reference resolution, normalization and percent-encoding.
//
// It mirrors the semantics implemented by rbgo's prelude URI module (the
// generic component model plus the scheme registry, RFC 3986 reference
// resolution, www-form encoding and the InvalidURIError/InvalidComponentError
// taxonomy) and is the library rbgo binds for its `require "uri"`.
//
// Unlike net/url, this package reproduces Ruby's exact decomposition: the
// 9-element URI.split tuple, opaque vs. hierarchical paths, default-port
// elision in to_s, www-form '+'-for-space encoding and MRI's error messages.
package uri

import (
	"sort"
	"strconv"
	"strings"
)

// URI is the parsed, idiomatic-Go representation of a Ruby URI::Generic (or a
// scheme-specific subclass such as URI::HTTP). It is the value rbgo wraps in a
// URI::Generic object.
//
// A URI is either hierarchical (it has a Path, optionally an authority) or
// opaque (Opaque is set and Path is empty), exactly as in MRI: a URI with a
// scheme whose remainder neither begins with "//" nor "/" is opaque
// (e.g. "mailto:foo@bar.com", "urn:isbn:1").
type URI struct {
	Scheme   string
	Userinfo string
	Host     string
	Port     int
	HasPort  bool // distinguishes an absent port from port 0
	Path     string
	Query    string
	HasQuery bool // distinguishes an absent query ("a") from an empty one ("a?")
	Fragment string
	HasFrag  bool // distinguishes an absent fragment from an empty one ("a#")
	Opaque   string

	// FTP-specific (RFC 1738): IsFTP marks an ftp:// URI whose Path has had its
	// leading slash stripped, and Typecode holds an optional ";type=X" suffix.
	IsFTP    bool
	Typecode string
}

// DefaultPorts maps a scheme to its well-known port, matching
// URI::Generic::DEFAULT_PORTS in MRI. A scheme absent from this map has no
// default port and its explicit port (if any) is always rendered.
var DefaultPorts = map[string]int{
	"http":   80,
	"https":  443,
	"ftp":    21,
	"ldap":   389,
	"ldaps":  636,
	"ws":     80,
	"wss":    443,
	"ssh":    22,
	"telnet": 23,
	"nntp":   119,
}

// DefaultPort returns the well-known port for u's scheme and whether one is
// defined, mirroring URI::Generic#default_port.
func (u *URI) DefaultPort() (int, bool) {
	p, ok := DefaultPorts[strings.ToLower(u.Scheme)]
	return p, ok
}

// EffectivePort returns the port MRI's URI#port getter would report: the
// explicit port if set, otherwise the scheme's default port, and whether any
// port applies. rbgo uses this for the .port reader.
func (u *URI) EffectivePort() (int, bool) {
	if u.HasPort {
		return u.Port, true
	}
	return u.DefaultPort()
}

// renderPort reports whether String should emit u's port: only when an explicit
// port is set and it differs from the scheme default, matching MRI's to_s
// (which omits @port when it equals default_port).
func (u *URI) renderPort() bool {
	if !u.HasPort {
		return false
	}
	p, ok := u.DefaultPort()
	return !(ok && p == u.Port)
}

// Parse decomposes a URI string into a *URI, matching URI.parse / URI(). A
// string that cannot be matched against the URI grammar yields an
// InvalidURIError, as does a port that is not all digits.
func Parse(s string) (*URI, error) {
	parts, err := splitRaw(s)
	if err != nil {
		return nil, err
	}
	u := &URI{}
	u.Scheme = parts[0]
	u.Userinfo = parts[1]
	u.Host = parts[2]
	if parts[3] != "" {
		// Split already validated the port is all digits.
		n, _ := strconv.Atoi(parts[3])
		u.Port = n
		u.HasPort = true
	}
	// parts[4] (registry) is unused by the generic grammar (always "").
	u.Opaque = parts[6]
	if u.Opaque == "" {
		u.Path = parts[5]
	}
	if parts[7] != noComponent {
		u.Query = parts[7]
		u.HasQuery = true
	}
	if parts[8] != noComponent {
		u.Fragment = parts[8]
		u.HasFrag = true
	}
	if strings.EqualFold(u.Scheme, "ftp") {
		u.applyFTP()
	}
	return u, nil
}

// applyFTP applies the FTP-scheme parsing rules (RFC 1738 §3.2): the leading
// slash of the path is dropped, and a trailing ";type=X" is split off into
// Typecode. URI::FTP#to_s reconstructs both, so String round-trips.
func (u *URI) applyFTP() {
	u.IsFTP = true
	u.Path = strings.TrimPrefix(u.Path, "/")
	if i := strings.LastIndex(u.Path, ";type="); i >= 0 {
		u.Typecode = u.Path[i+len(";type="):]
		u.Path = u.Path[:i]
	}
}

// MustParse is Parse without the error return; it panics on an invalid URI. It
// is a convenience for tests and constants.
func MustParse(s string) *URI {
	u, err := Parse(s)
	if err != nil {
		panic(err)
	}
	return u
}

// noComponent is a sentinel marking an absent (vs. empty) query/fragment in the
// Split tuple, so "a?" (empty query) is distinguishable from "a" (no query).
const noComponent = "\x00"

// Split decomposes a URI string into the 9-element tuple MRI's URI.split
// returns: [scheme, userinfo, host, port, registry, path, opaque, query,
// fragment]. Absent components are the empty string, except that a missing port
// is "" and an absent query/fragment is also "" in the returned array (MRI uses
// nil); callers needing the absent/empty distinction should use Parse.
//
// The element order and opaque-vs-path decision reproduce MRI exactly: a URI
// with a scheme whose remainder neither begins with "//" nor "/" is opaque.
func Split(s string) ([9]string, error) {
	out, err := splitRaw(s)
	if err != nil {
		return out, err
	}
	// MRI's tuple uses "" for an absent query/fragment; map the internal
	// presence sentinel back to "" for the public array.
	if out[7] == noComponent {
		out[7] = ""
	}
	if out[8] == noComponent {
		out[8] = ""
	}
	return out, nil
}

// splitRaw is Split's worker, preserving the noComponent sentinel in the
// query/fragment slots so Parse can tell an absent component from an empty one.
func splitRaw(s string) ([9]string, error) {
	var out [9]string
	// MRI's RFC3986 grammar admits no raw control characters or spaces; such a
	// string is not a URI.
	for i := 0; i < len(s); i++ {
		if c := s[i]; c < 0x20 || c == 0x7f || c == ' ' {
			return out, &InvalidURIError{s}
		}
	}
	rest := s

	out[7] = noComponent
	out[8] = noComponent

	// fragment (split off first, on the whole string)
	if i := strings.IndexByte(rest, '#'); i >= 0 {
		out[8] = rest[i+1:]
		rest = rest[:i]
	}
	// scheme: ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ) ":"
	if i := schemeEnd(rest); i > 0 {
		out[0] = rest[:i]
		rest = rest[i+1:]
	}

	if strings.HasPrefix(rest, "//") {
		// authority + hier-part. The query is split off the remainder.
		body, query, hasQuery := cutQuery(rest)
		if hasQuery {
			out[7] = query
		}
		auth := body[2:]
		if j := strings.IndexAny(auth, "/?#"); j >= 0 {
			body = auth[j:]
			auth = auth[:j]
		} else {
			body = ""
		}
		if at := strings.LastIndexByte(auth, '@'); at >= 0 {
			out[1] = auth[:at]
			auth = auth[at+1:]
		}
		host, port := splitHostPort(auth)
		out[2] = host
		out[3] = port
		if port != "" && !allDigits(port) {
			return out, &InvalidURIError{s}
		}
		out[5] = body
	} else if out[0] != "" && !strings.HasPrefix(rest, "/") {
		// scheme present, remainder neither "//..." nor "/...": opaque. In
		// RFC3986 the query is part of the opaque component (MRI appends it),
		// so it is NOT split off here.
		out[6] = rest
	} else {
		// hierarchical (absolute or relative path); split off the query.
		body, query, hasQuery := cutQuery(rest)
		if hasQuery {
			out[7] = query
		}
		out[5] = body
	}

	return out, nil
}

// cutQuery splits a "...?query" suffix off s, reporting whether a query was
// present (so an empty query "a?" is distinguishable from no query "a").
func cutQuery(s string) (body, query string, present bool) {
	if i := strings.IndexByte(s, '?'); i >= 0 {
		return s[:i], s[i+1:], true
	}
	return s, "", false
}

// schemeEnd returns the index of the ':' terminating a leading scheme in s, or
// 0 if s does not begin with a valid scheme. A valid scheme is
// ALPHA *( ALPHA / DIGIT / "+" / "-" / "." ).
func schemeEnd(s string) int {
	if s == "" || !isAlpha(s[0]) {
		return 0
	}
	for i := 1; i < len(s); i++ {
		c := s[i]
		if c == ':' {
			return i
		}
		if !isSchemeChar(c) {
			return 0
		}
	}
	return 0
}

// splitHostPort separates an authority's host from its port, keeping a
// bracketed IPv6 literal ("[::1]") intact and only treating a trailing ":port"
// after the closing bracket as the port.
func splitHostPort(auth string) (host, port string) {
	if strings.HasPrefix(auth, "[") {
		if end := strings.IndexByte(auth, ']'); end >= 0 {
			host = auth[:end+1]
			rest := auth[end+1:]
			if strings.HasPrefix(rest, ":") {
				port = rest[1:]
			}
			return host, port
		}
	}
	if i := strings.LastIndexByte(auth, ':'); i >= 0 {
		return auth[:i], auth[i+1:]
	}
	return auth, ""
}

// String renders u back to its URI string, round-tripping a parsed URI exactly
// as MRI's URI#to_s does: the scheme default port is elided, an opaque URI
// renders "scheme:opaque", and empty-but-present query/fragment ("a?", "a#")
// are preserved.
func (u *URI) String() string {
	var b strings.Builder
	if u.Scheme != "" {
		b.WriteString(u.Scheme)
		b.WriteByte(':')
	}
	if u.Opaque != "" {
		b.WriteString(u.Opaque)
	} else {
		// MRI emits "//" when there is a host, a userinfo, or the scheme is one
		// that always carries an authority ("file", "postgres").
		if u.Host != "" || u.Userinfo != "" || alwaysAuthority(u.Scheme) {
			b.WriteString("//")
		}
		if u.Userinfo != "" {
			b.WriteString(u.Userinfo)
			b.WriteByte('@')
		}
		b.WriteString(u.Host)
		if u.renderPort() {
			b.WriteByte(':')
			b.WriteString(strconv.Itoa(u.Port))
		}
		// Insert a separating "/" before a non-absolute path that follows an
		// authority (RFC mandates abs_path after authority).
		if u.IsFTP {
			// URI::FTP#to_s always re-roots the (leading-slash-stripped) path.
			b.WriteByte('/')
			b.WriteString(u.Path)
			if u.Typecode != "" {
				b.WriteString(";type=")
				b.WriteString(u.Typecode)
			}
		} else {
			if (u.Host != "" || u.HasPort) && u.Path != "" && !strings.HasPrefix(u.Path, "/") {
				b.WriteByte('/')
			}
			b.WriteString(u.Path)
		}
	}
	if u.HasQuery {
		b.WriteByte('?')
		b.WriteString(u.Query)
	}
	if u.HasFrag {
		b.WriteByte('#')
		b.WriteString(u.Fragment)
	}
	return b.String()
}

// alwaysAuthority reports whether scheme always renders a "//" authority marker
// in to_s even with an empty host, matching MRI's special-casing of "file" and
// "postgres".
func alwaysAuthority(scheme string) bool {
	s := strings.ToLower(scheme)
	return s == "file" || s == "postgres"
}

// Hostname returns u's host with surrounding brackets stripped from an IPv6
// literal, matching URI::Generic#hostname.
func (u *URI) Hostname() string {
	h := u.Host
	if strings.HasPrefix(h, "[") && strings.HasSuffix(h, "]") {
		return h[1 : len(h)-1]
	}
	return h
}

// Absolute reports whether u has a scheme (URI#absolute?). Relative is its
// negation (URI#relative?).
func (u *URI) Absolute() bool { return u.Scheme != "" }

// Relative reports whether u lacks a scheme (URI#relative?).
func (u *URI) Relative() bool { return u.Scheme == "" }

// clone returns a shallow copy of u (URI values hold no reference types).
func (u *URI) clone() *URI {
	c := *u
	return &c
}

// Join resolves each successive reference against the running base, matching
// URI.join(base, rels...).
func Join(base string, rels ...string) (*URI, error) {
	result, err := Parse(base)
	if err != nil {
		return nil, err
	}
	for _, r := range rels {
		result, err = result.Merge(r)
		if err != nil {
			return nil, err
		}
	}
	return result, nil
}

// Merge resolves the relative reference rel against u using RFC 3986 reference
// resolution, matching URI::Generic#merge / the + operator. If rel is absolute
// (has a scheme) it is returned as-is.
func (u *URI) Merge(rel string) (*URI, error) {
	r, err := Parse(rel)
	if err != nil {
		return nil, err
	}
	return u.mergeURI(r)
}

// mergeURI ports MRI's URI::Generic#merge against the (already parsed)
// reference r, following RFC 2396 §5.2 as MRI implements it.
func (u *URI) mergeURI(r *URI) (*URI, error) {
	if r.Absolute() {
		return r.clone(), nil // both absolute: MRI returns the reference.
	}
	if u.Relative() {
		return nil, &BadURIError{"both URI are relative"}
	}
	base := u.clone()
	hasAuthority := r.Host != "" || r.Userinfo != ""

	// RFC2396 §5.2 (2): empty path, no authority, no query -> only the
	// fragment is carried over.
	if r.Path == "" && r.Opaque == "" && !hasAuthority && !r.HasQuery {
		if r.HasFrag {
			base.setFrag(r)
		}
		return base, nil
	}

	base.HasQuery, base.Query = false, ""
	base.HasFrag, base.Fragment = false, ""

	if hasAuthority {
		base.Userinfo = r.Userinfo
		base.Host = r.Host
		base.Port = r.Port
		base.HasPort = r.HasPort
		base.Path = r.Path
	} else {
		base.Path = mergePath(u.Path, r.Path)
	}
	if r.HasQuery {
		base.setQuery(r)
	}
	if r.HasFrag {
		base.setFrag(r)
	}
	return base, nil
}

func (u *URI) setQuery(r *URI) { u.Query, u.HasQuery = r.Query, r.HasQuery }
func (u *URI) setFrag(r *URI)  { u.Fragment, u.HasFrag = r.Fragment, r.HasFrag }

// mergePath ports MRI's URI::Generic#merge_path (RFC 2396 §5.2 step 6),
// resolving rel against base by segment with "."/".." collapsing.
func mergePath(base, rel string) string {
	basePath := splitPath(base)
	relPath := splitPath(rel)

	// §5.2 6) a)
	if len(basePath) > 0 && basePath[len(basePath)-1] == ".." {
		basePath = append(basePath, "")
	}
	for {
		i := indexOf(basePath, "..")
		if i < 0 {
			break
		}
		basePath = append(basePath[:i-1], basePath[i+1:]...)
	}

	if len(relPath) > 0 && relPath[0] == "" {
		basePath = basePath[:0]
		relPath = relPath[1:]
	}

	// §5.2 6) c) d)
	if n := len(relPath); n > 0 && (relPath[n-1] == "." || relPath[n-1] == "..") {
		relPath = append(relPath, "")
	}
	relPath = deleteAll(relPath, ".")

	// §5.2 6) e)
	tmp := []string{}
	for _, x := range relPath {
		if x == ".." && !(len(tmp) == 0 || tmp[len(tmp)-1] == "..") {
			tmp = tmp[:len(tmp)-1]
		} else {
			tmp = append(tmp, x)
		}
	}

	addTrailerSlash := len(tmp) != 0
	if len(basePath) == 0 {
		basePath = []string{""} // keep '/' for root directory
	} else if addTrailerSlash {
		basePath = basePath[:len(basePath)-1]
	}
	for len(tmp) > 0 {
		x := tmp[0]
		tmp = tmp[1:]
		if x == ".." {
			if len(basePath) > 1 {
				basePath = basePath[:len(basePath)-1]
			}
		} else {
			basePath = append(basePath, x)
			basePath = append(basePath, tmp...)
			break
		}
	}
	// MRI re-appends a trailing slash here when the relative tail was entirely
	// "..", but the §5.2 6) c/d rule guarantees a non-".." final segment in that
	// case, so the loop always terminates through the else branch above; the
	// re-append is therefore unreachable and omitted.
	return strings.Join(basePath, "/")
}

// splitPath reproduces Ruby's path.split("/", -1): a leading empty field is
// kept, but an empty path yields an empty slice (not [""]).
func splitPath(p string) []string {
	if p == "" {
		return []string{}
	}
	return strings.Split(p, "/")
}

// indexOf returns the index of the first occurrence of v in s, or -1.
func indexOf(s []string, v string) int {
	for i, x := range s {
		if x == v {
			return i
		}
	}
	return -1
}

// deleteAll returns s with every element equal to v removed (Array#delete).
func deleteAll(s []string, v string) []string {
	out := s[:0:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

// Normalize returns a copy of u with the scheme and host lowercased and an
// empty hierarchical path set to "/", matching URI::Generic#normalize. Opaque
// URIs are returned with only the scheme lowercased.
func (u *URI) Normalize() *URI {
	out := u.clone()
	out.Scheme = strings.ToLower(out.Scheme)
	if out.Opaque != "" {
		return out
	}
	out.Host = strings.ToLower(out.Host)
	if (out.Host != "" || out.Userinfo != "") && out.Path == "" {
		out.Path = "/"
	}
	return out
}

// RouteTo returns the relative reference that, resolved against u, yields dst:
// the inverse of Merge, matching URI::Generic#route_to. dst is given as a
// string and the result is the shortest relative URI MRI would produce.
func (u *URI) RouteTo(dst string) (*URI, error) {
	d, err := Parse(dst)
	if err != nil {
		return nil, err
	}
	// route_to(dst) == dst.route_from(u): route_from0 requires both absolute,
	// checking self (dst) before oth (u).
	if d.Relative() {
		return nil, &BadURIError{"relative URI: " + d.String()}
	}
	if u.Relative() {
		return nil, &BadURIError{"relative URI: " + u.String()}
	}
	return u.routeTo(d), nil
}

// routeTo computes d.route_from(u): the relative reference yielding d when
// resolved against u. It follows MRI's route_from0 / route_from_path exactly.
func (u *URI) routeTo(d *URI) *URI {
	// route_from0(oth=u) on self=d.
	if !strings.EqualFold(d.Scheme, u.Scheme) {
		return d.clone() // different scheme: dst is the answer.
	}
	// rel starts as a relativized copy of self (d) without scheme.
	rel := d.clone()
	rel.Scheme = ""

	// Compare authority (host case-insensitive, port via effective default).
	if rel.Userinfo != u.Userinfo ||
		!strings.EqualFold(rel.Host, u.Host) ||
		!samePort(rel, u, d) {
		// Authority differs. If self had no userinfo and no host (relative
		// base), the whole self is the answer; otherwise keep the authority.
		if d.Userinfo == "" && d.Host == "" {
			return d.clone()
		}
		// Drop a redundant explicit default port.
		if rel.HasPort {
			if dp, ok := d.DefaultPort(); ok && rel.Port == dp {
				rel.HasPort = false
			}
		}
		return rel
	}

	// Same authority: strip it.
	rel.Userinfo, rel.Host, rel.HasPort, rel.Port = "", "", false, 0

	if rel.Opaque == "" && rel.Path == u.Path {
		rel.Path = ""
		if rel.HasQuery && d.Query == u.Query && u.HasQuery {
			rel.HasQuery, rel.Query = false, ""
		}
		return rel
	}
	if rel.Opaque != "" && rel.Opaque == u.Opaque {
		// An opaque component subsumes its query (RFC3986: the query is part of
		// path-rootless), so HasQuery is always false here and no query strip is
		// possible — unlike the hierarchical same-path case above.
		rel.Opaque = ""
		return rel
	}

	rel.Path = routeFromPath(u.Path, d.Path)
	if rel.Path == "./" && d.HasQuery {
		rel.Path = ""
	}
	return rel
}

// samePort reports whether d's and u's effective ports match for authority
// comparison, falling back to d's scheme default when a port is absent.
func samePort(rel, u, d *URI) bool {
	dp, _ := d.DefaultPort()
	pa := rel.Port
	if !rel.HasPort {
		pa = dp
	}
	pb := u.Port
	if !u.HasPort {
		pb = dp
	}
	return pa == pb
}

// routeFromPath ports MRI's route_from_path(src, dst): the relative path from
// src's directory to dst, using "../" hops, matching URI::Generic exactly.
func routeFromPath(src, dst string) string {
	if dst == src {
		return ""
	}
	// Abnormal absolute path in dst ("/./", "/../", "/x/../", ...) -> verbatim.
	if hasDotSegment(dst) {
		return dst
	}
	srcPath := scanDirs(src) // [^/]*/  (directory segments only)
	dstPath := scanSegs(dst) // [^/]*/? (all segments)
	for len(dstPath) > 0 && len(srcPath) > 0 && dstPath[0] == srcPath[0] {
		srcPath = srcPath[1:]
		dstPath = dstPath[1:]
	}
	tmp := strings.Join(dstPath, "")
	if len(srcPath) == 0 {
		if tmp == "" {
			return "./"
		}
		if len(dstPath) > 0 && strings.Contains(dstPath[0], ":") {
			return "./" + tmp
		}
		return tmp
	}
	return strings.Repeat("../", len(srcPath)) + tmp
}

// hasDotSegment reports whether path contains a "." or ".." segment, matching
// MRI's %r{(?:\A|/)\.\.?(?:/|\z)} test.
func hasDotSegment(path string) bool {
	for _, seg := range strings.Split(path, "/") {
		if seg == "." || seg == ".." {
			return true
		}
	}
	return false
}

// scanDirs reproduces src.scan(%r{[^/]*/}): every maximal run of non-slash
// characters followed by a slash, i.e. the directory segments (the trailing
// file segment is dropped).
func scanDirs(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			out = append(out, s[start:i+1])
			start = i + 1
		}
	}
	return out
}

// scanSegs reproduces dst.scan(%r{[^/]*/?}): every segment, each including its
// trailing slash if present. The final non-slash segment is included; an empty
// trailing match (after a final slash) is not.
func scanSegs(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '/' {
			out = append(out, s[start:i+1])
			start = i + 1
		}
	}
	if start < len(s) {
		out = append(out, s[start:])
	}
	return out
}

// Build constructs a URI from explicit components, matching URI::Generic.build
// with a component slice in [scheme, userinfo, host, port, path, query,
// fragment] order. A blank component is treated as absent.
func Build(scheme, userinfo, host string, port int, hasPort bool, path, query string, hasQuery bool, fragment string, hasFrag bool) *URI {
	return &URI{
		Scheme: scheme, Userinfo: userinfo, Host: host,
		Port: port, HasPort: hasPort, Path: path,
		Query: query, HasQuery: hasQuery,
		Fragment: fragment, HasFrag: hasFrag,
	}
}

// ---- character classes -----------------------------------------------------

func isAlpha(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

func isDigit(c byte) bool { return c >= '0' && c <= '9' }

func isSchemeChar(c byte) bool {
	return isAlpha(c) || isDigit(c) || c == '+' || c == '-' || c == '.'
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

// sortedKeys is a tiny helper used by tests to iterate DefaultPorts
// deterministically; kept here so the production file stays the only owner of
// DefaultPorts.
func sortedKeys(m map[string]int) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	return ks
}

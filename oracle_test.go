package uri

import (
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// The oracle tests run a local MRI ruby as a differential reference: they feed
// the same inputs to ruby's `uri` stdlib and to this package and require
// byte-identical results. They skip themselves on Windows and where ruby is
// absent or older than 4.0, so the deterministic golden tests in uri_test.go
// (which alone keep coverage at 100%) carry the no-ruby, qemu and Windows lanes.

// rubyOracle returns a function that evaluates a Ruby expression with `uri`
// required and stdout in binary mode, or skips the test if no suitable ruby is
// available. The MRI version gate matches the rbgo target (>= 4.0).
func rubyOracle(t *testing.T) func(expr string) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("oracle: skipped on Windows")
	}
	path, err := exec.LookPath("ruby")
	if err != nil {
		t.Skip("oracle: ruby not found")
	}
	ver, err := exec.Command(path, "-e", "$stdout.binmode; print RUBY_VERSION").Output()
	if err != nil {
		t.Skipf("oracle: cannot run ruby: %v", err)
	}
	if !rubyAtLeast(string(ver), 4, 0) {
		t.Skipf("oracle: ruby %s < 4.0", strings.TrimSpace(string(ver)))
	}
	return func(expr string) string {
		out, err := exec.Command(path, "-ruri", "-e", "$stdout.binmode; STDIN.binmode; "+expr).Output()
		if err != nil {
			t.Fatalf("oracle: ruby error for %q: %v", expr, err)
		}
		return string(out)
	}
}

// rubyAtLeast reports whether the "X.Y.Z" version string is at least major.minor.
func rubyAtLeast(v string, major, minor int) bool {
	parts := strings.SplitN(strings.TrimSpace(v), ".", 3)
	if len(parts) < 2 {
		return false
	}
	maj, err1 := strconv.Atoi(parts[0])
	min, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return false
	}
	return maj > major || (maj == major && min >= minor)
}

var oracleURIs = []string{
	"http://h/x", "http://a:b@h:9/p?q#f", "https://e.com:443/a?c#e",
	"mailto:foo@bar.com", "urn:isbn:0451450523", "ftp://h/x", "ldap://h:389/x",
	"//h/p", "/p?q#f", "?q", "#f", "a/b/c", "http://h", "http:foo", "http:/foo",
	"http://[::1]:8080/x", "http://[2001:db8::1]/", "http://h:/p",
	"http://h/a%20b", "http://user@host/", "scheme:opaque?x#y", "",
	"http://h/a/b/", "file:///etc/hosts", "ws://h/socket", "g:h",
}

func TestOracleParseToS(t *testing.T) {
	ruby := rubyOracle(t)
	for _, in := range oracleURIs {
		want := ruby("print URI.parse(" + rbString(in) + ").to_s")
		u, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q) error: %v", in, err)
			continue
		}
		if got := u.String(); got != want {
			t.Errorf("to_s mismatch for %q: go=%q ruby=%q", in, got, want)
		}
	}
}

func TestOracleComponents(t *testing.T) {
	ruby := rubyOracle(t)
	get := func(in, comp string) string {
		return ruby("v=URI.parse(" + rbString(in) + ")." + comp + "; print(v.nil? ? \"\\x00\" : v.to_s)")
	}
	for _, in := range oracleURIs {
		u, err := Parse(in)
		if err != nil {
			continue
		}
		check := func(comp, got string) {
			want := get(in, comp)
			if want == "\x00" {
				want = "" // ruby nil maps to our empty string
			}
			if got != want {
				t.Errorf("%s mismatch for %q: go=%q ruby=%q", comp, in, got, want)
			}
		}
		check("scheme", u.Scheme)
		check("userinfo", u.Userinfo)
		check("host", u.Host)
		check("path", u.Path)
		check("query", u.Query)
		check("fragment", u.Fragment)
		check("opaque", u.Opaque)
	}
}

func TestOracleMerge(t *testing.T) {
	ruby := rubyOracle(t)
	pairs := [][2]string{
		{"http://h/a/b/c", "/x"}, {"http://h/a/b/c", "x"}, {"http://h/a/b/c", "../x"},
		{"http://h/a/b/c", "./x"}, {"http://h/a/b?z", "?q"}, {"http://h/a/b", "#f"},
		{"http://h/a", "http://h2/b"}, {"http://h/a/b", ""}, {"http://h/a", "../../x"},
		{"http://h/a", "//h2/b"}, {"http://h/", "x/y"}, {"http://h", "/p"},
		{"http://h/a/b/", "."}, {"http://h/a/b/", ".."}, {"http://h/a/b/c", "g/h/../i"},
	}
	for _, p := range pairs {
		want := ruby("print URI.parse(" + rbString(p[0]) + ").merge(" + rbString(p[1]) + ").to_s")
		m, err := MustParse(p[0]).Merge(p[1])
		if err != nil {
			t.Errorf("Merge(%q,%q) error: %v", p[0], p[1], err)
			continue
		}
		if got := m.String(); got != want {
			t.Errorf("merge mismatch for (%q,%q): go=%q ruby=%q", p[0], p[1], got, want)
		}
	}
}

func TestOracleRouteTo(t *testing.T) {
	ruby := rubyOracle(t)
	pairs := [][2]string{
		{"http://h/a/b/c", "http://h/a/b/d"}, {"http://h/a/b/c", "http://h/a/x/d"},
		{"http://h/a/b/c", "http://h/a/b/c"}, {"http://h/a/b/", "http://h/a/b/c"},
		{"http://h/a/b/c", "http://h/a/b/c?q"}, {"http://h/", "http://h/a"},
		{"http://h/a/b/c", "https://h/a"}, {"http://h/a/b/c", "http://h2/b"},
		{"http://h/a/b/c", "http://h/a/b/"}, {"http://h/a/b/c", "http://h/a/"},
	}
	for _, p := range pairs {
		want := ruby("print URI.parse(" + rbString(p[0]) + ").route_to(" + rbString(p[1]) + ").to_s")
		r, err := MustParse(p[0]).RouteTo(p[1])
		if err != nil {
			t.Errorf("RouteTo(%q,%q) error: %v", p[0], p[1], err)
			continue
		}
		if got := r.String(); got != want {
			t.Errorf("route_to mismatch for (%q,%q): go=%q ruby=%q", p[0], p[1], got, want)
		}
	}
}

func TestOracleWWWForm(t *testing.T) {
	ruby := rubyOracle(t)
	comps := []string{"a b&c=d", "~hello world*", "aZ09-_.", "100%", "a+b", "é", "x/y?z"}
	for _, c := range comps {
		want := ruby("print URI.encode_www_form_component(" + rbString(c) + ")")
		if got := Encode(c); got != want {
			t.Errorf("encode_www_form_component mismatch for %q: go=%q ruby=%q", c, got, want)
		}
		// decode round-trip via ruby.
		back := ruby("print URI.decode_www_form_component(" + rbString(want) + ")")
		if Decode(want) != back {
			t.Errorf("decode mismatch for %q: go=%q ruby=%q", want, Decode(want), back)
		}
	}
	form := ruby("print URI.encode_www_form([[\"a\",\"1\"],[\"b\",\"x y\"],[\"c\",\"&=\"]])")
	if got := EncodeWWWForm([][2]string{{"a", "1"}, {"b", "x y"}, {"c", "&="}}); got != form {
		t.Errorf("encode_www_form mismatch: go=%q ruby=%q", got, form)
	}
}

func TestOracleErrors(t *testing.T) {
	ruby := rubyOracle(t)
	bad := []string{"http://h:xx/", "http://h\n/", "http://h /"}
	for _, s := range bad {
		// Confirm ruby also rejects it.
		res := ruby("begin; URI.parse(" + rbString(s) + "); print \"OK\"; rescue URI::InvalidURIError; print \"ERR\"; end")
		if res != "ERR" {
			t.Skipf("ruby accepted %q (=%q); skipping", s, res)
		}
		if _, err := Parse(s); err == nil {
			t.Errorf("Parse(%q): expected error to match ruby", s)
		}
	}
}

// rbString renders s as a Ruby double-quoted string literal for embedding in an
// eval'd expression, escaping the characters that matter.
func rbString(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for i := 0; i < len(s); i++ {
		switch c := s[i]; c {
		case '"', '\\':
			b.WriteByte('\\')
			b.WriteByte(c)
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		case '\r':
			b.WriteString("\\r")
		case '#':
			b.WriteString("\\#") // prevent #{} interpolation
		default:
			b.WriteByte(c)
		}
	}
	b.WriteByte('"')
	return b.String()
}

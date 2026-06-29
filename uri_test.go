package uri

import (
	"errors"
	"reflect"
	"testing"
)

// These golden tables were captured from MRI (CRuby) 4.0.5's `uri` stdlib and
// are deterministic, so they hold 100% coverage on the no-ruby, qemu and
// Windows CI lanes. The live differential oracle in oracle_test.go re-derives
// them from a local ruby when one is present.

func TestParseGolden(t *testing.T) {
	type want struct {
		scheme, userinfo, host string
		port                   int
		hasPort                bool
		path, query            string
		hasQuery               bool
		fragment               string
		hasFrag                bool
		opaque                 string
		str                    string
		absolute               bool
	}
	cases := map[string]want{
		"http://h/x":              {scheme: "http", host: "h", path: "/x", str: "http://h/x", absolute: true},
		"http://a:b@h:9/p?q#f":    {scheme: "http", userinfo: "a:b", host: "h", port: 9, hasPort: true, path: "/p", query: "q", hasQuery: true, fragment: "f", hasFrag: true, str: "http://a:b@h:9/p?q#f", absolute: true},
		"https://e.com:443/a?c#e": {scheme: "https", host: "e.com", port: 443, hasPort: true, path: "/a", query: "c", hasQuery: true, fragment: "e", hasFrag: true, str: "https://e.com/a?c#e", absolute: true},
		"mailto:foo@bar.com":      {scheme: "mailto", opaque: "foo@bar.com", str: "mailto:foo@bar.com", absolute: true},
		"urn:isbn:0451450523":     {scheme: "urn", opaque: "isbn:0451450523", str: "urn:isbn:0451450523", absolute: true},
		"//h/p":                   {host: "h", path: "/p", str: "//h/p"},
		"/p?q#f":                  {path: "/p", query: "q", hasQuery: true, fragment: "f", hasFrag: true, str: "/p?q#f"},
		"?q":                      {query: "q", hasQuery: true, str: "?q"},
		"#f":                      {fragment: "f", hasFrag: true, str: "#f"},
		"a/b/c":                   {path: "a/b/c", str: "a/b/c"},
		"http://h":                {scheme: "http", host: "h", str: "http://h", absolute: true},
		"http:foo":                {scheme: "http", opaque: "foo", str: "http:foo", absolute: true},
		"http:/foo":               {scheme: "http", path: "/foo", str: "http:/foo", absolute: true},
		"http://[::1]:8080/x":     {scheme: "http", host: "[::1]", port: 8080, hasPort: true, path: "/x", str: "http://[::1]:8080/x", absolute: true},
		"http://[2001:db8::1]/":   {scheme: "http", host: "[2001:db8::1]", path: "/", str: "http://[2001:db8::1]/", absolute: true},
		"http://h:/p":             {scheme: "http", host: "h", path: "/p", str: "http://h/p", absolute: true},
		"http://user@host/":       {scheme: "http", userinfo: "user", host: "host", path: "/", str: "http://user@host/", absolute: true},
		"scheme:opaque?x#y":       {scheme: "scheme", opaque: "opaque?x", fragment: "y", hasFrag: true, str: "scheme:opaque?x#y", absolute: true},
		"":                        {str: ""},
		"http://h/a/b/":           {scheme: "http", host: "h", path: "/a/b/", str: "http://h/a/b/", absolute: true},
		"g:h":                     {scheme: "g", opaque: "h", str: "g:h", absolute: true},
		"file:///etc/hosts":       {scheme: "file", path: "/etc/hosts", str: "file:///etc/hosts", absolute: true},
		"http://h/a?":             {scheme: "http", host: "h", path: "/a", hasQuery: true, str: "http://h/a?", absolute: true},
		"http://h/a#":             {scheme: "http", host: "h", path: "/a", hasFrag: true, str: "http://h/a#", absolute: true},
	}
	for in, w := range cases {
		u, err := Parse(in)
		if err != nil {
			t.Fatalf("Parse(%q) error: %v", in, err)
		}
		got := want{u.Scheme, u.Userinfo, u.Host, u.Port, u.HasPort, u.Path, u.Query, u.HasQuery, u.Fragment, u.HasFrag, u.Opaque, u.String(), u.Absolute()}
		if got != w {
			t.Errorf("Parse(%q)\n got  %+v\n want %+v", in, got, w)
		}
		if u.Relative() == u.Absolute() {
			t.Errorf("Parse(%q): Relative()==Absolute()", in)
		}
		// Round-trip.
		if u.String() != w.str {
			t.Errorf("Parse(%q).String() = %q, want %q", in, u.String(), w.str)
		}
	}
}

func TestParseErrors(t *testing.T) {
	bad := []string{"http://h:xx/", "http://h\n/", "http://h /", "ht tp://x", "http://h\t/"}
	for _, s := range bad {
		_, err := Parse(s)
		if err == nil {
			t.Fatalf("Parse(%q) expected error", s)
		}
		var ie *InvalidURIError
		if !errors.As(err, &ie) {
			t.Errorf("Parse(%q): want *InvalidURIError, got %T", s, err)
		}
		var fam Error
		if !errors.As(err, &fam) {
			t.Errorf("Parse(%q): error not in URI Error family", s)
		}
		want := "bad URI (is not URI?): " + quoteForTest(s)
		if err.Error() != want {
			t.Errorf("Parse(%q) msg = %q, want %q", s, err.Error(), want)
		}
	}
}

func quoteForTest(s string) string {
	// Mirror strconv.Quote for the messages we assert.
	return (&InvalidURIError{s}).Error()[len("bad URI (is not URI?): "):]
}

func TestSplitGolden(t *testing.T) {
	cases := map[string][9]string{
		"http://a:b@h:9/p?q#f": {"http", "a:b", "h", "9", "", "/p", "", "q", "f"},
		"mailto:x@y.com":       {"mailto", "", "", "", "", "", "x@y.com", "", ""},
		"//host/path":          {"", "", "host", "", "", "/path", "", "", ""},
		"/just/path":           {"", "", "", "", "", "/just/path", "", "", ""},
		"http://h":             {"http", "", "h", "", "", "", "", "", ""},
		"http://[::1]:8/p":     {"http", "", "[::1]", "8", "", "/p", "", "", ""},
		"urn:isbn:12345":       {"urn", "", "", "", "", "", "isbn:12345", "", ""},
		"http:foo":             {"http", "", "", "", "", "", "foo", "", ""},
		"":                     {"", "", "", "", "", "", "", "", ""},
		"#frag":                {"", "", "", "", "", "", "", "", "frag"},
		"?q":                   {"", "", "", "", "", "", "", "q", ""},
		"http://h:/p":          {"http", "", "h", "", "", "/p", "", "", ""},
		"scheme:opaque?x#y":    {"scheme", "", "", "", "", "", "opaque?x", "", "y"},
	}
	for in, w := range cases {
		got, err := Split(in)
		if err != nil {
			t.Fatalf("Split(%q) error: %v", in, err)
		}
		if got != w {
			t.Errorf("Split(%q)\n got  %q\n want %q", in, got, w)
		}
	}
	if _, err := Split("http://h:xx/"); err == nil {
		t.Error("Split bad port: expected error")
	}
}

func TestMergeGolden(t *testing.T) {
	cases := [][3]string{
		{"http://h/a/b/c", "/x", "http://h/x"},
		{"http://h/a/b/c", "x", "http://h/a/b/x"},
		{"http://h/a/b/c", "../x", "http://h/a/x"},
		{"http://h/a/b/c", "./x", "http://h/a/b/x"},
		{"http://h/a/b?z", "?q", "http://h/a/b?q"},
		{"http://h/a/b", "#f", "http://h/a/b#f"},
		{"http://h/a", "http://h2/b", "http://h2/b"},
		{"http://h/a/b", "", "http://h/a/b"},
		{"http://h/a", "../../x", "http://h/x"},
		{"http://h/a", "//h2/b", "http://h2/b"},
		{"http://h/a/b/c/", "../../d", "http://h/a/d"},
		{"http://h/", "x/y", "http://h/x/y"},
		{"http://h", "/p", "http://h/p"},
		{"http://h/a/b/", ".", "http://h/a/b/"},
		{"http://h/a/b/", "..", "http://h/a/"},
		{"http://example.com/foo/bar", "baz", "http://example.com/foo/baz"},
		{"http://h/a/b/c", "g/h/../i", "http://h/a/b/g/i"},
	}
	for _, c := range cases {
		u, err := Parse(c[0])
		if err != nil {
			t.Fatal(err)
		}
		m, err := u.Merge(c[1])
		if err != nil {
			t.Fatal(err)
		}
		if got := m.String(); got != c[2] {
			t.Errorf("Parse(%q).Merge(%q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

func TestJoin(t *testing.T) {
	u, err := Join("http://h/a/b/c", "d", "e")
	if err != nil {
		t.Fatal(err)
	}
	if got := u.String(); got != "http://h/a/b/e" {
		t.Errorf("Join = %q, want http://h/a/b/e", got)
	}
	if _, err := Join("http://h/a", "bad uri"); err == nil {
		t.Error("Join with bad reference: expected error")
	}
	if _, err := Join("http://h\n/"); err == nil {
		t.Error("Join with bad base: expected error")
	}
}

func TestRouteToGolden(t *testing.T) {
	cases := [][3]string{
		{"http://h/a/b/c", "http://h/a/b/d", "d"},
		{"http://h/a/b/c", "http://h/a/x/d", "../x/d"},
		{"http://h/a/b/c", "http://h/a/b/c", ""},
		{"http://h/a/b/", "http://h/a/b/c", "c"},
		{"http://h/a/b/c", "http://h/a/b/c?q", "?q"},
		{"http://h/a/b/c", "http://h/a/b/c#f", "#f"},
		{"http://h/", "http://h/a", "a"},
		{"http://h/a/b/c", "https://h/a", "https://h/a"},
		{"http://h/a/b/c", "http://h2/b", "//h2/b"},
		{"http://h/a/b/c", "http://h/a/b/", "./"},
		{"http://h/a/b/c", "http://h/a/", "../"},
		{"http://h/a/b/c", "http://h/x/y/z", "../../x/y/z"},
	}
	for _, c := range cases {
		u, err := Parse(c[0])
		if err != nil {
			t.Fatal(err)
		}
		r, err := u.RouteTo(c[1])
		if err != nil {
			t.Fatal(err)
		}
		if got := r.String(); got != c[2] {
			t.Errorf("Parse(%q).RouteTo(%q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
	if _, err := MustParse("http://h/a").RouteTo("bad uri"); err == nil {
		t.Error("RouteTo bad dst: expected error")
	}
}

func TestNormalize(t *testing.T) {
	cases := [][2]string{
		{"HTTP://Example.COM/a", "http://example.com/a"},
		{"http://h", "http://h/"},
		{"http://H/", "http://h/"},
		{"MAILTO:Foo@Bar", "mailto:Foo@Bar"},
		{"/a/b", "/a/b"},
	}
	for _, c := range cases {
		if got := MustParse(c[0]).Normalize().String(); got != c[1] {
			t.Errorf("Normalize(%q) = %q, want %q", c[0], got, c[1])
		}
	}
}

func TestEncodeDecode(t *testing.T) {
	type c struct{ in, enc string }
	cases := []c{
		{"a b&c=d", "a+b%26c%3Dd"},
		{"~hello world*", "%7Ehello+world*"},
		{"aZ09-_.", "aZ09-_."},
		{"100%", "100%25"},
		{"a+b", "a%2Bb"},
		{"é", "%C3%A9"},
		{"x/y?z", "x%2Fy%3Fz"},
	}
	for _, tc := range cases {
		if got := Encode(tc.in); got != tc.enc {
			t.Errorf("Encode(%q) = %q, want %q", tc.in, got, tc.enc)
		}
		if got := Decode(tc.enc); got != tc.in {
			t.Errorf("Decode(%q) = %q, want %q", tc.enc, got, tc.in)
		}
	}
	// '+' decodes to space; stray '%' is literal.
	if got := Decode("a+b%26c"); got != "a b&c" {
		t.Errorf("Decode = %q", got)
	}
	if got := Decode("100%"); got != "100%" {
		t.Errorf("Decode stray %% = %q", got)
	}
	if got := Decode("a%2"); got != "a%2" {
		t.Errorf("Decode truncated = %q", got)
	}
	if got := Decode("a%ZZb"); got != "a%ZZb" {
		t.Errorf("Decode non-hex = %q", got)
	}
}

func TestWWWForm(t *testing.T) {
	if got := EncodeWWWForm([][2]string{{"a", "1"}, {"b", "x y"}, {"c", "&="}}); got != "a=1&b=x+y&c=%26%3D" {
		t.Errorf("EncodeWWWForm = %q", got)
	}
	type pr = [2]string
	cases := map[string][]pr{
		"a=1&b=2":     {{"a", "1"}, {"b", "2"}},
		"a=1&b=x+y":   {{"a", "1"}, {"b", "x y"}},
		"a%20b=c%26d": {{"a b", "c&d"}},
		"a&b=2":       {{"a", ""}, {"b", "2"}},
		"x=%E2%9C%93": {{"x", "✓"}},
		"a=1&&b=2":    {{"a", "1"}, {"b", "2"}}, // empty field skipped
	}
	for in, want := range cases {
		got, err := DecodeWWWForm(in)
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, [][2]string(want)) {
			t.Errorf("DecodeWWWForm(%q) = %v, want %v", in, got, want)
		}
	}
	empty, err := DecodeWWWForm("")
	if err != nil || len(empty) != 0 {
		t.Errorf("DecodeWWWForm(\"\") = %v, %v", empty, err)
	}
}

func TestEscapeUnescape(t *testing.T) {
	if got := Escape("hello world", " "); got != "hello%20world" {
		t.Errorf("Escape custom = %q", got)
	}
	if got := Unescape("a%20b%2Fc"); got != "a b/c" {
		t.Errorf("Unescape = %q", got)
	}
	// Unescape leaves '+' alone (unlike Decode).
	if got := Unescape("a+b"); got != "a+b" {
		t.Errorf("Unescape plus = %q", got)
	}
	if got := Unescape("a%2"); got != "a%2" {
		t.Errorf("Unescape truncated = %q", got)
	}
	// EscapeDefault keeps reserved delimiters but escapes space and controls.
	if got := EscapeDefault("a b/c?d:e@f"); got != "a%20b/c?d:e@f" {
		t.Errorf("EscapeDefault = %q", got)
	}
	if got := EscapeDefault("é"); got != "%C3%A9" {
		t.Errorf("EscapeDefault utf8 = %q", got)
	}
	// Encode delegates to Escape when an unsafe set is supplied.
	if got := Encode("a b", " "); got != "a%20b" {
		t.Errorf("Encode(unsafe) = %q", got)
	}
}

func TestComponentSetters(t *testing.T) {
	u := MustParse("http://h/")
	if err := u.SetScheme("https"); err != nil || u.Scheme != "https" {
		t.Errorf("SetScheme: %v %q", err, u.Scheme)
	}
	if err := u.SetScheme("1bad"); err == nil {
		t.Error("SetScheme(1bad): expected error")
	} else {
		var ce *InvalidComponentError
		if !errors.As(err, &ce) || ce.Component != "scheme" {
			t.Errorf("SetScheme err = %v", err)
		}
		if err.Error() != "bad component(expected scheme component): 1bad" {
			t.Errorf("SetScheme msg = %q", err.Error())
		}
	}
	if err := u.SetScheme(""); err != nil || u.Scheme != "" {
		t.Errorf("SetScheme(\"\"): %v", err)
	}
	if err := u.SetHost("good.host"); err != nil || u.Host != "good.host" {
		t.Errorf("SetHost: %v", err)
	}
	if err := u.SetHost("bad host"); err == nil {
		t.Error("SetHost(bad host): expected error")
	} else if err.Error() != "bad component(expected host component): bad host" {
		t.Errorf("SetHost msg = %q", err.Error())
	}
	if err := u.SetPort("8080"); err != nil || !u.HasPort || u.Port != 8080 {
		t.Errorf("SetPort: %v", err)
	}
	if err := u.SetPort(""); err != nil || u.HasPort {
		t.Errorf("SetPort(\"\"): %v", err)
	}
	if err := u.SetPort("abc"); err == nil {
		t.Error("SetPort(abc): expected error")
	} else if err.Error() != `bad component(expected port component): "abc"` {
		t.Errorf("SetPort msg = %q", err.Error())
	}
	if err := u.SetUserinfo("x:y"); err != nil || u.Userinfo != "x:y" {
		t.Errorf("SetUserinfo: %v", err)
	}
	if err := u.SetUserinfo("a@b"); err == nil {
		t.Error("SetUserinfo(a@b): expected error")
	} else if err.Error() != "bad component(expected userinfo component): a@b" {
		t.Errorf("SetUserinfo msg = %q", err.Error())
	}
}

func TestDefaultAndEffectivePort(t *testing.T) {
	u := MustParse("http://h/x")
	if p, ok := u.DefaultPort(); !ok || p != 80 {
		t.Errorf("DefaultPort http = %d,%v", p, ok)
	}
	if p, ok := u.EffectivePort(); !ok || p != 80 {
		t.Errorf("EffectivePort http = %d,%v", p, ok)
	}
	u2 := MustParse("http://h:9000/")
	if p, ok := u2.EffectivePort(); !ok || p != 9000 {
		t.Errorf("EffectivePort explicit = %d,%v", p, ok)
	}
	u3 := MustParse("scheme://h/")
	if _, ok := u3.DefaultPort(); ok {
		t.Error("DefaultPort unknown scheme: want none")
	}
	if _, ok := u3.EffectivePort(); ok {
		t.Error("EffectivePort unknown scheme without port: want none")
	}
	for _, sc := range sortedKeys(DefaultPorts) {
		if DefaultPorts[sc] <= 0 {
			t.Errorf("DefaultPorts[%q] not positive", sc)
		}
	}
}

func TestHostnameAndBuild(t *testing.T) {
	if h := MustParse("http://[::1]:8/x").Hostname(); h != "::1" {
		t.Errorf("Hostname ipv6 = %q", h)
	}
	if h := MustParse("http://h/x").Hostname(); h != "h" {
		t.Errorf("Hostname = %q", h)
	}
	b := Build("http", "u:p", "h", 8080, true, "/path", "q=1", true, "frag", true)
	if got := b.String(); got != "http://u:p@h:8080/path?q=1#frag" {
		t.Errorf("Build = %q", got)
	}
}

func TestMustParsePanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Error("MustParse(bad) did not panic")
		}
	}()
	MustParse("http://h\n/")
}

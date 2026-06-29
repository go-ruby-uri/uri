package uri

import (
	"errors"
	"testing"
)

// TestRouteToEdge exercises the route_from0 branches MRI takes for opaque
// targets, differing authority components and ":"-bearing first segments.
func TestRouteToEdge(t *testing.T) {
	cases := [][3]string{
		{"mailto:a@b", "mailto:a@b?x", "a@b?x"},     // same opaque, extra query
		{"mailto:a@b", "mailto:c@d", "c@d"},         // different opaque
		{"http://h:80/a/x", "http://h/a/y", "y"},    // explicit default == implicit
		{"http://u@h/a", "http://v@h/b", "//v@h/b"}, // different userinfo -> authority
		{"http://h/a/b", "http://h/a/x:y", "./x:y"}, // ':' in first dst segment
		{"http://h/a", "http://h:99/b", "//h:99/b"}, // different explicit port
	}
	for _, c := range cases {
		u := MustParse(c[0])
		r, err := u.RouteTo(c[1])
		if err != nil {
			t.Fatalf("RouteTo(%q,%q): %v", c[0], c[1], err)
		}
		if got := r.String(); got != c[2] {
			t.Errorf("Parse(%q).RouteTo(%q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// TestRelativeBaseErrors checks the BadURIError family MRI raises when an
// operation needs an absolute URI but is given a relative one.
func TestRelativeBaseErrors(t *testing.T) {
	if _, err := MustParse("/a/b").Merge("c"); err == nil {
		t.Error("Merge on relative base: expected error")
	} else {
		var be *BadURIError
		if !errors.As(err, &be) {
			t.Errorf("want *BadURIError, got %T", err)
		}
		if err.Error() != "both URI are relative" {
			t.Errorf("msg = %q", err.Error())
		}
		var fam Error
		if !errors.As(err, &fam) {
			t.Error("BadURIError not in URI Error family")
		}
	}
	if _, err := MustParse("/a").RouteTo("/b"); err == nil {
		t.Error("RouteTo relative dst: expected error")
	} else if err.Error() != "relative URI: /b" {
		t.Errorf("msg = %q", err.Error())
	}
	if _, err := MustParse("/a").RouteTo("http://h/b"); err == nil {
		t.Error("RouteTo relative base: expected error")
	} else if err.Error() != "relative URI: /a" {
		t.Errorf("msg = %q", err.Error())
	}
}

// TestMergeOpaqueAndDots covers the merge_path "abnormal ../" handling and an
// opaque base.
func TestMergeOpaqueAndDots(t *testing.T) {
	cases := [][3]string{
		{"http://h/../a", "../b", "http://h/b"},
		{"http://h/a/b/c/", "../../d", "http://h/a/d"},
		{"mailto:a@b", "mailto:c@d", "mailto:c@d"}, // absolute ref wins
	}
	for _, c := range cases {
		m, err := MustParse(c[0]).Merge(c[1])
		if err != nil {
			t.Fatal(err)
		}
		if got := m.String(); got != c[2] {
			t.Errorf("Merge(%q,%q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// TestMergeBranches exercises the merge_path corner cases: a base path ending
// in "..", a relative reference carrying a fragment, and a "x/." trailer.
func TestMergeBranches(t *testing.T) {
	cases := [][3]string{
		{"http://h/a/..", "b", "http://h/b"},
		{"http://h/a/b", "x#f", "http://h/a/x#f"},
		{"http://h/a/b", "x/.", "http://h/a/x/"},
	}
	for _, c := range cases {
		m, err := MustParse(c[0]).Merge(c[1])
		if err != nil {
			t.Fatal(err)
		}
		if got := m.String(); got != c[2] {
			t.Errorf("Merge(%q,%q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// TestRouteToBranches covers the abnormal-absolute-path verbatim case, opaque
// query stripping, and the "./"->"" rewrite when the target carries a query.
func TestRouteToBranches(t *testing.T) {
	cases := [][3]string{
		{"http://h/a/b", "http://h/x/../y", "/x/../y"},
		{"mailto:a@b?z", "mailto:a@b?z", ""},
		{"http://h/a/b/c", "http://h/a/b/?q", "?q"},
	}
	for _, c := range cases {
		r, err := MustParse(c[0]).RouteTo(c[1])
		if err != nil {
			t.Fatal(err)
		}
		if got := r.String(); got != c[2] {
			t.Errorf("RouteTo(%q,%q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// TestErrorMarkers invokes the unexported family-marker methods so the marker
// interface is exercised (errors.As does not call them).
func TestErrorMarkers(t *testing.T) {
	(&InvalidURIError{}).uriError()
	(&InvalidComponentError{}).uriError()
	(&BadURIError{}).uriError()
}

// TestInternalHelpers exercises helper-function edge inputs not reachable from
// the public surface (an empty port string is never a valid all-digit run, and
// lowercase hex must decode the same as uppercase).
func TestInternalHelpers(t *testing.T) {
	if allDigits("") {
		t.Error("allDigits(\"\") should be false")
	}
	if Decode("%e2%9c%93") != "✓" {
		t.Errorf("Decode lowercase hex = %q", Decode("%e2%9c%93"))
	}
	if Unescape("%e9") != "\xe9" {
		t.Errorf("Unescape lowercase hex = %q", Unescape("%e9"))
	}
}

// TestRouteToMoreBranches covers route_from0's authority-differs branches: a
// scheme-only target with no authority, a target whose differing authority
// carries a redundant default port, and identical path+query stripping.
func TestRouteToMoreBranches(t *testing.T) {
	cases := [][3]string{
		{"http://h/a", "http:/x", "http:/x"},       // dst has scheme, no authority
		{"http://h/a", "http://h2:80/b", "//h2/b"}, // diff host, redundant :80
		{"http://h/a?q", "http://h/a?q", ""},       // same path and query
	}
	for _, c := range cases {
		r, err := MustParse(c[0]).RouteTo(c[1])
		if err != nil {
			t.Fatal(err)
		}
		if got := r.String(); got != c[2] {
			t.Errorf("RouteTo(%q,%q) = %q, want %q", c[0], c[1], got, c[2])
		}
	}
}

// TestMergeAllDotDot covers merge_path's all-".." relative path (the
// trailing-slash reconstruction after the segment loop).
func TestMergeAllDotDot(t *testing.T) {
	m, err := MustParse("http://h/a/b").Merge("../..")
	if err != nil {
		t.Fatal(err)
	}
	if got := m.String(); got != "http://h/" {
		t.Errorf("Merge(../..) = %q, want http://h/", got)
	}
}

// TestErrorTypes pins each error type's marker membership and message shape.
func TestErrorTypes(t *testing.T) {
	var (
		iu = &InvalidURIError{"x"}
		ic = &InvalidComponentError{Component: "host", Value: "y"}
		bu = &BadURIError{"boom"}
	)
	for _, e := range []Error{iu, ic, bu} {
		if e.Error() == "" {
			t.Errorf("%T has empty message", e)
		}
		var fam Error
		if !errors.As(error(e), &fam) {
			t.Errorf("%T not recognized as Error", e)
		}
	}
	if bu.Error() != "boom" {
		t.Errorf("BadURIError msg = %q", bu.Error())
	}
	if iu.Error() != `bad URI (is not URI?): "x"` {
		t.Errorf("InvalidURIError msg = %q", iu.Error())
	}
}

// TestEncodeAllBytes ensures Encode/Decode round-trip every byte value,
// matching MRI's component codec, including the high-bit bytes.
func TestEncodeAllBytes(t *testing.T) {
	var raw []byte
	for i := 0; i < 256; i++ {
		raw = append(raw, byte(i))
	}
	enc := Encode(string(raw))
	if dec := Decode(enc); dec != string(raw) {
		t.Errorf("byte round-trip failed")
	}
	// Every encoded space is '+', and decoding it yields a space.
	if Encode(" ") != "+" || Decode("+") != " " {
		t.Error("space encoding mismatch")
	}
}

// TestUnescapeAllBytes round-trips EscapeDefault/Unescape over all bytes.
func TestUnescapeAllBytes(t *testing.T) {
	var raw []byte
	for i := 0; i < 256; i++ {
		raw = append(raw, byte(i))
	}
	if dec := Unescape(EscapeDefault(string(raw))); dec != string(raw) {
		t.Error("EscapeDefault/Unescape round-trip failed")
	}
}

// TestFTP covers the FTP scheme's leading-slash stripping and ";type=" parsing,
// matching URI::FTP.
func TestFTP(t *testing.T) {
	cases := []struct {
		in, path, typecode, str string
	}{
		{"ftp://h/x", "x", "", "ftp://h/x"},
		{"ftp://h/a/b", "a/b", "", "ftp://h/a/b"},
		{"ftp://h/", "", "", "ftp://h/"},
		{"ftp://h", "", "", "ftp://h/"},
		{"ftp://h/x;type=i", "x", "i", "ftp://h/x;type=i"},
	}
	for _, c := range cases {
		u := MustParse(c.in)
		if !u.IsFTP {
			t.Errorf("%q: IsFTP false", c.in)
		}
		if u.Path != c.path || u.Typecode != c.typecode {
			t.Errorf("%q: path=%q typecode=%q, want %q/%q", c.in, u.Path, u.Typecode, c.path, c.typecode)
		}
		if got := u.String(); got != c.str {
			t.Errorf("%q: String()=%q, want %q", c.in, got, c.str)
		}
	}
	// DefaultPort and EffectivePort for FTP.
	if p, ok := MustParse("ftp://h/x").EffectivePort(); !ok || p != 21 {
		t.Errorf("ftp EffectivePort = %d,%v", p, ok)
	}
}

// TestSplitVariants covers host/port edge cases in Split.
func TestSplitVariants(t *testing.T) {
	got, err := Split("http://h:8080")
	if err != nil {
		t.Fatal(err)
	}
	if got[2] != "h" || got[3] != "8080" {
		t.Errorf("Split host:port = %q", got)
	}
	// Authority with no path or port.
	g2, _ := Split("scheme://host")
	if g2[2] != "host" || g2[5] != "" {
		t.Errorf("Split authority-only = %q", g2)
	}
	// IPv6 host without a trailing port (closing bracket then path).
	g3, _ := Split("http://[::1]/p")
	if g3[2] != "[::1]" || g3[3] != "" || g3[5] != "/p" {
		t.Errorf("Split ipv6 no port = %q", g3)
	}
}

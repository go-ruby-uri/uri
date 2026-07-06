<p align="center"><img src="https://raw.githubusercontent.com/go-ruby-uri/brand/main/social/go-ruby-uri-uri.png" alt="go-ruby-uri/uri" width="720"></p>

# uri — go-ruby-uri

[![License](https://img.shields.io/badge/license-BSD--3--Clause-blue)](LICENSE)
[![Go](https://img.shields.io/badge/go-1.26.4%2B-00ADD8)](https://go.dev/dl/)
[![Coverage](https://img.shields.io/badge/coverage-100%25-1a7f37)](#testing)
[![Arches](https://img.shields.io/badge/arches-6%C3%9764--bit-555)](#portability)

**A pure-Go (no cgo) reimplementation of Ruby's [`uri`](https://docs.ruby-lang.org/en/master/URI.html) standard library**,
matching the behavior of MRI (CRuby) **4.0.x** byte-for-byte: URI parsing and
assembly, the 9-element `URI.split` decomposition, the scheme registry with
default ports, RFC 3986 reference resolution (`merge` / `+` / `route_to`),
normalization, and the `escape`/`unescape` and `encode_www_form` /
`decode_www_form` percent-encoders — with MRI's
`InvalidURIError` / `InvalidComponentError` / `BadURIError` taxonomy.

It is the `URI` backend for
[go-embedded-ruby](https://github.com/go-embedded-ruby/ruby), but is a
**standalone, reusable** module with no dependency on the Ruby runtime. Unlike
`net/url`, it reproduces Ruby's exact semantics (opaque vs. hierarchical paths,
default-port elision in `to_s`, `'+'`-for-space www-form encoding, FTP path
re-rooting and `;type=`), so a parsed URI round-trips through `String()` exactly
as MRI's `to_s` does.

## Usage

```go
import "github.com/go-ruby-uri/uri"

u, _ := uri.Parse("http://user@host:8080/a/b?q=1#frag")
u.Scheme   // "http"
u.Userinfo // "user"
u.Host     // "host"
u.Port     // 8080  (u.HasPort == true)
u.Path     // "/a/b"
u.Query    // "q=1"
u.Fragment // "frag"
u.String() // round-trips: "http://user@host:8080/a/b?q=1#frag"

// Default-port elision matches MRI's to_s.
uri.MustParse("https://h:443/x").String() // "https://h/x"

// RFC 3986 reference resolution.
b, _ := uri.Parse("http://h/a/b/c")
m, _ := b.Merge("../x")          // http://h/a/x
r, _ := b.RouteTo("http://h/a/y") // ../y  (the inverse of Merge)

// The 9-element decomposition (scheme, userinfo, host, port, registry,
// path, opaque, query, fragment), identical to URI.split.
parts, _ := uri.Split("http://a:b@h:9/p?q#f")
// ["http" "a:b" "h" "9" "" "/p" "" "q" "f"]

// Percent-encoding (component form: '+' for space, uppercase hex).
uri.Encode("a b&c=d")              // "a+b%26c%3Dd"
uri.Decode("a+b%26c")              // "a b&c"
uri.EncodeWWWForm([][2]string{{"a","1"},{"b","x y"}}) // "a=1&b=x+y"
pairs, _ := uri.DecodeWWWForm("a=1&b=x+y")            // [[a 1] [b x y]]
```

## API

| Go                                     | Ruby                                       |
| -------------------------------------- | ------------------------------------------ |
| `Parse(s)`                             | `URI.parse(s)` / `URI(s)`                  |
| `Join(base, rels...)`                  | `URI.join(base, *rels)`                    |
| `Split(s) [9]string`                   | `URI.split(s)`                             |
| `(*URI).String()`                      | `URI#to_s`                                 |
| `(*URI).Merge(rel)`                    | `URI#merge` / `URI#+`                      |
| `(*URI).RouteTo(dst)`                  | `URI#route_to`                             |
| `(*URI).Normalize()`                   | `URI#normalize`                            |
| `(*URI).Absolute()` / `Relative()`     | `URI#absolute?` / `URI#relative?`          |
| `(*URI).Hostname()`                    | `URI#hostname`                             |
| `(*URI).DefaultPort()` / `EffectivePort()` | `URI#default_port` / `URI#port`        |
| `(*URI).SetScheme/SetHost/SetPort/SetUserinfo` | component setters (validated)      |
| `Encode(s)` / `Decode(s)`              | `URI.encode_www_form_component` / `decode_www_form_component` |
| `EncodeWWWForm` / `DecodeWWWForm`      | `URI.encode_www_form` / `decode_www_form`  |
| `Escape` / `Unescape` / `EscapeDefault`| `URI::DEFAULT_PARSER.escape` / `unescape`  |
| `InvalidURIError` / `InvalidComponentError` / `BadURIError` | `URI::InvalidURIError` / `InvalidComponentError` / `BadURIError` |

## Conformance

The test suite pins a broad **differential corpus** against MRI 4.0.5: parse →
components, `to_s` round-trip, `merge`/`route_to`, `encode_www_form` /
`decode_www_form`, and the error taxonomy. Deterministic golden tables (captured
from MRI) run on every lane; a **live MRI oracle** re-derives them from a local
`ruby` when one is present (gated on `RUBY_VERSION >= 4.0`, skipped on Windows /
where ruby is absent), so the ruby-free, qemu and Windows lanes still hold 100%
coverage.

One documented divergence is implemented to match MRI: the **FTP** scheme strips
the path's leading slash and parses a trailing `;type=X` typecode (RFC 1738),
re-rooting on `to_s`.

## Testing

```sh
go test -race -coverprofile=cover.out ./...   # 100.0% of statements
go tool cover -func=cover.out | tail -1
```

## Portability

Pure Go, `CGO_ENABLED=0`. Built and tested on the six supported 64-bit targets —
`amd64`, `arm64`, `riscv64`, `loong64`, `ppc64le`, `s390x` — across Linux, macOS
and Windows.

## License

BSD-3-Clause. See [LICENSE](LICENSE).

## WebAssembly

Being pure Go (CGO=0), this library also compiles to **WebAssembly** — both
`GOOS=js GOARCH=wasm` (browser / Node.js) and `GOOS=wasip1 GOARCH=wasm` (WASI).
CI builds both targets on every push, alongside the six 64-bit native/qemu arches.

```sh
GOOS=js     GOARCH=wasm go build ./...   # browser / Node
GOOS=wasip1 GOARCH=wasm go build ./...   # WASI (wasmtime, wasmer, wasmedge, …)
```

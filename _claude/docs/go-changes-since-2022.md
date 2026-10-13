# Go changes since 2022: through go1.27.2, plus Go 1.28 on master

Written 2026-10-13. This covers language, standard library, toolchain and runtime
changes from Go 1.19 (August 2022) through go1.27.2, the installed toolchain. It
also covers what golang/go `master` adds beyond 1.27.2: that's Go 1.28, in
development and expected in February 2027.

## Sources and verification

- `references/go` is a depth-1 clone of golang/go `master` at `0de8329`
  (2026-10-12). It is the Go 1.28 development tree, not the 1.27.2 source.
  The 1.27.2 source ships with the installed toolchain in `/usr/lib/golang/src`.
- Facts come from these files in `references/go`:
  - `doc/go_spec.html`: the "Language versions" appendix and the `[Go 1.xx]`
    markers in the spec body.
  - `api/go1.19.txt` … `api/go1.27.txt` and `api/next/`: every exported API
    added in each release.
  - `doc/godebug.md`: compatibility-relevant behavior changes and their GODEBUG
    settings.
  - `doc/next/`: the draft Go 1.28 release notes.
  - `src/cmd/vendor/golang.org/x/tools/go/analysis/`: the `go vet` and
    `go fix` suites.
  - `src/internal/goexperiment/flags.go` and `src/internal/buildcfg/exp.go`:
    experiments and their defaults.
- Key items were compiled with the installed go1.27.2 and with a toolchain
  built from `references/go` (`go1.28-devel_0de83291`).
- Since Go 1.22, full release notes live in golang/website, not in this tree.
  For a few runtime and toolchain items, the release version isn't recorded in
  the files: PGO, Swiss-table maps, the Green Tea GC default, telemetry, the
  `tool` and `ignore` directives, `GOAUTH`, JSON `omitzero`, and when each vet
  analyzer arrived. Those version numbers come from release-note knowledge. The
  features themselves are all present in the tree.

## Language

| Version | Change |
| --- | --- |
| 1.18 (Mar 2022) | Generics: type parameters, `any`, `comparable`, `~T` and union constraints. This is the starting point. |
| 1.20 | Slice-to-array conversion (`[4]byte(s)`); `unsafe.SliceData`, `unsafe.String`, `unsafe.StringData`; ordinary interfaces satisfy `comparable` constraints |
| 1.21 | Built-ins `min`, `max`, `clear`; inference uses interface methods and infers type arguments for generic functions assigned to variables or passed as arguments |
| 1.22 | Each loop iteration gets its own variables, so `x := x` is unnecessary; `for i := range n` |
| 1.23 | `range` over iterator functions: `func(yield func(V) bool)` and `func(yield func(K, V) bool)` |
| 1.24 | Generic type aliases: `type Set[T comparable] = map[T]struct{}` |
| 1.26 | `new(expr)` allocates a variable initialized to the expression's value; generic types may refer to themselves in their own type-parameter list |
| 1.27 | Generic methods; struct-literal keys may name promoted fields; generic-function inference in every assignment context (but see [Compiler crash](#compiler-crash-in-127-generic-function-inference)) |
| 1.28 (master) | Composite-literal types may be omitted when inferable; `string(x)` only converts `byte` and `rune` values (see [Go 1.28 on master](#go-128-on-master-not-in-1272)) |

The `go` line in `go.mod` decides which features a module gets. A module that
says `go 1.21` gets 1.21 semantics even when built with go1.27.2. That includes
loop variables shared across iterations.

```go
// 1.26: new with an expression
timeout := new(30 * time.Second) // *time.Duration pointing at 30s

// 1.27: a generic method declares its own type parameters
type List[E any] []E

func (l List[E]) Map[F any](f func(E) F) List[F] {
	r := make(List[F], len(l))
	for i, x := range l {
		r[i] = f(x)
	}
	return r
}

strs := List[int]{1, 2, 3}.Map(strconv.Itoa) // List[string]

// 1.27: keys may name promoted fields
type Inner struct{ A, B int }
type Outer struct {
	Inner
	C int
}

o := Outer{A: 1, B: 2, C: 3} // previously Outer{Inner: Inner{A: 1, B: 2}, C: 3}
```

## Standard library

### 1.19 (Aug 2022)

- Typed atomics in `sync/atomic`: `Int32`, `Int64`, `Uint32`, `Uint64`,
  `Uintptr`, `Bool`, `Pointer[T]`.
- `fmt.Append`, `Appendf`, `Appendln`.
- `runtime/debug.SetMemoryLimit` and the `GOMEMLIMIT` soft memory limit.
- Doc comments support links, lists and headings (new `go/doc/comment`).
- `os/exec`: PATH lookups no longer resolve to binaries in the current
  directory (`exec.ErrDot`); `Cmd.Environ`.
- `net/url.JoinPath`, `sort.Find`, `time.Duration.Abs`, `flag.TextVar`,
  `http.MaxBytesError`, `encoding/binary.AppendUvarint`/`AppendVarint`.
- All of `io/ioutil` is deprecated.

### 1.20 (Feb 2023)

- `errors.Join`; `fmt.Errorf` accepts multiple `%w`.
- `context.WithCancelCause` and `context.Cause`.
- `strings.CutPrefix`/`CutSuffix` (and in `bytes`); `bytes.Clone`.
- `net/http.ResponseController` (per-request deadlines, `Flush`, `Hijack`);
  `httputil.ReverseProxy.Rewrite`.
- New `crypto/ecdh`.
- `time.DateOnly`, `DateTime`, `TimeOnly`; `Time.Compare`.
- `exec.Cmd.Cancel` and `WaitDelay`.
- `fs.SkipAll`/`filepath.SkipAll`; `filepath.IsLocal`.
- `io.OffsetWriter`; `sync.Map.Swap`/`CompareAndSwap`/`CompareAndDelete`;
  `reflect.Value.SetZero`/`Comparable`/`Equal`/`Grow`; `testing.B.Elapsed`;
  `runtime/coverage`.
- The `math/rand` global source is seeded randomly, and `rand.Seed` is
  deprecated.

### 1.21 (Aug 2023)

- New packages: `slices`, `maps`, `cmp`, `log/slog`, `testing/slogtest`.
- `sync.OnceFunc`, `OnceValue`, `OnceValues`.
- `context.AfterFunc`, `WithoutCancel`, `WithDeadlineCause`,
  `WithTimeoutCause`.
- `errors.ErrUnsupported`, `runtime.Pinner`, `runtime.PanicNilError`,
  `testing.Testing()`, `flag.BoolFunc`, `bytes.Buffer.AvailableBuffer`,
  `binary.NativeEndian`, `go/ast.IsGenerated`.
- `crypto/tls` gains QUIC support (`QUICConn`) and session-ticket APIs.
- `net` supports Multipath TCP (opt-in).
- `reflect.SliceHeader` and `StringHeader` are deprecated; use `unsafe.Slice`
  and `unsafe.String`.
- Unicode 15.

### 1.22 (Feb 2024)

- `net/http.ServeMux` patterns with methods and wildcards:
  `mux.HandleFunc("GET /items/{id}", h)` with `r.PathValue("id")`.
- New `math/rand/v2` (ChaCha8, PCG, generic `rand.N`).
- New `go/version`; `cmp.Or`; `slices.Concat`; `database/sql.Null[T]`;
  `reflect.TypeFor[T]()`.
- `AppendEncode`/`AppendDecode` in `encoding/base32`, `base64` and `hex`.
- `archive/tar` and `archive/zip` `Writer.AddFS`; `http.FileServerFS`,
  `ServeFileFS`, `NewFileTransportFS`.
- `go/types.Alias`; `slog.SetLogLoggerLevel`.

### 1.23 (Aug 2024)

- New `iter` package: `Seq`, `Seq2`, `Pull`, `Pull2`.
- `slices`: `All`, `Values`, `Backward`, `Collect`, `AppendSeq`, `Sorted`,
  `SortedFunc`, `SortedStableFunc`, `Chunk`, `Repeat`.
- `maps`: `All`, `Keys`, `Values`, `Collect`, `Insert`.
- New `unique` package (value interning with `unique.Make`) and
  `structs.HostLayout`.
- `os.CopyFS`; `filepath.Localize`.
- Timer channels in `time` are unbuffered (synchronous), so `Timer.Stop` and
  `Timer.Reset` behave reliably.
- `net.KeepAliveConfig`; `http.ParseCookie`, `ParseSetCookie`,
  `Request.CookiesNamed`, `Request.Pattern`, `Cookie.Partitioned`.
- `sync.Map.Clear`; atomic `And`/`Or`; `reflect.Value.Seq`/`Seq2`;
  `debug.SetCrashOutput`; `binary.Encode`/`Decode`/`Append`.
- Client-side TLS Encrypted Client Hello.

### 1.24 (Feb 2025)

- `os.Root`, `os.OpenRoot`, `os.OpenInRoot`: file access confined to one
  directory.
- `testing`: `B.Loop`, `T.Context`, `T.Chdir` (also on `B`, `F` and `TB`).
- `runtime.AddCleanup`, a better `SetFinalizer`; `weak.Pointer`.
- `strings`/`bytes`: `Lines`, `SplitSeq`, `SplitAfterSeq`, `FieldsSeq`,
  `FieldsFuncSeq`.
- `encoding.TextAppender` and `BinaryAppender`, implemented across the standard
  library.
- `encoding/json` `omitzero` struct tag option.
- Crypto:
  - New `crypto/mlkem`, `crypto/hkdf`, `crypto/pbkdf2` and `crypto/sha3`.
  - `crypto/rand.Text`, `cipher.NewGCMWithRandomNonce`.
  - `crypto/fips140` and FIPS 140-3 mode.
  - TLS turns on X25519MLKEM768 by default and adds server-side ECH.
- `net/http.Protocols` (including unencrypted HTTP/2) and `HTTP2Config`.
- `hash/maphash.Comparable`; `slog.DiscardHandler`; iterator methods in
  `go/types`.
- `math/rand.Seed` is a no-op; `runtime.GOROOT` is deprecated.

### 1.25 (Aug 2025)

- `sync.WaitGroup.Go`.
- `testing/synctest` (`Test`, `Wait`) is stable: a fake clock and isolated
  "bubbles" for concurrent tests.
- `runtime/trace.FlightRecorder`; `runtime.SetDefaultGOMAXPROCS`.
- `net/http.CrossOriginProtection` (CSRF protection).
- `io/fs.ReadLinkFS`, `fs.Lstat`, `fs.ReadLink`; `testing/fstest.MapFS`
  implements them.
- New `os.Root` methods: `Chmod`, `Chown`, `Chtimes`, `Lchown`, `Link`,
  `MkdirAll`, `ReadFile`, `Readlink`, `RemoveAll`, `Rename`, `Symlink`,
  `WriteFile`.
- `reflect.TypeAssert[T]`; `hash.Cloner` and `hash.XOF`; `T.Attr`,
  `T.Output`; `slog.GroupAttrs`, `Record.Source`; `crypto.MessageSigner`;
  `go/ast.PreorderStack`.

### 1.26 (Feb 2026)

- `errors.AsType[T]`, a generic replacement for `errors.As`.
- `slog.NewMultiHandler`; `bytes.Buffer.Peek`; `T.ArtifactDir`.
- `reflect` iterators: `Type.Fields`, `Methods`, `Ins`, `Outs`;
  `Value.Fields`, `Methods`.
- `net.Dialer.DialTCP`, `DialUDP`, `DialIP`, `DialUnix`, which take a
  context and `netip` addresses.
- `http.Transport.NewClientConn` and `http.ClientConn`.
- Crypto:
  - New `crypto/hpke`.
  - `crypto.Encapsulator` and `crypto.Decapsulator`.
  - `crypto/fips140.Enforced`, `Version`, `WithoutEnforcement`.
  - Most crypto APIs ignore their `rand` parameter (`cryptocustomrand`).
  - More post-quantum hybrid TLS groups.
- `netip.Prefix.Compare`; `os.Process.WithHandle`; `go/ast.Directive`.

### 1.27 (Aug 2026)

- `encoding/json/v2` and `encoding/json/jsontext` are stable and on by
  default. The v1 `encoding/json` is now built on v2: for example,
  `json.RawMessage = jsontext.Value`, plus options that restore legacy
  behavior.
- New `uuid` package: `New`, `NewV4`, `NewV7`, `Parse`, `MustParse`, `Nil`,
  `Max`.
- New `crypto/mldsa` (ML-DSA-44/65/87), also wired into `crypto/tls` and
  `crypto/x509`. `tls.Config.Rand` is deprecated.
- `strings.CutLast` and `bytes.CutLast`.
- `math/rand/v2.(*Rand).N`, the standard library's first generic method.
- `httptest.NewTestServer(t, handler)`; `url.URL.Clone` and `Values.Clone`.
- `hash/maphash.Hasher[T]` and `ComparableHasher[T]`; `go/types.Hasher`.
- `math/big.Int.Divide` with a `RoundingMode` (`Round`, `Trunc`, `Floor`,
  `Ceil`).
- `testing/synctest.Sleep`; `database/sql/driver.RowsColumnScanner`;
  `http.Server.MaxHeaderValueCount` and `DisableClientPriority`.
- Unicode 17.

## Toolchain and runtime

### Modules and the go command

- 1.21: the `go` line became a minimum requirement. New `toolchain` line and
  `GOTOOLCHAIN` toolchain switching (the Fedora package pins it to `local`; see
  below).
- 1.21: `//go:debug` directives in main packages. 1.23: `godebug` blocks in
  `go.mod` and `go.work`.
- 1.20: whole-program coverage (`go build -cover`, `GOCOVERDIR`). 1.21: PGO
  is GA, and a `default.pgo` in the main package directory is used
  automatically.
- 1.23: opt-in telemetry (`go telemetry`) and `go env -changed`.
- 1.24:
  - `tool` directives in `go.mod`: `go get -tool`, then `go tool <name>`.
  - `GOAUTH`.
  - `go build -json`.
  - `go test -json` reports build errors as JSON.
- 1.25: `ignore` directive in `go.mod`; `go doc -http`.

### go fix runs the modernize suite (1.26+)

`go fix ./...` rewrites code to current idioms. Each fix is meant to preserve
behavior, but review the diff, because comments inside replaced loops can be
lost. The suite also includes the `buildtag`, `hostport` and `inline` analyzers.

| Analyzer | Rewrite |
| --- | --- |
| `any` | `interface{}` → `any` |
| `appendclipped` | append chains → `slices.Concat` (off by default) |
| `atomictypes` | `atomic.AddInt64(&x, …)` → `atomic.Int64` values |
| `bloop` | `for range b.N` → `for b.Loop()` (off by default) |
| `embedlit` | nested embedded-struct literals → promoted-field keys (1.27) |
| `errorsastype` | `errors.As` → `errors.AsType[T]` |
| `fmtappendf` | `[]byte(fmt.Sprintf(…))` → `fmt.Appendf` (off by default) |
| `forvar` | drops `x := x` in range loops |
| `importcomment` | drops `// import "path"` comments (master only) |
| `mapsloop` | map-copy loops → `maps.Copy`/`Insert`/`Clone`/`Collect` |
| `minmax` | if/else → `min`/`max` |
| `newexpr` | pointer helpers like `func ptr(x int) *int { return &x }` and calls to them → `new(expr)` |
| `omitzero` | `omitempty` on struct fields → `omitzero` |
| `plusbuild` | drops `// +build` lines |
| `rangeint` | `for i := 0; i < n; i++` → `for i := range n` |
| `reflecttypefor` | `reflect.TypeOf(x)` → `reflect.TypeFor[T]()` |
| `reflecttypeassert` | `v.Interface().(T)` → `reflect.TypeAssert[T](v)` (master only) |
| `slicesbackward` | backward index loops → `slices.Backward` |
| `slicesclip` | `s[:len(s):len(s)]` → `slices.Clip` (master only) |
| `slicescontains` | search loops → `slices.Contains`/`ContainsFunc` |
| `slicesdelete` | append-based deletion → `slices.Delete` |
| `slicessort` | `sort.Slice` on basic types → `slices.Sort` |
| `stditerators` | `Len`/`At` loops → iterator methods |
| `stringsbuilder` | `+=` in loops → `strings.Builder` |
| `stringscut` | `strings.Index` + slicing → `strings.Cut` |
| `stringscutprefix` | `HasPrefix` + `TrimPrefix` → `CutPrefix` |
| `stringsseq` | ranging over `Split`/`Fields` → `SplitSeq`/`FieldsSeq` |
| `testingcontext` | `context.WithCancel` in tests → `t.Context()` |
| `unsafefuncs` | pointer arithmetic → `unsafe.Add` and friends |
| `waitgroupgo` | `wg.Add(1)` + `go` + `wg.Done()` → `wg.Go` |

### go vet

The go1.27.2 vet suite has these analyzers:

`appends asmdecl assign atomic bools buildtag cgocall composite copylock
defers directive errorsas framepointer hostport httpresponse ifaceassert
loopclosure lostcancel nilfunc printf shift sigchanyzer slog stdmethods
stdversion stringintconv structtag testinggoroutine tests timeformat unmarshal
unreachable unsafeptr unusedresult waitgroup`

Added since 2022:

- 1.20: `timeformat`.
- 1.21: `slog` and `directive`.
- 1.22: `appends` and `defers`.
- 1.23: `stdversion`.
- 1.24: `tests`.
- 1.25: `waitgroup` and `hostport`.
- Master: `scannererr` and `sqlrowserr`.

### Runtime

- 1.24: maps use Swiss tables.
- 1.25: GOMAXPROCS respects cgroup CPU limits and updates while the program
  runs (`containermaxprocs`, `updatemaxprocs`).
- Green Tea GC: an experiment in 1.25, the default in 1.26. It's on in
  go1.27.2's baseline.
- 1.25: DWARF 5 debug info is the default. Fedora's build turns it off; see
  below.
- 1.26: heap base address randomization on 64-bit platforms.
- 1.27: a `goroutineleak` profile in `runtime/pprof`. In 1.26 it was the
  `goroutineleakprofile` experiment.
- 1.27: pprof goroutine labels appear in tracebacks by default
  (`tracebacklabels=1`).

### Experiments (GOEXPERIMENT)

| Experiment | go1.27.2 | master | Notes |
| --- | --- | --- | --- |
| `greenteagc` | on | on | Green Tea garbage collector |
| `jsonv2` | on | on | `encoding/json/v2`; `nojsonv2` turns it off |
| `randomizedheapbase64` | on | on | heap base randomization |
| `dwarf5` | on (Fedora sets `nodwarf5`) | on | DWARF 5 debug info |
| `sizespecializedmalloc` | on | flag removed | malloc implementations specialized per size class |
| `mapsplitgroup` | off | on | map groups stored as KKKKVVVV instead of KVKV |
| `golistexportnewformat` | — | on | new `go list -export` format |
| `simd` | opt-in | opt-in | portable `simd` package plus `simd/archsimd` intrinsics |
| `runtimesecret` | opt-in | opt-in | `runtime/secret` package |
| `arenas` | opt-in | opt-in | `arena` package |
| `runtimefreegc` | opt-in | opt-in | eager freeing with compiler help |
| `cgobindings` | — | opt-in | declare cgo bindings with the Go types they bind to |

## Go 1.28 on master (not in 1.27.2)

Taken from `doc/next/`, `api/next/` and the spec, with the code below compiled
by the master toolchain.

### Language

You can omit a composite literal's type whenever the context supplies it. For
a pointer type, both the `&` and the type can be dropped:

```go
type Point struct{ X, Y int }

func mid(a, b Point) Point {
	return {(a.X + b.X) / 2, (a.Y + b.Y) / 2}
}

var p Point
p = {1, 2}             // Point{1, 2}
var pp *Point
pp = {3, 4}            // &Point{3, 4}
m := mid({0, 0}, {4, 8})
q := {1, 2}            // error: missing type in composite literal
```

`string(x)` now works only when `x`'s underlying type is `byte` or `rune`, or
`x` is an untyped rune constant. With `go 1.28` in `go.mod`, `string(i)` for an
`int` fails with `cannot convert i (variable of type int) to type string:
argument must have type byte or rune with go1.28 or later`. Modules that declare
an older `go` line still compile.

### Library

- `regexp` iterators: `AllString`, `All`, `AllIndex`, `AllStringIndex`,
  `AllSubmatch`, `AllStringSubmatch` and the index variants, plus `SplitSeq`.
- `flag.All()` and `(*FlagSet).All()` iterate over flags; `Flag.IsSet`.
- `url.MustParse`; `http.MethodQuery` (RFC 10008); `synctest.Subtest`;
  `go/types.Scope.Objects`.
- Linux `syscall.SysProcAttr` gains `UseLandlock`, `LandlockFD`,
  `LandlockFlags` and `NoNewPrivs` for child processes.
- `net.IPMask`, `IPNet` and `HardwareAddr` gain text unmarshaling that accepts
  a readable format. `netmarshal=1` switches marshaling to that format; the
  default is expected to change in 1.30.
- Deprecated: `go/build.ArchChar`, `net.DNSConfigError` and the new
  `go/parser.ResolveFile`.

### Behavior

- `http.Transport.CancelRequest` does nothing; use `NewRequestWithContext`.
- `ProxyFromEnvironment` prefers lowercase `http_proxy`/`https_proxy`/`no_proxy`
  when both cases are set to different values.
- The HTTP/2 server lets idle connections' goroutines exit, which uses less
  memory with many idle connections.
- `crypto/x509` rejects trailing data in several certificate and key
  structures.
- `ReadAt` on `embed.FS` and `fstest.MapFS` files returns `io.EOF` at or past
  end of file.
- `base32`/`base64` `EncodedLen` panics on overflow instead of returning a
  negative number.
- Fixes the `slog` `WithGroup("")` bug described below.
- `httpservecontentmaxranges=200` ignores Range headers with too many ranges.
  This was already backported to 1.27.2 and 1.26.9.
- Building Go from source needs Go 1.26.0 or later.

### Vet

- `scannererr`: a `bufio.Scanner` loop whose `Err()` is never checked.
- `sqlrowserr`: a `sql.Rows` loop whose `Err()` is never checked.

## Findings in this environment

### Compiler crash in 1.27 generic-function inference

Go 1.27 lets an uninstantiated generic function be used in any assignment
context. In four of those contexts the type checker accepts the code but the
compiler backend crashes. This happens in go1.27.2 and is still unfixed on
master at `0de8329`.

| Context | Example | Result |
| --- | --- | --- |
| var declaration | `var f func(a, b int) int = cmp.Compare` | works |
| assignment | `f = cmp.Compare` | works |
| return | `return cmp.Compare` | works |
| call argument | `slices.SortedFunc(seq, cmp.Compare)` | works |
| slice or array literal element | `[]func(a, b int) int{cmp.Compare}` | internal compiler error |
| map literal value | `map[string]func(a, b int) int{"c": cmp.Compare}` | internal compiler error |
| struct literal field | `S{F: cmp.Compare}` | internal compiler error |
| channel send | `ch <- cmp.Compare` | internal compiler error |

Minimal repro (`go.mod` says `go 1.27`):

```go
package main

import (
	"cmp"
	"fmt"
)

func main() {
	fs := []func(a, b int) int{cmp.Compare}
	fmt.Println(fs[0](1, 2))
}
```

```
./main.go:9:32: internal compiler error: func[T cmp.Ordered](x T, y T) int is not assignable to func(a int, b int) int
```

The fatal error comes from `(*writer).convertExpr` in
`src/cmd/compile/internal/noder/writer.go`. That code is the same in 1.27.2 and
on master. The workaround is to instantiate explicitly, for example
`[]func(a, b int) int{cmp.Compare[int]}`. The crash is unfixed on master as of
`0de8329`. If golang/go has no issue for it yet, it's worth reporting.

### `slog` `WithGroup("")` adds an empty group in 1.27.2

Since 1.27.1, calling `Handler.WithGroup("")` directly, including through
`MultiHandler`, inserts a group named `""` instead of doing nothing. Keys come
out as `.k=v`. Reproduced on go1.27.2:

```go
h := slog.NewTextHandler(os.Stdout, nil)
slog.New(h.WithGroup("")).Info("direct", "k", "v") // time=… level=INFO msg=direct .k=v
```

The fix is on master. Avoid passing `""` to `WithGroup` on 1.27.2.

### Fedora toolchain defaults

The installed Go is Fedora's package at `/usr/lib/golang`. Its `go.env` sets:

- `GOTOOLCHAIN=local`: the packaged toolchain is always used. A `go.mod` that
  requires a newer Go (for example `go 1.28`) fails instead of downloading one.
- `GOEXPERIMENT=nodwarf5`: DWARF 5 is off because Go's DWARF 5 output breaks
  `debugedit` (sourceware bug 33204). That's why `go version` prints
  `go1.27.2-X:nodwarf5`.
- `GOPROXY` and `GOSUMDB` keep the upstream defaults.

### Master toolchain build

A master toolchain was built from `references/go` with
`GOROOT_BOOTSTRAP=/usr/lib/golang ./make.bash`. It's
`references/go/bin/go` (`go1.28-devel_0de83291`), about 115 MB including
`references/go/pkg`. Every build output is git-ignored in that clone, and
`git -C references/go clean -fdX` removes them. Re-cloning through `rake` drops
them too. To try 1.28 code, put `go 1.28` in `go.mod` and run
`GOTOOLCHAIN=local references/go/bin/go run .`.

## Practical notes for Go code in this repo

- Declare `go 1.27` or lower in `go.mod`, because the installed toolchain
  can't fetch 1.28.
- Instantiate generic functions explicitly in composite literals and channel
  sends (`cmp.Compare[int]`).
- Prefer the current idioms: `for range n`, `min`/`max`, `slices`/`maps`,
  iterators, `errors.AsType`, `wg.Go`, `t.Context()`, `b.Loop()`,
  `encoding/json/v2`. Running `go fix ./...` and `go vet ./...` applies or
  checks most of them.
- Don't use `slog` `WithGroup("")` on 1.27.2.

## Reference: GODEBUG history

These are from `doc/godebug.md` and `src/internal/godebugs/table.go`. A
setting's default follows the module's `go` line unless it was introduced in a
security release; those apply everywhere.

| Go | Setting | New default behavior | Status |
| --- | --- | --- | --- |
| 1.19 | `execerrdot` | PATH lookups don't resolve to binaries in `.` | kept |
| 1.19 | `netedns0` | DNS requests send EDNS0 headers | kept |
| 1.20 | `randautoseed` | `math/rand` global source seeded randomly | kept |
| 1.20 | `tarinsecurepath`, `zipinsecurepath` | can reject insecure archive paths; default still allows them | may flip |
| 1.20 | `x509usefallbackroots` | fallback roots for verification | kept |
| 1.20 | `installgoroot` | standard library `.a` files no longer preinstalled | kept |
| 1.21 | `panicnil` | `panic(nil)` is a run-time error | kept |
| 1.21 | `multipartmaxheaders`, `multipartmaxparts` | limits on MIME headers and multipart parts | kept |
| 1.21 | `multipathtcp` | Multipath TCP support (opt-in) | default changed in 1.24 |
| 1.22 | `httpmuxgo121` | extended `ServeMux` patterns | kept |
| 1.22 | `httplaxcontentlength` | empty `Content-Length` is an error | kept |
| 1.22 | `tlsmaxrsasize` | TLS RSA keys limited to 8192 bits | kept |
| 1.22 | `tls10server` | TLS 1.2 minimum for clients and servers | removed in 1.27 |
| 1.22 | `tlsrsakex` | RSA key-exchange suites off by default | removed in 1.27 |
| 1.22 | `tlsunsafeekm` | `ExportKeyingMaterial` requires TLS 1.3 or EMS | removed in 1.27 |
| 1.22 | `gotypesalias` | `go/types` produces `Alias` types (default from 1.23) | removed in 1.27 |
| 1.22 | `x509usepolicies` | marshal `Certificate.Policies` (default from 1.24) | kept |
| 1.22 | `disablethp` | no transparent-huge-page workaround; setting disables THP for the Go heap | may be removed |
| 1.23 | `asynctimerchan` | timer channels unbuffered | removed in 1.27 |
| 1.23 | `winsymlink`, `winreadlinkvolume` | Windows mount points and reparse points handled differently | kept |
| 1.23 | `x509negativeserial` | negative certificate serial numbers rejected | kept |
| 1.23 | `tls3des` | 3DES suites off by default | removed in 1.27 |
| 1.23 | `x509keypairleaf` | `X509KeyPair` populates `Leaf` | removed in 1.27 |
| 1.23 | `httpservecontentkeepheaders` | error responses from `ServeContent` drop caching headers | kept |
| 1.24 | `fips140` | FIPS 140-3 mode (`off` by default) | kept |
| 1.24 | `randseednop` | `math/rand.Seed` is a no-op | kept |
| 1.24 | `multipathtcp` | now `2`: MPTCP on for listeners by default | kept |
| 1.24 | `gotestjsonbuildtext` | `go test -json` emits build errors as JSON | removable from 1.28 |
| 1.24 | `rsa1024min` | RSA keys must be at least 1024 bits | kept |
| 1.24 | `tlsmlkem` | X25519MLKEM768 key exchange on by default | kept |
| 1.24 | `x509rsacrt` | `ParsePKCS1PrivateKey` uses and validates CRT values | kept |
| 1.24 | `x509sha1` | setting removed; SHA-1 certificate signatures unsupported | removed |
| 1.25 | `containermaxprocs`, `updatemaxprocs` | GOMAXPROCS follows cgroup limits and updates while running | kept |
| 1.25 | `decoratemappings` | Linux anonymous mappings labeled `[anon: Go: …]` | kept |
| 1.25 | `embedfollowsymlinks` | `//go:embed` doesn't follow symlinks | kept |
| 1.25 | `tlssha1` | SHA-1 signatures disabled in TLS 1.2 | kept |
| 1.25 | `x509sha256skid` | missing SubjectKeyId filled with SHA-256 | kept |
| 1.25 | `allowmultiplevcs` | no VCS stamping when multiple VCS are detected | kept |
| 1.26 | `httpcookiemaxnum` | at most 3000 cookies parsed | kept |
| 1.26 | `urlmaxqueryparams` | at most 10000 query parameters parsed | kept |
| 1.26 | `urlstrictcolons` | stray colons in hosts rejected (only `http`/`https` from 1.27) | kept |
| 1.26 | `tlssecpmlkem` | SecP256r1MLKEM768 and SecP384r1MLKEM1024 on | kept |
| 1.26 | `cryptocustomrand` | crypto APIs ignore the `rand` parameter | kept |
| 1.26 | `tracebacklabels` | pprof labels in tracebacks (default on from 1.27) | kept |
| 1.27 | `htmlmetacontenturlescape` | `html/template` escapes `url=` in meta `content` | kept |
| 1.27 | `x509sslcertoverrideplatform` | `SSL_CERT_FILE`/`SSL_CERT_DIR` honored on Windows and macOS | removal planned for 1.31 |
| 1.27 | `fips140ems` | `0` disables Extended Master Secret enforcement in FIPS mode | removal planned for 1.31 |
| 1.28 | `httpservecontentmaxranges` | Range headers with more than 200 ranges ignored; backported to 1.27.2 and 1.26.9 | master |
| 1.28 | `netmarshal` | readable text marshaling for `IPMask`/`IPNet`/`HardwareAddr`; stays `0` until 1.30 | master |

# Go 1.25 upgrade notes

## Why this repository requires Go 1.25

The `dbz` module uses the Bun SQLite shim in tests and the Bun observability smoke command. After
the Bun packages were aligned at v1.2.18, `github.com/uptrace/bun/driver/sqliteshim` v1.2.18 became
the dependency that sets the minimum Go version: its [module declaration][bun-go-mod] specifies
`go 1.25.0`.

Go requires a module's `go` version to be at least the version required by every module in its
dependency graph, and a workspace's version to be at least the version of every module it includes
([Go toolchain documentation](https://go.dev/doc/toolchain#module)). Therefore:

- `dbz/go.mod` declares `go 1.25.0` and requires the Bun packages at v1.2.18.
- `go.work` declares `go 1.25.0` because it includes `dbz`.
- The root, `cronz`, `kratosz`, and `queue` modules continue to declare Go 1.24. They do not
  independently require Go 1.25 when used outside this workspace.
- Users of `github.com/adobaai/pkg/dbz` need a Go 1.25 or newer toolchain. With the standard
  `GOTOOLCHAIN=auto` setting, the `go` command can select or download a suitable toolchain;
  environments that disable switching must provide one
  ([toolchain selection](https://go.dev/doc/toolchain#select)).

## Compatibility

Go 1.25 makes no language changes that affect Go programs and maintains the Go 1 compatibility
promise. Almost all Go 1.24 programs should compile and run unchanged
([Go 1.25 release notes](https://go.dev/doc/go1.25#language)). No source migration is required in
this repository solely because its Go directive changed.

One compiler correction can expose invalid existing code: dereferencing a possibly nil result before
checking the accompanying error now correctly panics. Error checks should immediately follow calls
that can return a nil value and a non-nil error
([compiler changes](https://go.dev/doc/go1.25#compiler)).

## Material changes from Go 1.24

### Runtime and compiler

- On Linux, the default `GOMAXPROCS` now considers cgroup CPU limits and may update when CPU
  availability changes. Explicit `GOMAXPROCS` settings retain their prior behavior. This can improve
  container latency, but it can also change effective parallelism
  ([runtime notes](https://go.dev/doc/go1.25#container-aware-gomaxprocs),
  [design overview](https://go.dev/blog/container-aware-gomaxprocs)).
- The compiler and linker emit DWARF 5 by default, reducing debug information size and link time.
  Older debuggers and binary-analysis tools may need upgrades; `GOEXPERIMENT=nodwarf5` is a
  temporary fallback ([DWARF 5 notes](https://go.dev/doc/go1.25#dwarf5)).
- More slice backing arrays can be allocated on the stack. Correct code benefits transparently,
  while invalid `unsafe.Pointer` use can become more visible
  ([slice allocation notes](https://go.dev/doc/go1.25#faster-slices)).
- An alternative garbage collector is available only through `GOEXPERIMENT=greenteagc`; it is not
  enabled by this upgrade
  ([runtime notes](https://go.dev/doc/go1.25#new-experimental-garbage-collector)).

### Tools

- `go vet` adds checks for misplaced `sync.WaitGroup.Add` calls and IPv6-unsafe host/port formatting
  ([vet notes](https://go.dev/doc/go1.25#vet)).
- `go.mod` supports an `ignore` directive, and the `work` package pattern selects all packages in
  the active module or workspace ([Go command notes](https://go.dev/doc/go1.25#go-command)).
- AddressSanitizer builds now enable C allocation leak detection by default
  ([Go command notes](https://go.dev/doc/go1.25#go-command)).

### Standard library and protocols

- `testing/synctest` is now a supported package for deterministic tests of concurrent code, and
  `sync.WaitGroup.Go` provides a concise way to start counted goroutines
  ([standard library notes](https://go.dev/doc/go1.25#stdlib)).
- `runtime/trace.FlightRecorder` can retain a bounded in-memory execution trace for capture when an
  uncommon event occurs ([flight recorder notes](https://go.dev/doc/go1.25#trace-flight-recorder)).
- TLS 1.2 no longer accepts SHA-1 signatures by default, and TLS peers are checked more strictly.
  Legacy peers may fail to connect; `GODEBUG=tlssha1=1` is a temporary compatibility switch
  ([`crypto/tls` notes](https://go.dev/doc/go1.25#crypto/tls)).
- The `encoding/json/v2` implementation is experimental and requires `GOEXPERIMENT=jsonv2`; regular
  builds continue to use the existing JSON implementation
  ([JSON v2 notes](https://go.dev/doc/go1.25#new-experimental-encoding-json-v2-package)).

### Platforms

- macOS 12 Monterey is the minimum supported Darwin version.
- `windows/arm` is deprecated and is removed in Go 1.26.
- With `GOAMD64=v3` or newer, fused multiply-add instructions can change exact floating-point
  results while improving speed and accuracy.

See the official [port notes](https://go.dev/doc/go1.25#ports) for details.

## Impact and validation for this repository

The upgrade is dependency-driven. Repository searches currently find no explicit `GOMAXPROCS`
configuration, direct `crypto/tls` use, `unsafe.Pointer` use, AddressSanitizer setup, or `GOAMD64`
override. The main practical effects are the newer toolchain requirement, the new vet checks, and
possible runtime behavior in applications that consume `dbz` and run under Linux CPU limits.

CI uses Go 1.27, so it already satisfies the Go 1.25 minimum. Local development and downstream CI
should use a supported patched Go 1.25 release or newer, keep automatic toolchain selection enabled
when appropriate, and ensure debuggers understand DWARF 5.

Validate future dependency or toolchain changes with:

```sh
go work sync
go fix ./...
go test ./...
go vet ./...
```

Run the commands from each workspace module when a tool does not traverse all `go.work` modules. For
deployed services, also compare effective `GOMAXPROCS` and latency under the production container
CPU limit, and test TLS connections to any legacy endpoints.

[bun-go-mod]: https://github.com/uptrace/bun/blob/v1.2.18/driver/sqliteshim/go.mod

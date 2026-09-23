# Firewizard Go Style Guide

Curated from [Effective Go](https://go.dev/doc/effective_go) and the
[Uber Go Style Guide](https://github.com/uber-go/guide/blob/master/style.md).
Apply to all new code; fix existing code opportunistically when touching it.

## Formatting

- use `make fmt` and rely on `make lint` which also enforces formatting
  (including gofumpt and goimports). No need to handle it manually.

## Naming

- Packages: short, lowercase, single word, no underscores or mixedCaps.
  The package name is the base name of its directory.
- Exported names avoid stutter: `bufio.Reader`, not `bufio.BufReader`.
- MixedCaps / mixedCaps, never `snake_case`.
- Getters omit `Get`: `obj.Owner()`, not `obj.GetOwner()`.
- One-method interfaces are `-er` verbs: `Reader`, `Writer`, `Prep` runner
  like our `ArticlePrepper`.
- Don't shadow builtins: never use `len`, `cap`, `new`, `copy`, `error`,
  `min`, `max` as identifiers.
- Prefix unexported globals with `_` and unexported globals never appear
  exported. Exported identifiers need doc comments.
- Be consistent: pick one name for a concept and stick with it everywhere.

## Doc comments

- Exported declarations get doc comments starting with the name of the
  declaration, e.g. `// Process runs all preprocessing steps.`
- Package comment at the top of any one file describing what the package is,
  not a pile of `// Package foo` on every file.

## Errors

- Error values end in `Err` (e.g. `ErrDirtyRepo`); error types end in
  `Error` (e.g. `PublishError`). Use one of these forms; don't mix.
- Wrap with `fmt.Errorf("%w: ...", err)` when adding context; use `%v` only
  when the type intentionally doesn't matter. Unwrap with `errors.Is` /
  `errors.As`, never string comparisons.
- Handle errors once: handle them or return them, not both (never
  `log.Info(err); return err`).
- Handle type assertion failures: always use the `, ok` form.
- Sentinel errors: `var ErrFoo = errors.New("...")` exported where callers
  need to test with `errors.Is`.
- log/slog: log errors at the site that decides, and the log is the source
  of truth (see AGENTS.md, "Debugging: read the log first").

## Function design

- Don't panic, always return (recover if needed, i.e. foreign libraries)
  errors!
- `main()` owns `os.Exit`/`os.ExitCode` through a `run()` function so exit
  codes are testable. Use `flag`-parsed args via a custom `run(args)` shape.
- Reduce nesting: keep the happy path left-aligned; handle errors with early
  returns. Avoid `else` after a `return`.
- Group similar declarations together (e.g. `const`/`var` blocks, receiver
  methods grouped below constructors) and respect decorder: within a file,
  declarations come in the order type, const, var, exported functions,
  unexported functions — `init()` always last.
- Omit, then group: variables `var x T` at function scope only when needed;
  inside bodies use short decl `:=` for non-zero values, `var` for zero.
- Don't export types whose fields aren't separable; avoid embedding in
  public structs — an embedded field is part of the public API.
- Local variables scoped to where they're used (`if err := x(); err != nil`).
- Keep APIs small, only export the minimal set needed. We can always export
  more later but unexporting is more complex.

## Structs

- Initialize with field names in public/multi-field structs; positional
  init allowed only for small, obvious private structs.
- Omit zero-value fields: rely on Go's default `0`/`""`/`false`.
- Use `var s SomeStruct` when you want the zero value.
- Pointer fields: only where sharing is on purpose; never wrap a `new(T)`
  when `var t T` works.
- Marshaled structs (JSON/YAML) always have explicit field tags.

## Interfaces & receivers

- Pass interfaces as values, never pointers to interfaces.
- Verify interface compliance at compile time: `var _ SomeIface = (*T)(nil)`.
- Receivers: value when you don't mutate; pointer when you mutate or the
  type is large. Be consistent — a type's methods either all pointer or
  all value receivers. Structs stored in maps need pointer receivers if
  you want to mutate them.

## Concurrency

- No pointers to mutexes; `var mu sync.Mutex` embedded as a named
  (non-embedded) field like `mu sync.Mutex`.
- Defer to clean up: locks, `Close`, `prep.Shutdown()` etc. — `defer
mu.Unlock()` right after `mu.Lock()`.
- Channels: unbuffered or size 1 unless you can justify more.
- No fire-and-forget goroutines: every goroutine must have a stop signal
  or a predictable end; wait for exit with `sync.WaitGroup` or a done
  channel. This matters in the TUI where Bubble Tea pumps events.
- Copy slices/maps at package boundaries you store or hand out.
- Lock only what you guard internally; exported mutex is a leak.

## Time

- `time.Time` for instants, `time.Duration` for periods — no `int`
  timestamps with the unit glued to the name unless forced to (then suffix
  like `IntervalMillis`).

## Performance

- `strconv` over `fmt.Sprintf` for conversions.
- Avoid `string([]byte(...))` in loops — cache conversions.
- Pre-size slices/maps (`make([]T, 0, n)`) when the size is known.

## Testing

- Only stdlib, no assert/testify/etc.
- Use modern (Go 1.27+) testing practices: `t.Context()`, `t.Setenv()`,
  `t.Cleanup()`, `t.ArtifactDir()`, `t.TempDir()`, etc.;
- Test functions come first in the file; any helper functions go at the
  end. Types are not helpers: they keep their normal decorder slot even
  when they only exist to support the tests.
- Test file mirror the tested file w/ respect to naming (foo.go gets foo_test.go);
- Table-driven tests with explicit `name` fields where cases > 1;
  Only ONE test function per tested function, all test functions
  ordered the same way the tested function are ordered in the
  corresponding file.
- Use functional options for config-heavy constructors.

## Project-specific

- TUI (Bubble Tea v2): keep event-handling logic in `Update`, keep model
  mutation local, and keep long work (git sync, builds, libvips) off the
  UI goroutine with message-based completion.
- Previews (`startPreviews`/`stopPreviews`): cleanup reachable on every
  exit path — use deferred unregistration.
- `internal/prep`: libvips starts lazily; `prep.Shutdown()` deferred from
  near main.
- Publish: the gating check is the fresh `siteSync` per attempt; do not
  cache failure.
- Generally no `init()` functions, except very rarely in libraries and NEVER
  when they return an error (prefer a Setup() error instead).
- No panic in library code.

## Verification

```console
make fmt   # gofumpt + goimports, applied in place
make lint  # golangci-lint with the repo's full config (gofumpt included)
go build ./... && go test ./...
```

`make lint` is the style gate: cyclop, decorder, err113, errname,
errorlint, funcorder, gochecknoglobals, gochecknoinits, gocritic, godot,
gosec, govet, lll, mnd, nilerr, nlreturn, noctx, paralleltest, perfsprint,
prealloc, recvcheck, varnamelen, wrapcheck, wsl and more are enforced by
config, not by prose. When this file and the linter disagree, the linter
wins — update the file, don't fight the config.

Run before considering work done.

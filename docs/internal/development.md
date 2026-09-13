# Development

## Build, test, lint

```bash
go build ./...
go vet ./...
go test ./...

make build/freepsd        # with version/commit ldflags
make build/freepsd-light  # connectors with external deps compiled out
```

The `Makefile` also has a `static_server_content` target that downloads two CSS/JS assets into
`connectors/http/static_server_content/`; they are required by `make build/freepsd` and are not
checked in.

## Repository layout

| Path | Contents |
|---|---|
| `base/` | operator framework: reflection, parameter binding, `OperatorIO`, `Context` |
| `freepsflow/` | flow engine, flow execution, built-in `flow`/`flowbytag`/`system`/`eval` operators, embedded flows |
| `connectors/` | one directory per operator |
| `connectors/ui/templates/` | the HTML templates of the web UI |
| `freepsd/` | `main`, plus `helper/` used by tests |
| `utils/` | config reader, case-insensitive map, conversions |
| `systemd/`, `scripts/` | service unit and update script |

## Adding an operator

1. Create `connectors/mything/opMyThing.go` with a struct named `OpMyThing` — the name becomes the
   URL prefix `mything`.
2. Write exported methods returning `*base.OperatorIO`. Any signature listed in
   [architecture.md](architecture.md#base--the-operator-framework) works; the common one is
   `(ctx *base.Context, input *base.OperatorIO, args MyArgs) *base.OperatorIO`.
3. Add `var _ base.FreepsOperator = &OpMyThing{}` so a wrong signature fails at compile time.
4. Register it in the `availableOperators` slice in `freepsd/freepsd.go`. Respect the ordering
   comment — store, alerts and sensors must come before operators that use them.
5. `curl localhost:8080/mything/<function>` — there is nothing else to wire up.
6. Run `make generate` to add the operator and its functions to the descriptions that
   `/flowbuilder/listOperators` and `/flowbuilder/listFunctions` serve. The generated file is
   committed, so the build works without this step — the descriptions are just missing.

To make it configurable, implement `FreepsOperatorWithConfig` and return a pointer to a config
struct from `GetDefaultConfig()`. The config section is read at startup, defaults are written back
into the config file, and a `"enabled": false` field disables the operator.

To give it a background goroutine or listener, implement `StartListening`/`Shutdown`.

### Conventions

- **Required arguments are plain fields, optional ones are pointers.** This is enforced by the
  framework, not a style choice.
- **Operator and function descriptions are the Go doc comments** on the operator type and its
  methods. `make generate` harvests them into
  `connectors/flowbuilder/operatorDescriptions_generated.go`; nothing reads them at runtime.
  Write them the usual Go way (`// Extract extracts the value of …`), the leading identifier is
  stripped for the API.
- **Argument descriptions go in an optional ``doc:"..."`` struct tag** on the parameter field.
  `GetArgumentDescriptions()` exposes them (with type and requiredness) for API and UI use.
  Tags are optional; add them where the argument name is not self-explanatory.
- **Return meaningful HTTP codes**: `base.MakeOutputError(404, ...)`, `400` for bad input, `417`
  for "a previous step failed". Never `panic`.
- **Value suggestions** for an argument come from, in this order:
  1. a ``options:"a,b,c"`` struct tag — use this for a fixed set of values, it needs no code:
     `Kind []string \`options:"manual,helper,event"\``
  2. a method on the *argument struct* named `<Field>Suggestions` returning `[]string` or
     `map[string]string` — use this when the values need labels or must be computed at runtime
     (flow names, sensor names, …). A method on the *operator* also works and serves all its
     functions.
  3. type defaults (`bool` → `true`/`false`, ints, durations for `int64` fields whose name
     contains time/duration/age).

  Suggestions feed the UI drop downs and `/flowbuilder/argDetails`; they are hints, nothing is
  validated against them. See `TestOptionsTag` in `base/argumentDescriptions_test.go`.
- **Never write to the config file directly** — go through `utils.ConfigReader`, which serializes
  and keeps backups.
- Flows and config are written with `WriteObjectToFile`, which renames the previous version to
  `<name>.<timestamp>.bak`. Do not bypass it.

## Testing

Use `freepsd/helper`, which builds a `FlowEngine` with a temp config dir and the common operators
(store, alert, sensor, metrics, utils):

```go
func TestSomething(t *testing.T) {
    ctx, ge, cr := helper.SetupEngineWithCommonOperators(t, nil)
    // config sections can be pre-seeded:
    // helper.SetupEngineWithCommonOperators(t, map[string]interface{}{"http": map[string]int{"port": 8081}})
}
```

Tests use `gotest.tools/v3/assert`. Look at `freepsflow/flow_test.go` for engine-level tests and
`connectors/flowbuilder/flowbuilder_test.go` for operator-level tests.

Operators with external dependencies keep a build-tagged stub (`*_dummy.go` with
`//go:build no<thing>`) so the default build and tests do not need the library.

## Gotchas

These have each cost someone time:

- **A method with a non-matching signature is silently skipped.** No error, no log line at default
  level — the function simply does not exist. Check the signature first when an endpoint 404s.
- **`GetFunctions()` is metadata, not an endpoint.** It exists to build UI forms and is never
  serialized as a response. Do not assume `GET /<operator>/` lists anything — for `flow` it is a
  404.
- **Legacy operators are case-sensitive.** `OpSystem` and friends use an exact `switch fn` on the
  raw URL segment, so `/system/GetFlowDescByTag` works and `/system/getFlowDescByTag` returns 400,
  even though operator and function lookup is otherwise case-insensitive. New operators should use
  the modern struct-parameter style and not inherit this.
- **A JSON POST body needs `Content-Type: application/json`.** With curl's default
  `application/x-www-form-urlencoded` the body is parsed into *arguments*, and the operator's input
  arrives empty.
- **Flow validation only checks structural references.** `${name}` interpolations are resolved at
  execution time, so a flow referencing a deleted operation saves fine and fails when run.
- **Store namespaces default to memory.** Any namespace not listed in the `store` config section
  is in-process only, including the `_flows` drafts.
- **`go test` and the daemon share no state**, but both write backups and temp files; the temp dir
  comes from `utils.GetTempDir()`.

## Releasing / versioning

Version, commit hash, branch and build time are injected via `-ldflags` in the Makefile into the
`utils` package and reported by `GET /system/version`. The version string comes from
`git describe --tags`.

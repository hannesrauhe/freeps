# Agent Rules

## Operator API best practice
- Argument documentation goes into the **`doc:"..."` struct tag** of the parameter struct field,
  NOT into a `//` comment above it. Only `doc:` tags are served by
  `/flowbuilder/operatorArgs` and `argDetails` (base/argumentDescriptions.go reads
  `Tag.Get("doc")`); line comments are invisible to the API.
- For an argument with a **fixed set of values**, list them in an **`options:"a,b,c"` struct tag**
  instead of writing a `<Field>Suggestions` method. The method stays the escape hatch for values
  that need per-value labels or must be computed at runtime.
- Keep the **function doc comment short** (1-3 lines: what it returns, notable defaults). The
  comment is exposed verbatim via `/flowbuilder/listFunctions`, so long comments bloat the
  metadata API. Details about individual arguments belong in the `doc:` tags.
- Example:
  ```go
  type ListFlowsArgs struct {
      Tags *string `doc:"comma separated tags, only flows with all of them are returned"`
  }

  // ListFlows returns all flows in the flow engine, filtered by Tags.
  func (m *OpFlowBuilder) ListFlows(...) *base.OperatorIO {
  ```
- After changing a function doc comment run `make generate` — the comments are mirrored into
  the committed `connectors/flowbuilder/operatorDescriptions_generated.go`. (`doc:` tags are
  read at runtime, no regeneration needed.)

## Editing
- Use the editor edit tools (string replace). No sed/awk/perl in the terminal for edits.
- Never run `go fmt`/`gofmt`. VS Code formats on save. Already done.

## Testing
- ALWAYS use the VS Code test tool first. If it reports 0/0 success, this usually indicates a build failure.
- Only fall back to `go test ./...` in the terminal if the tool says "No tests found" —
  that happens for tests created/changed during a session (discovery is stale until rescan).

## Terminal
- Manual daemon tests: port comes from the `curl` config section (OpCurl starts the HTTP
  server), not from `http`. Default 8080.

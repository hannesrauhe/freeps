# Agent Rules

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

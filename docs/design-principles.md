# Design principles

These decisions predate most of the code and explain why freeps looks the way it does. When in
doubt, optimize for these — a feature that violates one of them needs a good argument.

## The API is the product

Freeps started as one thing: **a pleasant API in front of the FritzBox.** The FritzBox does expose
an HTTP API, but it is awkward to use from anything else — its own authentication scheme, several
unrelated interfaces, and XML everywhere. The smart-home apps on top of it were slow and did not
integrate with anything.

What the FritzBox actually requires, and what [freepslib](https://github.com/hannesrauhe/freepslib)
hides:

- **Challenge/response login.** `GET /login_sid.lua` returns an XML challenge; the client must
  MD5-hash `"<challenge>-<password>"` encoded as UTF-16LE and ask again to receive a session SID.
  There is no plain basic auth to reuse.
- **XML responses** parsed into structs (`encoding/xml`) rather than JSON.
- **Several different APIs** for different device types — `homeautoswitch.lua` for the AVM plugs
  and thermostats, UPnP for the rest — each with its own calling convention.

Freeps turns all of that into something like `GET /fritz/setSwitch?switchid=…&active=true`.
Everything else in the project grew out of that: the REST-first shape, the meaningful status codes,
the "one curl call should be enough" attitude, and the `-m` one-shot CLI mode.

**Consequence:** when adding a connector, the job is to make a messy external system look like the
rest of freeps — not to pass its API through.

## Lean and small

Freeps is meant to run on a first or second generation Raspberry Pi. Early versions of the binary
were under 1 MB; today the full build is around **21 MB** (about 17 MB for
`make build/freepsd-light`, 15 MB stripped). That is still a single static binary with no runtime
dependencies, but the original target is long gone — treat size as a cost to keep in check, not a
badge.

The growth is almost entirely third-party code — a bare Go binary that imports `net/http` is
already ~5 MB. Measured savings from the individual build tags: `noinflux` 2.0 MiB, `nopostgres`
0.9 MiB, `nobluetooth` 0.8 MiB, `notelegram` 0.3 MiB, `nomuteme` 0.1 MiB. So the influxdb client is
by far the heaviest dependency, and the tags only add up to ~4 MiB of the ~16 MiB above the floor;
the rest is spread over the standard library and the remaining connectors (`golang.org/x/image`
comes in unconditionally via `pixeldisplay`, which has no build tag).

This is what the build tags are for — and why a new connector should think twice before adding a
heavy dependency to the *default* build.

Consequences in the code:

- **One static binary, no services.** No database, no broker, no external runtime. The store
  defaults to memory and only grows a backend (files, postgres) if you configure one.
- **Dependencies are opt-in at compile time.** Anything that pulls in a system library or a heavy
  Go dependency sits behind a build tag (`nopostgres`, `nobluetooth`, `noinflux`, `notelegram`,
  `nomuteme`, `noexec`) with a `_dummy.go` stub, so the default build stays small and portable.
- **The FritzBox protocol lives in a separate library**
  ([freepslib](https://github.com/hannesrauhe/freepslib)), not in this repo.
- **Config is one JSON file** read and written by a small hand-rolled reader — no config framework.

## Runs without internet access

Freeps is local-first. A complete setup — flows, store, UI, FritzBox control, MQTT, sensors — works
on a network with no uplink, and there is **no telemetry and no phone-home** anywhere in the code.

Consequences in the code:

- **No CDN.** The two frontend assets (`chota.min.css`, `screenfull.min.js`) are vendored and
  embedded in the binary; the UI loads nothing from a third party.
- **No update check, no usage reporting.** The `update-freeps.sh` script is opt-in and builds from
  your own source.
- **Nothing is fetched at startup** beyond what a configured connector explicitly asks for.

The exceptions are connectors whose subject matter is remote by nature — `telegram`
(api.telegram.org) and `weather` (OpenWeatherMap). Both are optional: `telegram` can be compiled out
with the `notelegram` build tag, and either can be switched off with `"enabled": false`. Everything
else keeps working when the uplink drops.

## No authentication, on purpose

Freeps accepts **unauthenticated requests from any source**: HTTP, SMTP, MQTT, Bluetooth,
Telegram, sensors. There are no users, no sessions, no tokens, no auth middleware — and adding
them is not on the roadmap.

The security model is **the network**: freeps belongs on a trusted home LAN (behind the FritzBox),
and whatever can reach the network is trusted to control the house. Do not expose `freepsd` to the
internet.

Consequences in the code:

- The single catch-all route dispatches straight into operators; there is no hook where auth *would*
  go.
- Operators answer with plain HTTP status codes and are meant to be callable from a `curl` one-liner
  or a cron job.
- The `-m` one-shot CLI mode uses the same execution path as an HTTP request — the API is designed
  to be trivially scriptable, which is the point.

Note the contrast with *The API is the product* above: freeps *does* speak credentials — to the
FritzBox, MQTT brokers, SMTP servers and the weather API, wherever the remote system demands them.
It just never demands them from its own callers.

If you need protection in front of freeps, put a reverse proxy with authentication in front of it —
that is outside freeps by design.

## Extensibility with minimal boilerplate

Writing an operator should mean writing a Go method, nothing else.

Consequences in the code:

- `FreepsOperator` is an **empty marker interface**; `MakeFreepsOperators` finds the functions by
  reflection. No registration calls, no code generation, no IDL, no route table.
- The method signature *is* the API contract: struct fields become arguments, a pointer field means
  optional, the returned `OperatorIO` carries the HTTP status and content type.
- One method automatically serves three transports — HTTP endpoint, flow operation, and `-m` CLI
  call — because all three funnel into `ExecuteOperatorByName`.
- Capability interfaces (`FreepsOperatorWithConfig`, `FreepsOperatorWithDynamicFunctions`,
  `FreepsOperatorWithShutdown`, …) add features by *implementing more methods*, never by changing
  existing ones.

The trade-off is accepted knowingly: a wrong method signature fails silently at startup instead of
at compile time. The `var _ base.FreepsOperator = &OpMyThing{}` idiom and the UI's function list are
the mitigations.

## A bare UI

The web UI exists to support the engine (flow editing, store browsing, config), not to be a product.
It should cost almost nothing on the server and on weak clients.

Consequences in the code:

- **Server-side `text/template` rendering only.** No JavaScript framework, no client build step, no
  API client. Two small vendored assets (a CSS file, one JS helper) and that's the whole frontend.
- **The server is stateless between requests.** The flow editor keeps its state in a hidden
  `FlowJSON` form field that travels with every request; there is no server-side editor session.
- No websockets, no polling — the editor's output pane is an `<iframe>` pointed at the execute URL.
- Templates are plain files that users can override per installation (`<config dir>/templates/`) and
  edit from the UI itself.

A prettier UI that needs a node build pipeline or a SPA framework contradicts the purpose; the
templates are meant to be readable enough to just edit.

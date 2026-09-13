# Flows

A flow is a named, reusable sequence of operator calls, written as JSON. It is the answer to
"one trigger should cause several things to happen".

## Anatomy

```json
{
  "DisplayName": "Evening lights",
  "Tags": ["ui", "tile"],
  "OutputFrom": "dim",
  "Operations": [
    {
      "Name": "isNight",
      "Operator": "time",
      "Function": "isNight",
      "Arguments": { "latitude": "52.5", "longitude": "13.4" }
    },
    {
      "Name": "setLights",
      "Operator": "curl",
      "Function": "get",
      "InputFrom": "isNight",
      "Arguments": { "url": "http://lights.local/evening" }
    },
    {
      "Name": "dim",
      "Operator": "utils",
      "Function": "echo",
      "InputFrom": "setLights",
      "Arguments": { "output": "done" }
    }
  ]
}
```

Top level fields:

| Field | Meaning |
|---|---|
| `DisplayName` | Shown in the UI; defaults to the flow ID |
| `Description` | Optional free-text description of what the flow does; returned by `listFlows` and `getFlow` |
| `Kind` | Who invokes the flow: `manual` (default), `helper` or `event`. Descriptive only — see [Kind](#kind-manual-helper-or-event) |
| `Tags` | Free-form labels, used for triggering and for UI grouping |
| `OutputFrom` | Which operation's output is the flow's output |
| `Operations` | The operations, in execution order |
| `FlowID` | Assigned by the engine from the filename — do not set it |
| `Source` | Set by the loader (`embedded`, `file`, …) |

## There are no edges

A flow is a **flat array executed in order**. Connections between operations are expressed by
*referencing the name of an earlier operation*:

| Field | Effect |
|---|---|
| `InputFrom` | Pipe the output of that operation into this one as its input |
| `ArgumentsFrom` | Take all arguments of this operation from that operation's output map |
| `UseMainArgs` | Merge the arguments the flow itself was called with into this operation |
| `ExecuteOnSuccessOf` | Only run if that operation succeeded |
| `ExecuteOnFailOf` | Only run if that operation failed |

`InputFrom: "_"` refers to the flow's own input.

If an operation's `InputFrom` produced an error, the operation is skipped and the error travels to
the end of the flow — which is why `ExecuteOnFailOf` is how you write "on error, do this instead".

Operation names are optional and default to `#0`, `#1`, … The name `_` is reserved.

**An operation may only reference operations defined before it in the array.** Reordering an array
element so it references a later one is a validation error.

## Arguments and variables

`Arguments` is a map of **strings**. Values may interpolate earlier outputs:

```json
"Arguments": { "output": "Hello ${name}!" }
```

- `${opName}` — the whole output of that operation as a string
- `${opName.key}` — one field from an operation whose output is an object

Interpolation happens at execution time and is **not validated when the flow is saved**: a
reference to a name that does not exist fails when the flow runs, not when you create it.

## Output

If `OutputFrom` is set, that operation's output becomes the flow's output. Otherwise the flow
returns an object with every operation's output keyed by its name.

## Timeouts

| Limit | Default | Override with tag |
|---|---|---|
| Whole flow | 2 minutes | `flowTimeout:5m` |
| Single operation | 1 minute | `operationTimeout:10s` |

Durations are Go durations (`30s`, `5m`, `1h30m`).

## Storing flows

Live flows are JSON files in **`<config dir>/graphs/<flowID>.json`** — the flow ID is the
filename. Editing that file and restarting works, but the API below is safer because it validates.

There is a second, separate place: **drafts** in the store namespace `_flows`. The web UI edits
drafts, and "Execute" runs a draft without registering it. Drafts are **in-memory by default** and
do not survive a restart unless you configure `_flows` as a file-backed namespace.

## Creating flows programmatically

All of these are ordinary REST calls (see [the REST API](rest-api.md)).

### In one call

The flow definition goes in the request **body** — remember the JSON content type:

```bash
curl -X POST 'localhost:8080/flowbuilder/createFlow?flowID=myflow' \
  -H 'Content-Type: application/json' \
  --data-binary '{
    "DisplayName": "my flow",
    "Operations": [{"Operator": "utils", "Function": "noop"}]
  }'
```

The flow is validated, registered in the engine and written to `graphs/myflow.json`, so it is
immediately executable with `GET /flow/myflow` and survives a restart. Add `&overwrite=true` to
replace an existing flow.

### Step by step

For building or patching a flow incrementally, use the operation-level endpoints. Pass
`live=true` to work on the flow in the engine; without it they work on the draft in the store.

```bash
# a skeleton — OutputFrom must reference an operation that already exists,
# so start with a placeholder and patch it later
curl -X POST 'localhost:8080/flowbuilder/createFlow?flowID=myflow' \
  -H 'Content-Type: application/json' \
  --data-binary '{"OutputFrom":"greeting","Operations":[
     {"Name":"args","Operator":"utils","Function":"echoArguments","UseMainArgs":true},
     {"Name":"greeting","Operator":"utils","Function":"noop"}]}'

# insert an operation at position 1
curl -X POST localhost:8080/flowbuilder/addOperation \
  --data-urlencode flowName=myflow --data-urlencode live=true \
  --data-urlencode operationNumber=1 --data-urlencode operationName=name \
  --data-urlencode operator=utils --data-urlencode function=extract \
  --data-urlencode inputFrom=args

# set one argument at a time
curl -X POST localhost:8080/flowbuilder/setOperation \
  --data-urlencode flowName=myflow --data-urlencode live=true \
  --data-urlencode operationNumber=1 \
  --data-urlencode argumentName=key --data-urlencode argumentValue=name

# turn the placeholder into the final operation
curl -X POST localhost:8080/flowbuilder/setOperation \
  --data-urlencode flowName=myflow --data-urlencode live=true \
  --data-urlencode operationNumber=2 \
  --data-urlencode operator=utils --data-urlencode function=echo
curl -X POST localhost:8080/flowbuilder/setOperation \
  --data-urlencode flowName=myflow --data-urlencode live=true \
  --data-urlencode operationNumber=2 \
  --data-urlencode argumentName=output --data-urlencode 'argumentValue=Hello ${name}!'

curl 'localhost:8080/flow/myflow?name=World'      # -> Hello World!
```

`removeOperation` takes `flowName`, `operationNumber` and `live`.

### Drafts and promoting

```bash
curl -X POST 'localhost:8080/flowbuilder/getFlowFromStore?flowName=draft&CreateIfMissing=true'
# ... addOperation / setOperation without live=true ...
curl -X POST localhost:8080/flowbuilder/executeFlowFromStore?flowName=draft   # test it
curl -X POST localhost:8080/flowbuilder/promoteFlow --data-urlencode flowName=draft   # make it live
```

`promoteFlow` is the step that validates and persists; it accepts `storeName` (if the draft is
stored under a different key) and `overwrite`.

### Reading and deleting

```bash
curl 'localhost:8080/flowbuilder/getFlow?flowID=myflow'
curl 'localhost:8080/flowbuilder/listFlows'
curl 'localhost:8080/flowbuilder/listFlows?details=true'
curl 'localhost:8080/flowbuilder/listFlows?tags=cron'
curl 'localhost:8080/flowbuilder/listFlows?kind=manual'
curl -X POST 'localhost:8080/flowbuilder/deleteFlow?flowID=myflow'
```

`deleteFlow` keeps a backup in the store as `deleted_<flowID>`;
`restoreDeletedFlowFromStore?flowName=<flowID>` puts it back.

## Tags and triggers

Tags are how things *call* your flow without anyone naming it in code. A connector looks for flows
with certain tags and executes them.

```bash
curl 'localhost:8080/flowbytag/<tag>'                       # all flows with that tag
curl 'localhost:8080/flowbytag/<tag>?additionalTags=a,b'    # must have all of them
```

Tags of the form `key:value` are matched on the key, and `key:*` matches any value.

Connectors use tags like these:

| Source | Tags |
|---|---|
| MQTT message | `mqtt`, `topic:<topic>` |
| Telegram command | `telegram` |
| Incoming e-mail | `smtp`, `sender:<from>` (and per recipient `smtp`, `to:<recipient>`) |
| Sensor value change | `sensor`, `sensorCategory:<c>`, `sensorName:<n>`, `sensorProperty:<p>` |
| FritzBox device change | the fritz instance name (default `fritz`) |
| Bluetooth discovery | `bluetooth`, `discovered` |
| Alert raised / cleared | `alert` plus `severity:<n>` and `set:<alert>` or `reset:<alert>` |

Each of these has a function that adds the right tags to a flow for you, so you do not have to
remember the tag syntax:

```bash
curl -X POST 'localhost:8080/mqtt/setTopicTrigger?flowID=myflow&topic=home/temperature'
curl -X POST 'localhost:8080/sensor/setSensorTrigger?flowID=myflow&sensorName=livingroom'
curl -X POST 'localhost:8080/telegram/setTopicTrigger?flowID=myflow'
curl -X POST 'localhost:8080/smtp/setSenderTrigger?flowID=myflow&sender=fritzbox@fritz.box'
curl -X POST 'localhost:8080/fritz/setHostActiveTrigger?flowID=myflow&macAddress=AA:BB:CC:DD:EE:FF'
```

The `smtp` connector is an SMTP **server** (port 2525 by default) that freeps listens on, not a way
to send mail — it exists so devices that cannot call an API can still report events by e-mailing
freeps. See [the FritzBox's missing push mechanism](../design-principles.md#the-smtp-connector-exists-because-the-fritzbox-cannot-call-back).

The UI reserves some tags for itself: `ui,tile` shows a flow as a tile on the dashboard,
`ui,footer` puts a link in the footer.

## Kind: manual, helper or event

Where tags say *how* a flow gets triggered, `Kind` says *who* is meant to invoke it. It is purely
descriptive metadata — it never changes execution, triggering stays driven by tags alone. It exists
so a UI or a script can tell "switch on the light" apart from a plumbing flow nobody calls by hand.

| Kind | Meaning |
|---|---|
| `manual` | Meant to be invoked by a human, e.g. "switch on the light". This is the default when `Kind` is empty, so flows written before the field existed keep their meaning. |
| `helper` | Only called by other flows (shared sub-steps). |
| `event` | Triggered by an event source — a connector or cron, usually via tags. |

Set it without touching the operations:

```bash
curl -X POST 'localhost:8080/flowbuilder/setFlowKind?flowName=myflow&kind=manual&live=true'
curl -X POST 'localhost:8080/flowbuilder/setFlowDescription?flowName=myflow&description=Switch+on+the+light&live=true'
```

Filter by it (`kind` is a repeatable argument, ANDed with `tags`):

```bash
curl 'localhost:8080/flowbuilder/listFlows?kind=manual'                  # only the human-facing actions
curl 'localhost:8080/flowbuilder/listFlows?kind=manual&kind=helper'      # several kinds
```

### Scheduling

**There is no scheduler inside freeps.** The embedded `garbageCollect` flow carries the tags
`system,cron,hourly` as a *convention* — something external has to call it. Use the system cron:

```cron
0 * * * * curl -s localhost:8080/flowbytag/hourly
```

## Listing flows

```bash
curl 'localhost:8080/flowbuilder/listFlows'                 # brief description of all flows
curl 'localhost:8080/flowbuilder/listFlows?details=true'    # full definitions incl. operations
curl 'localhost:8080/system/GetFlowDescByTag'               # same, legacy spelling is case-sensitive
curl 'localhost:8080/ui/flowInfo.html'                      # human readable
```

`GET /flow/` is **not** a listing — it is an attempt to execute a flow with an empty name and
returns a `400` that lists the available flow names.

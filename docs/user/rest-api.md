# The REST API

Freeps has **no hand-written routes**. Every operator that is enabled in the config is
automatically reachable over HTTP, and every function of that operator becomes an endpoint.

Freeps also has **no authentication at all** — this is deliberate, see
[design principles](../design-principles.md#no-authentication-on-purpose).

The URL scheme is always:

```
/<operator>/<function>?arg=value
```

So `POST /store/setSimpleValue?key=foo&value=bar` calls function `setSimpleValue` of operator
`store`. Operator and function names are matched **case-insensitively**.

## Finding out what is available

The operator metadata that drives the UI is available over HTTP as a four level ladder —
one endpoint per level, so a client can walk from operators down to argument details:

```bash
curl 'localhost:8080/flowbuilder/listOperators'                                # ["Utils", ...]
curl 'localhost:8080/flowbuilder/listFunctions?operator=utils'                 # ["Extract", ...]
curl 'localhost:8080/flowbuilder/operatorArgs?operator=utils&function=extract' # ["Key", "Type", ...]
curl 'localhost:8080/flowbuilder/argDetails?operator=utils&function=extract'
```

`argDetails` returns one object per argument with name, type, requiredness, description (from
the optional `doc` struct tag) and value suggestions — exactly the drop-down contents the flow
editor shows. It accepts an optional `otherArgs` argument in URL query format (e.g.
`otherArgs=namespace=testing`), which is passed to the suggestion functions so they can return
context sensitive suggestions.
Operator and function names are matched case-insensitively; an unknown operator is a 404.

In addition:

- **The flow editor at `/ui/editFlow`** shows, for the selected operation, a button for every
  operator and every function, and the argument names for the selected function. This is the
  quickest way to learn an API interactively.
- **`/ui/flowInfo.html`** lists all flows, `/ui/store.html` the store namespaces,
  `/ui/editconfig.html` the config, `/ui/alerts.html` and `/ui/sensors.html` the current alerts
  and sensor values.
- **`GET /config/getSection?sectionName=http`** returns one config section as JSON.
- The source is the ground truth: an operator's exported methods *are* its endpoints.

Which operators are reachable depends on which are enabled in the config — a section with
`"enabled": false` removes the operator and all of its endpoints.

## Arguments

Arguments come from the URL query string. For `POST` requests there are three conventions:

| Request | Result |
|---|---|
| Query string | Always used as arguments |
| `application/x-www-form-urlencoded` body | Form fields become **arguments** (if the query string had none) |
| Raw body with any other content type | Becomes the **input** of the operator |
| Form with fields `input` + `input-content-type` | Those become the input, the rest become arguments |

This matters: a JSON body sent with the default curl content type is treated as a form, not as
input. Send JSON explicitly:

```bash
curl -X POST 'localhost:8080/flowbuilder/createFlow?flowID=myflow' \
  -H 'Content-Type: application/json' \
  --data-binary '{"Operations":[{"Operator":"utils","Function":"noop"}]}'
```

Which of the two a function expects is visible in the UI form: a function that wants a body
(e.g. `createFlow`, `store/set`) reads the *input*, the others read *arguments*.

## Responses

Operators answer with an HTTP status code and a content type of their own choosing, so error
codes are meaningful:

| Code | Meaning |
|---|---|
| 200 | Success |
| 400 | Bad arguments, or a validation error — the body contains the message |
| 404 | Unknown flow / unknown key |
| 417 | A previous operation in a flow failed |
| 500 | Internal error |

Every response carries an `X-Freeps-ID` header with the request's trace ID, which you can find in
the log.

A `?redirect=<url>` argument turns a *successful but empty* response into a `302`, which is how
the UI chains actions and then shows a page.

## Executing flows

```bash
curl 'localhost:8080/flow/<flowID>?arg=value'      # run one flow
curl 'localhost:8080/flowbytag/<tag>'              # run all flows carrying a tag
```

Note that `GET /flow/` (with an empty flow name) is a 404, not a listing — see
[flows](flows.md#listing-flows) for how to list them.

## The flowbuilder API

`flowbuilder` is the operator for creating and modifying flows over HTTP. See
[flows](flows.md#creating-flows-programmatically) for the full description and examples.

| Endpoint | Purpose |
|---|---|
| `POST /flowbuilder/createFlow?flowID=…` | Create/replace a flow from a JSON body, directly in the engine |
| `GET /flowbuilder/getFlow?flowID=…` | Read a flow from the engine |
| `GET /flowbuilder/listFlows[?tags=a,b]` | List flows with their definitions |
| `POST /flowbuilder/deleteFlow?flowID=…` | Delete a flow (a backup is kept in the store) |
| `POST /flowbuilder/addOperation` | Insert an operation |
| `POST /flowbuilder/setOperation` | Change one operation |
| `POST /flowbuilder/removeOperation` | Delete one operation |
| `POST /flowbuilder/promoteFlow` | Copy a draft from the store into the engine |
| `GET /flowbuilder/executeFlowFromStore?flowName=…` | Run a draft without registering it |
| `GET /flowbuilder/listOperators` | Names of all registered operators |
| `GET /flowbuilder/listFunctions?operator=…` | Functions of one operator |
| `GET /flowbuilder/operatorArgs?operator=…&function=…` | Argument names of one function |
| `GET /flowbuilder/argDetails?operator=…&function=…` | Name, type, requiredness, description and suggestions of every argument |

## The web UI

| URL | Contents |
|---|---|
| `/ui/` | Flow editor home |
| `/ui/editFlow?flow=<id>` | Edit one flow |
| `/ui/<template>` | Render any bundled or custom template |
| `/ui/simpleTile?header=…&button_On=<flow>&button_Off=<flow>` | A single button tile |

Templates live in `connectors/ui/templates/` and can be overridden per installation by putting a
file of the same name into `<config dir>/templates/`.

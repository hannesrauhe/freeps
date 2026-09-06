# Freeps Documentation

Read [Design principles](design-principles.md) first if you want to understand *why* freeps is
built the way it is — its origin as a friendly API in front of the FritzBox's XML and
challenge-auth interfaces, and the deliberate choices that came from it: no authentication, no
internet dependency, no framework, no frontend build.

Documentation is split into two parts:

## [User documentation](user/)

For people who *run* freeps and want to configure it, call its API, or write flows.

| Document | Contents |
|---|---|
| [Getting started](user/getting-started.md) | Install, run, config file, command line flags, build tags |
| [The REST API](user/rest-api.md) | How operators become endpoints, calling them, the `flowbuilder` API |
| [Flows](user/flows.md) | Flow definitions, operations, arguments, variables, tags and triggers |

## [Internal documentation](internal/)

For people who want to *change* freeps or understand how it works internally.

| Document | Contents |
|---|---|
| [Architecture](internal/architecture.md) | Operator framework, the dynamic REST API, flow engine, storage |
| [Development](internal/development.md) | Building, testing, conventions, adding an operator |

## Where to start

- You want to control your FritzBox smart devices from a script → [The REST API](user/rest-api.md)
- You want to chain several actions together → [Flows](user/flows.md)
- You want to add a connector to freeps → [Development](internal/development.md)

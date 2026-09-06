# Getting started

## What freeps does

`freepsd` is a single Go binary that runs a small HTTP server and connects it to a set of
"connectors" (FritzBox, MQTT, Telegram, sensors, a key/value store, …). Everything is driven
through a plain REST API, and multi-step logic is expressed in JSON flows.

It runs standalone — no database, message broker or other service is required, and no internet
access: everything except the optional `telegram` and `weather` connectors works on a local network
without uplink. The result is a single static binary (around 21 MB, less with build tags) that
targets hardware as modest as a first generation Raspberry Pi, see
[design principles](../design-principles.md).

## Build and run

```bash
make build/freepsd        # full build, needs the system libraries listed below
./build/freepsd
```

The server listens on **port 8080** by default (configurable in the `http` config section).

A lighter build that compiles out the connectors with external dependencies:

```bash
make build/freepsd-light
```

## Install as a service

```bash
make install
```

This creates a `freeps` user, installs the systemd unit, and puts the binary in
`/usr/local/freeps/bin`. The service reads its config from `/etc/freepsd/config.json`.

## Configuration

The config file is a single JSON object whose first-level keys are *section names*:

```json
{
  "http": {
    "port": 8080,
    "enablePprof": false,
    "flowProcessingTimeout": 0
  },
  "fritz": {
    "Address": "fritz.box",
    "User": "freeps",
    "Password": "..."
  }
}
```

Default location is `<user config dir>/freeps/config.json`
(on Linux usually `~/.config/freeps/config.json`). Use `-c` to pick another file.

Two behaviours are worth knowing:

- **Missing sections are created with defaults.** On startup freeps writes the default config of
  every operator back into the file, so the file grows into a complete, self-documenting config.
- **A section with `"enabled": false` disables that operator** and it will not be registered at all.

### Multiple instances of one connector

Some connectors can be instantiated more than once. Add extra sections with a `.` suffix and the
instance gets its own name — and its own URL prefix:

```json
{
  "http":            { "port": 8080 },
  "http.internal":   { "port": 8081 }
}
```

## Command line flags

| Flag | Meaning |
|---|---|
| `-c <path>` | Config file to use |
| `-m <operator>` | Run one operator function and exit, without starting the server |
| `-f <function>` | Function to call with `-m` |
| `-a <args>` | Arguments as a URL-encoded query string |
| `-i <file>` | Input file, `-` reads stdin |
| `-v` | Verbose (debug) logging |

One-shot mode uses exactly the same execution path as an HTTP request, which makes it useful for
cron jobs and for testing:

```bash
freepsd -m store -f setSimpleValue -a 'key=foo&value=bar'
echo '{"temp":21.5}' | freepsd -m utils -f extract -a 'key=temp&type=float' -i -
```

## Build tags

Compile out connectors that need system libraries or that you do not want:

| Tag | Removes |
|---|---|
| `nobluetooth` | Bluetooth device discovery |
| `noexec` | Running configured external programs (Linux only anyway) |
| `noinflux` | InfluxDB metrics |
| `nomuteme` | MuteMe USB button |
| `nopostgres` | Postgres backend for the store |
| `notelegram` | Telegram bot |

```bash
go build -tags nopostgres,nobluetooth -o build/freepsd freepsd/freepsd.go
```

## USB button support

The MuteMe button needs libudev/libusb and a udev rule:

```bash
apt install libudev libusb1-dev
```

```
# /etc/udev/rules.d/50-usb-button.rules
SUBSYSTEMS=="usb", ATTRS{idVendor}=="20a0", ATTRS{idProduct}=="42da", GROUP="users", MODE="0666"
```

```bash
udevadm control --reload-rules && udevadm trigger
```

## Next steps

- [Call the API](rest-api.md)
- [Write flows](flows.md)

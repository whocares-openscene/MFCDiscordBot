# MFCDiscordBot — Combined Go Bot

A single Go service that both monitors MyFreeCams (MFC) over its websocket
and runs the Discord bot. It replaces the previous two-process setup:

- the Node.js `MFCDiscordBot` (Discord commands + polling), and
- the Go `adapter-mfc` microservice from `siren` (MFC websocket + `/online`).

The MFC websocket protocol implementation was ported from `siren/cmd/adapter-mfc`,
including frame parsing, bulk/session-state/room-data handling, username
lookups, keepalives, reconnects, and the bulk watchdog.

## Features

- Discord slash commands: `/addmodel`, `/updatemodel`, `/removemodel`,
  `/getmodelid`, `/listmodels`
- SQLite storage of watched models (compatible with the existing `mfcbot.db`)
- In-process MFC online monitoring over the real FCS websocket protocol
- Discord notifications on public/group/club shows and offline transitions
- HTTP endpoints: `/online`, `/status`, `/healthz`, `/version`

## Why one process?

The old bot polled `http://mfcgo/online` every two minutes and looked up
usernames through MFC's HTTP API. The combined bot keeps the online snapshot
in memory, matches watched models by MFC `uid`, and notifies directly. Only
one container and one database connection are needed.

## Building

```bash
go build -o mfcbot ./cmd/mfcbot
```

Or with Docker:

```bash
docker build -t mfcbot .
# Headless (no HTTP listener):
docker run --env-file .env -v "$PWD/mfcbot.db:/app/mfcbot.db" mfcbot
# With the HTTP API:
docker run --env-file .env -e LISTEN_ADDRESS=0.0.0.0:80 -p 80:80 \
    -v "$PWD/mfcbot.db:/app/mfcbot.db" mfcbot
```

## Running

```bash
./mfcbot
```

The process connects to MFC, connects to Discord, registers the slash
commands, and then waits for `SIGINT`/`SIGTERM`. The HTTP server is only
started when `LISTEN_ADDRESS` is set; everything else works headless.

### Running without Docker

Only the compiled binary is needed. From a directory containing `mfcbot`:

```bash
./mfcbot
```

At minimum the process needs:

- the `mfcbot` binary
- a writable directory for `mfcbot.db` (created automatically if absent)
- a `.env` file **or** `TOKEN` in the environment
- system CA certificates so HTTPS calls to MFC and Discord can be verified
  (`ca-certificates` on Debian/Ubuntu/Alpine)

A typical bare-metal layout is:

```
/opt/mfcbot/
├── mfcbot        # compiled binary
├── .env          # TOKEN=... (loaded automatically when present)
└── mfcbot.db     # watch list (created if missing; copy an existing one to keep it)
```

To expose the HTTP API, add `LISTEN_ADDRESS` to `.env` (for example
`LISTEN_ADDRESS=127.0.0.1:3000`). Leave it out to run without any open port.

The source tree (`cmd/`, `internal/`, `go.mod`, `go.sum`) is only needed to
build the binary. `legacy-node/`, `example.env` and `Dockerfile` are not
needed at runtime.

To run under systemd, use `WorkingDirectory=/opt/mfcbot` and either keep the
`.env` file there or set variables with `EnvironmentFile=`.

## Configuration

Copy `example.env` to `.env` and fill in `TOKEN`. When `.env` is present in
the working directory it is loaded automatically; real environment variables
take precedence over values in the file. Configuration is read from
environment variables:

| Variable | Default | Description |
| --- | --- | --- |
| `TOKEN` | — | Discord bot token (required) |
| `CLIENTID` | — | Discord application id (informational) |
| `LISTEN_ADDRESS` | unset | HTTP listen address. When unset, no port is bound and the HTTP API is disabled. |
| `DATABASE_PATH` | `mfcbot.db` | SQLite database path |
| `ONLINE_CHECK_INTERVAL` | `2m` | How often to compare models with the snapshot |
| `TIMEOUT_SECONDS` | `10` | HTTP client timeout |
| `MAX_SNAPSHOT_SIZE` | `20000` | Maximum online entries accepted from MFC |
| `WS_CONNECT_TIMEOUT` | `30s` | Websocket dial timeout |
| `WS_IDLE_TIMEOUT` | `30s` | Reconnect if no frames arrive for this long |
| `BULK_ARRIVAL_TIMEOUT` | `60s` | Reconnect if the initial bulk never arrives |
| `RECONNECT_BACKOFF_MAX` | `1m` | Maximum reconnect backoff |
| `EXT_DATA_FETCH_TIMEOUT` | `30s` | Timeout for MFC bulk payload fetches |
| `LOOKUP_TTL` | `5m` | Username-lookup dedupe window |
| `NAME_CACHE_TTL` | `24h` | How long resolved usernames are cached |
| `DEBUG` | `false` | Debug logging |
| `TRACE` | `false` | Trace logging |

## HTTP API

These endpoints are available only when `LISTEN_ADDRESS` is set (for example
`LISTEN_ADDRESS=0.0.0.0:80` or `LISTEN_ADDRESS=:3000`). Without it the bot
runs headless and none of the endpoints are served.

| Endpoint | Description |
| --- | --- |
| `GET /online` | Current online snapshot as JSON. `failed: true` until the first bulk dump arrives. |
| `GET /status?name=<username>` | Live MFC status lookup: 503 with no session, 504 on timeout. |
| `GET /healthz` | Liveness probe (`ok`). |
| `GET /version` | Version string. |

## Database compatibility

The existing `mfcbot.db` schema is preserved:

```sql
CREATE TABLE models (
    id INTEGER PRIMARY KEY,
    modelid INTEGER,
    modelname VARCHAR(75) NOT NULL,
    message TEXT NOT NULL,
    time INTEGER,
    channel VARCHAR(75),
    updatetime INTEGER,
    topic TEXT DEFAULT "",
    events INTEGER
);
```

`modelid` stores the numeric MFC uid, `time` stores the unix-millisecond
timestamp of the last public-show notification (0 when offline), and `topic`
stores `""`, `"2"` (group), `"3"` (club), or `"Public"`.

## Legacy files

The original Node.js implementation has been moved to `legacy-node/` and is
kept for reference only. It is no longer used or built; the Go bot in
`cmd/mfcbot` is the single supported implementation. See
`legacy-node/README.md` for the archived file list.

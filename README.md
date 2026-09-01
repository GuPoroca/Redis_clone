# Redis Clone in Go

A small Redis-compatible server built from scratch in Go. The project is a
learning-focused implementation of the pieces behind a networked data store:
TCP connections, the RESP2 protocol, concurrent access to shared state, key
expiry, and snapshot persistence.

It is intentionally not a replacement for Redis. It currently stores string
values in memory and implements a focused set of commands that can be used
from `redis-cli`.

## What is implemented

- A TCP server that accepts multiple client connections concurrently
- RESP2 parsing and serialization
  - Simple strings, errors, integers, bulk strings, and arrays
  - Null and empty bulk strings/arrays
  - Nested arrays and back-to-back values on a single stream
  - Input validation for malformed lengths, incomplete input, invalid type
    bytes, oversized values, and invalid bulk-string terminators
- String commands: `PING`, `ECHO`, `SET`, `GET`, `DEL`, and `EXISTS`
- Key expiry commands: `EXPIRE`, `TTL`, and `PERSIST`
  - Lazy expiry during reads
  - Active expiry sweep every 100 ms
- JSON snapshot persistence
  - Loads a snapshot when the server starts
  - Saves periodically (30 seconds by default)
  - Saves once more during graceful shutdown
  - Writes snapshots atomically through a temporary file and rename

## Project layout

```text
cmd/server/           Program entry point and shutdown handling
internal/server/      TCP server, command validation, and dispatch
internal/resp/        RESP2 Value type, reader, writer, and unit tests
internal/store/       Concurrent in-memory string store, TTLs, snapshots
dump.json             Default JSON snapshot file
```

## Requirements

- Go 1.22 or later
- Optional: `redis-cli` for manual testing

## Run the server

Start the server on the default Redis port (`6379`):

```sh
go run ./cmd/server
```

Available options:

```sh
go run ./cmd/server \
  -addr :6379 \
  -snapshot dump.json \
  -snapshot-interval 30s
```

Press `Ctrl+C` to stop the server gracefully. It closes the listener and
saves a final snapshot before exiting.

## Try it with redis-cli

With the server running in another terminal:

```sh
redis-cli -p 6379 PING
# PONG

redis-cli -p 6379 ECHO "hello from Go"
# "hello from Go"

redis-cli -p 6379 SET user:1 "Ada"
# OK

redis-cli -p 6379 GET user:1
# "Ada"

redis-cli -p 6379 EXISTS user:1 missing-key
# (integer) 1

redis-cli -p 6379 DEL user:1
# (integer) 1
```

### Key expiry

```sh
redis-cli -p 6379 SET session "abc123"
redis-cli -p 6379 EXPIRE session 5
redis-cli -p 6379 TTL session
# An integer near 5, counting down

redis-cli -p 6379 GET session
# "abc123" while the key is still alive

# After five seconds:
redis-cli -p 6379 GET session
# (nil)

redis-cli -p 6379 TTL session
# (integer) -2  (the key no longer exists)
```

`TTL` returns `-1` for a key that exists but has no expiry. `PERSIST key`
removes an existing expiry and returns the key to that permanent state.

## RESP implementation

Clients send commands as RESP arrays of bulk strings. For example, this
command:

```text
SET greeting hello
```

is sent on the wire as:

```text
*3\r\n$3\r\nSET\r\n$8\r\ngreeting\r\n$5\r\nhello\r\n
```

`internal/resp.Reader` consumes one complete value at a time, which allows
multiple values to be sent on the same connection. `internal/resp.Writer`
serializes responses using the same RESP2 value model.

## Tests and coverage

Run the complete test suite:

```sh
go test ./...
```

The RESP reader and writer have table-driven unit tests covering normal
values, null/empty values, nested arrays, invalid protocol input, stream
boundaries, unsupported types, and write errors.

To inspect RESP coverage locally:

```sh
go test ./internal/resp -coverprofile=coverage.out
go tool cover -func=coverage.out
go tool cover -html=coverage.out
```

## Current limitations

- Only Redis string values are supported; lists, sets, hashes, streams, and
  pub/sub are not implemented.
- Snapshots are JSON files, not Redis RDB or AOF persistence formats.
- There is no authentication, replication, clustering, transactions, or
  eviction policy.
- The goal is clarity and learning rather than Redis-level performance or full
  wire compatibility.

## Possible next steps

- Add tests for the store and server layers
- Add an additional data type such as lists or sets
- Support expiry options on `SET` (for example, `SET key value EX 10`)
- Add snapshot configuration and recovery tests
- Explore append-only persistence or a more Redis-like snapshot format

# redis-clone

A from-scratch Redis clone in Go, built step by step to learn networking,
concurrency, and protocol design.

## Current step: key expiry

Keys can now have a TTL. `internal/store` uses two complementary
expiry strategies, the way real Redis does: **lazy expiry** (checked
whenever a key is read) and **active expiry** (a background goroutine
sweeping every 100ms). New commands: `EXPIRE`, `TTL`, `PERSIST`.

## Run it

```
go run ./cmd/server
```

## Test it

```
redis-cli -p 6379 SET session "abc123"
redis-cli -p 6379 EXPIRE session 5
redis-cli -p 6379 TTL session          # should show ~5, counting down
redis-cli -p 6379 GET session          # works while it's still alive

# wait 6 seconds, then:
redis-cli -p 6379 GET session          # (nil) — expired
redis-cli -p 6379 TTL session          # (integer) -2 — key doesn't exist

# also worth checking the "no expiry" case:
redis-cli -p 6379 SET permanent "hi"
redis-cli -p 6379 TTL permanent        # (integer) -1 — exists, no TTL

# and PERSIST:
redis-cli -p 6379 SET temp "x"
redis-cli -p 6379 EXPIRE temp 100
redis-cli -p 6379 PERSIST temp
redis-cli -p 6379 TTL temp             # -1 again — expiry removed
```

## Next steps

- [x] `internal/resp/` — RESP protocol encoder/decoder
- [x] Command dispatch (`PING`, `ECHO`)
- [x] `internal/store/` — in-memory key-value store (map + mutex)
- [x] Real data commands (`SET`, `GET`, `DEL`, `EXISTS`)
- [x] Key expiry (TTL) — `EXPIRE`, `TTL`, `PERSIST`, lazy + active expiry
- [ ] Persistence (snapshotting)
- [ ] A second data type (List or Set) once String feels solid

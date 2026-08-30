# redis-clone

A from-scratch Redis clone in Go, built step by step to learn networking,
concurrency, and protocol design.

## Current step: real data — SET / GET / DEL / EXISTS

There's now an actual `internal/store` package backing the server: a
mutex-protected `map[string]string`. String type only, for now — see
"Next steps" for where List/Hash/Set would plug in later.

## Run it

```
go run ./cmd/server
```

## Test it

```
redis-cli -p 6379 SET alphabet "a, b, c"
redis-cli -p 6379 GET alphabet
redis-cli -p 6379 EXISTS alphabet missingkey
redis-cli -p 6379 DEL alphabet
redis-cli -p 6379 GET alphabet          # should come back nil now
```

Worth trying with two `redis-cli` sessions / terminals at once too —
`SET` in one, `GET` in the other — to confirm the store is genuinely
shared across connections, not per-connection state.

## Next steps

- [x] `internal/resp/` — RESP protocol encoder/decoder
- [x] Command dispatch (`PING`, `ECHO`)
- [x] `internal/store/` — in-memory key-value store (map + mutex)
- [x] Real data commands (`SET`, `GET`, `DEL`, `EXISTS`)
- [ ] Key expiry (TTL) — `EXPIRE`, `TTL`, and passive/active expiry
- [ ] Persistence (snapshotting)
- [ ] A second data type (List or Set) once String feels solid

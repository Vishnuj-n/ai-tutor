## Debug Session Summary

**Symptom:** Dashboard permanently stuck on "Loading study workspace..."

---

### Three bugs found and fixed

**1. Orphan debug block in `loadAgenda()` — `Dashboard.vue`**
The debugging instrumentation was left as a real code block inside the function. It called all 4 APIs, then the real block immediately re-set `loading = true` and called them all again from scratch — doubling every request on every load.

**2. `defer rows.Close()` before HTTP POST — `sync/sync.go`**
The cloud sync goroutine fires at startup, opens a `rows` cursor on the SQLite connection, then makes an HTTP POST — with `defer rows.Close()` holding the connection open for the entire network call. With `SetMaxOpenConns(1)`, any frontend DB call arriving during that window blocked forever. Fixed by closing `rows` explicitly before the HTTP call.

**3. Nested `conn.Query` inside `rows.Next()` loop — `db/notebooks_repo.go`**
`GetProfileRemainingWords` opened an outer `rows` cursor (holding the only connection), then called `GetRemainingWords` inside the loop, which tried to open another `conn.Query` — a guaranteed deadlock. Fixed by replacing the N+1 loop with a single SQL `JOIN` query.

---

**Root pattern:** All three bugs share the same cause — the single SQLite connection (`SetMaxOpenConns(1)`) being held across async boundaries, either by deferred closes or nested queries.
# Code Review - Live Fleet Grid

Independent pre-commit, pre-deploy audit of the `live-fleet-grid` repo (Vue 3 + Pinia + TypeScript client, Go + gorilla/websocket server). Reviewed as an adversary looking for real defects, not to rubber-stamp.

**Date:** 2026-06-15
**Reviewer:** automated independent review (read full source, ran all gates)
**Scope:** the whole repo (server/, client/, docs/, deploy config). The repo is not yet a git repository; this review is intentionally run before the first commit.

---

## Verdict: PASS WITH WARNINGS

The code is clean, well-structured, and honest. Every quality gate passes, including the Go race detector. No secrets, no AI artefacts, no methodology leaks, no `console.log`, no em dashes, and the client holds to the no-`any` / no-`as` discipline. The findings below are latent smells and documentation nits, not shipping blockers. There are **zero BLOCKERs**.

| Severity | Count |
|---|---|
| BLOCKER | 0 |
| WARNING | 3 |
| NOTE | 5 |

---

## Step 1 - Quality Gates (real evidence)

| Gate | Command | Result | Evidence |
|---|---|---|---|
| Go vet | `go vet ./...` | PASS | no output (clean) |
| Go build | `go build ./...` | PASS | no output (clean) |
| Go test | `go test ./...` | PASS | `ok live-fleet-grid/server (cached)` |
| Go test (race) | `go test -race ./...` | PASS | `ok live-fleet-grid/server 1.550s` |
| Client build | `npm run build` (`vue-tsc --noEmit && vite build`) | PASS | `51 modules transformed`, `built in 729ms`, bundle `126.94 kB` / gzip `41.19 kB` |
| Client test | `npm test` (`vitest run`) | PASS | `Test Files 3 passed (3)`, `Tests 16 passed (16)` |

The race detector passing is meaningful: it exercises the simulator and origin parsing under `-race` with no data race reported, which directly supports the concurrency claims in `hub.go` / `simulation.go`. Note the test suite does not spin up a live hub with concurrent clients, so the race detector only covers what the unit tests touch (see NOTE-3).

---

## Step 2 - No-Install Scanners

| Scanner | Result |
|---|---|
| Secrets (api_key/secret/password/token/AKIA/private_key) | CLEAN. Only hit is the README explicitly saying "There are no tokens and no database." |
| Placeholders / AI artefacts (TODO/FIXME/REPLACE_ME/"as an ai"/"i am claude"/model names/system-prompt strings) | CLEAN. The only `<your-` is the intended placeholder in `client/.env.example`. |
| `console.log` in client non-test code | CLEAN (none). |
| Em dash (U+2014) | CLEAN (none anywhere). |
| Methodology leaks (packet/cursor-task/subagent/continuation-prompt/build-packet/golden-goose/socratic/by-hand) | CLEAN. "cursor" appears only as "sequence cursor" (a data-structure term), never Cursor the IDE. No internal methodology references. |
| Client TS discipline: `any` / non-`const` `as` casts | CLEAN. The only textual hits are in human prose/comments ("any data change"), not type positions. `tsconfig.json` enables `strict`, `noUncheckedIndexedAccess`, `noImplicitReturns`, `noFallthroughCasesInSwitch`, `noUnusedLocals`, `noUnusedParameters`, `verbatimModuleSyntax`. |

---

## Step 3 - Findings

### WARNING-1 - Blocking snapshot send before the write pump starts (latent deadlock smell)

**File:** `server/ws.go:99-106`
**Tier:** C (architectural - ESCALATED, not changed)

In `handleWS`, the client is registered at line 99, then the snapshot is pushed with a **blocking** send:

```go
s.hub.register(client)          // line 99 - now visible to broadcast()
snapshot := s.sim.snapshot()
if payload, err := json.Marshal(snapshot); err == nil {
    client.send <- payload      // line 103 - BLOCKING send, no select/default
}
go client.writePump()           // line 106 - the only drainer starts here
```

Between `register` (99) and `writePump` starting (106), the simulation goroutine can call `hub.broadcast`, which does a non-blocking `select { case c.send <- msg: default: }` into this same 32-deep buffer. If the buffer were to fill in that window, the blocking send at line 103 would block forever, because the only goroutine that drains `client.send` (`writePump`) has not been started yet and is started *after* this line on the same goroutine. That stalls the HTTP handler goroutine and leaks the connection.

**Why it is a WARNING and not a BLOCKER:** broadcasts happen at most ~1/second, and the snapshot send executes microseconds after `register`. Filling all 32 slots in that sub-millisecond window is not reachable at the current 1 Hz tick. So it cannot trigger in practice today. It is a real latent fragility: a blocking send on a buffered channel whose sole drainer has not started, coupled to a registry that is already live to the broadcaster.

**Concrete fix (do not apply now - behaviour change):** start `writePump` *before* the snapshot send, and route the snapshot through the same non-blocking enqueue used by broadcast, OR perform the snapshot send with a `select { case client.send <- payload: default: }`. Cleanest is to register the client only after the pumps are running, or send the snapshot directly on the connection (synchronous `conn.WriteMessage`) before starting the pumps and before `register`, so a missed-window update cannot precede the snapshot. Each option has a tradeoff with the "register-before-snapshot so no update is missed" invariant the code comments at lines 87-90 deliberately chose; that is exactly why this is escalated rather than patched.

### WARNING-2 - Snapshot marshal failure is silently swallowed, client left empty

**File:** `server/ws.go:102-104`
**Tier:** C (behaviour - ESCALATED)

```go
if payload, err := json.Marshal(snapshot); err == nil {
    client.send <- payload
}
```

If `json.Marshal` ever errors, the client connects, the pumps start, but it never receives a snapshot. It would sit on "Waiting for the first snapshot..." until the next `update` arrives (which carries only changed vessels, not the full fleet), so the grid could stay partially empty for a while. Marshalling these plain structs effectively never fails, so this is low-probability. Recommended fix: `log.Printf` on the marshal error so the failure is observable, and/or close the connection so the client reconnects and retries the snapshot. Escalated because it changes connect-time behaviour.

### WARNING-3 - `.DS_Store` files present in the working tree

**Files:** `./.DS_Store`, `./docs/.DS_Store`
**Tier:** A (safe) - but intentionally NOT modified, see note

These macOS metadata files exist on disk. `.DS_Store` IS listed in `.gitignore`, so they will not be committed and pose no shipping risk. Flagged only so the author is aware they exist locally. No action required for the commit. (Not deleted by this review because deleting untracked local files is outside the safe-fix remit and has zero effect on what gets committed.)

### NOTE-1 - `seq` is `uint64` server-side but bounded by JS `Number` precision client-side

**Files:** `server/vessel.go:37` (`Seq uint64`), `client/src/lib/messages.ts:27,34` (`z.number().int().nonnegative()`)

JavaScript `Number` loses integer precision above 2^53. A `uint64` seq that exceeded 2^53 would deserialize lossily on the client and could corrupt gap detection. At 1 update/sec it takes on the order of 285 million years to reach 2^53, so this is purely theoretical. No change needed; documented for completeness.

### NOTE-2 - Origin check normalizes only `scheme://host`, drops path/userinfo (correct, but worth knowing)

**File:** `server/ws.go:54-60`

`newOriginChecker` parses the Origin header and compares `u.Scheme + "://" + u.Host` against the allow-list. This correctly handles the only fields a browser Origin carries (scheme + host + optional port, all folded into `u.Host`). I verified the edge cases against the tests (`simulation_test.go:145-170`): production domain ALLOWED, `evil.example.com` REJECTED, a dev origin not in the list REJECTED, and an empty Origin (non-browser client) ALLOWED. An unset `ALLOWED_ORIGINS` defaults to localhost-only via `parseAllowedOrigins` (tested at `simulation_test.go:114-126`), so there is **no accidental allow-all**. A malformed Origin that fails `url.Parse` returns `false` (rejected). This is correct. The only behavioural nuance to be aware of: any non-browser client with no Origin header is always allowed (by design, for curl / native ws clients); on a public deploy that means non-browser clients can connect regardless of `ALLOWED_ORIGINS`. That matches the README's "no auth" stance.

### NOTE-3 - Tests do not exercise a live multi-client hub

**Files:** `server/simulation_test.go`, `client/src/composables/useFleetSocket.test.ts`

The tests are genuinely meaningful, not trivial: the client suite includes a real reject-malformed-frame case (`messages.test.ts:39-72` rejects bad discriminant, invalid status, wrong-typed `lat`, missing fields, negative seq) and a real backoff-timing assertion (`useFleetSocket.test.ts:105-133` advances fake timers to 499ms/500ms and 999ms/1000ms to prove the 500ms -> 1000ms doubling). The Go suite asserts strict seq monotonicity over 500 ticks and a valid non-empty changed-set. What is NOT covered: an end-to-end hub test with concurrent register/unregister/broadcast (the `hub.go` fan-out and per-client drop-oldest are validated only by reading + the race detector on the simulator, not by a stress test). This matches the README's stated "unit tests, not full coverage" scope. No defect; a coverage gap the README already discloses honestly.

### NOTE-4 - Gap detection cannot fire on the first update after a snapshot reset

**File:** `client/src/composables/useFleetSocket.ts:84-89`, `client/src/stores/fleet.ts:47-53`

A snapshot sets `lastSeq` and does no gap accounting (correct - a snapshot resets the world). If frames are dropped between the snapshot and the first update, the very first update's `seq` is still checked against the snapshot's seq, so a gap there IS detected. This is correct behaviour; noting it because the logic is subtle and worth being able to defend verbally.

### NOTE-5 - Deploy config reviewed and correct

- **`server/Dockerfile`:** valid multi-stage build. Stage 1 (`golang:1.22-alpine`) caches modules then compiles a static binary with `CGO_ENABLED=0 -trimpath -ldflags="-s -w"`. Stage 2 (`alpine:3.20`) copies the binary, runs as non-root `appuser` (uid 10001), `ENTRYPOINT` runs the server. The static (cgo-disabled) binary runs on Alpine without libc issues. `$PORT` is respected: the server reads `os.Getenv("PORT")` via `normalizePort` (default 8080), so Render/Railway port injection works. `EXPOSE 8080` is correctly noted as informational.
- **`server/.dockerignore`:** sane - excludes docs, test artefacts, the committed `server` binary, `.git`, `.DS_Store`. Keeps the build context lean.
- **`render.yaml`:** correct - `runtime: docker`, `rootDir: server`, `dockerfilePath: ./Dockerfile`, `healthCheckPath: /healthz`, and `ALLOWED_ORIGINS` set to the client domain. PORT injected by platform. Good.
- **`client/vercel.json`:** correct for a `client/` root deploy - `framework: vite`, `buildCommand: npm run build`, `outputDirectory: dist`, and the SPA rewrite `/(.*) -> /index.html`. Matches the global memory note about Vercel needing the framework preset set (here it is `vite`, not `Other`).
- **`client/src/lib/config.ts`:** `resolveWsUrl` trims and falls back to `ws://localhost:8080/ws` only when blank/unset. Confirmed there is **no hardcoded `ws://` URL anywhere else** in `client/src` - the only literal is the documented dev fallback in `config.ts`, and the production value comes from `VITE_WS_URL`.

---

## Step 4/5 - Fixes Applied

**No Tier A or Tier B code fixes were required.** The source is already clean: gates green, scanners clean, TS discipline held, docs honest. The only Tier A candidate (`.DS_Store`) is already gitignored and has no effect on the commit, so it was intentionally left untouched (deleting untracked local files is outside the safe-fix remit and changes nothing that ships).

All three WARNINGs and the one behavioural NOTE that could change runtime behaviour are **Tier C** and are ESCALATED above, unfixed, with concrete fixes described. The author should decide on WARNING-1 / WARNING-2 before deploying to a public, higher-traffic environment, though neither can trigger at the current 1 Hz tick.

**Gates after review:** unchanged and still green (no files were modified):
- `go vet ./...` clean, `go build ./...` clean, `go test ./...` PASS, `go test -race ./...` PASS.
- `npm run build` PASS, `npm test` PASS (16/16).

---

## Honesty Assessment

The README and docs are framed correctly as a **demo, not production**. The "Scope and Limitations" section explicitly states: simulated data (random-walk goroutine, no real AIS feed), single-node in-memory hub (no horizontal scaling / Redis / sticky sessions), no auth and no persistence (the origin check is an allow-list, not authentication), unit tests rather than full coverage (no E2E / load harness), and a structural-not-literal VMS analogy. TLS / `wss://` is correctly attributed to the deploy platform, not the server. The CHANGELOG's "Notes" repeats "This is not production." No overclaim found. The verified-vs-not boundary is stated accurately.

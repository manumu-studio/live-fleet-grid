# PR-0.1.0 - Live Fleet Grid

**Branch:** `main`
**Version:** 0.1.0
**Type:** Feature
**Status:** Ready
**Date:** 2026-06-15

---

## Summary

Adds the full Live Fleet Grid: a Go WebSocket server that streams a simulated vessel fleet, and a Vue 3 + Pinia client that renders it as a live grid with a connection-health indicator. The headline of this release is the reliability layer. The socket is treated as an unreliable channel, and four concrete behaviours (heartbeat, backoff reconnect, sequence-gap detection, bounded back-pressure) make the feed survive server restarts and slow consumers without leaking memory or silently showing stale data.

The data is simulated on purpose. The transferable artifact is the real-time transport and its failure handling, which is a structural analogy to a video management system camera/sensor wall (many live streams with statuses, an operator watching connection health), minus the video.

## What Was Built

### Go WebSocket server

A single-process server (`server/`) on Go 1.22 with `gorilla/websocket` v1.5.3:

- `main.go` starts the simulator, wires the routes, and shuts down gracefully on `SIGINT`/`SIGTERM`.
- `ws.go` upgrades `/ws` connections behind a dev-origin check, serves `/healthz` (returns `ok clients=N`), and reads `PORT` (default 8080).
- `hub.go` keeps a mutex-guarded set of clients and fans out broadcasts; a full per-client send buffer drops that client's frame rather than blocking the whole fan-out.
- `client.go` runs per-connection read and write pumps, emits heartbeat pings, and enforces a 60s read deadline refreshed by the pong handler.
- `simulation.go` random-walks ~12 vessels each second and advances the monotonic sequence number only when it actually broadcasts.

### Vue 3 + Pinia client

A Vite client (`client/`) in strict TypeScript:

- `stores/fleet.ts` is a setup-style Pinia store holding a `Map<id, Vessel>`, the connection status, the last sequence number, and the dropped-frame count, plus getters (`vesselList`, `vesselCount`, `statusCounts`) and actions (`applySnapshot`, `applyUpdate`, `setStatus`, `recordGap`). It is pure state.
- `components/` render the grid: `App.vue` (layout + indicator + summary), `ConnectionIndicator.vue`, `FleetGrid.vue`, and `VesselCard.vue` (with a flash-on-change highlight).

### Wire protocol and validation

- `client/src/lib/messages.ts` defines `VesselSchema` and a `ServerMessageSchema` discriminated union on `type` (`snapshot` | `update`), with all TypeScript types inferred from the schemas.
- The first frame on connect is a full `snapshot`; subsequent frames are minimal `update`s carrying only changed vessels.
- Every inbound frame is `safeParse`d before it reaches the store. Non-strings, JSON errors, and schema failures are dropped.

### Reliability layer

`client/src/composables/useFleetSocket.ts` owns the socket and four behaviours:

1. Heartbeat plus read deadline on the server reaps dead sockets.
2. Exponential-backoff reconnect on the client (500ms doubling to a 10s cap, reset on open).
3. Sequence-gap detection: a non-contiguous `seq` increments the dropped-frame count surfaced in the UI.
4. Bounded back-pressure: a 32-deep server send buffer and a 120-frame client drop-oldest queue, both shedding the stalest frames.

## Files Changed

| File | Action | Notes |
|---|---|---|
| `server/main.go` | Added | Entry point, simulator startup, graceful shutdown |
| `server/ws.go` | Added | `/ws` upgrade + origin check, `/healthz`, routes, `PORT` |
| `server/hub.go` | Added | Mutex-guarded registry, non-blocking broadcast |
| `server/client.go` | Added | Read/write pumps, heartbeat, read deadline |
| `server/simulation.go` | Added | Fleet simulator, seq advanced only on broadcast |
| `server/vessel.go` | Added | Domain model + JSON wire envelope |
| `server/go.mod` | Added | Module definition (`gorilla/websocket` v1.5.3) |
| `client/src/main.ts` | Added | App bootstrap (`createApp` + `createPinia`) |
| `client/src/lib/messages.ts` | Added | Zod schemas + inferred wire types |
| `client/src/stores/fleet.ts` | Added | Pinia store: state, getters, actions |
| `client/src/composables/useFleetSocket.ts` | Added | Socket lifecycle + reliability layer |
| `client/src/App.vue` | Added | Root layout, opens socket on mount |
| `client/src/components/ConnectionIndicator.vue` | Added | Status + dropped-frame readout |
| `client/src/components/FleetGrid.vue` | Added | Responsive vessel grid |
| `client/src/components/VesselCard.vue` | Added | Single vessel card, flash on change |
| `client/src/components/VesselCard.types.ts` | Added | `VesselCard` props contract |
| `client/package.json` | Added | Client manifest, version 0.1.0 |
| `client/tsconfig.json` | Added | Strict TS + `noUncheckedIndexedAccess` + `verbatimModuleSyntax` |
| `client/vite.config.ts` | Added | Vue plugin + `@/` alias |
| `README.md` | Added | Project overview, architecture, run instructions |
| `CHANGELOG.md` | Added | 0.1.0 release notes |
| `docs/**` | Added | Journal, PR, system diagram, ADRs, docs index |

## Architecture Decisions

| Decision | Why |
|---|---|
| Standalone Go process owns the socket | A long-lived WebSocket needs a long-lived host; goroutines make the simulator, fan-out, and per-connection pumps cheap and concurrent |
| Snapshot on connect, then minimal updates | New clients get full state once; steady-state traffic carries only changed vessels, applied as O(1) patches by id |
| Monotonic sequence number advanced only on broadcast | Makes the client-visible stream strictly contiguous, so a gap is unambiguous evidence of loss and its size is the exact count |
| Bounded back-pressure on both ends | Unbounded buffering is how real-time systems exhaust memory; drop-oldest is safe because state is last-writer-wins by id and the seq cursor still reports the gap |
| Zod discriminated union validated per frame | A socket is an external boundary; parsing each frame and inferring types from the schema means the store never sees untrusted data and types cannot drift |
| WebSocket over SSE / polling | One bidirectional channel that also carries the ping/pong heartbeat used to reap dead peers, which is central to the reliability story |

## Testing Checklist

Manual verification (no automated tests yet):

- [ ] Start the server, then the client: the grid populates from the first snapshot and the indicator shows `Live`.
- [ ] Vessels move: lat/lng values change each second and cards flash on update.
- [ ] Kill the server: the indicator flips to `Reconnecting` and backs off; restart it and the indicator recovers to `Live`.
- [ ] Induce a gap (for example briefly pause the server or background the tab past the queue cap): the dropped-frame count increases in the indicator.
- [ ] `curl localhost:8080/healthz` returns `ok clients=N` reflecting the number of open clients.

## Deployment Notes

- Run locally over two terminals: `cd server && go mod tidy && go run .`, then `cd client && npm install && npm run dev`.
- The Go server needs a long-lived host (a container or VM), not a serverless function, because the WebSocket connection is persistent.
- A real deployment would terminate TLS and serve `wss://`, and would tighten the origin allow-list beyond the localhost dev values.
- The hub is single-node. Scaling horizontally would require a shared fan-out (for example Redis pub/sub) and sticky or shared sessions.

## Validation

```bash
# Client
cd client && npm install && npm run build
# vue-tsc --noEmit && vite build -> exit 0, ~127 kB JS bundle

# Server (on a machine with Go 1.22+)
cd server && go mod tidy && go build
# go mod tidy writes go.sum on first run; go build compiles cleanly
```

The client type-checks and builds clean. The Go server compiles after a one-time `go mod tidy` to generate `go.sum`.

# Live Fleet Grid - initial build

**Date:** 2026-06-15
**Type:** Feature
**Branch:** `main`
**Version:** `0.1.0`

---

## What I Did

Built a small but complete real-time slice: a Go WebSocket server that simulates a fleet of ~12 vessels and broadcasts their movement, and a Vue 3 + Pinia client that renders them as a live grid with a connection-health indicator.

The scope was chosen on purpose. Instead of building a wide, shallow product, I built a narrow, deep one: the real-time transport path and its failure handling, taken end to end. The data is simulated because the data is not the point. The point is that the socket is treated as an unreliable channel and hardened accordingly, which is the part that actually carries over to a production real-time product (for example a camera or sensor wall in a video management system).

Concretely, the build delivers:

- A single-process Go server with a mutex-guarded hub, a simulator goroutine, ping/pong heartbeats, read deadlines, and graceful shutdown.
- A JSON wire protocol with a full `snapshot` on connect and minimal `update` frames afterward, each carrying a monotonic sequence number.
- A Vue client where the Pinia store is pure state and a single composable owns the socket, validation, reconnect, gap detection, and back-pressure.
- A connection indicator that surfaces status, vessel count, current sequence, and a running dropped-frame count.

## Files Touched

| File | Role |
|---|---|
| `server/main.go` | Process entry point: starts the simulator, wires routes, graceful shutdown on signal |
| `server/ws.go` | `/ws` upgrade with dev-origin check, `/healthz` probe, route table, `PORT` handling |
| `server/hub.go` | Mutex-guarded client registry; non-blocking broadcast fan-out |
| `server/client.go` | Per-connection read/write pumps, heartbeat ping, read deadline + pong handler |
| `server/simulation.go` | ~12 vessels, ~1s random-walk, status flips, sequence advanced only on broadcast |
| `server/vessel.go` | Domain model and JSON wire types for vessels and the server message envelope |
| `client/src/main.ts` | Bootstrap: `createApp` + `createPinia`, mount `#app` |
| `client/src/lib/messages.ts` | Zod `VesselSchema` and `ServerMessageSchema` discriminated union; inferred types |
| `client/src/stores/fleet.ts` | Setup-style Pinia store: `Map<id, Vessel>`, status, seq, dropped count, getters, actions |
| `client/src/composables/useFleetSocket.ts` | Socket lifecycle plus the four reliability behaviours |
| `client/src/App.vue` | Root layout; opens the socket on mount |
| `client/src/components/ConnectionIndicator.vue` | Connection-health readout and dropped-frame count |
| `client/src/components/FleetGrid.vue` | Responsive grid of vessel cards |
| `client/src/components/VesselCard.vue` | One vessel card with a flash-on-change highlight |
| `client/src/components/VesselCard.types.ts` | Props contract for `VesselCard` |

## Decisions

**Why a standalone Go process owns the socket.**
A long-lived WebSocket needs a long-lived host. A serverless function model fights that. A small Go process is a natural fit: goroutines make the simulator, the broadcast fan-out, and each connection's pumps cheap to run concurrently, and a mutex-guarded map is enough state for a single node. It also keeps the transport concern fully separate from the UI, which is the honest shape of a real product.

**Why sequence numbers for gap detection.**
Without a cursor, a client that misses frames just shows slightly stale data and never knows. By tagging every broadcast with a monotonic `seq` and advancing it only when a frame actually goes out, the client-visible stream is strictly contiguous. A missing `seq` is then unambiguous evidence of loss, and the size of the jump is exactly the number of frames missed. That turns "the feed might be degraded" into a number on screen.

**Why bound back-pressure on both ends.**
Unbounded buffering is how real-time systems run out of memory under load. On the server, a slow reader gets a 32-deep send buffer and then has frames dropped for that one connection, so one stalled client cannot stall the whole broadcast. On the client, the render queue is capped at 120 and drops the oldest, so a backgrounded tab cannot grow memory without bound. Dropping is safe here because state is last-writer-wins by vessel id and the sequence cursor still reports the gap.

**Why Zod-validate every frame.**
TypeScript types vanish at runtime, and a socket is an external boundary just like an HTTP body. Parsing every frame with a Zod discriminated union means the store can only ever receive well-formed data, and inferring the TypeScript types from the same schemas removes any chance of the validator and the type system drifting apart.

**Why not SSE or polling.**
Polling adds latency and wasted requests for a feed that changes every second. Server-Sent Events are one-way and would work for the data path, but a WebSocket gives a single bidirectional channel that also carries the ping/pong heartbeat used to reap dead peers, which is central to the reliability story this demo is about.

## Still Open

- **No automated tests yet.** The store actions and the gap-detection logic are pure and isolated specifically so they are easy to unit test; the suite itself is the next step.
- **Not deployed.** Runs locally over two terminals. A deployment would need a long-lived host for the Go socket and a `wss://` origin, neither of which is set up.
- **Single-node only.** One in-memory hub. Multiple instances would need a shared fan-out (for example Redis pub/sub) and sticky or shared session handling.

## Validation

```bash
# Client: type-check + production build
cd client
npm install
npm run build         # vue-tsc --noEmit && vite build -> exit 0, ~127 kB JS

# Server: fetch deps and compile (run on a machine with the Go toolchain)
cd server
go mod tidy           # writes go.sum on first run
go build              # compiles the server binary
```

The client build passes clean (`vue-tsc` plus `vite build`, exit 0). The Go server requires `go mod tidy` once to generate `go.sum`, then `go build` / `go run .` on a machine with Go 1.22+.

# Live Fleet Grid

A focused real-time demo. A Go WebSocket server streams simulated vessel positions and statuses, and a Vue 3 + Pinia + TypeScript client renders a live-updating grid of vessel cards with a connection-health indicator. The interesting part is not the data: it is the real-time transport and how it behaves when the network or the server misbehaves.

**🛰️ Live demo: [lfg.manumustudio.com](https://lfg.manumustudio.com)** — the Vue client backed by the live Go WebSocket server on Railway. Open it and watch the grid populate from the first snapshot, then flash as updates stream in; stop watching and the connection indicator keeps reporting health in real time.

The "grid of live tracked entities" shape is a structural analogy to a VMS (Video Management System) camera/sensor wall: many independent live streams, each with a status, all updating concurrently, with an operator watching connection health. This demo streams JSON vessel positions, not video, so the analogy is structural rather than literal. The transferable part is the real-time transport layer and its failure handling.

## Tech Stack

- **Frontend:** Vue 3 (`<script setup>` single-file components) + Pinia (setup-style stores) for state, built with Vite.
- **Language (client):** TypeScript in strict mode, including `noUncheckedIndexedAccess` and `verbatimModuleSyntax`.
- **Realtime:** the browser-native `WebSocket` API, no socket library on the client.
- **Backend:** Go 1.22 with `github.com/gorilla/websocket` v1.5.3.
- **Validation:** Zod 3 on the client, parsing every inbound frame before it reaches the store.
- **Styling:** scoped CSS per component on a small dark theme, no UI framework.

## Architecture

Data flows one way: the Go simulator produces frames, the hub fans them out over WebSocket, the client composable validates and applies reliability logic, the Pinia store holds pure state, and the Vue components render it.

```
  +------------------------------+
  |  Go server (single process)   |
  |                               |
  |  simulation.go  ~12 vessels   |
  |    random-walk every ~1s      |
  |    seq++ only on broadcast     |
  |            |                  |
  |            v                  |
  |  hub.go  broadcast fan-out     |
  |    mutex-guarded client set    |
  |    32-deep send buffer/conn    |
  |    drop-oldest under load      |
  +-------------|----------------+
                |  JSON frames over ws://localhost:8080/ws
                |  first: { type:"snapshot", seq, sentAt, vessels:[all] }
                |  then:  { type:"update",   seq, sentAt, vessels:[changed] }
                |  ping/pong heartbeat + 60s read deadline
                v
  +------------------------------+
  |  useFleetSocket.ts (composable)|
  |    zod safeParse per frame     |
  |    sequence-gap detection      |
  |    exponential-backoff reconnect|
  |    bounded drop-oldest queue    |
  +-------------|----------------+
                |  validated frames
                v
  +------------------------------+
  |  stores/fleet.ts (Pinia)       |
  |    Map<id, Vessel>             |
  |    connectionStatus, lastSeq   |
  |    droppedFrames, getters      |
  +-------------|----------------+
                |  reactive getters
                v
  +------------------------------+
  |  Vue components                |
  |    App                         |
  |     > ConnectionIndicator       |
  |     > FleetGrid > VesselCard    |
  +------------------------------+
```

The store is pure state. All transport and reliability logic lives in the composable, which keeps the store trivially testable and the components free of socket concerns.

## How It Works

1. **Connect.** On mount, `App.vue` calls `useFleetSocket(url).connect()`, which opens a native `WebSocket`. The URL comes from `VITE_WS_URL` (resolved in `src/lib/config.ts`), falling back to `ws://localhost:8080/ws` for local dev. The store status moves to `connecting`.
2. **Receive the snapshot.** The server registers the client and immediately sends one full `snapshot` frame containing every vessel and the current sequence number. The grid populates and the indicator flips to `live`.
3. **Stream updates.** Each second the simulator random-walks the fleet and broadcasts an `update` frame containing only the vessels that changed. The store patches those by id, so each update is an O(1) write, not a full re-render.
4. **Validate every frame.** Before any frame touches the store, the composable JSON-parses it and runs `ServerMessageSchema.safeParse`. Anything that is not a string, fails JSON parsing, or fails the schema is dropped silently. The store never sees untrusted data.
5. **Detect gaps.** Every frame carries a monotonic `seq`. The server advances `seq` only when it actually broadcasts, so the client-visible stream is strictly contiguous. If an incoming `seq` is not `lastSeq + 1`, the difference is the number of frames that were lost. The store increments `droppedFrames` and the indicator surfaces the count.
6. **Reconnect with backoff.** On any close or error the client reconnects, starting at 500ms and doubling to a 10s cap, resetting to 500ms on a healthy open. A server restart or transient network drop recovers automatically without hammering the server.
7. **Apply back-pressure.** Validated frames are queued and drained on `requestAnimationFrame`. If the render loop falls behind, the queue is capped at 120 frames and drops the oldest. The server mirrors this: a slow client's 32-deep send buffer drops frames rather than blocking the whole broadcast.
8. **Render and teardown.** Components read reactive getters from the store. `VesselCard` flashes on any data change so liveness is visible. On unmount, the composable clears its timers, cancels the pending drain, and closes the socket cleanly.

## Documentation

- [CHANGELOG.md](CHANGELOG.md) - version history, starting at 0.1.0.
- [docs/DEPLOY.md](docs/DEPLOY.md) - deployment runbook: exact Railway/Vercel settings, monorepo build gotchas, and production verification.
- [docs/README.md](docs/README.md) - index of all project documentation.
- [docs/architecture/SYSTEM_DIAGRAM.md](docs/architecture/SYSTEM_DIAGRAM.md) - data-flow diagram, component responsibilities, and the wire protocol.
- [docs/decisions/ADR-001-resilient-websocket-transport.md](docs/decisions/ADR-001-resilient-websocket-transport.md) - why the socket is treated as unreliable and how it is hardened.
- [docs/decisions/ADR-002-validated-wire-protocol.md](docs/decisions/ADR-002-validated-wire-protocol.md) - why the wire protocol is a Zod discriminated union validated on every frame.
- [docs/journal/ENTRY-1.md](docs/journal/ENTRY-1.md) - developer-facing build journal: the "why" behind the decisions.
- [docs/pull-requests/PR-0.1.0-live-fleet-grid.md](docs/pull-requests/PR-0.1.0-live-fleet-grid.md) - team-facing summary of what was built and how to validate it.

## Getting Started

### Prerequisites

- Go 1.22 or newer (for the WebSocket server).
- Node 20 or newer (for the Vite client).

### Setup

Two terminals. The server first so the client has something to connect to.

```bash
# Terminal 1 - backend (Go)
cd server
go mod tidy   # fetches gorilla/websocket and writes go.sum
go run .      # listens on :8080 (override with PORT)

# Terminal 2 - frontend (Vue + Vite)
cd client
npm install
npm run dev   # serves http://localhost:5173
```

Open the printed Vite URL. The grid populates from the first snapshot, then cards flash as updates arrive. Stop the Go server to watch the indicator flip to `Reconnecting` with backoff, then restart it to watch it recover to `Live`.

Health check:

```bash
curl localhost:8080/healthz   # -> ok clients=0
```

### Configuration (environment variables)

The demo is configured entirely through environment variables, so the same build runs locally and on a host.

| Variable | Side | Default | Purpose |
|---|---|---|---|
| `PORT` | Server | `8080` | Listen port. Railway and Render inject this automatically. |
| `ALLOWED_ORIGINS` | Server | localhost dev origins | Comma-separated allow-list of browser origins permitted to open the socket. When unset, only `http://localhost:5173` and `http://127.0.0.1:5173` are allowed. In production set it to the client domain, e.g. `https://lfg.manumustudio.com`. Non-browser clients (no `Origin` header) are always allowed. |
| `VITE_WS_URL` | Client | `ws://localhost:8080/ws` | The WebSocket URL the client connects to. In production set it to the `wss://` form of the deployed server, e.g. `wss://<go-server-host>/ws`. See `client/.env.example`. |

### Tests

Both sides have a unit suite.

```bash
# Server (Go): simulator tick contract + origin allow-list
cd server
go test ./...

# Client (Vue): wire-schema contract, Pinia store actions, socket reconnect
cd client
npm test
```

The Go tests assert the broadcast sequence number strictly increments and every broadcast carries a valid, non-empty changed-set, plus the `ALLOWED_ORIGINS` parsing and origin check. The client tests cover the `ServerMessageSchema` discriminated union (accept valid snapshot/update, reject off-contract frames), the store actions (`applySnapshot`, `applyUpdate`, `recordGap`), and the `useFleetSocket` composable's reconnect/backoff behaviour with a mock WebSocket.

### Deployment

This is a static client plus a long-lived WebSocket server, so they deploy to different places. The live deployment runs the server on **Railway** (`https://live-fleet-grid-server-production.up.railway.app`) and the client on **Vercel** ([lfg.manumustudio.com](https://lfg.manumustudio.com)).

- **Server (Go):** a multi-stage `server/Dockerfile` builds a static binary and runs it on Alpine. On Railway, set the service **Root Directory** to `server` (so the Dockerfile's relative `COPY` resolves) and leave Dockerfile Path empty; Render uses the `render.yaml` blueprint. Set `ALLOWED_ORIGINS` to the client domain; `PORT` is injected by the platform.
- **Client (Vue):** a static Vite build. `client/vercel.json` configures the framework, build command, output directory, and SPA rewrite. Set `VITE_WS_URL` to the `wss://` URL of the deployed server.

Deploy the server first to get its public URL, set `VITE_WS_URL` to `wss://<that-host>/ws`, deploy the client, then make sure `ALLOWED_ORIGINS` on the server includes the final client domain. **See [docs/DEPLOY.md](docs/DEPLOY.md) for the full runbook** — exact per-field Railway/Vercel settings, the monorepo build gotchas, and the production verification commands.

## Project Structure

```
live-fleet-grid/
├── render.yaml                      # Render blueprint for the Go server (Docker)
├── server/                          # Go WebSocket server (single process)
│   ├── main.go                      # entry point, graceful shutdown on SIGINT/SIGTERM
│   ├── ws.go                        # /ws upgrade + ALLOWED_ORIGINS allow-list, /healthz, routes, PORT
│   ├── hub.go                       # mutex-guarded client registry + broadcast fan-out
│   ├── client.go                    # per-connection read/write pumps, ping heartbeat, deadlines
│   ├── simulation.go                # ~12 vessels, ~1s random-walk, monotonic seq on broadcast
│   ├── simulation_test.go           # simulator tick contract + origin allow-list tests
│   ├── vessel.go                    # domain model + JSON wire types
│   ├── Dockerfile                   # multi-stage build -> static binary on Alpine
│   ├── .dockerignore                # keep the Docker build context lean
│   ├── go.mod                       # module definition (gorilla/websocket v1.5.3)
│   └── go.sum                       # dependency checksums (written by go mod tidy)
└── client/                          # Vue 3 + Pinia + Vite client
    ├── index.html                   # Vite entry HTML
    ├── package.json                 # client manifest (version 0.1.1)
    ├── tsconfig.json                # strict TS, noUncheckedIndexedAccess, verbatimModuleSyntax
    ├── vite.config.ts               # Vue plugin + "@/" alias + Vitest (happy-dom) config
    ├── vercel.json                  # Vercel static deploy: framework, build, SPA rewrite
    ├── .env.example                 # documents VITE_WS_URL
    ├── env.d.ts                     # ambient *.vue, Vite client types, typed import.meta.env
    └── src/
        ├── main.ts                  # bootstrap: createApp + createPinia, mount #app
        ├── App.vue                  # root layout, opens the socket on mount
        ├── styles.css               # global dark-theme baseline
        ├── lib/
        │   ├── config.ts            # resolves VITE_WS_URL with a localhost dev fallback
        │   ├── messages.ts          # Zod schemas + inferred types (wire contract)
        │   └── messages.test.ts     # wire-schema contract tests
        ├── stores/
        │   ├── fleet.ts             # Pinia store: Map<id, Vessel>, status, seq, dropped count
        │   └── fleet.test.ts        # store action tests
        ├── composables/
        │   ├── useFleetSocket.ts    # socket lifecycle + four reliability behaviours
        │   └── useFleetSocket.test.ts # reconnect/backoff tests with a mock WebSocket
        └── components/
            ├── ConnectionIndicator.vue   # connection-health readout + dropped-frame count
            ├── FleetGrid.vue             # responsive grid of vessel cards
            ├── VesselCard.vue            # one vessel: name, type, status, live lat/lng
            └── VesselCard.types.ts       # props contract for VesselCard
```

## Scope and Limitations

This is a deliberately narrow vertical slice. It proves the real-time path and its failure handling end to end, cleanly. It is honest about what it is not:

- **Data is simulated.** Vessels random-walk in a goroutine. There is no real AIS feed. The transport and reliability layer is the real artifact, not the data.
- **Single-node hub.** One in-memory `Hub` guarded by a mutex. No horizontal scaling, no Redis fan-out, no sticky sessions.
- **No auth, no persistence.** There are no tokens and no database. The origin check is an `ALLOWED_ORIGINS` allow-list, not authentication: it controls which browser origins may open the socket, not who. TLS / `wss://` is terminated by the deploy platform (Railway/Render/Vercel), not by this server.
- **Unit tests, not full coverage.** The Go and Vitest suites cover the simulator tick contract, the origin allow-list, the wire schema, the store actions, and the socket reconnect logic. There is no end-to-end or load-test harness; that remains a roadmap item.
- **Structural, not literal, VMS analogy.** The grid-of-live-entities shape maps to a camera/sensor wall, but this demo carries JSON state, not video. The point of overlap is the real-time transport and its reliability, not the media path.

## License

Private. All rights reserved.

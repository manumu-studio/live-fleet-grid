# Changelog

All notable changes to Live Fleet Grid are documented here. Versions follow [Semantic Versioning](https://semver.org/).

## [0.1.1] - 2026-06-15

### Added

- **Env-driven configuration.** The server WebSocket origin allow-list is now read from `ALLOWED_ORIGINS` (comma-separated, trimmed, empties ignored), defaulting to localhost dev origins when unset. `PORT` is read from the environment (default 8080). The client WebSocket URL is read from `VITE_WS_URL` (resolved in `client/src/lib/config.ts`), falling back to `ws://localhost:8080/ws` for local development. `VITE_WS_URL` is typed in `client/env.d.ts` and documented in `client/.env.example`.
- **Container and deploy support.** A multi-stage `server/Dockerfile` (build on `golang:1.22-alpine`, run a static binary on Alpine as a non-root user) plus a `.dockerignore`. A root `render.yaml` blueprint deploys the server on Render; Railway auto-detects the Dockerfile. A `client/vercel.json` configures the Vite static deploy (framework, build command, output directory, SPA rewrite).
- **Automated tests.** Go tests (`server/simulation_test.go`) assert the broadcast sequence number strictly increments, every broadcast carries a valid non-empty changed-set, the snapshot reflects the current sequence, and the `ALLOWED_ORIGINS` parsing and origin check behave correctly. Vitest tests on the client cover the `ServerMessageSchema` discriminated union (accept valid snapshot/update, reject off-contract frames), the Pinia store actions (`applySnapshot`, `applyUpdate`, `recordGap`), and the `useFleetSocket` reconnect/backoff logic with a mock WebSocket. A `test` script runs each suite.

### Changed

- **README and architecture docs** reflect the new environment variables, the Docker/deploy story, the test commands, and the updated project structure. The "Scope and Limitations" section now describes the origin allow-list (not "localhost only") and the present unit-test coverage (no longer "no tests yet").

### Notes

- Still a focused demo: simulated data, single-node in-memory hub, no auth, no persistence. TLS / `wss://` is terminated by the deploy platform, not by the server. This is not production.

## [0.1.0] - 2026-06-15

### Added

- **Go WebSocket server.** A single-process Go 1.22 server (`gorilla/websocket` v1.5.3) that simulates a ~12-vessel fleet, fans out updates to all connected clients, and exposes `/ws` and `/healthz`. Graceful shutdown on `SIGINT`/`SIGTERM`; port configurable via `PORT` (default 8080).
- **Fleet simulator.** A goroutine that random-walks vessel positions every ~1s and occasionally flips a vessel's status. The monotonic sequence number advances only when a frame is actually broadcast, so the client-visible stream stays strictly contiguous.
- **Vue 3 + Pinia client.** A live grid of vessel cards built with Vue 3 (`<script setup>`), Pinia setup-style stores, Vite, and strict TypeScript. Cards flash on any data change to make liveness visible.
- **Zod-validated wire protocol.** A `ServerMessageSchema` discriminated union (`snapshot` | `update`) defined in `client/src/lib/messages.ts`. Every inbound frame is `safeParse`d before it reaches the store; types are inferred from the schemas as the single source of truth.
- **Snapshot-then-updates flow.** The first frame on connect is a full `snapshot` of every vessel; subsequent frames are `update`s carrying only the vessels that changed, applied as O(1) patches by id.
- **Heartbeat liveness.** Server-driven ping frames plus a 60s read deadline refreshed by the pong handler, so dead sockets are reaped instead of leaking.
- **Exponential-backoff reconnect.** The client reconnects after a close or error, starting at 500ms and doubling to a 10s cap, resetting on a healthy open.
- **Sequence-gap detection.** Non-contiguous `seq` values are counted as dropped frames and surfaced in the UI, so the operator knows when the feed degraded.
- **Bounded back-pressure on both ends.** A 32-deep per-connection send buffer on the server and a 120-frame drop-oldest render queue on the client, both shedding the stalest frames rather than growing memory without bound.
- **Connection-health indicator.** A header readout showing connection status, live vessel count, current sequence number, and the running dropped-frame count.
- **Project documentation.** README, this changelog, a system diagram, two architecture decision records, a development journal entry, and a pull-request summary.

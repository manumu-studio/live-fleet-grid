# ADR-001: Treat the WebSocket as an unreliable transport

**Date:** 2026-06-15
**Status:** Accepted

## Context

A WebSocket connection looks reliable until it is not. Tabs get backgrounded, laptops sleep, networks drop, and servers restart. A naive client that simply opens a socket and renders whatever arrives will, on any of these events, either hang silently on a dead connection or quietly show stale data with no signal that anything is wrong. For a real-time operations view (the structural target here is a video management system camera/sensor wall), silent degradation is the worst failure mode: an operator who does not know the feed is stale will trust it.

The decision was to design the transport layer as if the socket is unreliable from the start, and to make every failure either recover automatically or become visible.

## Decision

Layer four reliability behaviours over the raw WebSocket, split across the server and the client.

1. **Heartbeat plus read deadline (server).** The write pump emits a `ping` roughly every 54s. The read pump sets a 60s read deadline, and the pong handler refreshes it on every pong. A peer that goes silent misses its pong, the deadline fires, the read errors, and the connection is unregistered and closed. Dead sockets are reaped instead of accumulating in the hub. Files: `server/client.go`.

2. **Exponential-backoff reconnect (client).** On any close or error that was not a manual disconnect, the client reconnects, starting at 500ms and doubling to a 10s cap, resetting to 500ms on a healthy open. This recovers from a server restart or transient loss without hammering the server during an outage. File: `client/src/composables/useFleetSocket.ts` (`scheduleReconnect`).

3. **Monotonic-sequence gap detection (both ends).** The server advances `seq` only when it actually broadcasts, so the client-visible stream is strictly contiguous. The client compares each incoming `seq` to `lastSeq + 1` and records the difference as dropped frames, surfaced in the indicator. File: `server/simulation.go` (seq on broadcast), `client/src/composables/useFleetSocket.ts` (`detectGap`).

4. **Bounded drop-oldest back-pressure (both ends).** The server gives each connection a 32-deep send buffer and drops that client's frame when it is full, so one slow reader cannot stall the broadcast. The client caps its pending-frame queue at 120 and drops the oldest, so a stalled render loop cannot grow memory without bound. Dropping is safe because state is last-writer-wins by vessel id. Files: `server/hub.go` (non-blocking `select`), `client/src/composables/useFleetSocket.ts` (`enqueue`).

## Alternatives considered

- **Naive auto-reconnect only.** Reconnecting on close is necessary but not sufficient. Without a heartbeat, half-open connections (where the TCP socket is dead but no close ever fires) are never detected, so the hub leaks them and the client can sit on a silent dead socket. Without gap detection, a reconnect that misses frames shows stale data with no signal.
- **Unbounded buffering.** Holding every frame until the consumer catches up is simple but is exactly how a real-time system runs out of memory under back-pressure. A backgrounded tab or a stalled reader would grow the queue without limit. Bounding the buffer and dropping the oldest trades a known, reported loss for stable memory.
- **Trusting delivery.** Assuming the socket delivers every frame in order, with no cursor and no liveness probe, is the default that this whole ADR exists to reject. It fails silently, which is unacceptable for an operations view.

## Consequences

- **Pros:** The feed recovers automatically from server restarts and transient drops; dead peers are reaped; memory stays bounded under load; and any frame loss becomes a visible number rather than silent staleness.
- **Cons:** More moving parts than a bare socket (timers, buffers, a sequence cursor) and more state to reason about. The reliability logic is concentrated in one composable to keep that cost contained and the store pure.
- **Trade-off accepted:** Under sustained back-pressure the client deliberately drops frames rather than buffering them. Because state is last-writer-wins by id and the sequence cursor reports the gap, the displayed positions still converge and the loss is accounted for.

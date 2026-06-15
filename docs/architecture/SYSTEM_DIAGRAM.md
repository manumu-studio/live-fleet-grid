# System Diagram

How a vessel update travels from the Go simulator to a flashing card in the browser, and where each reliability behaviour sits along the way.

## Data Flow

```
 GO SERVER (single process, :8080)
 ----------------------------------------------------------------
   simulation.go                hub.go                 ws.go / client.go
   ------------                 ------                  -----------------
   every ~1s tick:              broadcast(payload):     on connect:
     step() random-walks          lock client set         upgrade /ws (ALLOWED_ORIGINS check)
     ~12 vessels, flips status    for each client:        register client
     if nothing changed:            send <- payload       send full snapshot
       skip (no seq burned)         (non-blocking;        start read + write pumps
     else:                          drop if 32-deep
       seq = seq + 1                buffer full)         write pump: drains send chan,
       build update frame                                  emits ping every ~54s
       marshal JSON               unregister on           read pump: 60s read deadline,
       hub.broadcast(payload)     read error or close       pong refreshes it
   ----------------------------------------------------------------
                                   |
                                   |  JSON frame over a single WebSocket
                                   |  { type, seq, sentAt, vessels }
                                   |  + ping / pong heartbeat
                                   v
 BROWSER (Vite client, :5173)
 ----------------------------------------------------------------
   useFleetSocket.ts (composable)
   ------------------------------
     ws.onmessage:
       data must be a string         ........ else drop
       JSON.parse                    ........ catch -> drop
       ServerMessageSchema.safeParse ........ fail -> drop
       enqueue(frame)
     enqueue:
       push to pending queue
       if length > 120: shift oldest  (bounded drop-oldest back-pressure)
       schedule a requestAnimationFrame drain
     drain:
       for each queued frame: dispatch(frame)
     dispatch:
       snapshot -> store.applySnapshot (resets the world)
       update   -> detectGap(seq) then store.applyUpdate
     detectGap:
       if seq > lastSeq + 1: store.recordGap(seq - lastSeq - 1)
     ws.onclose (not manual):
       status = reconnecting
       scheduleReconnect: 500ms, doubling to a 10s cap, reset on open
   ------------------------------
                                   |  validated frames only
                                   v
   stores/fleet.ts (Pinia)
   -----------------------
     vessels: Map<id, Vessel>      applySnapshot -> clear + set all, lastSeq = seq
     connectionStatus              applyUpdate   -> set changed by id, lastSeq = seq
     lastSeq, droppedFrames        recordGap     -> droppedFrames += missed
     getters: vesselList (sorted), vesselCount, statusCounts
   -----------------------
                                   |  reactive getters
                                   v
   Vue components
   --------------
     App.vue
       > ConnectionIndicator.vue   status dot, vessel count, seq, dropped count
       > FleetGrid.vue             grid keyed by vessel id
           > VesselCard.vue        name, type, status dot, live lat/lng, flash on change
```

## Component Responsibilities

| Component | Layer | Responsibility |
|---|---|---|
| `simulation.go` | Server | Owns fleet state; random-walks vessels each tick; advances `seq` only when broadcasting |
| `hub.go` | Server | Mutex-guarded client registry; non-blocking broadcast fan-out; per-connection back-pressure |
| `client.go` | Server | Per-connection read/write pumps; heartbeat ping; read deadline + pong handler |
| `ws.go` | Server | `/ws` upgrade with `ALLOWED_ORIGINS` allow-list; `/healthz`; route table; `PORT` handling |
| `main.go` | Server | Entry point; starts the simulator; graceful shutdown on signal |
| `vessel.go` | Server | Domain model and JSON wire types (vessel + message envelope) |
| `messages.ts` | Client | Zod schemas and inferred types; the single source of truth for the wire contract |
| `useFleetSocket.ts` | Client | Socket lifecycle; validation; reconnect; gap detection; bounded back-pressure |
| `fleet.ts` | Client | Pure state: vessel map, connection status, sequence cursor, dropped count, getters |
| `ConnectionIndicator.vue` | Client | Connection-health readout and dropped-frame count |
| `FleetGrid.vue` | Client | Responsive grid; keys cards by vessel id so Vue patches in place |
| `VesselCard.vue` | Client | One vessel; flash-on-change highlight to make liveness visible |

## Message Protocol

Every frame is a single JSON object with the same envelope:

```json
{
  "type": "snapshot" | "update",
  "seq": 0,
  "sentAt": 1718450000000,
  "vessels": [
    {
      "id": "vsl-1",
      "name": "Aurora",
      "vesselType": "Cargo",
      "status": "UNDERWAY",
      "lat": 41.234,
      "lng": -27.118
    }
  ]
}
```

### Field semantics

- **`type`** discriminates the two frame variants. The client validates against a discriminated union, so each variant is handled exhaustively.
- **`seq`** is a monotonically increasing integer. It is the heart of gap detection (see below).
- **`sentAt`** is the server's send time in unix milliseconds. It is carried for latency math and is not required by the current client logic.
- **`vessels`** is the payload. Its meaning depends on `type`.

### `snapshot` versus `update`

- **`snapshot`** is sent exactly once, as the first frame after a client connects. It carries the full fleet. The client clears its map and repopulates it, then sets `lastSeq` to the snapshot's `seq`. No gap accounting is done against any prior sequence, because a snapshot resets the world.
- **`update`** is sent on every subsequent change. It carries only the vessels that changed this tick. The client patches those by id, runs gap detection, then sets `lastSeq`.

### Sequence semantics and gap detection

The server advances `seq` only when it actually broadcasts a frame. If a tick produces no change, no sequence number is consumed. This makes the client-visible stream strictly contiguous: the next frame a client should see is always `lastSeq + 1`.

When an `update` arrives with `seq` greater than `lastSeq + 1`, the difference (`seq - lastSeq - 1`) is exactly the number of frames the client never saw. The store adds that to `droppedFrames`, which the indicator displays. State still converges, because updates are last-writer-wins by vessel id: a missed frame's vessel is corrected by any later frame that touches it. The dropped count is the honest signal that the feed degraded, even though the displayed positions catch back up.

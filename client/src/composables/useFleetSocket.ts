// useFleetSocket.ts - the transport layer. Owns the WebSocket lifecycle and the
// four reliability behaviours that make this credible for a real-time product:
//   1. Exponential-backoff reconnect (500ms -> 10s cap, reset on open).
//   2. Sequence-gap detection (non-contiguous seq => count the dropped frames).
//   3. Bounded in-memory back-pressure buffer (drop-oldest past a cap) so a
//      slow render loop can never grow memory without bound.
//   4. Clean teardown of socket + timers on unmount.
// Validated frames are dispatched to the Pinia store; everything else is
// dropped. The composable holds no rendering concerns.
import { onBeforeUnmount } from 'vue'
import { ServerMessageSchema } from '@/lib/messages'
import type { ServerMessage } from '@/lib/messages'
import { useFleetStore } from '@/stores/fleet'

const RECONNECT_BASE_MS = 500
const RECONNECT_CAP_MS = 10_000
// Max frames we will hold if the render loop falls behind. Past this we drop
// the oldest pending frame - bounded memory under back-pressure.
const MAX_PENDING_FRAMES = 120

export interface FleetSocketController {
  connect: () => void
  disconnect: () => void
}

export function useFleetSocket(url: string): FleetSocketController {
  const store = useFleetStore()

  let socket: WebSocket | null = null
  let reconnectDelay = RECONNECT_BASE_MS
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let drainHandle: number | null = null
  let manuallyClosed = false

  // Bounded FIFO of validated frames awaiting application to the store.
  const pending: ServerMessage[] = []

  // ---- back-pressure: enqueue + drain ---------------------------------

  function enqueue(message: ServerMessage): void {
    pending.push(message)
    // Drop-oldest: if the consumer can't keep up, shed the stalest frames so
    // memory stays bounded. A dropped pending update is a real lost frame, but
    // the seq cursor on the next applied frame will surface it as a gap.
    while (pending.length > MAX_PENDING_FRAMES) {
      pending.shift()
    }
    scheduleDrain()
  }

  function scheduleDrain(): void {
    if (drainHandle !== null) return
    drainHandle = requestAnimationFrame(() => {
      drainHandle = null
      drainPending()
    })
  }

  // Apply every queued frame on the next animation frame, in order.
  function drainPending(): void {
    let frame = pending.shift()
    while (frame !== undefined) {
      dispatch(frame)
      frame = pending.shift()
    }
  }

  // ---- sequence-gap detection + dispatch ------------------------------

  function dispatch(message: ServerMessage): void {
    if (message.type === 'snapshot') {
      // A snapshot resets the world; no gap accounting against the prior seq.
      store.applySnapshot(message.vessels, message.seq)
      return
    }
    // type === 'update'
    detectGap(message.seq)
    store.applyUpdate(message.vessels, message.seq)
  }

  // If the incoming seq is not exactly lastSeq + 1, the difference is the
  // number of frames we never saw. Record it; the next snapshot/update still
  // converges state because updates are last-writer-wins by id.
  function detectGap(seq: number): void {
    const prev = store.lastSeq
    if (prev !== null && seq > prev + 1) {
      store.recordGap(seq - prev - 1)
    }
  }

  // ---- socket lifecycle -----------------------------------------------

  function connect(): void {
    manuallyClosed = false
    store.setStatus(store.lastSeq === null ? 'connecting' : 'reconnecting')

    const ws = new WebSocket(url)
    socket = ws

    ws.onopen = (): void => {
      reconnectDelay = RECONNECT_BASE_MS // reset backoff on a healthy open
      store.setStatus('live')
    }

    ws.onmessage = (event: MessageEvent<unknown>): void => {
      handleRawFrame(event.data)
    }

    ws.onerror = (): void => {
      // onclose will fire next and own the reconnect; nothing to do here.
    }

    ws.onclose = (): void => {
      socket = null
      if (manuallyClosed) {
        store.setStatus('closed')
        return
      }
      store.setStatus('reconnecting')
      scheduleReconnect()
    }
  }

  // Validate the raw payload before it is trusted. Anything that is not a
  // string, or fails JSON parsing, or fails the zod contract, is dropped.
  function handleRawFrame(data: unknown): void {
    if (typeof data !== 'string') return

    let json: unknown
    try {
      json = JSON.parse(data)
    } catch {
      return
    }

    const result = ServerMessageSchema.safeParse(json)
    if (!result.success) return

    enqueue(result.data)
  }

  function scheduleReconnect(): void {
    if (reconnectTimer !== null) return
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, reconnectDelay)
    // Exponential backoff with a hard cap.
    reconnectDelay = Math.min(reconnectDelay * 2, RECONNECT_CAP_MS)
  }

  // ---- teardown -------------------------------------------------------

  function disconnect(): void {
    manuallyClosed = true
    if (reconnectTimer !== null) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    if (drainHandle !== null) {
      cancelAnimationFrame(drainHandle)
      drainHandle = null
    }
    pending.length = 0
    if (socket !== null) {
      socket.close()
      socket = null
    }
    store.setStatus('closed')
  }

  onBeforeUnmount(disconnect)

  return { connect, disconnect }
}

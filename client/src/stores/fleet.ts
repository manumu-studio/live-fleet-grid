// fleet.ts - the single source of truth for fleet state, as a setup-style Pinia
// store. It holds the vessels keyed by id, the connection status, and the
// sequence/drop bookkeeping that powers the reliability indicator. The store is
// deliberately dumb: it only mutates state. All transport logic (sockets,
// reconnect, gap handling) lives in the useFleetSocket composable.
import { defineStore } from 'pinia'
import { computed, reactive, ref } from 'vue'
import type { Vessel, VesselStatus } from '@/lib/messages'

// The lifecycle of the live connection, as a string literal union.
export type ConnectionStatus = 'connecting' | 'live' | 'reconnecting' | 'closed'

export const useFleetStore = defineStore('fleet', () => {
  // Keyed by vessel id so updates are O(1) patches, not array scans.
  const vessels = reactive(new Map<string, Vessel>())

  const connectionStatus = ref<ConnectionStatus>('connecting')
  const lastSeq = ref<number | null>(null)
  const droppedFrames = ref(0)

  // ---- getters ---------------------------------------------------------

  // Stable, name-sorted list for rendering (Map iteration order is insertion
  // order, which would make cards jump around as updates arrive).
  const vesselList = computed<Vessel[]>(() =>
    [...vessels.values()].sort((a, b) => a.name.localeCompare(b.name)),
  )

  const vesselCount = computed(() => vessels.size)

  // Count of vessels in each status, for an at-a-glance fleet summary.
  const statusCounts = computed<Record<VesselStatus, number>>(() => {
    const counts: Record<VesselStatus, number> = {
      UNDERWAY: 0,
      MOORED: 0,
      ANCHORED: 0,
    }
    for (const v of vessels.values()) {
      counts[v.status] += 1
    }
    return counts
  })

  // ---- actions ---------------------------------------------------------

  // Replace the whole fleet from a snapshot frame and reset the seq cursor.
  function applySnapshot(next: Vessel[], seq: number): void {
    vessels.clear()
    for (const v of next) {
      vessels.set(v.id, v)
    }
    lastSeq.value = seq
  }

  // Patch only the vessels present in an update frame, by id.
  function applyUpdate(changed: Vessel[], seq: number): void {
    for (const v of changed) {
      vessels.set(v.id, v)
    }
    lastSeq.value = seq
  }

  function setStatus(status: ConnectionStatus): void {
    connectionStatus.value = status
  }

  // Record one or more missed frames detected via a sequence gap.
  function recordGap(missed: number): void {
    if (missed > 0) {
      droppedFrames.value += missed
    }
  }

  return {
    // state
    vessels,
    connectionStatus,
    lastSeq,
    droppedFrames,
    // getters
    vesselList,
    vesselCount,
    statusCounts,
    // actions
    applySnapshot,
    applyUpdate,
    setStatus,
    recordGap,
  }
})

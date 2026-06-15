// fleet.test.ts - unit tests for the Pinia fleet store. They cover the three
// state transitions the transport layer drives: applySnapshot populates the
// vessel map and sets the seq cursor, applyUpdate patches a single vessel by id
// without disturbing the others, and recordGap accumulates dropped frames.
import { beforeEach, describe, expect, it } from 'vitest'
import { createPinia, setActivePinia } from 'pinia'
import { useFleetStore } from '@/stores/fleet'
import type { Vessel } from '@/lib/messages'

const aurora: Vessel = {
  id: 'vsl-1',
  name: 'Aurora',
  vesselType: 'Cargo',
  status: 'UNDERWAY',
  lat: 41.0,
  lng: -27.0,
}
const borealis: Vessel = {
  id: 'vsl-2',
  name: 'Borealis',
  vesselType: 'Tanker',
  status: 'MOORED',
  lat: 42.0,
  lng: -26.0,
}

describe('useFleetStore', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
  })

  it('applySnapshot populates the vessel map and sets the seq cursor', () => {
    const store = useFleetStore()
    store.applySnapshot([aurora, borealis], 5)

    expect(store.vesselCount).toBe(2)
    expect(store.vessels.get('vsl-1')).toEqual(aurora)
    expect(store.lastSeq).toBe(5)
    expect(store.statusCounts.UNDERWAY).toBe(1)
    expect(store.statusCounts.MOORED).toBe(1)
  })

  it('applySnapshot replaces prior state rather than merging it', () => {
    const store = useFleetStore()
    store.applySnapshot([aurora, borealis], 5)
    store.applySnapshot([aurora], 9)

    expect(store.vesselCount).toBe(1)
    expect(store.vessels.has('vsl-2')).toBe(false)
    expect(store.lastSeq).toBe(9)
  })

  it('applyUpdate patches only the vessel by id and advances the seq', () => {
    const store = useFleetStore()
    store.applySnapshot([aurora, borealis], 5)

    const moved: Vessel = { ...aurora, lat: 41.5, status: 'ANCHORED' }
    store.applyUpdate([moved], 6)

    expect(store.vessels.get('vsl-1')?.lat).toBe(41.5)
    expect(store.vessels.get('vsl-1')?.status).toBe('ANCHORED')
    // Borealis is untouched.
    expect(store.vessels.get('vsl-2')).toEqual(borealis)
    expect(store.lastSeq).toBe(6)
  })

  it('recordGap accumulates dropped frames and ignores non-positive counts', () => {
    const store = useFleetStore()
    expect(store.droppedFrames).toBe(0)

    store.recordGap(3)
    store.recordGap(2)
    expect(store.droppedFrames).toBe(5)

    store.recordGap(0)
    store.recordGap(-4)
    expect(store.droppedFrames).toBe(5)
  })

  it('vesselList is sorted by name regardless of insertion order', () => {
    const store = useFleetStore()
    store.applySnapshot([borealis, aurora], 1)
    expect(store.vesselList.map((v) => v.name)).toEqual(['Aurora', 'Borealis'])
  })

  it('setStatus updates the connection status', () => {
    const store = useFleetStore()
    store.setStatus('reconnecting')
    expect(store.connectionStatus).toBe('reconnecting')
  })
})

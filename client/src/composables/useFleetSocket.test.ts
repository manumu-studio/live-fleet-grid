// useFleetSocket.test.ts - transport-layer tests for the socket composable using
// a mock WebSocket and fake timers. They prove the two reliability behaviours
// that are hard to see in a screenshot: a valid snapshot frame flows through to
// the store, and after the socket closes the composable flips to "reconnecting"
// and retries on an exponential backoff schedule.
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { defineComponent, h } from 'vue'
import { mount } from '@vue/test-utils'
import { createPinia, setActivePinia } from 'pinia'
import { useFleetSocket } from '@/composables/useFleetSocket'
import { useFleetStore } from '@/stores/fleet'
import type { FleetSocketController } from '@/composables/useFleetSocket'

// MockWebSocket records instances and lets tests drive open/message/close.
class MockWebSocket {
  static instances: MockWebSocket[] = []
  onopen: (() => void) | null = null
  onmessage: ((e: { data: unknown }) => void) | null = null
  onerror: (() => void) | null = null
  onclose: (() => void) | null = null
  readonly url: string

  constructor(url: string) {
    this.url = url
    MockWebSocket.instances.push(this)
  }

  close(): void {
    this.onclose?.()
  }

  // Test helpers.
  emitOpen(): void {
    this.onopen?.()
  }
  emitMessage(data: unknown): void {
    this.onmessage?.({ data })
  }
  emitClose(): void {
    this.onclose?.()
  }
}

// Mount the composable inside a throwaway component so its onBeforeUnmount hook
// has a component instance to attach to.
function mountSocket(url: string): { controller: FleetSocketController; unmount: () => void } {
  let controller!: FleetSocketController
  const Host = defineComponent({
    setup() {
      controller = useFleetSocket(url)
      return () => h('div')
    },
  })
  const wrapper = mount(Host)
  return { controller, unmount: () => wrapper.unmount() }
}

const snapshotFrame = JSON.stringify({
  type: 'snapshot',
  seq: 0,
  sentAt: 1718450000000,
  vessels: [
    { id: 'vsl-1', name: 'Aurora', vesselType: 'Cargo', status: 'UNDERWAY', lat: 41, lng: -27 },
  ],
})

describe('useFleetSocket', () => {
  beforeEach(() => {
    setActivePinia(createPinia())
    MockWebSocket.instances = []
    vi.stubGlobal('WebSocket', MockWebSocket)
    // Drain the rAF queue synchronously so we do not depend on real frames.
    vi.stubGlobal('requestAnimationFrame', (cb: FrameRequestCallback): number => {
      cb(0)
      return 1
    })
    vi.stubGlobal('cancelAnimationFrame', () => {})
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
    vi.unstubAllGlobals()
  })

  it('applies a validated snapshot frame to the store and goes live', () => {
    const store = useFleetStore()
    const { controller, unmount } = mountSocket('ws://test/ws')
    controller.connect()

    const ws = MockWebSocket.instances[0]
    expect(ws).toBeDefined()
    expect(ws?.url).toBe('ws://test/ws')

    ws?.emitOpen()
    expect(store.connectionStatus).toBe('live')

    ws?.emitMessage(snapshotFrame)
    expect(store.vesselCount).toBe(1)
    expect(store.lastSeq).toBe(0)

    unmount()
  })

  it('flips to reconnecting and retries with backoff after an unexpected close', () => {
    const store = useFleetStore()
    const { controller, unmount } = mountSocket('ws://test/ws')
    controller.connect()

    const first = MockWebSocket.instances[0]
    first?.emitOpen()
    expect(store.connectionStatus).toBe('live')

    // Server drops the connection.
    first?.emitClose()
    expect(store.connectionStatus).toBe('reconnecting')
    expect(MockWebSocket.instances).toHaveLength(1)

    // First backoff is 500ms; nothing reconnects before it elapses.
    vi.advanceTimersByTime(499)
    expect(MockWebSocket.instances).toHaveLength(1)
    vi.advanceTimersByTime(1)
    expect(MockWebSocket.instances).toHaveLength(2)

    // Second close without a healthy open doubles the delay to 1000ms.
    MockWebSocket.instances[1]?.emitClose()
    vi.advanceTimersByTime(999)
    expect(MockWebSocket.instances).toHaveLength(2)
    vi.advanceTimersByTime(1)
    expect(MockWebSocket.instances).toHaveLength(3)

    unmount()
  })

  it('does not reconnect after a manual disconnect', () => {
    const { controller, unmount } = mountSocket('ws://test/ws')
    controller.connect()
    MockWebSocket.instances[0]?.emitOpen()

    controller.disconnect()
    vi.advanceTimersByTime(20_000)
    expect(MockWebSocket.instances).toHaveLength(1)

    unmount()
  })
})

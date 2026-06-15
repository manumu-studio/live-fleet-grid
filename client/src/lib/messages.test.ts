// messages.test.ts - contract tests for the Zod wire protocol. They assert the
// ServerMessageSchema accepts a valid snapshot and update, and rejects frames
// that drift off-contract (bad discriminant, invalid status, missing/typed-wrong
// fields). The schema is the boundary the store trusts, so these are the tests
// that keep that trust honest.
import { describe, expect, it } from 'vitest'
import { ServerMessageSchema } from '@/lib/messages'

describe('ServerMessageSchema', () => {
  const vessel = {
    id: 'vsl-1',
    name: 'Aurora',
    vesselType: 'Cargo',
    status: 'UNDERWAY',
    lat: 41.2,
    lng: -27.1,
  }

  it('accepts a valid snapshot frame', () => {
    const frame = { type: 'snapshot', seq: 0, sentAt: 1718450000000, vessels: [vessel] }
    const result = ServerMessageSchema.safeParse(frame)
    expect(result.success).toBe(true)
    if (result.success) {
      expect(result.data.type).toBe('snapshot')
      expect(result.data.vessels).toHaveLength(1)
    }
  })

  it('accepts a valid update frame', () => {
    const frame = { type: 'update', seq: 7, sentAt: 1718450001000, vessels: [vessel] }
    const result = ServerMessageSchema.safeParse(frame)
    expect(result.success).toBe(true)
    if (result.success) {
      expect(result.data.type).toBe('update')
      expect(result.data.seq).toBe(7)
    }
  })

  it('rejects a frame with an unknown discriminant type', () => {
    const frame = { type: 'delete', seq: 1, sentAt: 1, vessels: [] }
    expect(ServerMessageSchema.safeParse(frame).success).toBe(false)
  })

  it('rejects a frame with a vessel in an invalid status', () => {
    const frame = {
      type: 'update',
      seq: 1,
      sentAt: 1,
      vessels: [{ ...vessel, status: 'SINKING' }],
    }
    expect(ServerMessageSchema.safeParse(frame).success).toBe(false)
  })

  it('rejects a frame missing required fields', () => {
    const frame = { type: 'snapshot', seq: 0 } // no sentAt, no vessels
    expect(ServerMessageSchema.safeParse(frame).success).toBe(false)
  })

  it('rejects a frame where lat is the wrong type', () => {
    const frame = {
      type: 'update',
      seq: 1,
      sentAt: 1,
      vessels: [{ ...vessel, lat: '41.2' }],
    }
    expect(ServerMessageSchema.safeParse(frame).success).toBe(false)
  })

  it('rejects a negative sequence number', () => {
    const frame = { type: 'snapshot', seq: -1, sentAt: 1, vessels: [] }
    expect(ServerMessageSchema.safeParse(frame).success).toBe(false)
  })
})

// messages.ts - the wire contract. Zod schemas mirror exactly what the Go
// server emits, and every inbound frame is validated against these before it is
// allowed to touch the store. Types are inferred from the schemas (single
// source of truth) so the validator and the type system can never drift.
import { z } from 'zod'

// The constrained operational states. Using z.enum keeps it a literal union.
export const VesselStatusSchema = z.enum(['UNDERWAY', 'MOORED', 'ANCHORED'])
export type VesselStatus = z.infer<typeof VesselStatusSchema>

// A single tracked vessel.
export const VesselSchema = z.object({
  id: z.string().min(1),
  name: z.string(),
  vesselType: z.string(),
  status: VesselStatusSchema,
  lat: z.number(),
  lng: z.number(),
})
export type Vessel = z.infer<typeof VesselSchema>

// The server envelope, modelled as a discriminated union on `type`. Both
// variants share seq/sentAt/vessels but are kept distinct so consumers can
// switch exhaustively.
export const SnapshotMessageSchema = z.object({
  type: z.literal('snapshot'),
  seq: z.number().int().nonnegative(),
  sentAt: z.number().int(),
  vessels: z.array(VesselSchema),
})

export const UpdateMessageSchema = z.object({
  type: z.literal('update'),
  seq: z.number().int().nonnegative(),
  sentAt: z.number().int(),
  vessels: z.array(VesselSchema),
})

export const ServerMessageSchema = z.discriminatedUnion('type', [
  SnapshotMessageSchema,
  UpdateMessageSchema,
])
export type ServerMessage = z.infer<typeof ServerMessageSchema>

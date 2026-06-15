# ADR-002: Model the wire protocol as a validated Zod discriminated union

**Date:** 2026-06-15
**Status:** Accepted

## Context

The client receives JSON frames over a WebSocket. A socket is an external boundary, no different in principle from an HTTP request body or a third-party webhook: nothing guarantees the bytes that arrive match what the client expects. TypeScript types are erased at runtime, so a type annotation on `event.data` is a comment, not a check. If a frame is malformed, truncated, or simply a shape the client did not anticipate, the safest place to catch it is at the boundary, before it reaches application state.

There are also two frame shapes (`snapshot` and `update`) that share most fields but must be handled differently. The protocol description needs to live somewhere that both the runtime validator and the type system agree on, so the two cannot drift apart over time.

## Decision

Define the wire protocol once, in `client/src/lib/messages.ts`, as Zod schemas, and validate every inbound frame against them before anything else touches it.

- `VesselSchema` models a single vessel; `VesselStatusSchema` is a `z.enum` that pins the status to the three allowed literals.
- `ServerMessageSchema` is a `z.discriminatedUnion('type', [...])` over `SnapshotMessageSchema` and `UpdateMessageSchema`. The `type` field discriminates the variants, so downstream code can switch on it exhaustively.
- All TypeScript types are derived with `z.infer` from these schemas. The schema is the single source of truth; the types follow from it automatically.
- In `useFleetSocket.ts`, every frame goes through `handleRawFrame`: reject anything that is not a string, `JSON.parse` inside a try/catch, then `ServerMessageSchema.safeParse`. Only on success is the parsed, fully typed frame enqueued. Everything else is dropped silently.

The store and components therefore only ever see data that has been proven to match the contract at runtime.

## Alternatives considered

- **Trust the JSON shape / cast it.** Doing `JSON.parse(data) as ServerMessage` compiles and reads cleanly, but it is a lie to the type system: the cast asserts a shape that was never checked. A malformed frame would flow straight into the store and surface as a confusing runtime error far from its origin. This is the default the ADR exists to reject.
- **Hand-written type guards.** A set of `isSnapshot(x): x is SnapshotMessage` functions would validate at runtime, but they duplicate the type definition by hand, are verbose, and drift from the types as the protocol changes. Zod collapses the schema, the validator, and the type into one declaration.
- **A binary schema (for example protobuf).** Stronger typing and a smaller payload on the wire, but it adds a code-generation step, a build dependency, and a debugging cost (frames are no longer human-readable) that a ~12-vessel demo does not justify. JSON plus Zod is readable, dependency-light, and validated, which is the right balance at this scale.

## Consequences

- **Pros:** The store can never receive untrusted or malformed data; the validator and the TypeScript types cannot drift because both come from one schema; the two frame variants are handled exhaustively via the discriminant; and bad frames fail closed (dropped) rather than corrupting state.
- **Cons:** A small per-frame validation cost (negligible at this volume) and a schema that must be kept in step with the Go wire types in `server/vessel.go`. The two are aligned by hand today; a shared schema generator would remove that seam if the protocol grew.
- **Team norm:** Any new field or frame type is added to the Zod schema first, and the TypeScript types are inferred from it, never declared independently.

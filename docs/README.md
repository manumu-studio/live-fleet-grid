# Documentation

This folder holds the project documentation for Live Fleet Grid: the architecture, the decisions behind it, and the build record. The main project README lives at the repository root and is the best starting point for setup and a high-level tour.

| Document | Purpose |
|---|---|
| [DEPLOY.md](DEPLOY.md) | Operational runbook: exact Railway/Vercel settings, monorepo build gotchas, and production verification commands. |
| [DEVELOPMENT_JOURNAL.md](DEVELOPMENT_JOURNAL.md) | Master index of journal entries and pull requests, one row each. |
| [journal/ENTRY-1.md](journal/ENTRY-1.md) | Developer-facing build journal for the initial release: the "why" behind the design. |
| [pull-requests/PR-0.1.0-live-fleet-grid.md](pull-requests/PR-0.1.0-live-fleet-grid.md) | Team-facing summary of what was built, plus the validation and testing checklist. |
| [architecture/SYSTEM_DIAGRAM.md](architecture/SYSTEM_DIAGRAM.md) | Expanded data-flow diagram, per-component responsibility table, and the message protocol. |
| [decisions/ADR-001-resilient-websocket-transport.md](decisions/ADR-001-resilient-websocket-transport.md) | Decision to treat the socket as unreliable and layer heartbeat, reconnect, gap detection, and back-pressure. |
| [decisions/ADR-002-validated-wire-protocol.md](decisions/ADR-002-validated-wire-protocol.md) | Decision to model the wire protocol as a Zod discriminated union validated on every frame. |

See also the root [CHANGELOG.md](../CHANGELOG.md) for the version history.

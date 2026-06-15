// config.ts - runtime configuration resolved from Vite environment variables.
// Keeps env access in one place so the rest of the app depends on a plain value
// and the resolution logic stays unit-testable.

// Dev fallback used when VITE_WS_URL is not provided (local two-terminal setup).
const DEV_WS_URL = 'ws://localhost:8080/ws'

// resolveWsUrl returns the configured WebSocket URL, trimming surrounding
// whitespace and falling back to the localhost dev URL when unset or blank.
export function resolveWsUrl(raw: string | undefined): string {
  const trimmed = raw?.trim()
  return trimmed !== undefined && trimmed.length > 0 ? trimmed : DEV_WS_URL
}

// The WebSocket URL the client connects to. Production sets VITE_WS_URL to the
// wss:// form of the deployed Go server (e.g. wss://<host>/ws).
export const WS_URL = resolveWsUrl(import.meta.env.VITE_WS_URL)

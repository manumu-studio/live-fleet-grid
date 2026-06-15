// env.d.ts - ambient type declarations so the TypeScript compiler understands
// Vue single-file component imports (*.vue) and Vite client types.
/// <reference types="vite/client" />

declare module '*.vue' {
  import type { DefineComponent } from 'vue'
  const component: DefineComponent<Record<string, never>, Record<string, never>, unknown>
  export default component
}

// Typed environment variables exposed by Vite. VITE_WS_URL points the client at
// the Go WebSocket server (e.g. wss://<host>/ws in production); it is optional
// and falls back to the localhost dev URL when unset.
interface ImportMetaEnv {
  readonly VITE_WS_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

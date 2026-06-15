<!-- DEPLOY.md - operational runbook for deploying Live Fleet Grid: the Go
WebSocket server on Railway and the Vue client on Vercel, including the exact
platform settings, the monorepo gotchas, and the production verification steps. -->

# Deployment Runbook

Live Fleet Grid is a monorepo with two independently-deployed pieces:

- **`server/`** — a long-lived Go WebSocket server. Deployed to **Railway** from a Dockerfile.
- **`client/`** — a static Vite build. Deployed to **Vercel**.

They go to different hosts because their runtimes differ: the server needs a persistent process to hold open sockets; the client is static files served from a CDN.

Production URLs:

| Piece | URL |
|---|---|
| Server (Railway) | `https://live-fleet-grid-server-production.up.railway.app` |
| Client (Vercel) | `https://lfg.manumustudio.com` |

---

## Server — Railway

The server is a monorepo subdirectory with its own Dockerfile. Railway must be told to treat `server/` as the build root; otherwise its auto-detector scans the repo root, finds `client/ docs/ server/`, and cannot decide how to build.

### Exact settings

These live under the service's **Settings** tab, top to bottom:

| Section | Field | Value | Why |
|---|---|---|---|
| **Source** | Source Repo | `manumu-studio/live-fleet-grid` | Must match where you `git push`. |
| **Source** | Root Directory | `server` | Sets the build context to `server/` so the Dockerfile's relative `COPY go.mod ...` resolves. **This is the load-bearing setting.** |
| **Source** | Branch | `main` | Auto-deploys on push. |
| **Build** | Builder | `Dockerfile` | Use the committed Dockerfile, not the Railpack auto-detector. |
| **Build** | Dockerfile Path | *(empty)* | Leave blank. With Root Directory set, Railway auto-detects `server/Dockerfile`. Do **not** enter `server` or `server/Dockerfile` here (see gotchas). |
| **Build** | Watch Paths | `/server` | Only rebuild when server files change, not on client-only commits. |
| **Deploy** | Custom Start Command | *(empty)* | The Dockerfile `ENTRYPOINT` runs the binary. |
| **Deploy** | Healthcheck Path | `/healthz` | Railway waits for a 200 before marking the deploy live. |
| **Deploy** | Serverless | **OFF** | This is a live WebSocket server with a continuous simulation loop. Scale-to-zero would kill the simulation and drop connections when idle. |

### Variables tab

| Variable | Value | Notes |
|---|---|---|
| `ALLOWED_ORIGINS` | `https://lfg.manumustudio.com` | Comma-separated browser-origin allow-list. Without it the build succeeds but every WebSocket handshake from the client is rejected with `403`. |
| `PORT` | *(do not set)* | Railway injects it; the server reads it (`os.Getenv("PORT")`, default 8080). |

### Gotchas (each one cost a failed deploy)

1. **Wrong repo connected.** The service was originally pointed at a different repo. Symptom: the build runs but never reflects your latest commit. Fix: Source → Disconnect → reconnect to `live-fleet-grid`.
2. **Dockerfile Path set to a directory (`/server`).** Railway tries to parse the *directory* as a Dockerfile and fails with `dockerfile invalid: failed to parse dockerfile: file with no instructions`. Fix: clear the field — Root Directory already scopes everything to `server/`.
3. **`.dockerignore` excluding the `Dockerfile`.** Railway/BuildKit reads the Dockerfile out of the uploaded build-context snapshot. If `.dockerignore` lists `Dockerfile`, it gets stripped from the snapshot and the build fails with the same `file with no instructions` error. The committed `server/.dockerignore` no longer excludes it; do not re-add that line.
4. **"Redeploy" replays the old config.** Clicking *Redeploy* on a past failed deployment re-runs that deployment's exact snapshot and settings, ignoring your new ones. To pick up settings/commits, trigger a *fresh* deploy (push a commit, or use the top-level Deploy).

---

## Client — Vercel

A static Vite SPA configured by [`client/vercel.json`](../client/vercel.json) (framework `vite`, build `npm run build`, output `dist`, SPA rewrite to `index.html`).

| Setting | Value |
|---|---|
| Root Directory | `client` |
| Framework Preset | Vite (from `vercel.json`) |
| Env var `VITE_WS_URL` | `wss://live-fleet-grid-server-production.up.railway.app/ws` |

`VITE_WS_URL` is read at build time, so a change requires a redeploy.

---

## Order of operations

1. **Deploy the server** to Railway and note its public URL.
2. **Set `VITE_WS_URL`** on Vercel to the `wss://` form of that URL + `/ws`.
3. **Deploy the client** to Vercel and note its final domain.
4. **Set `ALLOWED_ORIGINS`** on Railway to that client domain.
5. Redeploy whichever side changed.

The two cross-references (`VITE_WS_URL` → server, `ALLOWED_ORIGINS` → client) are the only coupling between the deploys.

---

## Verifying production

The healthcheck confirms the process is up; the WebSocket handshake confirms the origin allow-list is correct. WebSocket upgrades require HTTP/1.1, so force it with `--http1.1`.

```bash
HOST="live-fleet-grid-server-production.up.railway.app"

# 1. Healthcheck -> HTTP 200, body "ok clients=N"
curl -sS -i "https://$HOST/healthz" | head -1

# 2. WS handshake from the allowed origin -> HTTP/1.1 101 Switching Protocols,
#    then a live "snapshot" frame followed by "update" frames. The connection
#    stays open (curl will time out streaming data — that is success).
curl -sS -i --http1.1 --max-time 10 \
  -H "Connection: Upgrade" -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  -H "Origin: https://lfg.manumustudio.com" \
  "https://$HOST/ws" | head -12

# 3. WS handshake from a disallowed origin -> HTTP/1.1 403 Forbidden
curl -sS -i --http1.1 --max-time 10 \
  -H "Connection: Upgrade" -H "Upgrade: websocket" \
  -H "Sec-WebSocket-Version: 13" -H "Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==" \
  -H "Origin: https://evil.example.com" \
  "https://$HOST/ws" | head -1
```

Expected: `200` on the healthcheck, `101` + streaming JSON for the allowed origin, `403` for the disallowed one. If the allowed origin returns `403`, `ALLOWED_ORIGINS` is missing or wrong.

---

## Troubleshooting

| Build/runtime symptom | Cause | Fix |
|---|---|---|
| `Railpack could not determine how to build the app` (lists `client/ docs/ server/`) | Root Directory unset → scanning repo root | Set Root Directory = `server`, Builder = Dockerfile |
| `dockerfile invalid: ... file with no instructions` | Dockerfile Path points at a directory, **or** `.dockerignore` excludes `Dockerfile` | Clear Dockerfile Path; ensure `.dockerignore` does not list `Dockerfile` |
| Build green, but WebSocket gets `403` in the browser | `ALLOWED_ORIGINS` missing/incorrect | Set it to the exact client origin (scheme + host, no path) |
| Connections drop when traffic is idle | Serverless / scale-to-zero enabled | Disable Serverless |
| New commit doesn't deploy | Looking at an old deployment, or Watch Paths excluded the change | Check the newest deployment; confirm the change is under `/server` |

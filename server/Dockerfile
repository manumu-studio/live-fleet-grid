# Dockerfile - multi-stage build for the live-fleet-grid Go WebSocket server.
# Build stage compiles a static binary; the runtime stage is a minimal alpine
# image that runs it. The server reads PORT from the environment (default 8080)
# and ALLOWED_ORIGINS for the WebSocket origin allow-list.

# ---- build stage ----------------------------------------------------------
FROM golang:1.22-alpine AS build
WORKDIR /src

# Cache module downloads independently of source changes.
COPY go.mod go.sum ./
RUN go mod download

# Compile a static binary (no cgo) so it runs on a minimal base image.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

# ---- runtime stage --------------------------------------------------------
FROM alpine:3.20
RUN adduser -D -u 10001 appuser
COPY --from=build /out/server /usr/local/bin/server
USER appuser

# EXPOSE is informational; the platform injects PORT and routes to it.
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/server"]

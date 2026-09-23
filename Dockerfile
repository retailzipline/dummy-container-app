# syntax=docker/dockerfile:1

# ---------- builder ----------
FROM golang:1.25-alpine AS builder

WORKDIR /src

# No third-party deps, so go.mod alone is enough to warm the module cache.
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

# ---------- runner ----------
FROM alpine:3.22 AS runner

ARG GIT_SHA=unknown

WORKDIR /app

COPY --from=builder /out/server /app/server

# Version stamped from the build arg; the app falls back to "unknown" when the
# file is absent.
RUN printf '%s' "${GIT_SHA}" > /app/.version

# Build-time secret, baked in plainly so /info can echo it back (test app only —
# never do this with a real secret: it persists in the image layer).
RUN --mount=type=secret,id=TEST_BUILD_SECRET,env=TEST_BUILD_SECRET \
    printf '%s' "${TEST_BUILD_SECRET:-unknown}" > /app/.build-secret

EXPOSE 8080

ENTRYPOINT ["/app/server"]

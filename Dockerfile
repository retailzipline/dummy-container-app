# syntax=docker/dockerfile:1

# ---------- builder ----------
# Docker Hardened Image: -dev variant runs as root and ships the Go toolchain
# plus a shell, which the RUN steps below need.
FROM dhi.io/golang:1.25-dev AS builder

WORKDIR /src

# No third-party deps, so go.mod alone is enough to warm the module cache.
COPY go.mod ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/server .

# Runtime data files are produced here, not in the runner: the hardened static
# base has no shell, so RUN is impossible there. Declared after the build so a
# new GIT_SHA does not invalidate the compile.
ARG GIT_SHA=unknown
RUN printf '%s' "${GIT_SHA}" > /out/.version

# Build-time secret, baked in plainly so /info can echo it back (test app only —
# never do this with a real secret: it persists in the image layer).
RUN --mount=type=secret,id=TEST_BUILD_SECRET,env=TEST_BUILD_SECRET \
    printf '%s' "${TEST_BUILD_SECRET:-unknown}" > /out/.build-secret

# ---------- runner ----------
# Docker Hardened Image: no shell, no package manager, runs as nonroot (65532).
# Suitable because the binary above is fully static (CGO_ENABLED=0).
FROM dhi.io/static:20260909-alpine AS runner

WORKDIR /app

COPY --from=builder /out/server /app/server
COPY --from=builder /out/.version /out/.build-secret /app/

EXPOSE 8080

ENTRYPOINT ["/app/server"]

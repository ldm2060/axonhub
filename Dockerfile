FROM --platform=$BUILDPLATFORM node:20-alpine AS frontend-builder

WORKDIR /build
RUN corepack enable && corepack prepare pnpm@10 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN --mount=type=cache,target=/root/.local/share/pnpm/store \
    pnpm install --frozen-lockfile

COPY ./frontend .
ENV NODE_OPTIONS="--max-old-space-size=4096"
# Build the frontend with outDir overridden to /build/dist. vite.config.ts
# points outDir at ../internal/server/static/dist so local builds land in the
# Go embed dir, but the Docker image copies from /build/dist (see frontend-dist
# stage). Run vite directly so we skip the build script's .gitkeep step, which
# writes to the embed dir and is irrelevant here (the embed dir is populated by
# the COPY below).
RUN pnpm exec vite build --emptyOutDir --outDir /build/dist

# Copy dist to a stage with the target platform to avoid architecture mismatch
FROM alpine AS frontend-dist
COPY --from=frontend-builder /build/dist /dist

FROM golang:alpine AS backend-builder

WORKDIR /build

RUN apk add --no-cache git ca-certificates tzdata

COPY go.mod go.sum ./
COPY llm/go.mod llm/go.sum llm/
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOTOOLCHAIN=auto go mod download

COPY . .
COPY --from=frontend-dist /dist /build/internal/server/static/dist

ENV GO111MODULE=on \
    CGO_ENABLED=0 \
    GOOS=linux

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    GOTOOLCHAIN=auto go build \
    -tags=nomsgpack \
    -ldflags "-s -w -X 'github.com/ldm2060/axonhub/internal/build.Version=$(cat internal/build/VERSION 2>/dev/null || echo dev)' -X 'github.com/ldm2060/axonhub/internal/build.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)'" \
    -o axonhub \
    ./cmd/axonhub && \
    ./scripts/verify-go-sse-dependency-test.sh && \
    ./scripts/verify-go-sse-dependency.sh ./axonhub

FROM alpine

RUN apk add --no-cache ca-certificates tzdata
RUN addgroup -S -g 65532 axonhub \
    && adduser -S -D -H -u 65532 -G axonhub axonhub

WORKDIR /app
COPY --from=backend-builder --chown=axonhub:axonhub /build/axonhub /app/axonhub

# The service does not need root privileges at runtime. Keep this in the image
# as well as in Compose so the protection is preserved for other deployments.
USER 65532:65532

EXPOSE 8090
ENTRYPOINT ["/app/axonhub"]

---
# Dockerfile – GPU Platform Operator
# Multi-stage build: compile Go binary → minimal distroless runtime image.
# The resulting image runs as a non-root user with no shell, minimising attack surface.

# ── Stage 1: Build ────────────────────────────────────────────────────────────
FROM golang:1.22-alpine AS builder

WORKDIR /workspace

# Download dependencies first (cached layer)
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# Copy source
COPY main.go main.go
COPY api/ api/
COPY controllers/ controllers/

# Build the operator binary (CGO disabled for static binary)
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o manager main.go

# ── Stage 2: Runtime ──────────────────────────────────────────────────────────
FROM gcr.io/distroless/static:nonroot

WORKDIR /
COPY --from=builder /workspace/manager .

# Use non-root user provided by distroless nonroot image (uid=65532)
USER 65532:65532

ENTRYPOINT ["/manager"]

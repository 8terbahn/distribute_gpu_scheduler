---
# Dockerfile – GPU Workflow Engine
FROM golang:1.22-alpine AS builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY workflow/engine/ workflow/engine/
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o workflow-engine ./workflow/engine/

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/workflow-engine .
USER 65532:65532
ENTRYPOINT ["/workflow-engine"]

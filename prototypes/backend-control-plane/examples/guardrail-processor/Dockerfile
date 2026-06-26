# Build the guardrail-processor binary
# NOTE: Build context must be the prototypes/ directory (parent of backend-control-plane/)
# e.g.: docker build -f backend-control-plane/examples/guardrail-processor/Dockerfile .
FROM golang:1.25 AS builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY backend-control-plane/examples/guardrail-processor/ backend-control-plane/examples/guardrail-processor/
RUN CGO_ENABLED=0 go build -o /guardrail-processor ./backend-control-plane/examples/guardrail-processor/

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /guardrail-processor /guardrail-processor
USER 65532:65532
ENTRYPOINT ["/guardrail-processor"]

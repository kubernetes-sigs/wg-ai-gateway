# Build the payload-processor-controller binary
# NOTE: Build context must be the prototypes/ directory (parent of payload-processor/)
# e.g.: docker build -f payload-processor/controller/Dockerfile .
FROM golang:1.26 AS builder

WORKDIR /workspace
# Copy Go module files first for caching
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# Copy source code
COPY payload-processor/api/ payload-processor/api/
COPY payload-processor/controller/ payload-processor/controller/

RUN CGO_ENABLED=0 go build -o payload-processor-controller ./payload-processor/controller/cmd/

FROM alpine:3.21
COPY --from=builder /workspace/payload-processor-controller /usr/local/bin/
ENTRYPOINT ["payload-processor-controller"]

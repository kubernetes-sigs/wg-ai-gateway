FROM golang:1.24 AS builder
WORKDIR /workspace
COPY ../../go.mod ../../go.sum ../../
COPY . .
RUN CGO_ENABLED=0 go build -o /guardrail-processor .

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /guardrail-processor /guardrail-processor
USER 65532:65532
ENTRYPOINT ["/guardrail-processor"]

# Multi-stage build for the token-lifetime webhook.
FROM golang:1.22 AS builder
WORKDIR /workspace
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o token-lifetime-webhook ./cmd/webhook

FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/token-lifetime-webhook /token-lifetime-webhook
USER 65532:65532
ENTRYPOINT ["/token-lifetime-webhook"]

# Multi-stage build for local development and CI.
# GoReleaser uses Dockerfile.goreleaser instead (pre-built binary).

FROM golang:1.26-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o stoat-bridge ./cmd/stoat-bridge/

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=builder /build/stoat-bridge /stoat-bridge

USER 65534:65534
ENTRYPOINT ["/stoat-bridge"]

FROM golang:1.26.2 AS builder

ARG TARGETARCH

WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} go build \
    -ldflags="-s -w" \
    -o openvox-code \
    ./cmd/openvox-code

FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="openvox-code"
LABEL org.opencontainers.image.description="Fast, Git-native Puppet environment deployment tool"
LABEL org.opencontainers.image.vendor="Simon Lauger"
LABEL org.opencontainers.image.url="https://github.com/slauger/openvox-code"
LABEL org.opencontainers.image.source="https://github.com/slauger/openvox-code"
LABEL org.opencontainers.image.licenses="MIT"

WORKDIR /
COPY --from=builder /workspace/openvox-code .

USER 65532:65532

ENTRYPOINT ["/openvox-code"]

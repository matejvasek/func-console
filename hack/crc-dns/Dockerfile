FROM --platform=$BUILDPLATFORM registry.access.redhat.com/ubi9/go-toolset:1.26 AS builder

USER root
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY main.go .
ARG TARGETOS=linux
ARG TARGETARCH
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -ldflags="-s -w" -o crc-dns .

FROM scratch
COPY --from=builder /build/crc-dns /crc-dns
ENTRYPOINT ["/crc-dns"]

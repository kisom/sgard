# Build stage
FROM golang:1.25-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG VERSION=dev
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /sgardd ./cmd/sgardd

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -h /srv/sgard sgard

COPY --from=builder /sgardd /usr/local/bin/sgardd

VOLUME /srv/sgard
EXPOSE 9473

USER sgard

ENTRYPOINT ["sgardd", \
    "--repo", "/srv/sgard", \
    "--authorized-keys", "/srv/sgard/authorized_keys", \
    "--tls-cert", "/srv/sgard/certs/sgard.pem", \
    "--tls-key", "/srv/sgard/certs/sgard.key"]

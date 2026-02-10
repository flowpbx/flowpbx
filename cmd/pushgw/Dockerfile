# syntax=docker/dockerfile:1

# Multi-stage build for the FlowPBX Push Gateway.
# Build from the repo root:
#   docker build -f cmd/pushgw/Dockerfile -t flowpbx/pushgw .

# --- Build stage ---
FROM golang:1.25-bookworm AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /pushgw ./cmd/pushgw

# --- Runtime stage ---
FROM alpine:3.21

RUN apk add --no-cache ca-certificates wget \
    && addgroup -S pushgw && adduser -S pushgw -G pushgw

COPY --from=build /pushgw /usr/local/bin/pushgw

EXPOSE 8081

USER pushgw:pushgw

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -qO- http://localhost:8081/healthz || exit 1

ENTRYPOINT ["pushgw"]

# Orchis Foundry — single static binary with the frontend embedded (go:embed).
# Multi-stage: build with CGO disabled (pure-Go SQLite), run on a small Alpine
# image that includes `git` (the server shells out to it for clones/packfiles).

FROM golang:1.25-alpine AS build
WORKDIR /src
RUN apk add --no-cache git
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION=docker
RUN CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=${VERSION}" \
    -o /out/orchis-foundry ./cmd/orchis

FROM alpine:3.20
RUN apk add --no-cache git ca-certificates \
    && adduser -D -h /var/lib/orchis-foundry foundry \
    && mkdir -p /var/lib/orchis-foundry \
    && chown -R foundry:foundry /var/lib/orchis-foundry
COPY --from=build /out/orchis-foundry /usr/local/bin/orchis-foundry
USER foundry
# Bind all interfaces inside the container (default config is loopback-only).
ENV ORCHIS_HTTP_ADDR=:8080
EXPOSE 8080 2222
VOLUME ["/var/lib/orchis-foundry"]
ENTRYPOINT ["/usr/local/bin/orchis-foundry"]

FROM golang:1.27-alpine AS builder

ENV GOWORK=off CGO_ENABLED=0 GOOS=linux GOARCH=amd64
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download
COPY . .
RUN go build -o /out/rookeryd ./cmd/rookeryd

# :nonroot — the node holds no local state (no volume to own), just relays
# traffic and talks to the panel it's registered with.
FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/rookeryd /rookeryd
ENTRYPOINT ["/rookeryd", "-config", "/etc/rookery/node.yaml"]

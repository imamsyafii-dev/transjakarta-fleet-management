# syntax=docker/dockerfile:1

FROM golang:1.25-alpine AS builder

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/fleet-server ./cmd/server \
    && CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/geofence-worker ./cmd/geofenceworker

FROM alpine:3.21 AS runtime

RUN addgroup -S fleet && adduser -S -G fleet fleet

WORKDIR /app

COPY --from=builder /out/fleet-server /app/fleet-server
COPY --from=builder /out/geofence-worker /app/geofence-worker

USER fleet

EXPOSE 8080

CMD ["/app/fleet-server"]

# Build stage
FROM golang:alpine AS builder

RUN apk add --no-cache git

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

ARG TARGETOS
ARG TARGETARCH
ARG VERSION=dev

RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    go build -ldflags "-s -w -X main.version=${VERSION}" \
    -o /mibeehive ./cmd/mibeehive

# Final stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata

WORKDIR /app

COPY --from=builder /mibeehive /app/mibeehive
COPY configs/config.yaml /app/config.yaml

RUN mkdir -p /app/data /app/backups

EXPOSE 9090 9443

ENTRYPOINT ["/app/mibeehive"]

FROM golang:1.26-alpine AS builder

WORKDIR /build

# Dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o atap-platform ./cmd/server

# Runtime
FROM alpine:3.19

RUN apk add --no-cache ca-certificates tzdata curl

WORKDIR /app
COPY --from=builder /build/atap-platform .
COPY --from=builder /build/migrations/ ./migrations/

EXPOSE 8080

CMD ["./atap-platform"]

# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the Go binary statically
RUN CGO_ENABLED=0 go build -o speedtest_exporter main.go

# Final minimal image
FROM gcr.io/distroless/static-debian11

COPY --from=builder /app/speedtest_exporter /speedtest_exporter

EXPOSE 9353

ENTRYPOINT ["/speedtest_exporter"]

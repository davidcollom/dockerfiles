# Build stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build the Go binary statically
RUN CGO_ENABLED=0 go build -o isp_exporter main.go

# Final minimal image
FROM gcr.io/distroless/static-debian11

COPY --from=builder /app/isp_exporter /isp_exporter

EXPOSE 9353

ENTRYPOINT ["/isp_exporter"]

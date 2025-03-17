# Start with the official Golang image for building the binary
FROM golang:1.23 as builder

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum first to cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY . .

# Build the application
RUN go build -o vmhub5_exporter .

# Start a new minimal image for running the application
FROM alpine:latest

# Install CA certificates to allow HTTPS connections
RUN apk --no-cache add ca-certificates

# Copy the compiled binary from the builder stage
COPY --from=builder /app/vmhub5_exporter /usr/local/bin/vmhub5_exporter

# Set the port the container will expose
EXPOSE 8080

# Command to run the binary
ENTRYPOINT ["/usr/local/bin/vmhub5_exporter"]

# Default command line arguments (can be overridden)
CMD ["--host", "192.168.0.1", "--interval", "10s"]

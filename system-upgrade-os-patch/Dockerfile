# Stage 1: Build the Go application
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS builder

# Enable Go modules and set necessary environment variables
ENV CGO_ENABLED=0 \
    GOOS=linux

# Set the working directory inside the container
WORKDIR /app

# Copy go.mod and go.sum first to leverage Docker cache
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy the rest of the application
COPY . .

# Define the target architecture for the build
ARG TARGETARCH

# Build the Go binary for the target architecture
RUN GOARCH=$TARGETARCH go build -ldflags="-s -w" -o server

# Stage 2: Create a minimal runtime image
FROM --platform=$TARGETPLATFORM alpine:latest

# Install necessary certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

# Set the working directory inside the container
WORKDIR /app

# Copy the built Go binary from the builder stage
COPY --from=builder /app/server .

# Expose port 8080
EXPOSE 8080

# Run the Go server
ENTRYPOINT ["./server"]

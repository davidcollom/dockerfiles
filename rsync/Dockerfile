FROM --platform=${TARGETPLATFORM} alpine:latest

# Install rsync
RUN apk add --no-cache rsync

# Set the entrypoint to rsync
ENTRYPOINT ["rsync"]

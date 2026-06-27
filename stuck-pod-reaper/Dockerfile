FROM golang:1.26 AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /stuck-pod-reaper ./cmd/

FROM gcr.io/distroless/static-debian12:nonroot

COPY --from=build /stuck-pod-reaper /stuck-pod-reaper
USER nonroot:nonroot

ENTRYPOINT ["/stuck-pod-reaper"]

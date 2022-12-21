FROM alpine:latest as builder
ARG VERSION

RUN apk add --no-cache go

COPY . /app

WORKDIR /app

RUN go mod download
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o hub3-exporter .

FROM alpine:latest

COPY --from=builder /app/hub3-exporter /bin/hub3-exporter

EXPOSE 9463
USER nobody

ENTRYPOINT ["/bin/hub3-exporter"]

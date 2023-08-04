FROM alpine:latest as builder
ARG VERSION

RUN apk add --no-cache go git curl
RUN git clone https://github.com/joe-elliott/cert-exporter && \
    cd cert-exporter && \
    git checkout v${VERSION}

WORKDIR /cert-exporter
RUN go mod download
RUN CGO_ENABLED=0 go build -a -installsuffix cgo -o cert-exporter .


FROM alpine:latest as output

RUN addgroup -g 1000 app && \
    adduser -u 1000 -h /app -G app -S app
WORKDIR /app
USER app

COPY --from=builder /cert-exporter/cert-exporter /app/app

CMD [ "./app" ]

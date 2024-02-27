FROM golang:1.20.0 as builder
ENV GOPATH /go/
ENV PATH $GOPATH/bin:$PATH

RUN apt-get update --fix-missing && apt-get --yes install libsystemd-dev
RUN go version

RUN git clone https://github.com/m-lab/alertmanager-github-receiver.git /go/src/github.com/m-lab/alertmanager-github-receiver
WORKDIR /go/src/github.com/m-lab/alertmanager-github-receiver

RUN go mod download

RUN CGO_ENABLED=0 \
    go build \
    -v \
    -ldflags "-X github.com/m-lab/go/prometheusx.GitShortCommit=$(git log -1 --format=%h) -w -s" \
    ./cmd/github_receiver

FROM gcr.io/distroless/static

COPY --from=builder /go/src/github.com/m-lab/alertmanager-github-receiver/github_receiver ./
ENTRYPOINT ["/github_receiver"]

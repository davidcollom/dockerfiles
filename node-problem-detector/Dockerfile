#syntax=docker/dockerfile:1

# Copyright 2016 The Kubernetes Authors All rights reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.
ARG BASEIMAGE=registry.k8s.io/build-image/debian-base:bullseye-v1.4.3

FROM --platform=${TARGETPLATFORM} golang:1.21 as builder
LABEL maintainer="Andy Xie <andy.xning@gmail.com>"

ENV GOPATH /gopath/
ENV PATH $GOPATH/bin:$PATH


RUN apt-get update --fix-missing && apt-get --yes install libsystemd-dev
RUN go version
RUN go install github.com/tools/godep@latest
RUN godep version

ARG VERSION=v0.8.12
RUN git clone https://github.com/kubernetes/node-problem-detector.git /gopath/src/k8s.io/node-problem-detector
WORKDIR /gopath/src/k8s.io/node-problem-detector

RUN git checkout ${VERSION};
RUN CGO_ENABLED=1 GOOS=linux GOARCH=${TARGETARCH} go build \
      -o bin/node-problem-detector \
      -ldflags '-X k8s.io/node-problem-detector/pkg/version.version=v0.8.18' \
      -tags "journald " \
      ./cmd/nodeproblemdetector
RUN CGO_ENABLED=1 GOOS=linux GOARCH=${TARGETARCH} go build \
      -o bin/health-checker \
      -ldflags '-X k8s.io/node-problem-detector/pkg/version.version=v0.8.18' \
      -tags "journald " \
      cmd/healthchecker/health_checker.go

RUN CGO_ENABLED=1 GOOS=linux GOARCH=${TARGETARCH} go build \
    -o bin/log-counter \
    -ldflags '-X k8s.io/node-problem-detector/pkg/version.version=v0.8.18' \
    -tags "journald " \
    cmd/logcounter/log_counter.go


ARG BASEIMAGE
FROM --platform=${TARGETPLATFORM} ${BASEIMAGE}

RUN clean-install util-linux bash libsystemd-dev libc6

# Avoid symlink of /etc/localtime.
RUN test -h /etc/localtime && rm -f /etc/localtime && cp /usr/share/zoneinfo/UTC /etc/localtime || true

COPY --from=builder /gopath/src/k8s.io/node-problem-detector/bin/node-problem-detector /node-problem-detector

ARG LOGCOUNTER
COPY --from=builder /gopath/src/k8s.io/node-problem-detector/bin/health-checker /gopath/src/k8s.io/node-problem-detector/${LOGCOUNTER} /home/kubernetes/bin/

COPY --from=builder /gopath/src/k8s.io/node-problem-detector/config/ /config
ENTRYPOINT ["/node-problem-detector", "--config.system-log-monitor=/config/kernel-monitor.json"]

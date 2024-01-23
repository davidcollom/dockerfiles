#!/usr/bin/env bash

VERSION=$0

ARCH=$(uname -m)

if [ $ARCH == "armv7l" ]; then
ARCH="arm"
elif [ $ARCH == "aarch64"]; then
ARCH="arm64"
fi

curl -LO "https://dl.k8s.io/release/${VERSION}/bin/linux/${ARCH}/kubectl"

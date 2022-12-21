# Copyright (c) 2021 Tailscale Inc & AUTHORS All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
ARG VERSION

FROM ghcr.io/tailscale/tailscale:$VERSION@sha256:8d6bfd726de0e5d7c0f8a19f8f7990c9e60cd4de5250fd184e87ecdc0d6e6130
COPY run.sh /run.sh
CMD "/run.sh"

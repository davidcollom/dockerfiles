# Copyright (c) 2021 Tailscale Inc & AUTHORS All rights reserved.
# Use of this source code is governed by a BSD-style
# license that can be found in the LICENSE file.
ARG VERSION

FROM --platform=${TARGETPLATFORM} ghcr.io/tailscale/tailscale:${VERSION}
COPY run.sh /run.sh
CMD "/run.sh"

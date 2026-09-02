# minio

This directory does not build MinIO from source. Its `image.<arch>` files identify existing architecture-specific upstream images that the root build script combines into a single `davidcollom/minio:<VERSION>` manifest.

The purpose is compatibility with older multi-architecture publishing workflows: consumers can pull one tag and Docker selects the matching AMD64, ARM64, or ARMv7 image.

Change `VERSION` only after confirming that every referenced upstream architecture tag exists. Runtime configuration, ports, volumes, and credentials remain those of the upstream MinIO image for that historical release.

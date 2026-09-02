# mkv-auto

This image clones the upstream `philiptn/mkv-auto` project, checks out the revision in `VERSION`, initializes submodules, and installs its prerequisites into Ubuntu. My customization adds the repository's `config.ini`, giving media-processing jobs a reproducible configuration rather than configuring a generic image on every run.

The upstream entrypoint is retained:

```sh
docker run --rm -v "$PWD/media:/media" davidcollom/mkv-auto:master
```

Inspect and adjust `config.ini` before building. Mount source/output media at paths expected by that configuration, and test against disposable files first because automated media processing can rename or replace content.

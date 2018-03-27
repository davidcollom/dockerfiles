#!/usr/bin/env bash
set -e

for i in $(ls -d *); do
  if [[ -e "${i}/Dockerfile" ]]; then
    echo "[${i}] Building..."
    docker build --pull -f ${i}/Dockerfile -t "${i}" ${i}/
    echo "[${i}] Done"
  fi
done

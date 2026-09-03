#!/usr/bin/env bash
set -Eeuo pipefail

publish_script="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/publish-nostr.sh"
interval="${FRACT_INTERVAL_SECONDS:-1800}"

[[ "$interval" =~ ^[1-9][0-9]*$ ]] || {
  printf 'error: FRACT_INTERVAL_SECONDS must be a positive integer\n' >&2
  exit 1
}

trap 'exit 0' INT TERM

while true; do
  printf 'starting scheduled publication at %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >&2
  if "$publish_script" --env /dev/null "$@"; then
    printf 'scheduled publication completed\n' >&2
  else
    printf 'scheduled publication failed; retrying in %s seconds\n' "$interval" >&2
  fi
  sleep "$interval" &
  wait $!
done

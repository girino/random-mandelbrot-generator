#!/usr/bin/env bash
set -Eeuo pipefail

script_dir="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)"
root_dir="$(cd -- "$script_dir/.." && pwd)"

grep -Fq 'FRACT_RELAYS=wss://relay.damus.io,wss://nos.lol' "$root_dir/.env.example"
grep -Fq 'nak blossom' "$script_dir/publish-nostr.sh"
grep -Fq 'imeta=url' "$script_dir/publish-nostr.sh"
grep -Fq 'reproduce_command=' "$script_dir/publish-nostr.sh"
grep -Fq 'blob-descriptor.json' "$script_dir/publish-nostr.sh"
grep -Fq 'env_file="$project_root/.env"' "$script_dir/publish-nostr.sh"
grep -Fq 'verified hash from Blob URL filename' "$script_dir/publish-nostr.sh"
grep -Fq 'FRACT_INTERVAL_SECONDS' "$script_dir/publish-loop.sh"
printf 'publish-nostr shell checks passed\n'

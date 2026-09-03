#!/usr/bin/env bash
set -Eeuo pipefail

PROJECT_URL="https://github.com/girino/random-mandelbrot-generator"
DEFAULT_ALT="Imagem do conjunto de Mandelbrot gerada proceduralmente."

env_file=".env"
alt_text="$DEFAULT_ALT"
extra_content=""
dry_run=false
keep_failed=false
fract_args=()
tmp_dir=""

usage() {
  cat <<'EOF'
Usage: scripts/publish-nostr.sh [options] [fract random options]

Options:
  --env FILE       Configuration file (default: .env)
  --alt TEXT       Image alt text
  --content TEXT   Caption prepended to the Nostr note
  --dry-run        Generate artifacts and print the note without upload or relay access
  --keep-failed    Preserve temporary artifacts after a failure
  --help           Show this help

Output, output-dir, and metadata flags are managed by this script and cannot
be passed through to fract random.
EOF
}

fail() {
  printf 'error: %s\n' "$*" >&2
  exit 1
}

cleanup() {
  local status=$?
  unset NOSTR_CLIENT_KEY || true
  if [[ -n "$tmp_dir" && -d "$tmp_dir" && ( $status -eq 0 || "$keep_failed" != true ) ]]; then
    rm -rf "$tmp_dir"
  elif [[ -n "$tmp_dir" && -d "$tmp_dir" ]]; then
    printf 'preserved failed artifacts in %s\n' "$tmp_dir" >&2
  fi
}
trap cleanup EXIT

while (($#)); do
  case "$1" in
    --env)
      (($# >= 2)) || fail '--env requires a file path'
      env_file="$2"
      shift 2
      ;;
    --alt)
      (($# >= 2)) || fail '--alt requires text'
      alt_text="$2"
      shift 2
      ;;
    --content)
      (($# >= 2)) || fail '--content requires text'
      extra_content="$2"
      shift 2
      ;;
    --dry-run)
      dry_run=true
      shift
      ;;
    --keep-failed)
      keep_failed=true
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    --output|--output-dir|--metadata)
      fail "$1 is managed by this script"
      ;;
    --output=*|--output-dir=*|--metadata=*)
      fail "${1%%=*} is managed by this script"
      ;;
    *)
      fract_args+=("$1")
      shift
      ;;
  esac
done

[[ -f "$env_file" ]] || fail "configuration file not found: $env_file"
# The local .env is trusted configuration and must not be committed.
set -a
# shellcheck source=/dev/null
. "$env_file"
set +a

: "${FRACT_BIN:=fract}"
: "${NAK_BIN:=nak}"
: "${FRACT_OUTPUT_DIR:?FRACT_OUTPUT_DIR is required}"

if [[ "$dry_run" != true ]]; then
  : "${FRACT_BUNKER_URI:?FRACT_BUNKER_URI is required}"
  : "${FRACT_BLOSSOM_SERVER:?FRACT_BLOSSOM_SERVER is required}"
  : "${FRACT_RELAYS:?FRACT_RELAYS is required}"
fi

for command in "$FRACT_BIN" jq sha256sum; do
  command -v "$command" >/dev/null 2>&1 || fail "required command not found: $command"
done
if [[ "$dry_run" != true ]]; then
  command -v "$NAK_BIN" >/dev/null 2>&1 || fail "required command not found: $NAK_BIN"
fi

mkdir -p "$FRACT_OUTPUT_DIR"
umask 077
tmp_dir="$(mktemp -d "${TMPDIR:-/tmp}/fract-nostr.XXXXXX")"
png_path="$tmp_dir/image.png"
metadata_path="$tmp_dir/image.json"

"$FRACT_BIN" random --output "$png_path" --metadata "$metadata_path" "${fract_args[@]}"

center_real="$(jq -er '.center_real' "$metadata_path")"
center_imag="$(jq -er '.center_imag' "$metadata_path")"
zoom="$(jq -er '.zoom' "$metadata_path")"
iterations="$(jq -er '.iterations' "$metadata_path")"
max_iterations="$(jq -er '.max_iterations' "$metadata_path")"
adaptive="$(jq -er '.adaptive' "$metadata_path")"
palette="$(jq -er '.palette' "$metadata_path")"
smooth="$(jq -er '.smooth' "$metadata_path")"
width="$(jq -er '.width' "$metadata_path")"
height="$(jq -er '.height' "$metadata_path")"

reproduce_command="fract render mandelbrot --center \"${center_real},${center_imag}\" --zoom ${zoom} --iterations ${iterations} --max-iterations ${max_iterations} --adaptive=${adaptive} --palette ${palette} --smooth=${smooth} --width ${width} --height ${height} --output reproduced.png"
sha256="$(sha256sum "$png_path" | awk '{print $1}')"
size="$(wc -c < "$png_path" | tr -d '[:space:]')"

if [[ "$dry_run" == true ]]; then
  cat >&2 <<EOF
dry run: image generated without upload or publication
reproduce with: $reproduce_command
project: $PROJECT_URL
EOF
  blob_url="https://example.invalid/not-uploaded.png"
else
  export NOSTR_CLIENT_KEY
  NOSTR_CLIENT_KEY="$("$NAK_BIN" key generate)"
  blossom_host="${FRACT_BLOSSOM_SERVER#https://}"
  blossom_host="${blossom_host#http://}"
  blossom_host="${blossom_host%%/*}"
  blob_descriptor="$("$NAK_BIN" blossom --server "$blossom_host" --sec "$FRACT_BUNKER_URI" upload "$png_path")"
  blob_url="$(jq -er '.url' <<<"$blob_descriptor")"
  blob_sha256="$(jq -er '.sha256' <<<"$blob_descriptor")"
  blob_size="$(jq -er '.size' <<<"$blob_descriptor")"
  blob_type="$(jq -er '.type' <<<"$blob_descriptor")"
  [[ "$blob_url" == https://* ]] || fail 'Blossom returned a non-HTTPS URL'
  [[ "$blob_sha256" == "$sha256" ]] || fail 'Blossom SHA-256 does not match upload'
  [[ "$blob_size" == "$size" ]] || fail 'Blossom size does not match upload'
  [[ "$blob_type" == image/png ]] || fail 'Blossom MIME type is not image/png'

  note="Imagem gerada com fract.

$blob_url

Reproduzir:
$reproduce_command

Projeto:
$PROJECT_URL"
  if [[ -n "$extra_content" ]]; then
    note="$extra_content

$note"
  fi
  IFS=',' read -r -a relays <<<"$FRACT_RELAYS"
  relay_args=()
  for relay in "${relays[@]}"; do
    relay="${relay//[[:space:]]/}"
    [[ "$relay" == ws://* || "$relay" == wss://* ]] || fail "invalid relay URL"
    relay_args+=("$relay")
  done
  ((${#relay_args[@]})) || fail 'FRACT_RELAYS contains no relay URLs'
  event_json="$("$NAK_BIN" event --sec "$FRACT_BUNKER_URI" --kind 1 --content "$note" --tag "imeta=url $blob_url;m image/png;x $sha256;size $size;dim ${width}x${height};alt $alt_text" "${relay_args[@]}")"
  event_id="$(jq -er '.id' <<<"$event_json")"
  printf 'published %s\nevent: %s\n' "$blob_url" "$event_id" >&2
fi

number=0
while :; do
  base="$(printf 'frac%03d' "$number")"
  final_png="$FRACT_OUTPUT_DIR/$base.png"
  final_metadata="$FRACT_OUTPUT_DIR/$base.json"
  if [[ ! -e "$final_png" && ! -e "$final_metadata" ]]; then
    mv "$png_path" "$final_png"
    mv "$metadata_path" "$final_metadata"
    break
  fi
  ((number += 1))
done
printf 'saved %s and %s\n' "$final_png" "$final_metadata" >&2

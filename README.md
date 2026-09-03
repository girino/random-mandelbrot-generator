# fract

`fract` is a script-friendly CLI for rendering Mandelbrot images. The first
release renders Mandelbrot sets on the CPU and writes opaque RGB PNG files.

## Install

Download the archive for your platform from GitHub Releases, verify its entry
in `SHA256SUMS`, and place `fract` on your `PATH`.

With Go installed, you can also run:

```sh
go install github.com/girino/random-mandelbrot-generator@latest
```

## Usage

```sh
fract render mandelbrot --output mandelbrot.png
fract render mandelbrot --size 1920x1080 --center "-0.25 + .37 i" --zoom 400 --iterations 2000 --palette viridis --smooth --output detail.png
fract render mandelbrot --output - --palette fire | uploader
fract random --seed 42 --size 1080x1080 --metadata post.json --output post.png
fract random --output-dir images
fract random --random-palette --output-dir images
```

Use either `--output` or `--output-dir`. `--output -` writes only PNG bytes to
standard output; all status, warnings, errors, and optional progress remain on
standard error. Existing output files require `--force` to overwrite.

## Commands

```text
fract render mandelbrot [options]
fract random [options]
fract palettes list
fract --version
```

## Render options

| Option | Default | Description |
| --- | --- | --- |
| `--output PATH|-` | one output mode required | PNG destination or standard output |
| `--output-dir PATH` | | Write the next sequential `fracNNN.png` in a directory |
| `--width N`, `--height N` | `1024`, `1024` | Image dimensions |
| `--size WIDTHxHEIGHT` | | Alternative to width and height |
| `--center VALUE` | `-0.5,0` | `REAL,IMAG` or complex notation such as `-0.25+.37i` |
| `--zoom N` | `1` | Linear zoom factor |
| `--iterations N` | `64` | Initial full-image iteration count |
| `--max-iterations N` | `262144` | Adaptive iteration ceiling |
| `--adaptive` | on | Refine only interior boundary pixels up to the ceiling |
| `--palette NAME` | `rgb` | Named color palette |
| `--smooth` | off | Smooth escape coloring |
| `--threads N` | CPU count | Render workers |
| `--force` | off | Overwrite output files |
| `--unsafe-limits` | off | Permit oversized renders |
| `--progress` | off | Emit row progress to standard error |
| `--quiet` | off | Suppress normal status messages |

The default viewport has width `3.5`, centered at `-0.5 + 0i`. Its height is
derived from the image aspect ratio, keeping pixels square. Positive imaginary
coordinates are at the top of the image.

The safe limits are 16,384 pixels per side, 100,000,000 total pixels, and
1,048,576 iterations. Use `--unsafe-limits` only when you intentionally need
to exceed them. Very deep zooms can exceed useful `float64` precision; `fract`
emits a warning and continues.

By default, rendering follows the Android adaptive policy: a complete pass at
`--iterations`, then successive doubles up to `--max-iterations` (at most 18
rounds). Only interior pixels touching escaped pixels, plus interior viewport
edges, are re-tested. Use `--adaptive=false` for a fixed-iteration render.

## Palettes

`rgb`, `fire`, `ocean`, `grayscale`, `sunset`, `neon`, `viridis`, and
`electric` reproduce the selected palettes from
[android-mandelbrot-set](https://github.com/girino/android-mandelbrot-set).

## Random exploration

`fract random` finds Mandelbrot detail without relying on a fixed coordinate
catalog. It renders a low-resolution classification grid, finds cells where
interior and escaping samples meet, selects one weighted by local transition
complexity, and then recenters and zooms. This repeats for a random number of
passes before the final render.

| Option | Default | Description |
| --- | --- | --- |
| `--seed N` | current time | Reproducible exploration seed |
| `--min-passes N` | `3` | Minimum border exploration passes |
| `--max-passes N` | `12` | Maximum border exploration passes |
| `--min-zoom N` | `1.5` | Minimum zoom applied per pass |
| `--max-zoom N` | `4` | Maximum zoom applied per pass |
| `--sample-size N` | `128` | Width of the low-resolution classification grid |
| `--random-palette` | off | Select a palette using the seeded random generator |
| `--metadata PATH` | | Optional JSON record of the generated coordinates |

All render options such as `--size`, `--iterations`, `--threads`, `--smooth`,
and `--palette` are available. The default palette is `rgb`.
Random exploration grids use the Android Scale-with-zoom defaults: base `40`,
multiplier `1.2`, and a range of `10` to `1,048,576` iterations. Their count
increases logarithmically with each selected zoom. Adaptive refinement runs
only once, for the final full-resolution image. That final refinement continues
until it reaches at least the highest iteration count used by an exploration
grid, even if an earlier adaptive level found no new escaping pixels.
The metadata stores the seed, chosen center, final zoom, dimensions, smoothing,
palette, and every exploration step, so a generated image can be reproduced
with `fract render`.

After every `fract random` render, standard error prints a `fract render
mandelbrot` command with the exact final render parameters. It writes to
`reproduced.png`; change that output path before running it.

## Nostr Publishing

`scripts/publish-nostr.sh` generates a random image, uploads it to Blossom
through `nak` using a NIP-46 bunker, and publishes a `kind:1` Nostr note. The
note includes the image URL, NIP-92 `imeta` metadata, the reproduction command,
and the project URL.

Prerequisites: Bash 4+, `fract`, `nak`, `jq`, and `sha256sum`. Use Linux, Git
Bash, or WSL. Copy `.env.example` to `.env` in the project root and provide
your bunker URI and output directory; `.env` is ignored by Git. The script
loads this project-root `.env` regardless of the current directory.

```bash
cp .env.example .env
bash scripts/publish-nostr.sh --seed 42 --size 1080x1080 --random-palette
```

Use `--dry-run` to generate the PNG and metadata without requesting a bunker
signature, uploading, or publishing. `--alt` and `--content` customize the
image description and caption. The script reserves `--output`, `--output-dir`,
and `--metadata`; all other arguments are passed to `fract random`.

### Docker

Docker Compose loads `.env` as container environment variables without
modifying or copying it into the image. Generated PNG and JSON files are saved
to `./output` on the host. The worker publishes immediately, then every 30
minutes (`FRACT_INTERVAL_SECONDS=1800`):

```bash
docker compose up --build
```

Override `FRACT_INTERVAL_SECONDS` in Compose or the runtime environment to
change the interval. To run one safe dry-run instead of the worker:

```bash
docker compose run --rm --entrypoint /app/scripts/publish-nostr.sh nostr-publish --env /dev/null --dry-run --seed 42 --size 1080x1080
```

To publish once without starting the loop:

```bash
docker compose run --rm --entrypoint /app/scripts/publish-nostr.sh nostr-publish --env /dev/null --seed 42 --size 1080x1080 --random-palette
```

## Development

```sh
go test ./...
go build ./...
```

The executable is in `cmd/fract`. The CLI, generic escape-time rendering, and
palette catalog live respectively in `internal/cli`, `internal/fractal`, and
`internal/palette`. The visual regression fixture is versioned beside the
fractal tests. Tests compare RGB pixels rather than encoded PNG bytes so
encoder metadata cannot cause noise.

## License

This project is licensed under the Girino Anarchist License (GAL). See
[`LICENSE`](LICENSE).

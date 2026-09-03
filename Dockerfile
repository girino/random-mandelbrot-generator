FROM golang:1.27-bookworm AS build

WORKDIR /src

COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal

RUN CGO_ENABLED=0 go build -o /out/fract ./cmd/fract
RUN GOBIN=/out go install github.com/fiatjaf/nak@v0.20.6

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends bash ca-certificates coreutils jq \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=build /out/fract /usr/local/bin/fract
COPY --from=build /out/nak /usr/local/bin/nak
COPY scripts/publish-nostr.sh /app/scripts/publish-nostr.sh
COPY scripts/publish-loop.sh /app/scripts/publish-loop.sh

RUN chmod 0755 /app/scripts/publish-nostr.sh /app/scripts/publish-loop.sh

ENTRYPOINT ["/app/scripts/publish-loop.sh"]
CMD ["--random-palette"]

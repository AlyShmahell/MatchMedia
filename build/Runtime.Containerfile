# syntax=docker/dockerfile:1
FROM docker.io/library/debian:trixie-slim
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /home/matchmedia
ENTRYPOINT ["/home/matchmedia/.local/bin/matchmedia"]

# Workflow

Container toolchains only. Do not run host `go` or browsers against this repo’s automated path. Podman-first (`Containerfile` spelling). Images are pulled on first compose build.

## Dist

[build/Containerfile](../../build/Containerfile) is a one-shot **builder**, not a runtime. Go stage: `docker.io/library/golang:1.26-bookworm`, `CGO_ENABLED=0 GOOS=linux GOARCH=amd64`. Final stage: `debian:trixie-slim` copies the binary to `.local/bin/matchmedia`, `share/config` → `.local/share/matchmedia/config/`, and `gui/` → `.local/share/matchmedia/public/`. Dist is that home tree only. Compose bind-mounts `build/dist` (`:z`) and copies out; the container exits.

## Run the app

```bash
./build/run
```

Choose **run**, **(re)build & run**, or **(re)build & package** (arrow keys, Enter). **run** starts the server in a container (error if the binary is missing) with `build/cache/xdg` mounted as `/home/matchmedia`. The rebuild options refresh `build/dist/` first. Package writes one archive with root `matchmedia/` and MatchMedia’s `LICENSE`: `build/package/matchmedia-<version>-linux-amd64.tar.gz`.

The admin console is served at `http.addr` from `config/default.yaml`, shipped as `127.0.0.1:7680`. The dev container uses the host network, so that bind is this computer's localhost. Set `http.addr` to `:7680` in the overlay to listen on every interface.

`build/dist` mirrors the home tree: `.local/bin/matchmedia`, `.local/share/matchmedia/config/default.yaml` (from [matchmedia/share/config/default.yaml](../../matchmedia/share/config/default.yaml)), and `.local/share/matchmedia/public/`. `./build/run` copies those into `build/cache/xdg` and mounts that directory at `/home/matchmedia`. The container sets `HOME=/home/matchmedia` and does not set `XDG_*`, so the overlay, secrets, and catalog land in `.local/share/matchmedia` on that mount and session jobs land in `.local/state/matchmedia`. Host `/mnt` and `/media` are bind-mounted at the same paths without `:z`, so Podman does not relabel them. The app service sets `label=disable` so SELinux still allows the container to read those directories.

## Tests

Podman only. The runner image is Debian trixie-slim. Tests do not run the builder image. They set `HOME=/home/matchmedia` and mount the dist binary, `default.yaml`, and `public/` onto files created in the image. [tests/config.yaml](../../tests/config.yaml) is the overlay at `.local/share/matchmedia/config/overlay.yaml`. The library fixture is `/home/matchmedia/library`.

```bash
./tests/run
```

`./tests/run` first builds `build/dist/` via [build/compose.yaml](../../build/compose.yaml), then runs check, unit, and smoke. Smoke hits `/health`, the admin page (including counter chips and secrets), `GET /v1/config`, `GET`/`POST /v1/secrets` (set and clear a dummy OMDb key, waiting until `/health` drops then returns after each restart), `POST /v1/ingest` (stub metadata), `POST /v1/scan` (`202` with `session`, `files`, and `mode`, filesystem grouping into shows), polls `GET /v1/jobs?session=` until rows are matched, then `POST /v1/retry?session=`. Check asserts the dist layout (binary / config / public). Unit tests are `go test ./lib/match ./lib/scan ./lib/config ./lib/jobs ./lib/ingest ./lib/library` in the Go toolchain image.

Live is a skip unless you pass `MATCHMEDIA_LIVE=1` to `./tests/run` (compose `--profile live`, real TVMaze/Jikan).

Equivalent after dist exists:

```bash
cd tests
podman compose -f compose.yaml down
podman compose -f compose.yaml up --build --abort-on-container-exit --exit-code-from tester
podman compose -f compose.yaml down
```

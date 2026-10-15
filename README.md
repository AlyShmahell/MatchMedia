<p align="center">
  <img src="docs/matchmedia.svg" alt="MatchMedia" width="180">
</p>

<h1 align="center">MatchMedia</h1>
<h6 align="center">Match media to metadata.</h6>

- **MatchMedia** is a statistical engine for directory hierarchy discovery of media libraries, title extraction and grouping, metadata search-and-fetch from YAML providers (shipped [TMDB](https://developer.themoviedb.org/)), scoring and ranking candidates, and mapping them to the appropriate files.
- **MatchMedia** can ingest a list of titles and search, fetch, and map metadata for them.

## Install the precompiled CLI

```bash
curl -fL https://github.com/alyshmahell/matchmedia/releases/latest/download/matchmedia-linux-amd64.tar.gz \
  | tar -xz -C "$HOME" --strip-components=1 matchmedia/.local
```

This unpacks `.local/bin/matchmedia` under `$HOME`.

## Run as a CLI

```bash
matchmedia -path /media/Shows -mode nfo
```

`-save` writes NFO and posters next to the media.

| Flag | Meaning |
| --- | --- |
| `-overlay` | Overlay YAML. Empty means shipped default only. |
| `-path` | Library directory. Required unless `-ingest` is set. |
| `-mode` | `rescan`, `changes`, `missing`, or `nfo`. Default is `rescan`. |
| `-ingest` | CSV or JSON title list instead of `-path`. |
| `-report` | Markdown report path. Default is `match.md`. |
| `-save` | Write NFO and posters next to the media. Default is off. |
| `-help` | Print usage, license, and attribution, then exit. |

Set the TMDB key in overlay YAML file and pass it with `-overlay <file-path>`:

```yaml
providers:
  tmdb:
    api_key: YOUR_KEY
```

## Use as a Go module

Module path: `github.com/alyshmahell/matchmedia/src`.
Import prefix: `github.com/alyshmahell/matchmedia/src/mod`.

```bash
go get github.com/alyshmahell/matchmedia/src
```

```go
import (
	"github.com/alyshmahell/matchmedia/src/mod/config"
	"github.com/alyshmahell/matchmedia/src/mod/match"
	"github.com/alyshmahell/matchmedia/src/mod/sidecar"
	"github.com/alyshmahell/matchmedia/src/mod/scan"
)

cfg, err := config.New("overlay.yaml")
jobs, err := scan.Run(cfg, "/media/Shows", scan.ModeMissing)
jobs = match.Run(ctx, cfg, jobs)
_, err = sidecar.Write(ctx, cfg, jobs)
```

For grouping, ranking, scan modes, TMDB YAML, and the rest of the CLI, see [docs/manual.md](docs/manual.md).

## Attribution
#### TMDb
<div align="center">
  <img src="docs/tmdb-attribution.svg" alt="The Movie Database" width="140">
  </br>
  <b>
  This product uses the TMDb API but is not endorsed or certified by TMDb.
  </b>
  </br>
  <p>https://www.themoviedb.org/</p>

</div>



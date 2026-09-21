<p align="center">
  <img src="matchmedia/share/assets/matchmedia.svg" alt="MatchMedia" width="180">
</p>

<h1 align="center">MatchMedia</h1>
<h6 align="center">Match Media to Metadata.</h6>

  
- **MatchMedia** is a statistical engine responsible for directory hierarchy discovery of media libraries, title extraction and grouping, metadata search-and-fetch from providers like TVMaze, statistically scoring and ranking candidates and mapping them to the appropriate files.  
- **MatchMedia** is also capable of ingesting a list of titles for which it can search, fetch and map metadata.  
- The browser UI is a dev console.


Shipped providers: 
- no apikey required:
  - [TVMaze](https://www.tvmaze.com/api) 
  - [Jikan](https://docs.api.jikan.moe/) (no key). 
- apikey required:
  - [OMDb](https://www.omdbapi.com/)
  - [TMDB](https://developer.themoviedb.org/)

## Install
Download `matchmedia-<version>-linux-amd64.tar.gz` from [GitHub Releases](https://github.com/alyshmahell/matchmedia/releases). Unpack it. The archive root is `matchmedia/` (binary, `config/`, `public/`, `LICENSE`).

## Run
From that directo
```bash
./matchmedia
```

The process listens on `http.addr` from `config/default.yaml` (shipped as port 7680). The dev console can also be reached on that host and port.

Runtime data lives in `data/` next to the binary: session job files, `secrets`, an optional `config.yaml` overlay, and the NFO catalog. Pass `-config` to load a different `default.yaml`.

## Library and keys

The folder picker stays inside `browse_root`, which defaults to `data/`. Point it at a real library by setting `browse_root` in `data/config.yaml` (same keys as `default.yaml`).

TVMaze and Jikan need no key. Set OMDb and TMDB keys in `data/secrets`, or in the dev console secrets panel.

## Use

### As a RESTful API (Intended Use Case):

Check [architecture](docs/architecture.md) then [match](docs/design/match.md).

### As a WebUI (Intended for Development):
Check [gui](docs/design/gui.md):
- **Scan** a path under the browse root. MatchMedia groups files into titles, then searches providers.
- **Ingest** a CSV or JSON list of titles (optional year, type, season, episode, IMDb id).
- High-confidence hits auto-match. Close scores stay **manual** until you pick a candidate.
- Matched titles are written under `data/catalog` as NFO trees with posters.



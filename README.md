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
Download `matchmedia-<version>-linux-amd64.tar.gz` from [GitHub Releases](https://github.com/alyshmahell/matchmedia/releases). Unpack it. The archive root is `matchmedia/` (`.local/bin/matchmedia`, `.local/share/matchmedia/`, `LICENSE`).

## Run
From that directo
```bash
./matchmedia
```

The process listens on `http.addr` from `config/default.yaml`, shipped as `127.0.0.1:7680`, so only this computer can open it. Set `http.addr` to `:7680` in `$XDG_DATA_HOME/matchmedia/config/overlay.yaml` to listen on every interface.

The binary belongs in `$HOME/.local/bin/matchmedia`. Shipped `config/default.yaml` and `public/` live in `$XDG_DATA_HOME/matchmedia` (default `~/.local/share/matchmedia`). The user overlay is `config/overlay.yaml` and provider keys are `config/secrets` in that same directory. The NFO catalog is `catalog/` there. Session job files live in `$XDG_STATE_HOME/matchmedia` (default `~/.local/state/matchmedia`). Pass `-config` to load a different `default.yaml`.

## Library and keys

The folder picker stays inside `browse_roots` from `$XDG_DATA_HOME/matchmedia/config/default.yaml`. The shipped list is `/mnt`, `/media`, `$XDG_VIDEOS_DIR`, and `$XDG_MUSIC_DIR` (`$HOME/Videos` and `$HOME/Music`, or the paths in `user-dirs.dirs`). A path is allowed only when that directory exists. Replace the list in `$XDG_DATA_HOME/matchmedia/config/overlay.yaml`.

TVMaze and Jikan need no key. Set OMDb and TMDB keys in `$XDG_DATA_HOME/matchmedia/config/secrets`, or in the dev console secrets panel.

## Use

### As a RESTful API (Intended Use Case):

Check [architecture](docs/architecture.md) then [match](docs/design/match.md).

### As a WebUI (Intended for Development):
Check [gui](docs/design/gui.md):
- **Scan** a path under the browse root. MatchMedia groups files into titles, then searches providers.
- **Ingest** a CSV or JSON list of titles (optional year, type, season, episode, IMDb id).
- High-confidence hits auto-match. Close scores stay **manual** until you pick a candidate.
- Matched titles are written under `$XDG_DATA_HOME/matchmedia/catalog` as NFO trees with posters.



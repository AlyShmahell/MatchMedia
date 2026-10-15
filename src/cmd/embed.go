package main

import _ "embed"

//go:generate cp ../../LICENSE LICENSE
//go:generate cp ../../docs/logo.png logo.png
//go:generate cp ../../docs/tmdb-attribution.md tmdb-attribution.md

//go:embed LICENSE
var licenseText string

//go:embed logo.png
var logoPNG []byte

//go:embed tmdb-attribution.md
var attributionText string

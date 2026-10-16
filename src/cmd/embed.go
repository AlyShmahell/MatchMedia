package main

import _ "embed"

//go:generate cp ../../LICENSE LICENSE
//go:generate cp ../../docs/logo.png logo.png
//go:generate cp ../../docs/kofi.png kofi.png
//go:generate cp ../../docs/attribution.md attribution.md

//go:embed LICENSE
var licenseText string

//go:embed logo.png
var logoPNG []byte

//go:embed kofi.png
var kofiPNG []byte

//go:embed attribution.md
var attributionText string

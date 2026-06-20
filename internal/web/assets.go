package web

import "embed"

//go:embed templates/*.html static/*
var assets embed.FS

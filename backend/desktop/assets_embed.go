package main

import "embed"

// embeddedRuntimeAssets is populated by package-exe.ps1 before desktop build.
//go:embed all:runtime/frontend/dist all:runtime/migrations
var embeddedRuntimeAssets embed.FS

//go:build no_embed
// +build no_embed

package main

import (
	"embed"
)

// WebAssets placeholder - we won't actually embed when no_embed is used,
// so this won't be used, but we still need it defined to satisfy the compiler
var WebAssets embed.FS

// UseEmbedded indicates whether to use embedded assets
const UseEmbedded = false
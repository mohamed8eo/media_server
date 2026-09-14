package web

import (
	"embed"
	"strconv"
	"time"
)

//go:embed "assets"
var Files embed.FS

// AssetVersion is appended as a cache-busting query param to every static
// asset URL rendered in templates. It's computed once when the server starts,
// so every deploy/restart automatically invalidates browsers' cached copies
// of JS/CSS files (which are served with a 1-year immutable Cache-Control
// header) — without anyone needing to remember to bump a version number by
// hand for each changed file.
var AssetVersion = strconv.FormatInt(time.Now().Unix(), 10)

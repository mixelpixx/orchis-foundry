// Package web embeds the Orchis Foundry frontend prototype so the binary is
// self-contained. The files here are a copy of the top-level prototype
// (index.html + src/ + tweaks-panel.jsx); treat the top-level copy as the
// source of truth and re-sync on frontend changes.
package web

import "embed"

//go:embed index.html tweaks-panel.jsx src
var FS embed.FS

// Package web embeds the Orchis Foundry frontend so the binary is
// self-contained. We ship the *precompiled* assets: index.html loads the
// vendored React UMD builds and a single transpiled bundle (app.bundle.js).
//
// Source of truth is web/src/*.jsx; rebuild the bundle with
// `cd build && npm run build` (scripts/build-web.mjs) after editing.
package web

import "embed"

//go:embed index.html app.bundle.js vendor
var FS embed.FS

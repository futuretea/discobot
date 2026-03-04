// Package static embeds the API browser HTML served at /api/ui.
// api-ui.html is kept in sync with server/static/api-ui.html — copy it
// manually when the server's version is updated.
package static

import "embed"

//go:embed api-ui.html
var Files embed.FS

// Package templates holds the files mcinit writes into a new server directory.
// They are compiled into the binary, so a release is a single .exe with no
// companion files to lose.
package templates

import "embed"

// FS contains every template, rendered by internal/server.
//
//go:embed *.tmpl
var FS embed.FS

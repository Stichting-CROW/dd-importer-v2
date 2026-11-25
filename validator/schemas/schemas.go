// Package schemas embeds the official JSON Schemas the feed validator checks
// against. See README.md for where each file comes from.
package schemas

import "embed"

//go:embed gbfs/*/*.json mds/*/*.json
var FS embed.FS

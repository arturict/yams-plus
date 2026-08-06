package stackassets

import "embed"

// Files contains the audited runtime templates shipped in the static binary.
//
//go:embed compose.yaml.tmpl stack.lock.yaml plugins.yaml recyclarr/*.yaml.tmpl
var Files embed.FS

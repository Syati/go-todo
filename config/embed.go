package settings

import "embed"

// Files contains only distributable defaults. Local overrides are never embedded.
//
//go:embed settings/development.yml settings/test.yml settings/production.yml
var Files embed.FS

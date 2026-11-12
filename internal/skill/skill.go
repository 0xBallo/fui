// Package skill embeds the agent skill document (SKILL.md) fui ships for Claude Code and other
// skill-compatible agents: when to use fui, the command reference, the --json output contract,
// typical flows, and the functional spec's safety rules. `fui skill show` prints it and
// `fui skill install` writes a copy into an agent's skills directory.
package skill

import (
	_ "embed"
	"strings"
)

//go:embed SKILL.md
var template string

// versionPlaceholder is replaced by Render with the running fui version, so the skill always
// names the binary it was installed from (the same version string `fui --version` prints).
const versionPlaceholder = "{{FUI_VERSION}}"

// Render returns the skill document with every occurrence of the version placeholder in its
// front matter replaced by version.
func Render(version string) string {
	return strings.ReplaceAll(template, versionPlaceholder, version)
}
